package controller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type supportAttachmentTestResponse struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type supportAttachmentTestUpload struct {
	Name string
	Data []byte
}

type supportAttachmentMessageData struct {
	Message      model.SupportMessage      `json:"message"`
	Conversation model.SupportConversation `json:"conversation"`
}

type supportAttachmentMessagesData struct {
	Messages     []model.SupportMessage    `json:"messages"`
	Conversation model.SupportConversation `json:"conversation"`
}

type supportChatSettingsTestPayload struct {
	RetentionDays  int    `json:"retention_days"`
	UIScale        int    `json:"ui_scale"`
	TextScale      int    `json:"text_scale"`
	Prominence     int    `json:"prominence"`
	CustomTitle    string `json:"custom_title"`
	CustomSubLogin string `json:"custom_sub_login"`
	CustomSubGuest string `json:"custom_sub_guest"`
	BottomOffset   int    `json:"bottom_offset"`
}

func setupSupportAttachmentControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false

	previousOptionMap := common.OptionMap
	common.OptionMap = map[string]string{}

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(
		&model.SupportConversation{},
		&model.SupportMessage{},
		&model.SupportMessageAttachment{},
		&model.Option{},
	))

	t.Cleanup(func() {
		sqlDB, closeErr := db.DB()
		if closeErr == nil {
			_ = sqlDB.Close()
		}
		model.DB = nil
		model.LOG_DB = nil
		common.OptionMap = previousOptionMap
	})
	return db
}

func setupSupportAttachmentControllerTestDir(t *testing.T) string {
	t.Helper()
	previous, hadPrevious := os.LookupEnv("SUPPORT_ATTACHMENT_DIR")
	dir := filepath.Join(t.TempDir(), "support-attachments")
	require.NoError(t, os.Setenv("SUPPORT_ATTACHMENT_DIR", dir))
	t.Cleanup(func() {
		if hadPrevious {
			_ = os.Setenv("SUPPORT_ATTACHMENT_DIR", previous)
		} else {
			_ = os.Unsetenv("SUPPORT_ATTACHMENT_DIR")
		}
	})
	return dir
}

func setSupportChatTestRetentionDays(t *testing.T, days int) {
	t.Helper()
	cfg, ok := config.GlobalConfig.Get("support_chat").(*system_setting.SupportChatSettings)
	require.True(t, ok)
	previous := cfg.RetentionDays
	cfg.RetentionDays = days
	t.Cleanup(func() {
		cfg.RetentionDays = previous
	})
}

func supportAttachmentTestPNG(t *testing.T, width int, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.RGBA{R: 20, G: 100, B: 220, A: 255}}, image.Point{}, draw.Src)
	var buffer bytes.Buffer
	require.NoError(t, png.Encode(&buffer, img))
	return buffer.Bytes()
}

func buildSupportAttachmentMultipartBody(t *testing.T, content string, uploads []supportAttachmentTestUpload) (*bytes.Buffer, string) {
	t.Helper()
	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)
	if content != "" {
		require.NoError(t, writer.WriteField("content", content))
	}
	for _, upload := range uploads {
		part, err := writer.CreateFormFile("files", upload.Name)
		require.NoError(t, err)
		_, err = part.Write(upload.Data)
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())
	return &buffer, writer.FormDataContentType()
}

func newSupportAttachmentTestContext(
	t *testing.T,
	method string,
	target string,
	body []byte,
	contentType string,
	userId int,
	role int,
	params gin.Params,
) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(method, target, bytes.NewReader(body))
	if contentType != "" {
		ctx.Request.Header.Set("Content-Type", contentType)
	}
	ctx.Params = params
	if userId > 0 {
		ctx.Set("id", userId)
	}
	ctx.Set("role", role)
	ctx.Set("username", fmt.Sprintf("user-%d", userId))
	return ctx, recorder
}

func decodeSupportAttachmentResponse(t *testing.T, recorder *httptest.ResponseRecorder) supportAttachmentTestResponse {
	t.Helper()
	var response supportAttachmentTestResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	return response
}

func supportAttachmentIDParam(id int) gin.Params {
	return gin.Params{{Key: "id", Value: fmt.Sprintf("%d", id)}}
}

