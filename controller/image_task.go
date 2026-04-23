package controller

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/gin-gonic/gin"
	"github.com/minio/minio-go/v7"
	"github.com/samber/lo"
)

var imageTaskProcessingCount int64

func StartImageTaskWorker() {
	if !common.IsMasterNode {
		return
	}
	gopool.Go(func() {
		for {
			setting := operation_setting.GetAIImageAsyncSetting()
			interval := setting.PollIntervalSec
			if interval <= 0 {
				interval = 3
			}
			if setting.Enabled {
				processImageTaskBatch(setting.WorkerConcurrency)
			}
			time.Sleep(time.Duration(interval) * time.Second)
		}
	})
	gopool.Go(func() {
		for {
			time.Sleep(1 * time.Hour)
			cutoff := time.Now().Add(-7 * 24 * time.Hour).Unix()
			cleaned, _ := model.CleanStaleFailedImageTasks(cutoff)
			if cleaned > 0 {
				common.SysLog(fmt.Sprintf("cleaned %d stale failed image tasks older than 7 days", cleaned))
			}
			cleanExpiredTasksAndS3Objects(cutoff)
		}
	})
}

func CreateImageTask(c *gin.Context) {
	req := &dto.CreateImageTaskRequest{}
	if err := common.UnmarshalBodyReusable(c, req); err != nil {
		common.ApiError(c, err)
		return
	}
	if strings.TrimSpace(req.Model) == "" || strings.TrimSpace(req.Prompt) == "" {
		common.ApiErrorMsg(c, "model and prompt are required")
		return
	}
	userID := c.GetInt("id")
	userCache, err := model.GetUserCache(userID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	usingGroup := strings.TrimSpace(req.Group)
	if usingGroup == "" {
		usingGroup = userCache.Group
	}
	if usingGroup != userCache.Group && !service.GroupInUserUsableGroups(userCache.Group, usingGroup) {
		common.ApiErrorMsg(c, "group access denied")
		return
	}
	pendingCount, err := model.CountPendingImageTasks()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if limit := operation_setting.GetAIImageAsyncSetting().QueueLimit; limit > 0 && pendingCount >= int64(limit) {
		common.ApiErrorMsg(c, "ai image queue is full, please retry later")
		return
	}
	channel, _, err := buildImageTaskChannelContext(c, userCache, usingGroup, req.Model)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	task := &model.ImageTask{
		UserID:    userID,
		Group:     usingGroup,
		Model:     req.Model,
		Size:      req.Size,
		Prompt:    req.Prompt,
		Status:    model.ImageTaskStatusPending,
		ChannelID: channel.Id,
	}
	if strings.TrimSpace(req.ReferenceImage) != "" {
		refKey, err := uploadReferenceImage(context.Background(), userID, req.ReferenceImage)
		if err != nil {
			common.ApiError(c, fmt.Errorf("upload reference image failed: %w", err))
			return
		}
		task.ReferenceImageKey = refKey
	}
	if err := model.CreateImageTask(task); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, toImageTaskDTO(task, false))
}

func GetUserImageTask(c *gin.Context) {
	task, err := model.GetImageTaskByTaskIDAndUserID(c.Param("task_id"), c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, toImageTaskDTO(task, false))
}

