package controller

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"

	_ "image/jpeg"
)

const (
	ecommerceWorkflowBatchSize = 8
	ecommerceImageSource       = "ai_ecommerce_template"
)

type ecommerceSegmentSpec struct {
	Key         string
	Label       string
	Description string
	Stage       string
}

var ecommerceSegmentSpecs = []ecommerceSegmentSpec{
	{Key: "top", Label: "首屏主视觉", Description: "商品主图、核心卖点、品牌氛围和第一屏购买动机", Stage: "segment_top"},
	{Key: "middle", Label: "中段卖点拆解", Description: "功能模块、细节特写、参数表和场景说明", Stage: "segment_middle"},
	{Key: "bottom", Label: "尾段信任收口", Description: "包装、保障、适用人群、礼赠或 CTA 收尾", Stage: "segment_bottom"},
}

var ecommerceWorkflowLocks sync.Map

func getEcommerceWorkflowLock(workflowID string) *sync.Mutex {
	lock, _ := ecommerceWorkflowLocks.LoadOrStore(workflowID, &sync.Mutex{})
	return lock.(*sync.Mutex)
}

func processEcommerceWorkflowBatch() {
	workflows, err := model.GetRunnableEcommerceWorkflows(ecommerceWorkflowBatchSize)
	if err != nil {
		common.SysLog("load ecommerce workflows failed: " + err.Error())
		return
	}
	for _, workflow := range workflows {
		if err := processEcommerceWorkflow(context.Background(), workflow); err != nil {
			common.SysLog(fmt.Sprintf("process ecommerce workflow %s failed: %s", workflow.WorkflowID, err.Error()))
		}
	}
}

