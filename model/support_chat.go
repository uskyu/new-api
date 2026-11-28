package model

import (
	"errors"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	SupportConversationStatusOpen   = "open"
	SupportConversationStatusClosed = "closed"
)

const (
	SupportSenderTypeUser  = "user"
	SupportSenderTypeAdmin = "admin"
)

const (
	supportMessagePreviewMaxRunes = 200
	supportMessageListMaxLimit    = 100
	supportMessageListDefault     = 50
)

var (
	supportConversationCreateMu = sync.Mutex{}

	// ErrSupportConversationNotFound 会话不存在。
	ErrSupportConversationNotFound = errors.New("support conversation not found")
)

// SupportConversation 客服会话
type SupportConversation struct {
	Id                 int    `json:"id" gorm:"primaryKey;autoIncrement"`
	UserId             int    `json:"user_id" gorm:"not null;index;index:idx_support_conversations_user_status,priority:1"`
	Status             string `json:"status" gorm:"type:varchar(20);not null;index;index:idx_support_conversations_status_last_message_at,priority:1;index:idx_support_conversations_user_status,priority:2"`
	AssignedAdminId    int    `json:"assigned_admin_id" gorm:"not null;default:0;index"`
	LastMessageAt      int64  `json:"last_message_at" gorm:"type:bigint;not null;default:0;index;index:idx_support_conversations_status_last_message_at,priority:2"`
	LastMessagePreview string `json:"last_message_preview" gorm:"type:varchar(200);not null;default:''"`
	UserUnread         int    `json:"user_unread" gorm:"not null;default:0"`
	AdminUnread        int    `json:"admin_unread" gorm:"not null;default:0"`
	UserLastReadId     int    `json:"user_last_read_id" gorm:"not null;default:0"`
	AdminLastReadId    int    `json:"admin_last_read_id" gorm:"not null;default:0"`
	CreatedAt          int64  `json:"created_at" gorm:"type:bigint;not null;default:0;autoCreateTime:false"`
	UpdatedAt          int64  `json:"updated_at" gorm:"type:bigint;not null;default:0;autoUpdateTime:false"`
	ClosedAt           int64  `json:"closed_at" gorm:"type:bigint;not null;default:0"`
}

// SupportMessage 客服会话消息
type SupportMessage struct {
	Id             int    `json:"id" gorm:"primaryKey;autoIncrement;index:idx_support_messages_conversation_id_id,priority:2"`
	ConversationId int    `json:"conversation_id" gorm:"not null;index:idx_support_messages_conversation_id_id,priority:1"`
	SenderType     string `json:"sender_type" gorm:"type:varchar(10);not null"`
	SenderId       int    `json:"sender_id" gorm:"not null;default:0"`
	SenderName     string `json:"sender_name" gorm:"type:varchar(64);not null;default:''"`
	Content        string `json:"content" gorm:"type:text;not null"`
	CreatedAt      int64  `json:"created_at" gorm:"type:bigint;not null;default:0;autoCreateTime:false"`

	// Attachments 由 LoadSupportMessageAttachments 或创建消息时填充，不参与 GORM 关联。
	Attachments []SupportMessageAttachment `json:"attachments" gorm:"-"`
}

// GetOpenSupportConversationByUserId 获取用户当前进行中的会话，没有则返回 nil。
func GetOpenSupportConversationByUserId(userId int) (*SupportConversation, error) {
	if userId <= 0 {
		return nil, errors.New("invalid user id")
	}
	var conversation SupportConversation
	err := DB.Where("user_id = ? AND status = ?", userId, SupportConversationStatusOpen).
		Order("id DESC").
		First(&conversation).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &conversation, nil
}

// GetLatestSupportConversationByUserId 获取用户最近一段会话（包含已关闭会话）。
// 客户重新打开客服面板时用它恢复上一段历史记录。
func GetLatestSupportConversationByUserId(userId int) (*SupportConversation, error) {
	if userId <= 0 {
		return nil, errors.New("invalid user id")
	}
	var conversation SupportConversation
	err := DB.Where("user_id = ?", userId).
		Order("id DESC").
		First(&conversation).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &conversation, nil
}