func GetUserImageTasks(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	items, total, err := model.ListUserImageTasks(c.GetInt("id"), pageInfo.GetStartIdx(), pageInfo.GetPageSize(), c.Query("status"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(toImageTaskDTOs(items, false))
	common.ApiSuccess(c, pageInfo)
}

func ProxyImageDownload(c *gin.Context) {
	taskID := c.Param("task_id")
	userID := c.GetInt("id")
	task, err := model.GetImageTaskByTaskIDAndUserID(taskID, userID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if task.Status != model.ImageTaskStatusSucceeded {
		common.ApiErrorMsg(c, "task not succeeded")
		return
	}
	imageURL := task.ResultURL
	if task.ResultKey != "" && !strings.Contains(task.ResultKey, "://") && service.IsObjectStorageEnabled() {
		signedURL, err := service.GenerateObjectStorageAccessURL(context.Background(), task.ResultKey)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		imageURL = signedURL
	}
	if imageURL == "" {
		common.ApiErrorMsg(c, "no image url available")
		return
	}
	req, err := http.NewRequestWithContext(c.Request.Context(), "GET", imageURL, nil)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	defer resp.Body.Close()
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/png"
	}
	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="ai-image-%s.png"`, taskID))
	c.Header("Cache-Control", "no-store")
	c.DataFromReader(resp.StatusCode, resp.ContentLength, contentType, resp.Body, nil)
}

func ProxyImageData(c *gin.Context) {
	taskID := c.Param("task_id")
	userID := c.GetInt("id")
	task, err := model.GetImageTaskByTaskIDAndUserID(taskID, userID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if task.Status != model.ImageTaskStatusSucceeded {
		common.ApiErrorMsg(c, "task not succeeded")
		return
	}
	imageURL := task.ResultURL
	if task.ResultKey != "" && !strings.Contains(task.ResultKey, "://") && service.IsObjectStorageEnabled() {
		signedURL, err := service.GenerateObjectStorageAccessURL(context.Background(), task.ResultKey)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		imageURL = signedURL
	}
	if imageURL == "" {
		common.ApiErrorMsg(c, "no image url available")
		return
	}
	req, err := http.NewRequestWithContext(c.Request.Context(), "GET", imageURL, nil)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	defer resp.Body.Close()
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/png"
	}
	c.Header("Content-Type", contentType)
	c.Header("Cache-Control", "private, max-age=3600")
	c.DataFromReader(resp.StatusCode, resp.ContentLength, contentType, resp.Body, nil)
}

func DeleteUserImageTask(c *gin.Context) {
	taskID := c.Param("task_id")
	userID := c.GetInt("id")
	task, err := model.DeleteImageTaskByTaskIDAndUserID(taskID, userID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if task.ResultKey != "" && !strings.Contains(task.ResultKey, "://") && service.IsObjectStorageEnabled() {
		_ = service.DeleteObjectFromStorage(context.Background(), task.ResultKey)
	}
	if task.ReferenceImageKey != "" && !strings.Contains(task.ReferenceImageKey, "://") && service.IsObjectStorageEnabled() {
		_ = service.DeleteObjectFromStorage(context.Background(), task.ReferenceImageKey)
	}
	common.ApiSuccess(c, nil)
}

func GetAllImageTasks(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	userID := common.String2Int(c.Query("user_id"))
	items, total, err := model.ListAllImageTasks(pageInfo.GetStartIdx(), pageInfo.GetPageSize(), userID, c.Query("model"), c.Query("status"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(toImageTaskDTOs(items, true))
	common.ApiSuccess(c, pageInfo)
}

func GetImageTaskStats(c *gin.Context) {
	pending, _ := model.CountImageTasksByStatus(model.ImageTaskStatusPending)
	processing, _ := model.CountImageTasksByStatus(model.ImageTaskStatusProcessing)
	succeeded, _ := model.CountImageTasksByStatus(model.ImageTaskStatusSucceeded)
	failed, _ := model.CountImageTasksByStatus(model.ImageTaskStatusFailed)
	setting := operation_setting.GetAIImageAsyncSetting()
	common.ApiSuccess(c, dto.ImageTaskStatsResponse{
		Pending:           pending,
		Processing:        processing,
		Succeeded:         succeeded,
		Failed:            failed,
		WorkerConcurrency: setting.WorkerConcurrency,
		QueueLimit:        setting.QueueLimit,
	})
}

func TestS3Connection(c *gin.Context) {
	setting := operation_setting.GetAIImageAsyncSetting()
	if !setting.S3Enabled || setting.S3Endpoint == "" || setting.S3Bucket == "" || setting.S3AccessKey == "" || setting.S3SecretKey == "" {
		common.ApiErrorMsg(c, "S3 configuration is incomplete, please save config first")
		return
	}
	client, err := service.NewObjectStorageClient()
	if err != nil {
		common.ApiError(c, fmt.Errorf("create S3 client failed: %w", err))
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	testKey := "_healthcheck_" + time.Now().Format("20060102150405") + ".txt"
	testData := []byte("new-api s3 connectivity test")
	_, err = client.PutObject(ctx, setting.S3Bucket, testKey, bytes.NewReader(testData), int64(len(testData)), minio.PutObjectOptions{ContentType: "text/plain"})
	if err != nil {
		common.ApiError(c, fmt.Errorf("write test object failed: %w", err))
		return
	}
	_ = client.RemoveObject(ctx, setting.S3Bucket, testKey, minio.RemoveObjectOptions{})
	common.ApiSuccess(c, map[string]string{"status": "ok", "message": "S3 connection successful"})
}

func processImageTaskBatch(concurrency int) {
	if concurrency <= 0 {
		concurrency = 1
	}
	available := concurrency - int(atomic.LoadInt64(&imageTaskProcessingCount))
	if available <= 0 {
		return
	}
	tasks, err := model.GetPendingImageTasks(available)
	if err != nil {
		common.SysLog("load pending image tasks failed: " + err.Error())
		return
	}
	for _, task := range tasks {
		updates := map[string]any{"started_at": time.Now().Unix()}
		ok, err := model.UpdateImageTaskStatus(task.TaskID, model.ImageTaskStatusPending, model.ImageTaskStatusProcessing, updates)
		if err != nil || !ok {
			continue
		}
		atomic.AddInt64(&imageTaskProcessingCount, 1)
		localTask := task
		gopool.Go(func() {
			defer atomic.AddInt64(&imageTaskProcessingCount, -1)
			processOneImageTask(localTask)
		})
	}
}

func processOneImageTask(task *model.ImageTask) {
	var resultURL, resultKey string
	var err error
	if task.ReferenceImageKey != "" {
		resultURL, resultKey, err = executeImageEditTask(task)
	} else {
		resultURL, resultKey, err = executeImageGenerationTask(task)
	}
	finishedAt := time.Now().Unix()
	if err != nil {
		_ = model.UpdateImageTaskFields(task.TaskID, map[string]any{
			"status":        model.ImageTaskStatusFailed,
			"error_message": err.Error(),
			"finished_at":   finishedAt,
		})
		cleanupReferenceImage(task)
		return
	}
	_ = model.UpdateImageTaskFields(task.TaskID, map[string]any{
		"status":        model.ImageTaskStatusSucceeded,
		"result_url":    resultURL,
		"result_key":    resultKey,
		"error_message": "",
		"finished_at":   finishedAt,
	})
	cleanupReferenceImage(task)
}

func cleanupReferenceImage(task *model.ImageTask) {
	if task.ReferenceImageKey != "" && service.IsObjectStorageEnabled() {
		_ = service.DeleteObjectFromStorage(context.Background(), task.ReferenceImageKey)
	}
}

func executeImageGenerationTask(task *model.ImageTask) (string, string, error) {
	channel, err := model.GetChannelById(task.ChannelID, true)
	if err != nil {
		return "", "", err
	}
	userCache, err := model.GetUserCache(task.UserID)
	if err != nil {
		return "", "", err
	}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	reqSize := task.Size
	if reqSize == "" {
		reqSize = "1024x1024"
	}
	reqBody := dto.ImageRequest{Model: task.Model, Prompt: task.Prompt, N: lo.ToPtr(uint(1)), Size: reqSize}
	requestJSON, err := common.Marshal(reqBody)
	if err != nil {
		return "", "", err
	}
	req := httptest.NewRequest("POST", "/v1/images/generations", bytes.NewReader(requestJSON))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	c.Set("id", task.UserID)
	common.SetContextKey(c, constant.ContextKeyUserId, task.UserID)
	common.SetContextKey(c, constant.ContextKeyUsingGroup, task.Group)
	userCache.WriteContext(c)
	if err := setupImageTaskChannelContext(c, channel, task.Model); err != nil {
		return "", "", err
	}
	info, err := relaycommon.GenRelayInfo(c, types.RelayFormatOpenAIImage, &reqBody, nil)
	if err != nil {
		return "", "", err
	}
	info.InitChannelMeta(c)
	if err := helper.ModelMappedHelper(c, info, &reqBody); err != nil {
		return "", "", err
	}
	reqBody.SetModelName(info.UpstreamModelName)
	if _, err := helper.ModelPriceHelper(c, info, 0, reqBody.GetTokenCountMeta()); err != nil {
		return "", "", err
	}
	apiType, _ := common.ChannelType2APIType(channel.Type)
	adaptor := relay.GetAdaptor(apiType)
	if adaptor == nil {
		return "", "", fmt.Errorf("invalid api type: %d", apiType)
	}
	adaptor.Init(info)
	convertedRequest, err := adaptor.ConvertImageRequest(c, info, reqBody)
	if err != nil {
		return "", "", err
	}
	var requestBody io.Reader
	switch v := convertedRequest.(type) {
	case *bytes.Buffer:
		requestBody = v
	default:
		jsonData, err := common.Marshal(convertedRequest)
		if err != nil {
			return "", "", err
		}
		requestBody = bytes.NewBuffer(jsonData)
	}
	respAny, err := adaptor.DoRequest(c, info, requestBody)
	if err != nil {
		return "", "", err
	}
	resp := respAny.(*http.Response)
	defer service.CloseResponseBodyGracefully(resp)
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", "", fmt.Errorf("upstream returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}
	var imageResp dto.ImageResponse
	if err := common.Unmarshal(body, &imageResp); err != nil {
		return "", "", err
	}
	if len(imageResp.Data) == 0 {
		return "", "", fmt.Errorf("no image returned from upstream")
	}
	first := imageResp.Data[0]
	if service.IsObjectStorageEnabled() {
		return uploadImageResult(context.Background(), task.UserID, first)
	}
	if first.Url != "" {
		return first.Url, "", nil
	}
	return uploadImageResult(context.Background(), task.UserID, first)
}

func uploadImageResult(ctx context.Context, userID int, imageData dto.ImageData) (string, string, error) {
	if strings.TrimSpace(imageData.B64Json) != "" {
		mimeType, base64Data, err := service.DecodeBase64FileData(imageData.B64Json)
		if err != nil {
			return "", "", err
		}
		decoded, err := base64.StdEncoding.DecodeString(base64Data)
		if err != nil {
			return "", "", err
		}
		ext := strings.TrimPrefix(strings.TrimSpace(strings.TrimPrefix(mimeType, "image/")), ".")
		key := service.BuildAIImageObjectKey(userID, ext)
		objectKey, _, err := service.UploadBytesToObjectStorage(ctx, key, mimeType, decoded)
		if err != nil {
			return "", "", err
		}
		return "", objectKey, nil
	}
	if strings.TrimSpace(imageData.Url) == "" {
		return "", "", fmt.Errorf("empty image url")
	}
	mimeType, base64Data, err := service.GetImageFromUrl(imageData.Url)
	if err != nil {
		return "", "", err
	}
	decoded, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return "", "", err
	}
	ext := strings.TrimPrefix(strings.TrimSpace(strings.TrimPrefix(mimeType, "image/")), ".")
	key := service.BuildAIImageObjectKey(userID, ext)
	objectKey, _, err := service.UploadBytesToObjectStorage(ctx, key, mimeType, decoded)
	if err != nil {
		return "", "", err
	}
	return "", objectKey, nil
}

func uploadReferenceImage(ctx context.Context, userID int, refImageBase64 string) (string, error) {
	mimeType, base64Data, err := service.DecodeBase64FileData(refImageBase64)
	if err != nil {
		mimeType = "image/png"
		base64Data = refImageBase64
	}
	decoded, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return "", fmt.Errorf("invalid base64 reference image: %w", err)
	}
	ext := strings.TrimPrefix(strings.TrimSpace(strings.TrimPrefix(mimeType, "image/")), ".")
	key := service.BuildAIImageRefObjectKey(userID, ext)
	objectKey, _, err := service.UploadBytesToObjectStorage(ctx, key, mimeType, decoded)
	if err != nil {
		return "", err
	}
	return objectKey, nil
}

func executeImageEditTask(task *model.ImageTask) (string, string, error) {
	channel, err := model.GetChannelById(task.ChannelID, true)
	if err != nil {
		return "", "", err
	}
	userCache, err := model.GetUserCache(task.UserID)
	if err != nil {
		return "", "", err
	}

	ctx := context.Background()
	refData, _, err := service.DownloadObjectFromStorage(ctx, task.ReferenceImageKey)
	if err != nil {
		return "", "", fmt.Errorf("download reference image from S3 failed: %w", err)
	}

	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)
	writer.WriteField("model", task.Model)
	writer.WriteField("prompt", task.Prompt)
	writer.WriteField("n", "1")
	if task.Size != "" {
		writer.WriteField("size", task.Size)
	}

	ext := "png"
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="image"; filename="image.%s"`, ext))
	h.Set("Content-Type", "image/png")
	part, err := writer.CreatePart(h)
	if err != nil {
		return "", "", fmt.Errorf("create multipart part failed: %w", err)
	}
	if _, err := part.Write(refData); err != nil {
		return "", "", fmt.Errorf("write reference image to multipart failed: %w", err)
	}
	writer.Close()

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest("POST", "/v1/images/edits", &requestBody)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	c.Request = req

	c.Set("id", task.UserID)
	common.SetContextKey(c, constant.ContextKeyUserId, task.UserID)
	common.SetContextKey(c, constant.ContextKeyUsingGroup, task.Group)
	userCache.WriteContext(c)
	if err := setupImageTaskChannelContext(c, channel, task.Model); err != nil {
		return "", "", err
	}

	reqSize := task.Size
	if reqSize == "" {
		reqSize = "1024x1024"
	}
	reqBody := dto.ImageRequest{Model: task.Model, Prompt: task.Prompt, N: lo.ToPtr(uint(1)), Size: reqSize}
	info, err := relaycommon.GenRelayInfo(c, types.RelayFormatOpenAIImage, &reqBody, nil)
	if err != nil {
		return "", "", err
	}
	info.RelayMode = relayconstant.RelayModeImagesEdits
	info.InitChannelMeta(c)
	if err := helper.ModelMappedHelper(c, info, &reqBody); err != nil {
		return "", "", err
	}
	reqBody.SetModelName(info.UpstreamModelName)
	if _, err := helper.ModelPriceHelper(c, info, 0, reqBody.GetTokenCountMeta()); err != nil {
		return "", "", err
	}

	apiType, _ := common.ChannelType2APIType(channel.Type)
	adaptor := relay.GetAdaptor(apiType)
	if adaptor == nil {
		return "", "", fmt.Errorf("invalid api type: %d", apiType)
	}
	adaptor.Init(info)

	convertedRequest, err := adaptor.ConvertImageRequest(c, info, reqBody)
	if err != nil {
		return "", "", err
	}

	var convertedBody io.Reader
	switch v := convertedRequest.(type) {
	case *bytes.Buffer:
		convertedBody = v
	default:
		jsonData, err := common.Marshal(convertedRequest)
		if err != nil {
			return "", "", err
		}
		convertedBody = bytes.NewBuffer(jsonData)
	}

	respAny, err := adaptor.DoRequest(c, info, convertedBody)
	if err != nil {
		return "", "", err
	}
	resp := respAny.(*http.Response)
	defer service.CloseResponseBodyGracefully(resp)
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", "", fmt.Errorf("upstream returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}
	var imageResp dto.ImageResponse
	if err := common.Unmarshal(body, &imageResp); err != nil {
		return "", "", err
	}
	if len(imageResp.Data) == 0 {
		return "", "", fmt.Errorf("no image returned from upstream")
	}
	first := imageResp.Data[0]
	if service.IsObjectStorageEnabled() {
		return uploadImageResult(ctx, task.UserID, first)
	}
	if first.Url != "" {
		return first.Url, "", nil
	}
	return uploadImageResult(ctx, task.UserID, first)
}