func TestSendSupportConversationMessageWithMultipartImages(t *testing.T) {
	setupSupportAttachmentControllerTestDB(t)
	setupSupportAttachmentControllerTestDir(t)

	conversation, err := model.GetOrCreateSupportConversation(1001)
	require.NoError(t, err)

	uploads := []supportAttachmentTestUpload{
		{Name: "first.png", Data: supportAttachmentTestPNG(t, 60, 40)},
		{Name: "second.png", Data: supportAttachmentTestPNG(t, 80, 50)},
	}
	body, contentType := buildSupportAttachmentMultipartBody(t, "hello", uploads)
	ctx, recorder := newSupportAttachmentTestContext(
		t, http.MethodPost, fmt.Sprintf("/api/support/conversation/%d/messages", conversation.Id),
		body.Bytes(), contentType, 1001, common.RoleCommonUser, supportAttachmentIDParam(conversation.Id),
	)
	SendSupportConversationMessage(ctx)

	response := decodeSupportAttachmentResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var data supportAttachmentMessageData
	require.NoError(t, common.Unmarshal(response.Data, &data))
	assert.Equal(t, "hello", data.Message.Content)
	require.Len(t, data.Message.Attachments, 2)
	for i, attachment := range data.Message.Attachments {
		assert.NotZero(t, attachment.Id)
		assert.Equal(t, data.Message.Id, attachment.MessageId)
		assert.Equal(t, conversation.Id, attachment.ConversationId)
		assert.Equal(t, uploads[i].Name, attachment.FileName)
		assert.Equal(t, "image/png", attachment.MimeType)
		assert.NotZero(t, attachment.SizeBytes)
		assert.NotZero(t, attachment.Width)
		assert.NotZero(t, attachment.Height)
	}
	updated, err := model.GetSupportConversationById(conversation.Id)
	require.NoError(t, err)
	assert.Equal(t, "hello", updated.LastMessagePreview)
	assert.Equal(t, 1, updated.AdminUnread)

	// 纯图片消息允许 content 为空，预览为 [图片]。
	pureImageUpload := []supportAttachmentTestUpload{
		{Name: "large.png", Data: supportAttachmentTestPNG(t, 1200, 800)},
	}
	body, contentType = buildSupportAttachmentMultipartBody(t, "", pureImageUpload)
	ctx, recorder = newSupportAttachmentTestContext(
		t, http.MethodPost, fmt.Sprintf("/api/support/conversation/%d/messages", conversation.Id),
		body.Bytes(), contentType, 1001, common.RoleCommonUser, supportAttachmentIDParam(conversation.Id),
	)
	SendSupportConversationMessage(ctx)
	response = decodeSupportAttachmentResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	data = supportAttachmentMessageData{}
	require.NoError(t, common.Unmarshal(response.Data, &data))
	assert.Equal(t, "", data.Message.Content)
	require.Len(t, data.Message.Attachments, 1)
	assert.Equal(t, "image/jpeg", data.Message.Attachments[0].ThumbnailMimeType)
	updated, err = model.GetSupportConversationById(conversation.Id)
	require.NoError(t, err)
	assert.Equal(t, "[图片]", updated.LastMessagePreview)
	assert.Equal(t, 2, updated.AdminUnread)

	// 列表接口批量返回附件。
	ctx, recorder = newSupportAttachmentTestContext(
		t, http.MethodGet, fmt.Sprintf("/api/support/conversation/%d/messages", conversation.Id),
		nil, "", 1001, common.RoleCommonUser, supportAttachmentIDParam(conversation.Id),
	)
	GetSupportConversationMessages(ctx)
	response = decodeSupportAttachmentResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var messagesData supportAttachmentMessagesData
	require.NoError(t, common.Unmarshal(response.Data, &messagesData))
	require.Len(t, messagesData.Messages, 2)
	require.Len(t, messagesData.Messages[0].Attachments, 2)
	require.Len(t, messagesData.Messages[1].Attachments, 1)

	// 纯文本 JSON 请求保持兼容。
	jsonBody, err := common.Marshal(map[string]any{"content": "plain text"})
	require.NoError(t, err)
	ctx, recorder = newSupportAttachmentTestContext(
		t, http.MethodPost, fmt.Sprintf("/api/support/conversation/%d/messages", conversation.Id),
		jsonBody, "application/json", 1001, common.RoleCommonUser, supportAttachmentIDParam(conversation.Id),
	)
	SendSupportConversationMessage(ctx)
	response = decodeSupportAttachmentResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	data = supportAttachmentMessageData{}
	require.NoError(t, common.Unmarshal(response.Data, &data))
	assert.Equal(t, "plain text", data.Message.Content)
	assert.Empty(t, data.Message.Attachments)
}