func CreateEcommerceWorkflow(c *gin.Context) {
	if !service.IsObjectStorageEnabled() {
		common.ApiErrorMsg(c, "object storage must be enabled for ecommerce template workflows")
		return
	}
	req := &dto.CreateEcommerceWorkflowRequest{}
	if err := common.UnmarshalBodyReusable(c, req); err != nil {
		common.ApiError(c, err)
		return
	}
	if strings.TrimSpace(req.Model) == "" {
		common.ApiErrorMsg(c, "model is required")
		return
	}
	if len(req.ReferenceImages) == 0 {
		common.ApiErrorMsg(c, "reference_images is required")
		return
	}
	if len(req.ReferenceImages) > maxReferenceImagesPerTask {
		common.ApiErrorMsg(c, fmt.Sprintf("at most %d reference images are supported", maxReferenceImagesPerTask))
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
	channel, _, err := buildImageTaskChannelContext(c, userCache, usingGroup, req.Model)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	refKeys, err := uploadReferenceImages(context.Background(), userID, req.ReferenceImages)
	if err != nil {
		common.ApiError(c, fmt.Errorf("upload reference image failed: %w", err))
		return
	}
	modulesData, _ := common.Marshal(req.Template.Modules)
	workflow := &model.EcommerceWorkflow{
		UserID:            userID,
		TemplateKey:       strings.TrimSpace(req.Template.Key),
		TemplateName:      strings.TrimSpace(req.Template.Name),
		ProductName:       strings.TrimSpace(req.ProductName),
		ProductType:       strings.TrimSpace(req.ProductType),
		Platform:          strings.TrimSpace(req.Platform),
		SellingPoints:     strings.TrimSpace(req.SellingPoints),
		ExtraRequirements: strings.TrimSpace(req.ExtraRequirements),
		StylePrompt:       strings.TrimSpace(req.Template.StylePrompt),
		Modules:           string(modulesData),
		Model:             req.Model,
		Group:             usingGroup,
		Size:              req.Size,
		Status:            model.EcommerceWorkflowStatusMotherPending,
	}
	if workflow.TemplateName == "" {
		workflow.TemplateName = "AI 电商绘图模板"
	}
	if workflow.TemplateKey == "" {
		workflow.TemplateKey = "custom"
	}
	if workflow.Size == "" {
		workflow.Size = "1024x1024"
	}
	if err := workflow.SetReferenceImageKeys(refKeys); err != nil {
		cleanupReferenceImages(refKeys)
		common.ApiError(c, err)
		return
	}
	if err := model.CreateEcommerceWorkflow(workflow); err != nil {
		cleanupReferenceImages(refKeys)
		common.ApiError(c, err)
		return
	}

	motherPrompt := buildEcommerceMotherPrompt(workflow)
	motherTask := &model.ImageTask{
		UserID:        userID,
		Group:         usingGroup,
		Model:         workflow.Model,
		Size:          workflow.Size,
		Prompt:        motherPrompt,
		Status:        model.ImageTaskStatusPending,
		Source:        ecommerceImageSource,
		WorkflowID:    workflow.WorkflowID,
		WorkflowStage: "mother",
		ChannelID:     channel.Id,
	}
	if err := motherTask.SetReferenceImageKeys(refKeys); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.CreateImageTask(motherTask); err != nil {
		common.ApiError(c, err)
		return
	}
	_ = model.UpdateEcommerceWorkflowFields(workflow.WorkflowID, map[string]any{
		"mother_task_id": motherTask.TaskID,
		"status":         model.EcommerceWorkflowStatusMotherProcessing,
	})
	workflow.MotherTaskID = motherTask.TaskID
	workflow.Status = model.EcommerceWorkflowStatusMotherProcessing
	common.ApiSuccess(c, toEcommerceWorkflowDTO(workflow, nil, false))
}

func GetUserEcommerceWorkflow(c *gin.Context) {
	workflow, err := model.GetEcommerceWorkflowByWorkflowIDAndUserID(c.Param("workflow_id"), c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	_ = processEcommerceWorkflow(context.Background(), workflow)
	workflow, _ = model.GetEcommerceWorkflowByWorkflowIDAndUserID(c.Param("workflow_id"), c.GetInt("id"))
	segments, _ := model.GetEcommerceWorkflowSegments(workflow.WorkflowID)
	common.ApiSuccess(c, toEcommerceWorkflowDTO(workflow, segments, false))
}

func ListUserEcommerceWorkflows(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	items, total, err := model.ListUserEcommerceWorkflows(c.GetInt("id"), pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	dtos := make([]*dto.EcommerceWorkflowDTO, 0, len(items))
	for _, workflow := range items {
		segments, _ := model.GetEcommerceWorkflowSegments(workflow.WorkflowID)
		dtos = append(dtos, toEcommerceWorkflowDTO(workflow, segments, false))
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(dtos)
	common.ApiSuccess(c, pageInfo)
}

func ConfirmEcommerceWorkflow(c *gin.Context) {
	workflow, err := model.GetEcommerceWorkflowByWorkflowIDAndUserID(c.Param("workflow_id"), c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if workflow.Status != model.EcommerceWorkflowStatusWaitingConfirm && workflow.Status != model.EcommerceWorkflowStatusSucceeded {
		_ = processEcommerceWorkflow(context.Background(), workflow)
		workflow, _ = model.GetEcommerceWorkflowByWorkflowIDAndUserID(workflow.WorkflowID, c.GetInt("id"))
	}
	if workflow.Status != model.EcommerceWorkflowStatusWaitingConfirm && workflow.Status != model.EcommerceWorkflowStatusSucceeded {
		common.ApiErrorMsg(c, "mother image is not ready for confirmation")
		return
	}
	if workflow.Status == model.EcommerceWorkflowStatusSucceeded {
		segments, _ := model.GetEcommerceWorkflowSegments(workflow.WorkflowID)
		common.ApiSuccess(c, toEcommerceWorkflowDTO(workflow, segments, false))
		return
	}
	if err := model.UpdateEcommerceWorkflowFields(workflow.WorkflowID, map[string]any{
		"status":       model.EcommerceWorkflowStatusSegmentsPending,
		"confirmed_at": time.Now().Unix(),
	}); err != nil {
		common.ApiError(c, err)
		return
	}
	workflow.Status = model.EcommerceWorkflowStatusSegmentsPending
	workflow.ConfirmedAt = time.Now().Unix()
	if err := processEcommerceWorkflow(context.Background(), workflow); err != nil {
		common.ApiError(c, err)
		return
	}
	workflow, _ = model.GetEcommerceWorkflowByWorkflowIDAndUserID(workflow.WorkflowID, c.GetInt("id"))
	segments, _ := model.GetEcommerceWorkflowSegments(workflow.WorkflowID)
	common.ApiSuccess(c, toEcommerceWorkflowDTO(workflow, segments, false))
}

func RedrawEcommerceWorkflowSegment(c *gin.Context) {
	workflow, err := model.GetEcommerceWorkflowByWorkflowIDAndUserID(c.Param("workflow_id"), c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	req := &dto.RedrawEcommerceSegmentRequest{}
	if err := common.UnmarshalBodyReusable(c, req); err != nil {
		common.ApiError(c, err)
		return
	}
	segment, err := model.GetEcommerceWorkflowSegment(workflow.WorkflowID, c.Param("segment_key"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if segment.SliceKey == "" {
		common.ApiErrorMsg(c, "segment slice is missing")
		return
	}
	if err := createOrReplaceEcommerceSegmentTask(context.Background(), workflow, segment, strings.TrimSpace(req.Prompt), true); err != nil {
		common.ApiError(c, err)
		return
	}
	_ = model.UpdateEcommerceWorkflowFields(workflow.WorkflowID, map[string]any{"status": model.EcommerceWorkflowStatusSegmentsProcessing, "assembled_url": "", "assembled_key": ""})
	workflow, _ = model.GetEcommerceWorkflowByWorkflowIDAndUserID(workflow.WorkflowID, c.GetInt("id"))
	segments, _ := model.GetEcommerceWorkflowSegments(workflow.WorkflowID)
	common.ApiSuccess(c, toEcommerceWorkflowDTO(workflow, segments, false))
}

func GetAllEcommerceWorkflows(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	userID := common.String2Int(c.Query("user_id"))
	startTime, _ := strconv.ParseInt(c.Query("start_time"), 10, 64)
	endTime, _ := strconv.ParseInt(c.Query("end_time"), 10, 64)
	items, total, err := model.ListAllEcommerceWorkflows(pageInfo.GetStartIdx(), pageInfo.GetPageSize(), userID, c.Query("status"), c.Query("template_key"), startTime, endTime)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	dtos := make([]*dto.EcommerceWorkflowDTO, 0, len(items))
	for _, workflow := range items {
		segments, _ := model.GetEcommerceWorkflowSegments(workflow.WorkflowID)
		dtos = append(dtos, toEcommerceWorkflowDTO(workflow, segments, true))
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(dtos)
	common.ApiSuccess(c, pageInfo)
}

func processEcommerceWorkflow(ctx context.Context, workflow *model.EcommerceWorkflow) error {
	if workflow == nil {
		return nil
	}
	lock := getEcommerceWorkflowLock(workflow.WorkflowID)
	lock.Lock()
	defer lock.Unlock()

	latest, err := model.GetEcommerceWorkflowByWorkflowID(workflow.WorkflowID)
	if err == nil && latest != nil {
		workflow = latest
	}
	switch workflow.Status {
	case model.EcommerceWorkflowStatusMotherPending, model.EcommerceWorkflowStatusMotherProcessing:
		return syncEcommerceMotherTask(ctx, workflow)
	case model.EcommerceWorkflowStatusSegmentsPending, model.EcommerceWorkflowStatusSegmentsProcessing:
		return progressEcommerceSegments(ctx, workflow)
	default:
		return nil
	}
}

func syncEcommerceMotherTask(ctx context.Context, workflow *model.EcommerceWorkflow) error {
	if workflow.MotherTaskID == "" {
		return nil
	}
	task, err := model.GetImageTaskByTaskID(workflow.MotherTaskID)
	if err != nil {
		return err
	}
	if task.Status == model.ImageTaskStatusFailed {
		return model.UpdateEcommerceWorkflowFields(workflow.WorkflowID, map[string]any{"status": model.EcommerceWorkflowStatusMotherFailed, "error_message": task.ErrorMessage, "finished_at": time.Now().Unix()})
	}
	if task.Status != model.ImageTaskStatusSucceeded {
		return nil
	}
	resultURL := imageTaskResultURL(ctx, task)
	return model.UpdateEcommerceWorkflowFields(workflow.WorkflowID, map[string]any{
		"status":            model.EcommerceWorkflowStatusWaitingConfirm,
		"mother_result_url": resultURL,
		"mother_result_key": task.ResultKey,
		"error_message":     "",
	})
}

func progressEcommerceSegments(ctx context.Context, workflow *model.EcommerceWorkflow) error {
	segments, err := model.GetEcommerceWorkflowSegments(workflow.WorkflowID)
	if err != nil {
		return err
	}
	if len(segments) == 0 {
		if err := ensureEcommerceMotherSlices(ctx, workflow); err != nil {
			_ = model.UpdateEcommerceWorkflowFields(workflow.WorkflowID, map[string]any{"status": model.EcommerceWorkflowStatusFailed, "error_message": err.Error(), "finished_at": time.Now().Unix()})
			return err
		}
		segments, err = model.GetEcommerceWorkflowSegments(workflow.WorkflowID)
		if err != nil {
			return err
		}
	}
	for _, segment := range segments {
		if segment.TaskID == "" {
			return createOrReplaceEcommerceSegmentTask(ctx, workflow, segment, "", false)
		}
		task, err := model.GetImageTaskByTaskID(segment.TaskID)
		if err != nil {
			return err
		}
		if task.Status == model.ImageTaskStatusPending || task.Status == model.ImageTaskStatusProcessing {
			return nil
		}
		if task.Status == model.ImageTaskStatusFailed {
			_ = model.UpdateEcommerceWorkflowSegmentFields(workflow.WorkflowID, segment.SegmentKey, map[string]any{"status": task.Status, "custom_redraw_prompt": segment.CustomRedrawPrompt})
			return model.UpdateEcommerceWorkflowFields(workflow.WorkflowID, map[string]any{"status": model.EcommerceWorkflowStatusFailed, "error_message": task.ErrorMessage, "finished_at": time.Now().Unix()})
		}
		if task.Status == model.ImageTaskStatusSucceeded && segment.Status != model.ImageTaskStatusSucceeded {
			_ = model.UpdateEcommerceWorkflowSegmentFields(workflow.WorkflowID, segment.SegmentKey, map[string]any{"status": task.Status, "result_url": imageTaskResultURL(ctx, task), "result_key": task.ResultKey})
			segment.Status = task.Status
			segment.ResultURL = imageTaskResultURL(ctx, task)
			segment.ResultKey = task.ResultKey
		}
	}
	segments, err = model.GetEcommerceWorkflowSegments(workflow.WorkflowID)
	if err != nil {
		return err
	}
	for _, segment := range segments {
		if segment.Status != model.ImageTaskStatusSucceeded {
			return nil
		}
	}
	assembledKey, assembledURL, err := stitchEcommerceSegments(ctx, workflow, segments)
	if err != nil {
		return err
	}
	return model.UpdateEcommerceWorkflowFields(workflow.WorkflowID, map[string]any{"status": model.EcommerceWorkflowStatusSucceeded, "assembled_key": assembledKey, "assembled_url": assembledURL, "error_message": "", "finished_at": time.Now().Unix()})
}

func ensureEcommerceMotherSlices(ctx context.Context, workflow *model.EcommerceWorkflow) error {
	existingSegments, err := model.GetEcommerceWorkflowSegments(workflow.WorkflowID)
	if err != nil {
		return err
	}
	if len(existingSegments) >= len(ecommerceSegmentSpecs) {
		return nil
	}
	if workflow.MotherResultKey == "" && workflow.MotherTaskID != "" {
		if task, err := model.GetImageTaskByTaskID(workflow.MotherTaskID); err == nil {
			workflow.MotherResultKey = task.ResultKey
			workflow.MotherResultURL = imageTaskResultURL(ctx, task)
		}
	}
	motherData, _, err := loadWorkflowImageBytes(ctx, workflow.MotherResultKey, workflow.MotherResultURL)
	if err != nil {
		return err
	}
	sliceKeys, err := cropAndUploadMotherSlices(ctx, workflow.UserID, workflow.WorkflowID, motherData)
	if err != nil {
		return err
	}
	for index, spec := range ecommerceSegmentSpecs {
		if _, err := model.GetEcommerceWorkflowSegment(workflow.WorkflowID, spec.Key); err == nil {
			continue
		}
		segment := &model.EcommerceWorkflowSegment{WorkflowID: workflow.WorkflowID, SegmentKey: spec.Key, SegmentIndex: index, Label: spec.Label, Description: spec.Description, Status: model.ImageTaskStatusPending, SliceKey: sliceKeys[index]}
		if err := model.CreateEcommerceWorkflowSegment(segment); err != nil {
			return err
		}
	}
	return nil
}

func createOrReplaceEcommerceSegmentTask(ctx context.Context, workflow *model.EcommerceWorkflow, segment *model.EcommerceWorkflowSegment, redrawPrompt string, isRedraw bool) error {
	channel, err := selectEcommerceWorkflowChannel(workflow)
	if err != nil {
		return err
	}
	prompt := buildEcommerceSegmentPrompt(workflow, segment, redrawPrompt)
	referenceKeys := buildEcommerceSegmentReferenceKeys(workflow, segment)
	task := &model.ImageTask{UserID: workflow.UserID, Group: workflow.Group, Model: workflow.Model, Size: "1024x1536", Prompt: prompt, Status: model.ImageTaskStatusPending, Source: ecommerceImageSource, WorkflowID: workflow.WorkflowID, WorkflowStage: "segment_" + segment.SegmentKey, ChannelID: channel.Id}
	if isRedraw {
		task.WorkflowStage = "segment_redraw"
	}
	if err := task.SetReferenceImageKeys(referenceKeys); err != nil {
		return err
	}
	if err := model.CreateImageTask(task); err != nil {
		return err
	}
	updates := map[string]any{"task_id": task.TaskID, "status": model.ImageTaskStatusPending, "prompt": prompt, "result_url": "", "result_key": ""}
	if isRedraw {
		updates["custom_redraw_prompt"] = redrawPrompt
		updates["redraw_count"] = segment.RedrawCount + 1
	}
	if err := model.UpdateEcommerceWorkflowSegmentFields(workflow.WorkflowID, segment.SegmentKey, updates); err != nil {
		return err
	}
	return model.UpdateEcommerceWorkflowFields(workflow.WorkflowID, map[string]any{"status": model.EcommerceWorkflowStatusSegmentsProcessing, "error_message": ""})
}

func buildEcommerceSegmentReferenceKeys(workflow *model.EcommerceWorkflow, segment *model.EcommerceWorkflowSegment) []string {
	productKeys := workflow.GetReferenceImageKeys()
	if segment.SegmentIndex == 0 && len(productKeys) > 3 {
		productKeys = productKeys[:3]
	}
	if segment.SegmentIndex > 0 && len(productKeys) > 2 {
		productKeys = productKeys[:2]
	}
	keys := append([]string{}, productKeys...)
	if workflow.MotherResultKey != "" {
		keys = append(keys, workflow.MotherResultKey)
	}
	if segment.SliceKey != "" {
		keys = append(keys, segment.SliceKey)
	}
	if segment.SegmentIndex > 0 {
		if previous, err := findPreviousEcommerceSegment(workflow.WorkflowID, segment.SegmentIndex); err == nil && previous.ResultKey != "" {
			keys = append(keys, previous.ResultKey)
		}
	}
	if len(keys) > maxReferenceImagesPerTask {
		return keys[:maxReferenceImagesPerTask]
	}
	return keys
}

func findPreviousEcommerceSegment(workflowID string, currentIndex int) (*model.EcommerceWorkflowSegment, error) {
	segments, err := model.GetEcommerceWorkflowSegments(workflowID)
	if err != nil {
		return nil, err
	}
	for _, segment := range segments {
		if segment.SegmentIndex == currentIndex-1 {
			return segment, nil
		}
	}
	return nil, fmt.Errorf("previous segment not found")
}

func selectEcommerceWorkflowChannel(workflow *model.EcommerceWorkflow) (*model.Channel, error) {
	userCache, err := model.GetUserCache(workflow.UserID)
	if err != nil {
		return nil, err
	}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/api/ai-ecommerce/workflows/internal", nil)
	channel, _, err := buildImageTaskChannelContext(c, userCache, workflow.Group, workflow.Model)
	return channel, err
}

func buildEcommerceMotherPrompt(workflow *model.EcommerceWorkflow) string {
	return strings.Join([]string{
		"请基于上传的商品参考图，生成一张 1:1 方形的中文电商详情页母版图。",
		"这张图不是最终长图，而是后续分段扩展的视觉参考母版。",
		"商品名称：" + fallbackText(workflow.ProductName, "未命名商品"),
		"商品品类：" + fallbackText(workflow.ProductType, "通用商品"),
		"目标平台：" + fallbackText(workflow.Platform, "淘宝 / 天猫 / 京东"),
		"核心卖点：" + fallbackText(workflow.SellingPoints, "突出商品质感、卖点和购买理由"),
		"模板风格：" + workflow.StylePrompt,
		"页面模块：" + modulesText(workflow.Modules),
		"画面要求：真实中国电商详情页语言，包含主视觉、卖点区、参数/规格块、细节展示、信任背书和收尾 CTA 区域。",
		"排版要求：三段纵向结构浓缩在一个方形板内，模块感清晰，留出大量中文标题和正文排版区域。",
		"文字要求：可以生成文字感占位和局部中文短句，不要求最终可商用文案准确，但必须像真实详情页。",
		"商品一致性：必须保留参考图中的商品外观、材质、颜色和核心识别特征。",
		optionalLine("补充要求：", workflow.ExtraRequirements),
	}, "\n")
}

func buildEcommerceSegmentPrompt(workflow *model.EcommerceWorkflow, segment *model.EcommerceWorkflowSegment, redrawPrompt string) string {
	parts := []string{
		fmt.Sprintf("请生成电商详情页长图的第 %d 段：%s。", segment.SegmentIndex+1, segment.Label),
		"参考图包含：原始商品图、完整母版图、当前母版切片，以及上一段结果（如果有）。",
		"请严格延续母版图的商品身份、视觉风格、模块语言、配色和电商详情页排版。",
		"本段定位：" + segment.Description,
		"商品名称：" + fallbackText(workflow.ProductName, "未命名商品"),
		"商品品类：" + fallbackText(workflow.ProductType, "通用商品"),
		"目标平台：" + fallbackText(workflow.Platform, "淘宝 / 天猫 / 京东"),
		"核心卖点：" + fallbackText(workflow.SellingPoints, "突出商品质感、卖点和购买理由"),
		"模板风格：" + workflow.StylePrompt,
		"输出要求：生成一张竖向详情页分段图，适合后续和其他段落上下拼接。",
		"连续性要求：背景色温、边距、模块密度、光影、商品比例要尽量承接上一段。",
		"文字要求：保留真实电商详情页的标题区、说明区和参数块感觉，不要求最终文案完全准确。",
		optionalLine("补充要求：", workflow.ExtraRequirements),
	}
	if redrawPrompt != "" {
		parts = append(parts, "本次重绘额外要求："+redrawPrompt)
	}
	return strings.Join(parts, "\n")
}

func fallbackText(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

func optionalLine(prefix, value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return prefix + strings.TrimSpace(value)
}

func modulesText(raw string) string {
	var modules []string
	if err := common.Unmarshal([]byte(raw), &modules); err == nil && len(modules) > 0 {
		return strings.Join(modules, "、")
	}
	return raw
}

func loadWorkflowImageBytes(ctx context.Context, objectKey, imageURL string) ([]byte, string, error) {
	if objectKey != "" && !strings.Contains(objectKey, "://") {
		return service.DownloadObjectFromStorage(ctx, objectKey)
	}
	if imageURL == "" {
		return nil, "", fmt.Errorf("image source is empty")
	}
	mimeType, base64Data, err := service.GetImageFromUrlWithContext(ctx, imageURL)
	if err != nil {
		return nil, "", err
	}
	decoded, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return nil, "", err
	}
	return decoded, mimeType, nil
}

func cropAndUploadMotherSlices(ctx context.Context, userID int, _ string, data []byte) ([]string, error) {
	source, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	bounds := source.Bounds()
	sliceHeight := bounds.Dy() / len(ecommerceSegmentSpecs)
	keys := make([]string, 0, len(ecommerceSegmentSpecs))
	for index := range ecommerceSegmentSpecs {
		y0 := bounds.Min.Y + index*sliceHeight
		y1 := y0 + sliceHeight
		if index == len(ecommerceSegmentSpecs)-1 {
			y1 = bounds.Max.Y
		}
		canvas := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), y1-y0))
		draw.Draw(canvas, canvas.Bounds(), source, image.Point{X: bounds.Min.X, Y: y0}, draw.Src)
		var buf bytes.Buffer
		if err := png.Encode(&buf, canvas); err != nil {
			return nil, err
		}
		key := service.BuildAIImageRefObjectKey(userID, "png")
		objectKey, _, err := service.UploadBytesToObjectStorage(ctx, key, "image/png", buf.Bytes())
		if err != nil {
			return nil, err
		}
		keys = append(keys, objectKey)
	}
	return keys, nil
}

func stitchEcommerceSegments(ctx context.Context, workflow *model.EcommerceWorkflow, segments []*model.EcommerceWorkflowSegment) (string, string, error) {
	images := make([]image.Image, 0, len(segments))
	maxWidth := 0
	totalHeight := 0
	for _, segment := range segments {
		data, _, err := loadWorkflowImageBytes(ctx, segment.ResultKey, segment.ResultURL)
		if err != nil {
			return "", "", err
		}
		img, _, err := image.Decode(bytes.NewReader(data))
		if err != nil {
			return "", "", err
		}
		images = append(images, img)
		bounds := img.Bounds()
		if bounds.Dx() > maxWidth {
			maxWidth = bounds.Dx()
		}
		totalHeight += bounds.Dy()
	}
	if maxWidth == 0 || totalHeight == 0 {
		return "", "", fmt.Errorf("empty ecommerce segment images")
	}
	canvas := image.NewRGBA(image.Rect(0, 0, maxWidth, totalHeight))
	offsetY := 0
	for _, img := range images {
		bounds := img.Bounds()
		x := (maxWidth - bounds.Dx()) / 2
		draw.Draw(canvas, image.Rect(x, offsetY, x+bounds.Dx(), offsetY+bounds.Dy()), img, bounds.Min, draw.Src)
		offsetY += bounds.Dy()
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, canvas); err != nil {
		return "", "", err
	}
	key := service.BuildAIImageObjectKey(workflow.UserID, "png")
	objectKey, accessURL, err := service.UploadBytesToObjectStorage(ctx, key, "image/png", buf.Bytes())
	return objectKey, accessURL, err
}

func imageTaskResultURL(ctx context.Context, task *model.ImageTask) string {
	if task == nil {
		return ""
	}
	if task.ResultKey != "" && !strings.Contains(task.ResultKey, "://") && service.IsObjectStorageEnabled() {
		if signedURL, err := service.GenerateObjectStorageAccessURL(ctx, task.ResultKey); err == nil {
			return signedURL
		}
	}
	return task.ResultURL
}

func objectKeyURL(ctx context.Context, key string) string {
	if key == "" || strings.Contains(key, "://") || !service.IsObjectStorageEnabled() {
		return key
	}
	url, _ := service.GenerateObjectStorageAccessURL(ctx, key)
	return url
}

func toEcommerceWorkflowDTO(workflow *model.EcommerceWorkflow, segments []*model.EcommerceWorkflowSegment, fillUser bool) *dto.EcommerceWorkflowDTO {
	if workflow == nil {
		return nil
	}
	if segments == nil {
		segments, _ = model.GetEcommerceWorkflowSegments(workflow.WorkflowID)
	}
	result := &dto.EcommerceWorkflowDTO{ID: workflow.ID, WorkflowID: workflow.WorkflowID, UserID: workflow.UserID, TemplateKey: workflow.TemplateKey, TemplateName: workflow.TemplateName, ProductName: workflow.ProductName, ProductType: workflow.ProductType, Platform: workflow.Platform, SellingPoints: workflow.SellingPoints, ExtraRequirements: workflow.ExtraRequirements, Model: workflow.Model, Group: workflow.Group, Size: workflow.Size, Status: string(workflow.Status), MotherTaskID: workflow.MotherTaskID, MotherResultURL: workflow.MotherResultURL, MotherResultKey: workflow.MotherResultKey, AssembledURL: workflow.AssembledURL, AssembledKey: workflow.AssembledKey, ErrorMessage: workflow.ErrorMessage, ConfirmedAt: workflow.ConfirmedAt, FinishedAt: workflow.FinishedAt, CreatedAt: workflow.CreatedAt, UpdatedAt: workflow.UpdatedAt, Segments: make([]*dto.EcommerceWorkflowSegmentDTO, 0, len(segments))}
	ctx := context.Background()
	if result.MotherResultURL == "" && workflow.MotherResultKey != "" {
		result.MotherResultURL = objectKeyURL(ctx, workflow.MotherResultKey)
	}
	if result.AssembledURL == "" && workflow.AssembledKey != "" {
		result.AssembledURL = objectKeyURL(ctx, workflow.AssembledKey)
	}
	if fillUser {
		if user, err := model.GetUserCache(workflow.UserID); err == nil {
			result.Username = user.Username
		}
	}
	for _, segment := range segments {
		item := &dto.EcommerceWorkflowSegmentDTO{ID: segment.ID, WorkflowID: segment.WorkflowID, SegmentKey: segment.SegmentKey, SegmentIndex: segment.SegmentIndex, Label: segment.Label, Description: segment.Description, Prompt: segment.Prompt, CustomRedrawPrompt: segment.CustomRedrawPrompt, TaskID: segment.TaskID, Status: string(segment.Status), ResultURL: segment.ResultURL, ResultKey: segment.ResultKey, SliceURL: objectKeyURL(ctx, segment.SliceKey), SliceKey: segment.SliceKey, RedrawCount: segment.RedrawCount, CreatedAt: segment.CreatedAt, UpdatedAt: segment.UpdatedAt}
		if item.ResultURL == "" && segment.ResultKey != "" {
			item.ResultURL = objectKeyURL(ctx, segment.ResultKey)
		}
		result.Segments = append(result.Segments, item)
	}
	return result
}

func todayTimeRange() (int64, int64) {
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Unix()
	end := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location()).Unix()
	return start, end
}

func GetAIImageDailyStats(c *gin.Context) {
	start, end := todayTimeRange()
	aiImageTotal, _ := model.CountImageTasksBySourceAndTime("ai_image", start, end)
	ecommerceTasks, _ := model.CountImageTasksBySourceAndTime(ecommerceImageSource, start, end)
	ecommerceWorkflows, _ := model.CountEcommerceWorkflowsByTime(start, end)
	common.ApiSuccess(c, gin.H{"ai_image_tasks": aiImageTotal, "ai_ecommerce_tasks": ecommerceTasks, "ai_ecommerce_workflows": ecommerceWorkflows, "start_time": start, "end_time": end})
}