func buildImageTaskChannelContext(c *gin.Context, userCache *model.UserBase, usingGroup, modelName string) (*model.Channel, string, error) {
	common.SetContextKey(c, constant.ContextKeyUsingGroup, usingGroup)
	service.EnsureRequestedGroup(c, usingGroup)
	channel, selectGroup, err := service.CacheGetRandomSatisfiedChannel(&service.RetryParam{
		Ctx:        c,
		ModelName:  modelName,
		TokenGroup: usingGroup,
		Retry:      lo.ToPtr(0),
	})
	if err != nil {
		return nil, "", err
	}
	if channel == nil {
		return nil, "", fmt.Errorf("no channel available for %s", modelName)
	}
	if selectGroup != "" {
		common.SetContextKey(c, constant.ContextKeyUsingGroup, selectGroup)
	}
	userCache.WriteContext(c)
	return channel, selectGroup, nil
}

func setupImageTaskChannelContext(c *gin.Context, channel *model.Channel, modelName string) error {
	if channel == nil {
		return fmt.Errorf("channel is nil")
	}
	common.SetContextKey(c, constant.ContextKeyOriginalModel, modelName)
	common.SetContextKey(c, constant.ContextKeyChannelId, channel.Id)
	common.SetContextKey(c, constant.ContextKeyChannelName, channel.Name)
	common.SetContextKey(c, constant.ContextKeyChannelType, channel.Type)
	common.SetContextKey(c, constant.ContextKeyChannelCreateTime, channel.CreatedTime)
	common.SetContextKey(c, constant.ContextKeyChannelSetting, channel.GetSetting())
	common.SetContextKey(c, constant.ContextKeyChannelOtherSetting, channel.GetOtherSettings())
	common.SetContextKey(c, constant.ContextKeyChannelParamOverride, channel.GetParamOverride())
	common.SetContextKey(c, constant.ContextKeyChannelHeaderOverride, channel.GetHeaderOverride())
	common.SetContextKey(c, constant.ContextKeyChannelAutoBan, channel.GetAutoBan())
	common.SetContextKey(c, constant.ContextKeyChannelModelMapping, channel.GetModelMapping())
	common.SetContextKey(c, constant.ContextKeyChannelStatusCodeMapping, channel.GetStatusCodeMapping())
	key, index, newAPIError := channel.GetNextEnabledKey()
	if newAPIError != nil {
		return newAPIError
	}
	if channel.ChannelInfo.IsMultiKey {
		common.SetContextKey(c, constant.ContextKeyChannelIsMultiKey, true)
		common.SetContextKey(c, constant.ContextKeyChannelMultiKeyIndex, index)
	} else {
		common.SetContextKey(c, constant.ContextKeyChannelIsMultiKey, false)
	}
	common.SetContextKey(c, constant.ContextKeyChannelKey, key)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, channel.GetBaseURL())
	apiType, _ := common.ChannelType2APIType(channel.Type)
	c.Set("channel", channel.Type)
	c.Set("base_url", channel.GetBaseURL())
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ApiType: apiType}}
	_ = info
	return nil
}