func TestSendSupportConversationMessageRejectsInvalidUploads(t *testing.T) {
	setupSupportAttachmentControllerTestDB(t)
	setupSupportAttachmentControllerTestDir(t)

	conversation, err := model.GetOrCreateSupportConversation(1002)
	require.NoError(t, err)
	conversationDir := filepath.Join(service.GetSupportAttachmentRootDir(), fmt.Sprintf("%d", conversation.Id))

	fiveUploads := make([]supportAttachmentTestUpload, 0, 5)
	for i := 0; i < 5; i++ {
		fiveUploads = append(fiveUploads, supportAttachmentTestUpload{
			Name: fmt.Sprintf("image-%d.png", i),
			Data: supportAttachmentTestPNG(t, 10, 10),
		})
	}
	body, contentType := buildSupportAttachmentMultipartBody(t, "too many", fiveUploads)
	ctx, recorder := newSupportAttachmentTestContext(
		t, http.MethodPost, fmt.Sprintf("/api/support/conversation/%d/messages", conversation.Id),
		body.Bytes(), contentType, 1002, common.RoleCommonUser, supportAttachmentIDParam(conversation.Id),
	)
	SendSupportConversationMessage(ctx)
	response := decodeSupportAttachmentResponse(t, recorder)
	assert.False(t, response.Success)
	assert.Contains(t, response.Message, "4 张图片")
	assertNoSupportMessageRows(t, conversation.Id)
	_, statErr := os.Stat(conversationDir)
	assert.True(t, os.IsNotExist(statErr))

	invalidUpload := []supportAttachmentTestUpload{
		{Name: "valid.png", Data: supportAttachmentTestPNG(t, 12, 12)},
		{Name: "notes.txt", Data: []byte("not an image")},
	}
	body, contentType = buildSupportAttachmentMultipartBody(t, "invalid", invalidUpload)
	ctx, recorder = newSupportAttachmentTestContext(
		t, http.MethodPost, fmt.Sprintf("/api/support/conversation/%d/messages", conversation.Id),
		body.Bytes(), contentType, 1002, common.RoleCommonUser, supportAttachmentIDParam(conversation.Id),
	)
	SendSupportConversationMessage(ctx)
	response = decodeSupportAttachmentResponse(t, recorder)
	assert.False(t, response.Success)
	assert.Contains(t, response.Message, "仅支持 JPG、PNG、GIF、WebP 图片")
	assertNoSupportMessageRows(t, conversation.Id)
	// 首张合法图片写入后因后续文件失败必须被回滚删除。
	assertNoSupportAttachmentFiles(t, conversationDir)

	jsonBody, err := common.Marshal(map[string]any{"content": "   "})
	require.NoError(t, err)
	ctx, recorder = newSupportAttachmentTestContext(
		t, http.MethodPost, fmt.Sprintf("/api/support/conversation/%d/messages", conversation.Id),
		jsonBody, "application/json", 1002, common.RoleCommonUser, supportAttachmentIDParam(conversation.Id),
	)
	SendSupportConversationMessage(ctx)
	response = decodeSupportAttachmentResponse(t, recorder)
	assert.False(t, response.Success)
	assert.Equal(t, "消息内容不能为空", response.Message)

	tooLong := strings.Repeat("中", supportMessageMaxRunes+1)
	jsonBody, err = common.Marshal(map[string]any{"content": tooLong})
	require.NoError(t, err)
	ctx, recorder = newSupportAttachmentTestContext(
		t, http.MethodPost, fmt.Sprintf("/api/support/conversation/%d/messages", conversation.Id),
		jsonBody, "application/json", 1002, common.RoleCommonUser, supportAttachmentIDParam(conversation.Id),
	)
	SendSupportConversationMessage(ctx)
	response = decodeSupportAttachmentResponse(t, recorder)
	assert.False(t, response.Success)
	assert.Contains(t, response.Message, "2000")
}

