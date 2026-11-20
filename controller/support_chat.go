package controller

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

const (
	supportMessageMaxRunes             = 2000
	supportConversationDefaultPageSize = 20
	supportConversationMaxPageSize     = 100
)

type supportAdminConversationItem struct {
	model.SupportConversation
	Username          string `json:"username"`
	DisplayName       string `json:"display_name"`
	AssignedAdminName string `json:"assigned_admin_name"`
}

type supportMessageRequest struct {
	Content string `json:"content"`
}

type supportReadRequest struct {
	LastMessageId int `json:"last_message_id"`
}

func supportChatSuccess(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    data,
	})
}

func parseSupportConversationId(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorI18n(c, i18n.MsgInvalidId)
		return 0, false
	}
	return id, true
}

func getSupportConversationForUser(c *gin.Context, conversationId int, userId int) (*model.SupportConversation, bool) {
	conversation, err := model.GetSupportConversationById(conversationId)
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return nil, false
	}
	if conversation == nil {
		common.ApiErrorI18n(c, i18n.MsgNotFound)
		return nil, false
	}
	if conversation.UserId != userId {
		common.ApiErrorI18n(c, i18n.MsgForbidden)
		return nil, false
	}
	return conversation, true
}

func getSupportConversationForAdmin(c *gin.Context, conversationId int) (*model.SupportConversation, bool) {
	conversation, err := model.GetSupportConversationById(conversationId)
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return nil, false
	}
	if conversation == nil {
		common.ApiErrorI18n(c, i18n.MsgNotFound)
		return nil, false
	}
	return conversation, true
}

func parseSupportMessageContent(c *gin.Context) (string, bool) {
	var request supportMessageRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return "", false
	}
	content := strings.TrimSpace(request.Content)
	if content == "" {
		common.ApiErrorMsg(c, "消息内容不能为空")
		return "", false
	}
	if utf8.RuneCountInString(content) > supportMessageMaxRunes {
		common.ApiErrorMsg(c, "消息内容不能超过 2000 个字符")
		return "", false
	}
	return content, true
}

func parseSupportReadRequest(c *gin.Context) (int, bool) {
	var request supportReadRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return 0, false
	}
	if request.LastMessageId < 0 {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return 0, false
	}
	return request.LastMessageId, true
}

func parseSupportMessageQuery(c *gin.Context) (int, int) {
	afterId, err := strconv.Atoi(c.DefaultQuery("after_id", "0"))
	if err != nil || afterId < 0 {
		afterId = 0
	}
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if err != nil || limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	return afterId, limit
}

// GetCurrentSupportConversation 获取当前用户进行中的客服会话，没有则返回 null。
func GetCurrentSupportConversation(c *gin.Context) {
	conversation, err := model.GetOpenSupportConversationByUserId(c.GetInt("id"))
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	supportChatSuccess(c, conversation)
}

// CreateSupportConversation 创建或获取当前用户的客服会话。
func CreateSupportConversation(c *gin.Context) {
	conversation, err := model.GetOrCreateSupportConversation(c.GetInt("id"))
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	supportChatSuccess(c, conversation)
}

// GetSupportConversationMessages 用户拉取自己的会话消息。
func GetSupportConversationMessages(c *gin.Context) {
	conversationId, ok := parseSupportConversationId(c)
	if !ok {
		return
	}
	userId := c.GetInt("id")
	if _, ok = getSupportConversationForUser(c, conversationId, userId); !ok {
		return
	}

	afterId, limit := parseSupportMessageQuery(c)
	messages, err := model.ListSupportMessages(conversationId, afterId, limit)
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	conversation, err := model.GetSupportConversationById(conversationId)
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	supportChatSuccess(c, gin.H{
		"messages":     messages,
		"conversation": conversation,
	})
}

