package controller

import (
	"errors"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
)

const (
	supportMessageRequestMaxBytes = 21 << 20
	supportMessageFormMemoryBytes = 8 << 20
)

type parsedSupportMessagePayload struct {
	Content     string
	Attachments []model.SupportMessageAttachment
	Stored      []service.StoredSupportAttachment
}

type supportChatSettingsRequest struct {
	RetentionDays *int `json:"retention_days"`
}

// parseSupportMessagePayload 解析文本/ multipart 发消息请求，并保存上传的图片。
// 返回 false 时已写入错误响应；返回 true 时调用方负责在后续失败时清理 Stored 文件。
func parseSupportMessagePayload(c *gin.Context, conversationId int) (*parsedSupportMessagePayload, bool) {
	var content string
	var files []*multipart.FileHeader

	if isSupportMultipartRequest(c.GetHeader("Content-Type")) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, supportMessageRequestMaxBytes)
		if err := c.Request.ParseMultipartForm(supportMessageFormMemoryBytes); err != nil {
			var maxBytesErr *http.MaxBytesError
			if errors.As(err, &maxBytesErr) {
				common.ApiErrorMsg(c, "图片总大小不能超过 20MB")
			} else {
				common.ApiErrorI18n(c, i18n.MsgInvalidParams)
			}
			return nil, false
		}
		if c.Request.MultipartForm != nil {
			defer c.Request.MultipartForm.RemoveAll()
		}
		content = strings.TrimSpace(c.PostForm("content"))
		if c.Request.MultipartForm != nil {
			files = c.Request.MultipartForm.File["files"]
		}
	} else {
		var request supportMessageRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			common.ApiErrorI18n(c, i18n.MsgInvalidParams)
			return nil, false
		}
		content = strings.TrimSpace(request.Content)
	}

	if utf8.RuneCountInString(content) > supportMessageMaxRunes {
		common.ApiErrorMsg(c, "消息内容不能超过 2000 个字符")
		return nil, false
	}
	if content == "" && len(files) == 0 {
		common.ApiErrorMsg(c, "消息内容不能为空")
		return nil, false
	}
	if len(files) > service.MaxSupportAttachmentsPerMessage {
		common.ApiErrorMsg(c, "每条消息最多发送 4 张图片")
		return nil, false
	}

	var declaredTotal int64
	for _, file := range files {
		if file.Size > 0 {
			declaredTotal += file.Size
		}
	}
	if declaredTotal > service.MaxSupportMessageAttachmentBytes {
		common.ApiErrorMsg(c, "单条消息总大小不能超过 20MB")
		return nil, false
	}

	stored := make([]service.StoredSupportAttachment, 0, len(files))
	cleanup := func() {
		service.CleanupStoredSupportAttachments(stored)
	}

	for _, header := range files {
		file, err := header.Open()
		if err != nil {
			cleanup()
			common.ApiErrorMsg(c, "读取图片失败")
			return nil, false
		}
		result, err := service.SaveSupportAttachment(conversationId, file, header)
		_ = file.Close()
		if err != nil {
			cleanup()
			common.ApiErrorMsg(c, err.Error())
			return nil, false
		}
		stored = append(stored, *result)
	}

	attachments := make([]model.SupportMessageAttachment, 0, len(stored))
	for _, item := range stored {
		attachments = append(attachments, model.SupportMessageAttachment{
			ConversationId:    conversationId,
			FileName:          item.FileName,
			StorageKey:        item.StorageKey,
			ThumbnailKey:      item.ThumbnailKey,
			ThumbnailMimeType: item.ThumbnailMimeType,
			MimeType:          item.MimeType,
			SizeBytes:         item.SizeBytes,
			Width:             item.Width,
			Height:            item.Height,
		})
	}

	return &parsedSupportMessagePayload{
		Content:     content,
		Attachments: attachments,
		Stored:      stored,
	}, true
}

func isSupportMultipartRequest(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}
	return strings.EqualFold(mediaType, "multipart/form-data")
}

// GetSupportAttachment 下载客服消息图片，支持 variant=thumb|full。
func GetSupportAttachment(c *gin.Context) {
	attachmentId, err := strconv.Atoi(c.Param("id"))
	if err != nil || attachmentId <= 0 {
		common.ApiErrorI18n(c, i18n.MsgInvalidId)
		return
	}

	userId := c.GetInt("id")
	if userId <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "请先登录",
		})
		return
	}

	attachment, err := model.GetSupportAttachmentById(attachmentId)
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	if attachment == nil {
		respondSupportAttachmentNotFound(c)
		return
	}

	conversation, err := model.GetSupportConversationById(attachment.ConversationId)
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	if conversation == nil {
		respondSupportAttachmentNotFound(c)
		return
	}

	if conversation.UserId != userId &&
		!common.RoleHasPermission(c.GetInt("role"), common.PermissionSupportReply) {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "无权访问该图片",
		})
		return
	}

	storageKey := attachment.StorageKey
	mimeType := attachment.MimeType
	if strings.EqualFold(strings.TrimSpace(c.DefaultQuery("variant", "full")), "thumb") &&
		attachment.ThumbnailKey != "" {
		storageKey = attachment.ThumbnailKey
		mimeType = attachment.ThumbnailMimeType
		if mimeType == "" {
			mimeType = attachment.MimeType
		}
	}

	filePath, err := service.ResolveSupportAttachmentPath(storageKey)
	if err != nil {
		respondSupportAttachmentNotFound(c)
		return
	}
	file, err := os.Open(filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			respondSupportAttachmentNotFound(c)
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "读取图片失败",
		})
		return
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil || info.IsDir() {
		respondSupportAttachmentNotFound(c)
		return
	}

	c.Header("Content-Type", mimeType)
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Cache-Control", "private, max-age=86400")
	http.ServeContent(c.Writer, c.Request, attachment.FileName, info.ModTime(), file)
}

func respondSupportAttachmentNotFound(c *gin.Context) {
	c.JSON(http.StatusNotFound, gin.H{
		"success": false,
		"message": "图片已过期或不存在",
	})
}

func requireSupportSettingsPermission(c *gin.Context) bool {
	if common.RoleHasPermission(c.GetInt("role"), common.PermissionSupportManage) {
		return true
	}
	c.JSON(http.StatusForbidden, gin.H{
		"success": false,
		"message": "无权管理客服留存设置",
	})
	return false
}

// GetSupportChatSettings 返回客服会话留存设置。
func GetSupportChatSettings(c *gin.Context) {
	if !requireSupportSettingsPermission(c) {
		return
	}
	settings := system_setting.GetSupportChatSettings()
	supportChatSuccess(c, gin.H{"retention_days": settings.RetentionDays})
}

// UpdateSupportChatSettings 更新客服会话留存设置，0 表示永久保留。
func UpdateSupportChatSettings(c *gin.Context) {
	if !requireSupportSettingsPermission(c) {
		return
	}

	var request supportChatSettingsRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiErrorMsg(c, "留存天数格式不正确")
		return
	}
	if request.RetentionDays == nil {
		common.ApiErrorMsg(c, "留存天数不能为空")
		return
	}
	days := *request.RetentionDays
	if days < 0 || days > 3650 {
		common.ApiErrorMsg(c, "留存天数必须在 0 到 3650 之间（0 表示永久保留）")
		return
	}
	if err := model.UpdateOption(system_setting.SupportChatRetentionDaysOption, strconv.Itoa(days)); err != nil {
		common.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	supportChatSuccess(c, gin.H{"retention_days": system_setting.GetSupportChatSettings().RetentionDays})
}