func assertNoSupportAttachmentFiles(t *testing.T, dir string) {
	t.Helper()
	walkErr := filepath.WalkDir(dir, func(currentPath string, entry os.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if !entry.IsDir() {
			t.Errorf("unexpected support attachment file left after rollback: %s", currentPath)
		}
		return nil
	})
	if walkErr != nil && !os.IsNotExist(walkErr) {
		require.NoError(t, walkErr)
	}
}

func assertNoSupportMessageRows(t *testing.T, conversationId int) {
	t.Helper()
	var messageCount int64
	require.NoError(t, model.DB.Model(&model.SupportMessage{}).Where("conversation_id = ?", conversationId).Count(&messageCount).Error)
	assert.Zero(t, messageCount)
	var attachmentCount int64
	require.NoError(t, model.DB.Model(&model.SupportMessageAttachment{}).Where("conversation_id = ?", conversationId).Count(&attachmentCount).Error)
	assert.Zero(t, attachmentCount)
}

func TestAdminSendSupportMessageWithMultipartImage(t *testing.T) {
	setupSupportAttachmentControllerTestDB(t)
	setupSupportAttachmentControllerTestDir(t)

	conversation, err := model.GetOrCreateSupportConversation(1005)
	require.NoError(t, err)
	uploads := []supportAttachmentTestUpload{{Name: "reply.png", Data: supportAttachmentTestPNG(t, 70, 30)}}
	body, contentType := buildSupportAttachmentMultipartBody(t, "admin reply", uploads)
	ctx, recorder := newSupportAttachmentTestContext(
		t, http.MethodPost, fmt.Sprintf("/api/support/admin/conversations/%d/messages", conversation.Id),
		body.Bytes(), contentType, 3001, common.RoleSupportUser, supportAttachmentIDParam(conversation.Id),
	)
	AdminSendSupportMessage(ctx)

	response := decodeSupportAttachmentResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var data supportAttachmentMessageData
	require.NoError(t, common.Unmarshal(response.Data, &data))
	assert.Equal(t, "admin reply", data.Message.Content)
	assert.Equal(t, model.SupportSenderTypeAdmin, data.Message.SenderType)
	require.Len(t, data.Message.Attachments, 1)
	assert.Equal(t, "reply.png", data.Message.Attachments[0].FileName)

	updated, err := model.GetSupportConversationById(conversation.Id)
	require.NoError(t, err)
	assert.Equal(t, 3001, updated.AssignedAdminId)
	assert.Equal(t, 1, updated.UserUnread)
}