// GetOrCreateSupportConversation 获取或创建用户当前进行中的会话。
// 同一用户只保留一个 open 会话，通过事务和进程内互斥锁保证并发安全。
func GetOrCreateSupportConversation(userId int) (*SupportConversation, error) {
	if userId <= 0 {
		return nil, errors.New("invalid user id")
	}

	supportConversationCreateMu.Lock()
	defer supportConversationCreateMu.Unlock()

	var conversation SupportConversation
	err := DB.Transaction(func(tx *gorm.DB) error {
		err := lockForUpdate(tx).
			Where("user_id = ? AND status = ?", userId, SupportConversationStatusOpen).
			Order("id DESC").
			First(&conversation).Error
		if err == nil {
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		now := common.GetTimestamp()
		conversation = SupportConversation{
			UserId:    userId,
			Status:    SupportConversationStatusOpen,
			CreatedAt: now,
			UpdatedAt: now,
		}
		return tx.Create(&conversation).Error
	})
	if err != nil {
		// 多实例场景下可能由其他实例先创建成功，回查一次避免误报。
		if existing, findErr := GetOpenSupportConversationByUserId(userId); findErr == nil && existing != nil {
			return existing, nil
		}
		return nil, err
	}
	return &conversation, nil
}

// GetSupportConversationById 按 id 获取会话，不存在返回 nil。
func GetSupportConversationById(id int) (*SupportConversation, error) {
	if id <= 0 {
		return nil, errors.New("invalid conversation id")
	}
	var conversation SupportConversation
	err := DB.First(&conversation, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &conversation, nil
}

// ListSupportMessages 拉取会话消息。
// afterId=0 时返回最近 limit 条，结果始终按 id 升序；limit 最大 100。
func ListSupportMessages(conversationId int, afterId int, limit int) ([]SupportMessage, error) {
	if conversationId <= 0 {
		return nil, errors.New("invalid conversation id")
	}
	if limit <= 0 {
		limit = supportMessageListDefault
	}
	if limit > supportMessageListMaxLimit {
		limit = supportMessageListMaxLimit
	}

	messages := make([]SupportMessage, 0, limit)
	if afterId > 0 {
		err := DB.Where("conversation_id = ? AND id > ?", conversationId, afterId).
			Order("id ASC").
			Limit(limit).
			Find(&messages).Error
		return messages, err
	}

	err := DB.Where("conversation_id = ?", conversationId).
		Order("id DESC").
		Limit(limit).
		Find(&messages).Error
	if err != nil {
		return nil, err
	}
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}
	return messages, nil
}

// CreateSupportMessage 创建消息并同步更新会话的未读数、预览与最后消息时间。
// 无附件场景的包装函数，保留旧调用行为。
func CreateSupportMessage(message *SupportMessage) error {
	return CreateSupportMessageWithAttachments(message, nil)
}

// MarkSupportConversationRead 标记会话已读。
// readerType 为 user 时清 user_unread，为 admin 时清 admin_unread；
// lastMessageId <= 0 时使用会话当前最大消息 id。
func MarkSupportConversationRead(conversationId int, readerType string, lastMessageId int) error {
	if conversationId <= 0 {
		return errors.New("invalid conversation id")
	}
	if readerType != SupportSenderTypeUser && readerType != SupportSenderTypeAdmin {
		return errors.New("invalid reader type")
	}
	var conversation SupportConversation
	if err := DB.First(&conversation, "id = ?", conversationId).Error; err != nil {
		return err
	}

	if lastMessageId <= 0 {
		var maxMessageId int64
		if err := DB.Model(&SupportMessage{}).
			Where("conversation_id = ?", conversationId).
			Select("COALESCE(MAX(id), 0)").
			Scan(&maxMessageId).Error; err != nil {
			return err
		}
		lastMessageId = int(maxMessageId)
	}

	updates := map[string]interface{}{}
	if readerType == SupportSenderTypeUser {
		updates["user_unread"] = 0
		updates["user_last_read_id"] = lastMessageId
	} else {
		updates["admin_unread"] = 0
		updates["admin_last_read_id"] = lastMessageId
	}
	return DB.Model(&SupportConversation{}).
		Where("id = ?", conversationId).
		Updates(updates).Error
}

// ClaimSupportConversation 认领会话，仅 open 且未分配时可成功。
func ClaimSupportConversation(conversationId int, adminId int) (bool, error) {
	if conversationId <= 0 || adminId <= 0 {
		return false, errors.New("invalid claim params")
	}
	result := DB.Model(&SupportConversation{}).
		Where("id = ? AND status = ? AND assigned_admin_id = 0", conversationId, SupportConversationStatusOpen).
		Updates(map[string]interface{}{
			"assigned_admin_id": adminId,
			"updated_at":        common.GetTimestamp(),
		})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

// CloseSupportConversation 关闭会话，重复关闭幂等。
func CloseSupportConversation(conversationId int) error {
	if conversationId <= 0 {
		return errors.New("invalid conversation id")
	}
	now := common.GetTimestamp()
	result := DB.Model(&SupportConversation{}).
		Where("id = ? AND status <> ?", conversationId, SupportConversationStatusClosed).
		Updates(map[string]interface{}{
			"status":     SupportConversationStatusClosed,
			"closed_at":  now,
			"updated_at": now,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		return nil
	}

	var count int64
	if err := DB.Model(&SupportConversation{}).Where("id = ?", conversationId).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return ErrSupportConversationNotFound
	}
	return nil
}

// ListSupportConversations 管理端分页列出会话，按最后消息时间倒序。
func ListSupportConversations(status string, offset int, limit int) ([]SupportConversation, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	query := DB.Model(&SupportConversation{})
	if status != "" {
		query = query.Where("status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	conversations := make([]SupportConversation, 0, limit)
	err := query.Order("last_message_at DESC, id DESC").
		Limit(limit).
		Offset(offset).
		Find(&conversations).Error
	if err != nil {
		return nil, 0, err
	}
	return conversations, total, nil
}

// CountSupportUnreadForAgent 统计待客服处理的会话数。
// 客服共享队列口径：所有 open 且 admin_unread > 0 的会话。
func CountSupportUnreadForAgent(adminId int) (int64, error) {
	_ = adminId
	var count int64
	err := DB.Model(&SupportConversation{}).
		Where("status = ? AND admin_unread > 0", SupportConversationStatusOpen).
		Count(&count).Error
	return count, err
}

// buildSupportMessagePreview 生成会话预览，按字符截断到 200 个字符，兼容中文。
func buildSupportMessagePreview(content string) string {
	preview := strings.TrimSpace(content)
	preview = strings.ReplaceAll(preview, "\r\n", " ")
	preview = strings.ReplaceAll(preview, "\n", " ")
	preview = strings.ReplaceAll(preview, "\r", " ")
	preview = strings.TrimSpace(preview)

	runes := []rune(preview)
	if len(runes) > supportMessagePreviewMaxRunes {
		preview = string(runes[:supportMessagePreviewMaxRunes])
	}
	return preview
}