func cleanExpiredTasksAndS3Objects(cutoff int64) {
	if !service.IsObjectStorageEnabled() {
		return
	}
	tasks, err := model.GetStaleImageTasks(cutoff, 200)
	if err != nil || len(tasks) == 0 {
		return
	}
	ctx := context.Background()
	var ids []int64
	for _, task := range tasks {
		if task.ResultKey != "" && !strings.Contains(task.ResultKey, "://") {
			_ = service.DeleteObjectFromStorage(ctx, task.ResultKey)
		}
		if task.ReferenceImageKey != "" && !strings.Contains(task.ReferenceImageKey, "://") {
			_ = service.DeleteObjectFromStorage(ctx, task.ReferenceImageKey)
		}
		ids = append(ids, task.ID)
	}
	if len(ids) > 0 {
		_ = model.BatchDeleteImageTasksByIDs(ids)
		common.SysLog(fmt.Sprintf("cleaned %d expired image tasks (succeeded/failed) and their S3 objects older than 7 days", len(ids)))
	}
}

func toImageTaskDTO(task *model.ImageTask, fillUser bool) *dto.ImageTaskDTO {
	if task == nil {
		return nil
	}
	result := &dto.ImageTaskDTO{
		ID:           task.ID,
		TaskID:       task.TaskID,
		UserID:       task.UserID,
		Group:        task.Group,
		Model:        task.Model,
		Size:         task.Size,
		Prompt:       task.Prompt,
		Status:       string(task.Status),
		ChannelID:    task.ChannelID,
		ResultURL:    task.ResultURL,
		ResultKey:    task.ResultKey,
		ErrorMessage: task.ErrorMessage,
		StartedAt:    task.StartedAt,
		FinishedAt:   task.FinishedAt,
		CreatedAt:    task.CreatedAt,
		UpdatedAt:    task.UpdatedAt,
	}
	if fillUser {
		if user, err := model.GetUserCache(task.UserID); err == nil {
			result.Username = user.Username
		}
	}
	if task.ResultKey != "" && !strings.Contains(task.ResultKey, "://") && service.IsObjectStorageEnabled() {
		if signedURL, err := service.GenerateObjectStorageAccessURL(context.Background(), task.ResultKey); err == nil {
			result.ResultURL = signedURL
		}
	}
	return result
}

func toImageTaskDTOs(tasks []*model.ImageTask, fillUser bool) []*dto.ImageTaskDTO {
	items := make([]*dto.ImageTaskDTO, 0, len(tasks))
	for _, task := range tasks {
		items = append(items, toImageTaskDTO(task, fillUser))
	}
	return items
}