func TestGetSupportAttachmentAuthorizationAndVariants(t *testing.T) {
	setupSupportAttachmentControllerTestDB(t)
	setupSupportAttachmentControllerTestDir(t)

	conversation, err := model.GetOrCreateSupportConversation(1003)
	require.NoError(t, err)
	uploads := []supportAttachmentTestUpload{{Name: "large.png", Data: supportAttachmentTestPNG(t, 1200, 800)}}
	body, contentType := buildSupportAttachmentMultipartBody(t, "image", uploads)
	ctx, recorder := newSupportAttachmentTestContext(
		t, http.MethodPost, fmt.Sprintf("/api/support/conversation/%d/messages", conversation.Id),
		body.Bytes(), contentType, 1003, common.RoleCommonUser, supportAttachmentIDParam(conversation.Id),
	)
	SendSupportConversationMessage(ctx)
	response := decodeSupportAttachmentResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var data supportAttachmentMessageData
	require.NoError(t, common.Unmarshal(response.Data, &data))
	require.Len(t, data.Message.Attachments, 1)
	attachmentId := data.Message.Attachments[0].Id
	require.NotZero(t, attachmentId)

	// 本人 full 原图。
	ctx, recorder = newSupportAttachmentTestContext(
		t, http.MethodGet, fmt.Sprintf("/api/support/attachments/%d", attachmentId),
		nil, "", 1003, common.RoleCommonUser, supportAttachmentIDParam(attachmentId),
	)
	GetSupportAttachment(ctx)
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "image/png", recorder.Header().Get("Content-Type"))
	assert.Equal(t, "nosniff", recorder.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "private, max-age=86400", recorder.Header().Get("Cache-Control"))
	assert.NotEmpty(t, recorder.Body.Bytes())

	// 本人 thumb 缩略图。
	ctx, recorder = newSupportAttachmentTestContext(
		t, http.MethodGet, fmt.Sprintf("/api/support/attachments/%d?variant=thumb", attachmentId),
		nil, "", 1003, common.RoleCommonUser, supportAttachmentIDParam(attachmentId),
	)
	GetSupportAttachment(ctx)
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "image/jpeg", recorder.Header().Get("Content-Type"))
	thumbnailConfig, format, err := image.DecodeConfig(bytes.NewReader(recorder.Body.Bytes()))
	require.NoError(t, err)
	assert.Equal(t, "jpeg", format)
	assert.Equal(t, service.SupportAttachmentThumbnailMaxSide, thumbnailConfig.Width)

	// 其他普通用户无权限。
	ctx, recorder = newSupportAttachmentTestContext(
		t, http.MethodGet, fmt.Sprintf("/api/support/attachments/%d", attachmentId),
		nil, "", 1004, common.RoleCommonUser, supportAttachmentIDParam(attachmentId),
	)
	GetSupportAttachment(ctx)
	assert.Equal(t, http.StatusForbidden, recorder.Code)
	response = decodeSupportAttachmentResponse(t, recorder)
	assert.False(t, response.Success)

	// 客服角色可访问。
	ctx, recorder = newSupportAttachmentTestContext(
		t, http.MethodGet, fmt.Sprintf("/api/support/attachments/%d", attachmentId),
		nil, "", 3002, common.RoleSupportUser, supportAttachmentIDParam(attachmentId),
	)
	GetSupportAttachment(ctx)
	assert.Equal(t, http.StatusOK, recorder.Code)

	// 管理员角色可访问。
	ctx, recorder = newSupportAttachmentTestContext(
		t, http.MethodGet, fmt.Sprintf("/api/support/attachments/%d", attachmentId),
		nil, "", 3003, common.RoleAdminUser, supportAttachmentIDParam(attachmentId),
	)
	GetSupportAttachment(ctx)
	assert.Equal(t, http.StatusOK, recorder.Code)

	// 未登录。
	ctx, recorder = newSupportAttachmentTestContext(
		t, http.MethodGet, fmt.Sprintf("/api/support/attachments/%d", attachmentId),
		nil, "", 0, 0, supportAttachmentIDParam(attachmentId),
	)
	GetSupportAttachment(ctx)
	assert.Equal(t, http.StatusUnauthorized, recorder.Code)

	// 记录不存在。
	ctx, recorder = newSupportAttachmentTestContext(
		t, http.MethodGet, "/api/support/attachments/999999",
		nil, "", 1003, common.RoleCommonUser, supportAttachmentIDParam(999999),
	)
	GetSupportAttachment(ctx)
	assert.Equal(t, http.StatusNotFound, recorder.Code)
	response = decodeSupportAttachmentResponse(t, recorder)
	assert.False(t, response.Success)
	assert.Equal(t, "图片已过期或不存在", response.Message)

	// 文件缺失。
	attachment, err := model.GetSupportAttachmentById(attachmentId)
	require.NoError(t, err)
	require.NotNil(t, attachment)
	require.NoError(t, service.RemoveSupportAttachmentThumbnail(service.StoredSupportAttachment{
		StorageKey:   attachment.StorageKey,
		ThumbnailKey: attachment.ThumbnailKey,
	}))
	require.NoError(t, service.RemoveSupportAttachmentFile(attachment.StorageKey))
	ctx, recorder = newSupportAttachmentTestContext(
		t, http.MethodGet, fmt.Sprintf("/api/support/attachments/%d", attachmentId),
		nil, "", 1003, common.RoleCommonUser, supportAttachmentIDParam(attachmentId),
	)
	GetSupportAttachment(ctx)
	assert.Equal(t, http.StatusNotFound, recorder.Code)
	response = decodeSupportAttachmentResponse(t, recorder)
	assert.False(t, response.Success)
	assert.Equal(t, "图片已过期或不存在", response.Message)
}