// SendSupportConversationMessage 用户发送消息。
func SendSupportConversationMessage(c *gin.Context) {
	conversationId, ok := parseSupportConversationId(c)
	if !ok {
		return
	}
	userId := c.GetInt("id")
	conversation, ok := getSupportConversationForUser(c, conversationId, userId)
	if !ok {
		return
	}
	if conversation.Status != model.SupportConversationStatusOpen {
		common.ApiErrorMsg(c, "会话已关闭")
		return
	}

	content, ok := parseSupportMessageContent(c)
	if !ok {
		return
	}
	message := &model.SupportMessage{
		ConversationId: conversationId,
		SenderType:     model.SupportSenderTypeUser,
		SenderId:       userId,
		SenderName:     c.GetString("username"),
		Content:        content,
	}
	if err := model.CreateSupportMessage(message); err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	conversation, err := model.GetSupportConversationById(conversationId)
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	supportChatSuccess(c, gin.H{
		"message":      message,
		"conversation": conversation,
	})
}

// MarkSupportConversationRead 用户标记会话已读。
func MarkSupportConversationRead(c *gin.Context) {
	conversationId, ok := parseSupportConversationId(c)
	if !ok {
		return
	}
	userId := c.GetInt("id")
	if _, ok = getSupportConversationForUser(c, conversationId, userId); !ok {
		return
	}
	lastMessageId, ok := parseSupportReadRequest(c)
	if !ok {
		return
	}
	if err := model.MarkSupportConversationRead(conversationId, model.SupportSenderTypeUser, lastMessageId); err != nil {
		common.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	conversation, err := model.GetSupportConversationById(conversationId)
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	supportChatSuccess(c, gin.H{"conversation": conversation})
}

// CloseSupportConversation 用户关闭自己的会话。
func CloseSupportConversation(c *gin.Context) {
	conversationId, ok := parseSupportConversationId(c)
	if !ok {
		return
	}
	userId := c.GetInt("id")
	if _, ok = getSupportConversationForUser(c, conversationId, userId); !ok {
		return
	}
	if err := model.CloseSupportConversation(conversationId); err != nil {
		common.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	conversation, err := model.GetSupportConversationById(conversationId)
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	supportChatSuccess(c, gin.H{"conversation": conversation})
}

// ListAdminSupportConversations 管理端分页列出客服会话。
func ListAdminSupportConversations(c *gin.Context) {
	status := c.DefaultQuery("status", model.SupportConversationStatusOpen)
	if status != model.SupportConversationStatusOpen && status != model.SupportConversationStatusClosed {
		status = ""
	}

	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page <= 0 {
		page = 1
	}
	pageSize, err := strconv.Atoi(c.DefaultQuery("page_size", strconv.Itoa(supportConversationDefaultPageSize)))
	if err != nil || pageSize <= 0 {
		pageSize = supportConversationDefaultPageSize
	}
	if pageSize > supportConversationMaxPageSize {
		pageSize = supportConversationMaxPageSize
	}

	conversations, total, err := model.ListSupportConversations(status, (page-1)*pageSize, pageSize)
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}

	items := make([]supportAdminConversationItem, 0, len(conversations))
	for _, conversation := range conversations {
		item := supportAdminConversationItem{SupportConversation: conversation}
		if user, userErr := model.GetUserById(conversation.UserId, false); userErr == nil && user != nil {
			item.Username = user.Username
			item.DisplayName = user.DisplayName
		}
		if conversation.AssignedAdminId > 0 {
			if admin, adminErr := model.GetUserById(conversation.AssignedAdminId, false); adminErr == nil && admin != nil {
				item.AssignedAdminName = admin.Username
			}
		}
		items = append(items, item)
	}

	supportChatSuccess(c, gin.H{
		"items":     items,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// AdminGetSupportConversationMessages 管理端拉取会话消息。
func AdminGetSupportConversationMessages(c *gin.Context) {
	conversationId, ok := parseSupportConversationId(c)
	if !ok {
		return
	}
	if _, ok = getSupportConversationForAdmin(c, conversationId); !ok {
		return
	}
	afterId, limit := parseSupportMessageQuery(c)
	messages, err := model.ListSupportMessages(conversationId, afterId, limit)
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	conversation, err := model.GetSupportConversationById(conversationId)
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	supportChatSuccess(c, gin.H{
		"messages":     messages,
		"conversation": conversation,
	})
}

// AdminSendSupportMessage 客服发送消息，未分配时会自动认领。
func AdminSendSupportMessage(c *gin.Context) {
	conversationId, ok := parseSupportConversationId(c)
	if !ok {
		return
	}
	conversation, ok := getSupportConversationForAdmin(c, conversationId)
	if !ok {
		return
	}
	if conversation.Status != model.SupportConversationStatusOpen {
		common.ApiErrorMsg(c, "会话已关闭")
		return
	}

	content, ok := parseSupportMessageContent(c)
	if !ok {
		return
	}
	message := &model.SupportMessage{
		ConversationId: conversationId,
		SenderType:     model.SupportSenderTypeAdmin,
		SenderId:       c.GetInt("id"),
		SenderName:     c.GetString("username"),
		Content:        content,
	}
	if err := model.CreateSupportMessage(message); err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	conversation, err := model.GetSupportConversationById(conversationId)
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	supportChatSuccess(c, gin.H{
		"message":      message,
		"conversation": conversation,
	})
}

// AdminClaimSupportConversation 客服认领会话。
func AdminClaimSupportConversation(c *gin.Context) {
	conversationId, ok := parseSupportConversationId(c)
	if !ok {
		return
	}
	adminId := c.GetInt("id")
	conversation, ok := getSupportConversationForAdmin(c, conversationId)
	if !ok {
		return
	}

	claimed, err := model.ClaimSupportConversation(conversationId, adminId)
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	conversation, err = model.GetSupportConversationById(conversationId)
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	if conversation == nil {
		common.ApiErrorI18n(c, i18n.MsgNotFound)
		return
	}
	if !claimed && conversation.AssignedAdminId != adminId {
		if conversation.Status != model.SupportConversationStatusOpen {
			common.ApiErrorMsg(c, "会话已关闭")
		} else {
			common.ApiErrorMsg(c, "会话已被其他客服认领")
		}
		return
	}
	supportChatSuccess(c, gin.H{"conversation": conversation})
}

// AdminCloseSupportConversation 管理端关闭会话。
func AdminCloseSupportConversation(c *gin.Context) {
	conversationId, ok := parseSupportConversationId(c)
	if !ok {
		return
	}
	if _, ok = getSupportConversationForAdmin(c, conversationId); !ok {
		return
	}
	if err := model.CloseSupportConversation(conversationId); err != nil {
		if errors.Is(err, model.ErrSupportConversationNotFound) {
			common.ApiErrorI18n(c, i18n.MsgNotFound)
			return
		}
		common.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	conversation, err := model.GetSupportConversationById(conversationId)
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	supportChatSuccess(c, gin.H{"conversation": conversation})
}

// AdminMarkSupportConversationRead 客服标记会话已读。
func AdminMarkSupportConversationRead(c *gin.Context) {
	conversationId, ok := parseSupportConversationId(c)
	if !ok {
		return
	}
	if _, ok = getSupportConversationForAdmin(c, conversationId); !ok {
		return
	}
	lastMessageId, ok := parseSupportReadRequest(c)
	if !ok {
		return
	}
	if err := model.MarkSupportConversationRead(conversationId, model.SupportSenderTypeAdmin, lastMessageId); err != nil {
		common.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	conversation, err := model.GetSupportConversationById(conversationId)
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	supportChatSuccess(c, gin.H{"conversation": conversation})
}

// AdminSupportUnreadCount 管理端获取待处理会话数量。
func AdminSupportUnreadCount(c *gin.Context) {
	count, err := model.CountSupportUnreadForAgent(c.GetInt("id"))
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	supportChatSuccess(c, gin.H{"count": count})
}
