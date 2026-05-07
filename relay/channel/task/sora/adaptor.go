package sora

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	taskcommon "github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/tidwall/sjson"
)

// ============================
// Request / Response structures
// ============================

type ContentItem struct {
	Type     string    `json:"type"`                // "text" or "image_url"
	Text     string    `json:"text,omitempty"`      // for text type
	ImageURL *ImageURL `json:"image_url,omitempty"` // for image_url type
}

type ImageURL struct {
	URL string `json:"url"`
}

type responseTask struct {
	ID                 string         `json:"id"`
	TaskID             string         `json:"task_id,omitempty"` //兼容旧接口
	Object             string         `json:"object"`
	Model              string         `json:"model"`
	Status             string         `json:"status"`
	Progress           int            `json:"progress"`
	CreatedAt          int64          `json:"created_at"`
	CompletedAt        int64          `json:"completed_at,omitempty"`
	ExpiresAt          int64          `json:"expires_at,omitempty"`
	Seconds            string         `json:"seconds,omitempty"`
	Size               string         `json:"size,omitempty"`
	VideoURL           string         `json:"video_url,omitempty"`
	URL                string         `json:"url,omitempty"`
	Metadata           map[string]any `json:"metadata,omitempty"`
	MetaData           map[string]any `json:"meta_data,omitempty"`
	RemixedFromVideoID string         `json:"remixed_from_video_id,omitempty"`
	Error              *struct {
		Message string `json:"message"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}

func (t responseTask) resultURL() string {
	if t.VideoURL != "" {
		return t.VideoURL
	}
	if t.URL != "" {
		return t.URL
	}
	if value, ok := t.Metadata["url"].(string); ok {
		return value
	}
	if value, ok := t.MetaData["url"].(string); ok {
		return value
	}
	return ""
}

// ============================
// Adaptor implementation
// ============================

type TaskAdaptor struct {
	taskcommon.BaseBilling
	ChannelType int
	apiKey      string
	baseURL     string
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	a.ChannelType = info.ChannelType
	a.baseURL = info.ChannelBaseUrl
	a.apiKey = info.ApiKey
}

func validateRemixRequest(c *gin.Context) *dto.TaskError {
	var req relaycommon.TaskSubmitReq
	if err := common.UnmarshalBodyReusable(c, &req); err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}
	if strings.TrimSpace(req.Prompt) == "" {
		return service.TaskErrorWrapperLocal(fmt.Errorf("field prompt is required"), "invalid_request", http.StatusBadRequest)
	}
	// 存储原始请求到 context，与 ValidateMultipartDirect 路径保持一致
	c.Set("task_request", req)
	return nil
}

func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) (taskErr *dto.TaskError) {
	if info.Action == constant.TaskActionRemix {
		return validateRemixRequest(c)
	}
	return relaycommon.ValidateMultipartDirect(c, info)
}

// EstimateBilling 根据用户请求的 seconds 和 size 计算 OtherRatios。
func (a *TaskAdaptor) EstimateBilling(c *gin.Context, info *relaycommon.RelayInfo) map[string]float64 {
	// remix 路径的 OtherRatios 已在 ResolveOriginTask 中设置
	if info.Action == constant.TaskActionRemix {
		return nil
	}

	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil
	}

	seconds, _ := strconv.Atoi(req.Seconds)
	if seconds == 0 {
		seconds = req.Duration
	}
	if seconds <= 0 {
		seconds = 4
	}

	size := req.Size
	if size == "" {
		size = "720x1280"
	}

	ratios := map[string]float64{
		"seconds": float64(seconds),
		"size":    1,
	}
	if size == "1792x1024" || size == "1024x1792" {
		ratios["size"] = 1.666667
	}
	return ratios
}

func (a *TaskAdaptor) BuildRequestURL(info *relaycommon.RelayInfo) (string, error) {
	if info.Action == constant.TaskActionRemix {
		return fmt.Sprintf("%s/v1/videos/%s/remix", a.baseURL, info.OriginTaskID), nil
	}
	return fmt.Sprintf("%s/v1/videos", a.baseURL), nil
}

// BuildRequestHeader sets required headers.
func (a *TaskAdaptor) BuildRequestHeader(c *gin.Context, req *http.Request, info *relaycommon.RelayInfo) error {
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	if isDomesticVideoModel(info.UpstreamModelName) {
		req.Header.Set("Content-Type", "application/json")
		return nil
	}
	req.Header.Set("Content-Type", c.Request.Header.Get("Content-Type"))
	return nil
}

func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return nil, errors.Wrap(err, "get_request_body_failed")
	}
	cachedBody, err := storage.Bytes()
	if err != nil {
		return nil, errors.Wrap(err, "read_body_bytes_failed")
	}
	contentType := c.GetHeader("Content-Type")

	if isDomesticVideoModel(info.UpstreamModelName) {
		return a.buildDomesticVideoRequestBody(c, info, cachedBody, contentType)
	}

	if strings.HasPrefix(contentType, "application/json") {
		var bodyMap map[string]interface{}
		if err := common.Unmarshal(cachedBody, &bodyMap); err == nil {
			bodyMap["model"] = info.UpstreamModelName
			if newBody, err := common.Marshal(bodyMap); err == nil {
				return bytes.NewReader(newBody), nil
			}
		}
		return bytes.NewReader(cachedBody), nil
	}

	if strings.Contains(contentType, "multipart/form-data") {
		formData, err := common.ParseMultipartFormReusable(c)
		if err != nil {
			return bytes.NewReader(cachedBody), nil
		}
		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		writer.WriteField("model", info.UpstreamModelName)
		for key, values := range formData.Value {
			if key == "model" {
				continue
			}
			for _, v := range values {
				writer.WriteField(key, v)
			}
		}
		for fieldName, fileHeaders := range formData.File {
			for _, fh := range fileHeaders {
				f, err := fh.Open()
				if err != nil {
					continue
				}
				ct := fh.Header.Get("Content-Type")
				if ct == "" || ct == "application/octet-stream" {
					buf512 := make([]byte, 512)
					n, _ := io.ReadFull(f, buf512)
					ct = http.DetectContentType(buf512[:n])
					// Re-open after sniffing so the full content is copied below
					f.Close()
					f, err = fh.Open()
					if err != nil {
						continue
					}
				}
				h := make(textproto.MIMEHeader)
				h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, fieldName, fh.Filename))
				h.Set("Content-Type", ct)
				part, err := writer.CreatePart(h)
				if err != nil {
					f.Close()
					continue
				}
				io.Copy(part, f)
				f.Close()
			}
		}
		writer.Close()
		c.Request.Header.Set("Content-Type", writer.FormDataContentType())
		return &buf, nil
	}

	return common.ReaderOnly(storage), nil
}

func isDomesticVideoModel(modelName string) bool {
	normalized := strings.ToLower(strings.TrimSpace(modelName))
	return strings.HasPrefix(normalized, "kling") || strings.HasPrefix(normalized, "king")
}

func firstFormValue(values map[string][]string, key string) string {
	if valueList := values[key]; len(valueList) > 0 {
		return strings.TrimSpace(valueList[0])
	}
	return ""
}

func imageExtFromFile(filename, contentType string) string {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(filename)), ".")
	switch ext {
	case "jpg", "jpeg", "png", "webp":
		return ext
	}
	switch strings.ToLower(strings.TrimSpace(contentType)) {
	case "image/jpeg", "image/jpg":
		return "jpg"
	case "image/webp":
		return "webp"
	default:
		return "png"
	}
}

func uploadAIVideoReferenceFiles(c *gin.Context, formData *multipart.Form, userID int) ([]map[string]string, error) {
	if formData == nil {
		return nil, nil
	}
	fileHeaders := formData.File["input_reference"]
	if len(fileHeaders) == 0 {
		return nil, nil
	}
	if !service.IsObjectStorageEnabled() {
		return nil, fmt.Errorf("AI video reference images require object storage to be enabled")
	}
	fileInfos := make([]map[string]string, 0, len(fileHeaders))
	for _, fileHeader := range fileHeaders {
		file, err := fileHeader.Open()
		if err != nil {
			return nil, fmt.Errorf("open reference image failed: %w", err)
		}
		fileBytes, err := io.ReadAll(file)
		_ = file.Close()
		if err != nil {
			return nil, fmt.Errorf("read reference image failed: %w", err)
		}
		contentType := fileHeader.Header.Get("Content-Type")
		if contentType == "" || contentType == "application/octet-stream" {
			contentType = http.DetectContentType(fileBytes)
		}
		if !strings.HasPrefix(strings.ToLower(contentType), "image/") {
			return nil, fmt.Errorf("reference file %s is not an image", fileHeader.Filename)
		}
		ext := imageExtFromFile(fileHeader.Filename, contentType)
		objectKey := service.BuildAIImageRefObjectKey(userID, ext)
		_, accessURL, err := service.UploadBytesToObjectStorage(c.Request.Context(), objectKey, contentType, fileBytes)
		if err != nil {
			return nil, err
		}
		fileInfos = append(fileInfos, map[string]string{
			"type":     "Url",
			"category": "Image",
			"url":      accessURL,
		})
	}
	return fileInfos, nil
}

func (a *TaskAdaptor) buildDomesticVideoRequestBody(c *gin.Context, info *relaycommon.RelayInfo, cachedBody []byte, contentType string) (io.Reader, error) {
	req, _ := relaycommon.GetTaskRequest(c)
	if strings.HasPrefix(contentType, "application/json") {
		_ = common.Unmarshal(cachedBody, &req)
	}

	var fileInfos []map[string]string
	if strings.Contains(contentType, "multipart/form-data") {
		formData, err := common.ParseMultipartFormReusable(c)
		if err != nil {
			return nil, err
		}
		if req.Prompt == "" {
			req.Prompt = firstFormValue(formData.Value, "prompt")
		}
		if req.Seconds == "" {
			req.Seconds = firstFormValue(formData.Value, "seconds")
		}
		fileInfos, err = uploadAIVideoReferenceFiles(c, formData, info.UserId)
		if err != nil {
			return nil, err
		}
	}

	for _, imageURL := range req.Images {
		imageURL = strings.TrimSpace(imageURL)
		if imageURL == "" {
			continue
		}
		fileInfos = append(fileInfos, map[string]string{
			"type":     "Url",
			"category": "Image",
			"url":      imageURL,
		})
	}
	if strings.TrimSpace(req.InputReference) != "" {
		fileInfos = append(fileInfos, map[string]string{
			"type":     "Url",
			"category": "Image",
			"url":      strings.TrimSpace(req.InputReference),
		})
	}

	outputConfig := map[string]any{
		"resolution":       "1080P",
		"audio_generation": "Enabled",
	}
	metadata := map[string]any{
		"offpeak":       true,
		"output_config": outputConfig,
	}
	if len(fileInfos) > 0 {
		metadata["file_infos"] = fileInfos
	}

	seconds := strings.TrimSpace(req.Seconds)
	if seconds == "" && req.Duration > 0 {
		seconds = strconv.Itoa(req.Duration)
	}
	if seconds == "" {
		seconds = "5"
	}

	bodyMap := map[string]any{
		"model":         info.UpstreamModelName,
		"prompt":        req.Prompt,
		"seconds":       seconds,
		"metadata":      metadata,
		"output_config": outputConfig,
	}
	body, err := common.Marshal(bodyMap)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(body), nil
}

// DoRequest delegates to common helper.
func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, requestBody)
}

// DoResponse handles upstream response, returns taskID etc.
func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (taskID string, taskData []byte, taskErr *dto.TaskError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		taskErr = service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
		return
	}
	_ = resp.Body.Close()

	// Parse Sora response
	var dResp responseTask
	if err := common.Unmarshal(responseBody, &dResp); err != nil {
		taskErr = service.TaskErrorWrapper(errors.Wrapf(err, "body: %s", responseBody), "unmarshal_response_body_failed", http.StatusInternalServerError)
		return
	}

	upstreamID := dResp.ID
	if upstreamID == "" {
		upstreamID = dResp.TaskID
	}
	if upstreamID == "" {
		taskErr = service.TaskErrorWrapper(fmt.Errorf("task_id is empty"), "invalid_response", http.StatusInternalServerError)
		return
	}

	// 使用公开 task_xxxx ID 返回给客户端
	dResp.ID = info.PublicTaskID
	dResp.TaskID = info.PublicTaskID
	c.JSON(http.StatusOK, dResp)
	return upstreamID, responseBody, nil
}

// FetchTask fetch task status
func (a *TaskAdaptor) FetchTask(baseUrl, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid task_id")
	}

	uri := fmt.Sprintf("%s/v1/videos/%s", baseUrl, taskID)

	req, err := http.NewRequest(http.MethodGet, uri, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+key)

	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	return client.Do(req)
}

func (a *TaskAdaptor) GetModelList() []string {
	return ModelList
}

func (a *TaskAdaptor) GetChannelName() string {
	return ChannelName
}

func (a *TaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	resTask := responseTask{}
	if err := common.Unmarshal(respBody, &resTask); err != nil {
		return nil, errors.Wrap(err, "unmarshal task result failed")
	}

	taskResult := relaycommon.TaskInfo{
		Code: 0,
	}

	switch resTask.Status {
	case "queued", "pending":
		taskResult.Status = model.TaskStatusQueued
	case "processing", "in_progress":
		taskResult.Status = model.TaskStatusInProgress
	case "completed":
		taskResult.Status = model.TaskStatusSuccess
		taskResult.Url = resTask.resultURL()
	case "failed", "cancelled":
		taskResult.Status = model.TaskStatusFailure
		if resTask.Error != nil {
			taskResult.Reason = resTask.Error.Message
		} else {
			taskResult.Reason = "task failed"
		}
	default:
	}
	if resTask.Progress > 0 && resTask.Progress < 100 {
		taskResult.Progress = fmt.Sprintf("%d%%", resTask.Progress)
	}

	return &taskResult, nil
}

func (a *TaskAdaptor) ConvertToOpenAIVideo(task *model.Task) ([]byte, error) {
	var resTask responseTask
	if err := common.Unmarshal(task.Data, &resTask); err != nil {
		return nil, errors.Wrap(err, "unmarshal sora task data failed")
	}

	data := task.Data
	var err error
	if data, err = sjson.SetBytes(data, "id", task.TaskID); err != nil {
		return nil, errors.Wrap(err, "set id failed")
	}
	if data, err = sjson.SetBytes(data, "task_id", task.TaskID); err != nil {
		return nil, errors.Wrap(err, "set task_id failed")
	}
	if data, err = sjson.SetBytes(data, "status", task.Status.ToVideoStatus()); err != nil {
		return nil, errors.Wrap(err, "set status failed")
	}
	if task.Progress != "" {
		progress := strings.TrimSuffix(task.Progress, "%")
		if progressValue, convErr := strconv.Atoi(progress); convErr == nil {
			if data, err = sjson.SetBytes(data, "progress", progressValue); err != nil {
				return nil, errors.Wrap(err, "set progress failed")
			}
		}
	}
	resultURL := resTask.resultURL()
	if resultURL == "" {
		resultURL = task.GetResultURL()
	}
	if resultURL != "" {
		if data, err = sjson.SetBytes(data, "video_url", resultURL); err != nil {
			return nil, errors.Wrap(err, "set video_url failed")
		}
		if data, err = sjson.SetBytes(data, "metadata.url", resultURL); err != nil {
			return nil, errors.Wrap(err, "set metadata url failed")
		}
	}
	return data, nil
}