func TestSupportChatSettingsAdminAPI(t *testing.T) {
	setupSupportAttachmentControllerTestDB(t)
	setSupportChatTestRetentionDays(t, system_setting.DefaultSupportChatRetentionDays)

	// 客服可读取默认值。
	ctx, recorder := newSupportAttachmentTestContext(t, http.MethodGet, "/api/support/admin/settings", nil, "", 3004, common.RoleSupportUser, nil)
	GetSupportChatSettings(ctx)
	response := decodeSupportAttachmentResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var settings supportChatSettingsTestPayload
	require.NoError(t, common.Unmarshal(response.Data, &settings))
	assert.Equal(t, system_setting.DefaultSupportChatRetentionDays, settings.RetentionDays)

	// 普通用户无权限。
	ctx, recorder = newSupportAttachmentTestContext(t, http.MethodGet, "/api/support/admin/settings", nil, "", 1006, common.RoleCommonUser, nil)
	GetSupportChatSettings(ctx)
	assert.Equal(t, http.StatusForbidden, recorder.Code)
	response = decodeSupportAttachmentResponse(t, recorder)
	assert.False(t, response.Success)

	// 客服更新成功。
	putBody, err := common.Marshal(map[string]any{"retention_days": 90})
	require.NoError(t, err)
	ctx, recorder = newSupportAttachmentTestContext(t, http.MethodPut, "/api/support/admin/settings", putBody, "application/json", 3004, common.RoleSupportUser, nil)
	UpdateSupportChatSettings(ctx)
	response = decodeSupportAttachmentResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	require.NoError(t, common.Unmarshal(response.Data, &settings))
	assert.Equal(t, 90, settings.RetentionDays)
	assert.Equal(t, 90, system_setting.GetSupportChatSettings().RetentionDays)

	var option model.Option
	require.NoError(t, model.DB.First(&option, "key = ?", system_setting.SupportChatRetentionDaysOption).Error)
	assert.Equal(t, "90", option.Value)

	// 边界 3650 与 0 合法。
	for _, days := range []int{3650, 0} {
		putBody, err = common.Marshal(map[string]any{"retention_days": days})
		require.NoError(t, err)
		ctx, recorder = newSupportAttachmentTestContext(t, http.MethodPut, "/api/support/admin/settings", putBody, "application/json", 3005, common.RoleAdminUser, nil)
		UpdateSupportChatSettings(ctx)
		response = decodeSupportAttachmentResponse(t, recorder)
		require.True(t, response.Success, response.Message)
		require.NoError(t, common.Unmarshal(response.Data, &settings))
		assert.Equal(t, days, settings.RetentionDays)
	}

	// 非法取值。
	for _, body := range []map[string]any{
		{"retention_days": 3651},
		{"retention_days": -1},
		{},
	} {
		putBody, err = common.Marshal(body)
		require.NoError(t, err)
		ctx, recorder = newSupportAttachmentTestContext(t, http.MethodPut, "/api/support/admin/settings", putBody, "application/json", 3005, common.RoleAdminUser, nil)
		UpdateSupportChatSettings(ctx)
		response = decodeSupportAttachmentResponse(t, recorder)
		assert.False(t, response.Success)
		assert.NotEmpty(t, response.Message)
		if value, ok := body["retention_days"]; ok && value.(int) > 3650 {
			assert.Contains(t, response.Message, "3650")
		}
	}

	// 非法 JSON。
	ctx, recorder = newSupportAttachmentTestContext(t, http.MethodPut, "/api/support/admin/settings", []byte("{bad"), "application/json", 3005, common.RoleAdminUser, nil)
	UpdateSupportChatSettings(ctx)
	response = decodeSupportAttachmentResponse(t, recorder)
	assert.False(t, response.Success)
	assert.Contains(t, response.Message, "格式不正确")

	// root 可读取。
	ctx, recorder = newSupportAttachmentTestContext(t, http.MethodGet, "/api/support/admin/settings", nil, "", 3006, common.RoleRootUser, nil)
	GetSupportChatSettings(ctx)
	response = decodeSupportAttachmentResponse(t, recorder)
	require.True(t, response.Success, response.Message)
}

func TestSupportChatAppearanceSettings(t *testing.T) {
	setupSupportAttachmentControllerTestDB(t)

	cfg, ok := config.GlobalConfig.Get("support_chat").(*system_setting.SupportChatSettings)
	require.True(t, ok)
	previous := *cfg
	*cfg = system_setting.SupportChatSettings{
		RetentionDays: system_setting.DefaultSupportChatRetentionDays,
		UIScale:       100,
		TextScale:     100,
		Prominence:    100,
		BottomOffset:  96,
	}
	t.Cleanup(func() { *cfg = previous })

	// 公开外观接口无需登录即可访问。
	ctx, recorder := newSupportAttachmentTestContext(t, http.MethodGet, "/api/support/appearance", nil, "", 0, common.RoleGuestUser, nil)
	GetSupportChatAppearance(ctx)
	response := decodeSupportAttachmentResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var appearance supportChatSettingsTestPayload
	require.NoError(t, common.Unmarshal(response.Data, &appearance))
	assert.Equal(t, 100, appearance.UIScale)
	assert.Equal(t, 100, appearance.TextScale)
	assert.Equal(t, 100, appearance.Prominence)
	assert.Equal(t, 96, appearance.BottomOffset)
	assert.Equal(t, "", appearance.CustomTitle)

	// 客服保存外观设置。
	body, err := common.Marshal(map[string]any{
		"ui_scale":         130,
		"text_scale":       120,
		"prominence":       0,
		"custom_title":     "专属客服",
		"custom_sub_login": "有问题随时联系我",
		"custom_sub_guest": "登录后联系我们",
		"bottom_offset":    120,
	})
	require.NoError(t, err)
	ctx, recorder = newSupportAttachmentTestContext(t, http.MethodPut, "/api/support/admin/settings", body, "application/json", 3004, common.RoleSupportUser, nil)
	UpdateSupportChatSettings(ctx)
	response = decodeSupportAttachmentResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	require.NoError(t, common.Unmarshal(response.Data, &appearance))
	assert.Equal(t, 130, appearance.UIScale)
	assert.Equal(t, 120, appearance.TextScale)
	assert.Equal(t, 0, appearance.Prominence)
	assert.Equal(t, "专属客服", appearance.CustomTitle)
	assert.Equal(t, 120, appearance.BottomOffset)
	assert.Equal(t, "有问题随时联系我", appearance.CustomSubLogin)
	assert.Equal(t, "登录后联系我们", appearance.CustomSubGuest)
	assert.Equal(t, 120, appearance.BottomOffset)

	var scaleOption model.Option
	require.NoError(t, model.DB.First(&scaleOption, "key = ?", system_setting.SupportChatUIScaleOption).Error)
	assert.Equal(t, "130", scaleOption.Value)
	var prominenceOption model.Option
	require.NoError(t, model.DB.First(&prominenceOption, "key = ?", system_setting.SupportChatProminenceOption).Error)
	assert.Equal(t, "0", prominenceOption.Value)

	ctx, recorder = newSupportAttachmentTestContext(t, http.MethodGet, "/api/support/appearance", nil, "", 0, common.RoleGuestUser, nil)
	GetSupportChatAppearance(ctx)
	response = decodeSupportAttachmentResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	require.NoError(t, common.Unmarshal(response.Data, &appearance))
	assert.Equal(t, 130, appearance.UIScale)
	assert.Equal(t, 0, appearance.Prominence)
	assert.Equal(t, "专属客服", appearance.CustomTitle)

	var titleOption model.Option
	require.NoError(t, model.DB.First(&titleOption, "key = ?", system_setting.SupportChatCustomTitleOption).Error)
	assert.Equal(t, "专属客服", titleOption.Value)

	// 越界值拒绝。
	for _, payload := range []map[string]any{
		{"ui_scale": 70},
		{"ui_scale": 170},
		{"text_scale": 70},
		{"text_scale": 170},
		{"prominence": -5},
		{"prominence": 105},
		{"bottom_offset": 8},
		{"bottom_offset": 300},
		{"custom_title": "12345678901234567890123456789012345678901"},
	} {
		body, err = common.Marshal(payload)
		require.NoError(t, err)
		ctx, recorder = newSupportAttachmentTestContext(t, http.MethodPut, "/api/support/admin/settings", body, "application/json", 3005, common.RoleAdminUser, nil)
		UpdateSupportChatSettings(ctx)
		response = decodeSupportAttachmentResponse(t, recorder)
		assert.False(t, response.Success)
		assert.NotEmpty(t, response.Message)
	}
}

func TestGetCurrentSupportConversationCanIncludeLatestClosed(t *testing.T) {
	db := setupSupportAttachmentControllerTestDB(t)

	closed := model.SupportConversation{
		UserId:    4100,
		Status:    model.SupportConversationStatusClosed,
		CreatedAt: 100,
		UpdatedAt: 200,
		ClosedAt:  300,
	}
	require.NoError(t, db.Create(&closed).Error)
	open := model.SupportConversation{
		UserId:    4100,
		Status:    model.SupportConversationStatusOpen,
		CreatedAt: 400,
		UpdatedAt: 500,
	}
	require.NoError(t, db.Create(&open).Error)
	latestClosed := model.SupportConversation{
		UserId:    4100,
		Status:    model.SupportConversationStatusClosed,
		CreatedAt: 600,
		UpdatedAt: 700,
		ClosedAt:  800,
	}
	require.NoError(t, db.Create(&latestClosed).Error)

	ctx, recorder := newSupportAttachmentTestContext(
		t,
		http.MethodGet,
		"/api/support/conversation/current?include_closed=1",
		nil,
		"",
		4100,
		common.RoleCommonUser,
		nil,
	)
	GetCurrentSupportConversation(ctx)
	response := decodeSupportAttachmentResponse(t, recorder)
	require.True(t, response.Success, response.Message)

	var conversation model.SupportConversation
	require.NoError(t, common.Unmarshal(response.Data, &conversation))
	assert.Equal(t, latestClosed.Id, conversation.Id)
	assert.Equal(t, model.SupportConversationStatusClosed, conversation.Status)

	ctx, recorder = newSupportAttachmentTestContext(
		t,
		http.MethodGet,
		"/api/support/conversation/current",
		nil,
		"",
		4100,
		common.RoleCommonUser,
		nil,
	)
	GetCurrentSupportConversation(ctx)
	response = decodeSupportAttachmentResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	require.NoError(t, common.Unmarshal(response.Data, &conversation))
	assert.Equal(t, open.Id, conversation.Id)
	assert.Equal(t, model.SupportConversationStatusOpen, conversation.Status)
}

func TestListAdminSupportConversationsDefaultsToAllStatuses(t *testing.T) {
	db := setupSupportAttachmentControllerTestDB(t)

	open := model.SupportConversation{
		UserId:    4200,
		Status:    model.SupportConversationStatusOpen,
		CreatedAt: 100,
		UpdatedAt: 200,
	}
	require.NoError(t, db.Create(&open).Error)
	closed := model.SupportConversation{
		UserId:    4200,
		Status:    model.SupportConversationStatusClosed,
		CreatedAt: 300,
		UpdatedAt: 400,
		ClosedAt:  500,
	}
	require.NoError(t, db.Create(&closed).Error)

	ctx, recorder := newSupportAttachmentTestContext(
		t,
		http.MethodGet,
		"/api/support/admin/conversations",
		nil,
		"",
		1,
		common.RoleAdminUser,
		nil,
	)
	ListAdminSupportConversations(ctx)
	response := decodeSupportAttachmentResponse(t, recorder)
	require.True(t, response.Success, response.Message)

	var data struct {
		Items []model.SupportConversation `json:"items"`
		Total int                         `json:"total"`
	}
	require.NoError(t, common.Unmarshal(response.Data, &data))
	assert.Equal(t, 2, data.Total)
	assert.Len(t, data.Items, 2)
}
