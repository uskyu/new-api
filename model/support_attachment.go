package model

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	supportClosedConversationCleanupLimit = 100
	supportClosedConversationCleanupMax   = 200
)

// SupportMessageAttachment 客服消息图片附件。
type SupportMessageAttachment struct {
	Id                int    `json:"id" gorm:"primaryKey;autoIncrement"`
	MessageId         int    `json:"message_id" gorm:"not null;index"`
	ConversationId    int    `json:"conversation_id" gorm:"not null;index"`
	FileName          string `json:"file_name" gorm:"type:varchar(255);not null;default:''"`
	StorageKey        string `json:"-" gorm:"type:varchar(500);not null;default:''"`
	ThumbnailKey      string `json:"-" gorm:"type:varchar(500);not null;default:''"`
	ThumbnailMimeType string `json:"thumbnail_mime_type" gorm:"type:varchar(100);not null;default:''"`
	MimeType          string `json:"mime_type" gorm:"type:varchar(100);not null;default:''"`
	SizeBytes         int64  `json:"size_bytes" gorm:"not null;default:0"`
	Width             int    `json:"width" gorm:"not null;default:0"`
	Height            int    `json:"height" gorm:"not null;default:0"`
	CreatedAt         int64  `json:"created_at" gorm:"type:bigint;not null;default:0;autoCreateTime:false"`
}

// CreateSupportMessageWithAttachments 在同一个事务中写入消息与附件，并更新会话状态。
// 纯图片消息 content 为空时，会话预览使用 "[图片]"。
func CreateSupportMessageWithAttachments(message *SupportMessage, attachments []SupportMessageAttachment) error {
	if message == nil {
		return errors.New("message is nil")
	}
	if message.ConversationId <= 0 {
		return errors.New("invalid conversation id")
	}
	switch message.SenderType {
	case SupportSenderTypeUser, SupportSenderTypeAdmin:
	default:
		return errors.New("invalid sender type")
	}

	err := DB.Transaction(func(tx *gorm.DB) error {
		var conversation SupportConversation
		if err := lockForUpdate(tx).
			Where("id = ?", message.ConversationId).
			First(&conversation).Error; err != nil {
			return err
		}
		if conversation.Status != SupportConversationStatusOpen {
			return errors.New("support conversation is closed")
		}

		now := common.GetTimestamp()
		message.CreatedAt = now
		if err := tx.Create(message).Error; err != nil {
			return err
		}

		for i := range attachments {
			attachments[i].Id = 0
			attachments[i].MessageId = message.Id
			attachments[i].ConversationId = message.ConversationId
			attachments[i].CreatedAt = now
			if err := tx.Create(&attachments[i]).Error; err != nil {
				return err
			}
		}

		preview := buildSupportMessagePreview(message.Content)
		if preview == "" && len(attachments) > 0 {
			preview = "[图片]"
		}

		updates := map[string]interface{}{
			"last_message_at":      now,
			"last_message_preview": preview,
			"updated_at":           now,
		}
		if message.SenderType == SupportSenderTypeUser {
			updates["admin_unread"] = gorm.Expr("admin_unread + ?", 1)
		} else {
			updates["user_unread"] = gorm.Expr("user_unread + ?", 1)
			if conversation.AssignedAdminId == 0 {
				updates["assigned_admin_id"] = message.SenderId
			}
		}

		return tx.Model(&SupportConversation{}).
			Where("id = ?", message.ConversationId).
			Updates(updates).Error
	})
	if err != nil {
		return err
	}

	if attachments == nil {
		message.Attachments = []SupportMessageAttachment{}
	} else {
		message.Attachments = attachments
	}
	return nil
}

// LoadSupportMessageAttachments 为一批消息批量填充附件，避免逐条查询。
func LoadSupportMessageAttachments(messages []SupportMessage) error {
	if len(messages) == 0 {
		return nil
	}

	messageIds := make([]int, 0, len(messages))
	for i := range messages {
		messages[i].Attachments = []SupportMessageAttachment{}
		if messages[i].Id > 0 {
			messageIds = append(messageIds, messages[i].Id)
		}
	}
	if len(messageIds) == 0 {
		return nil
	}

	var attachments []SupportMessageAttachment
	if err := DB.Where("message_id IN ?", messageIds).
		Order("id ASC").
		Find(&attachments).Error; err != nil {
		return err
	}

	byMessageId := make(map[int][]SupportMessageAttachment, len(messageIds))
	for _, attachment := range attachments {
		byMessageId[attachment.MessageId] = append(byMessageId[attachment.MessageId], attachment)
	}
	for i := range messages {
		if grouped, ok := byMessageId[messages[i].Id]; ok {
			messages[i].Attachments = grouped
		}
	}
	return nil
}

// GetSupportAttachmentById 按 id 获取附件，不存在返回 nil。
func GetSupportAttachmentById(id int) (*SupportMessageAttachment, error) {
	if id <= 0 {
		return nil, errors.New("invalid attachment id")
	}
	var attachment SupportMessageAttachment
	err := DB.First(&attachment, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &attachment, nil
}

// ListClosedSupportConversationsBefore 按关闭时间升序返回早于 cutoff 的已关闭会话。
// limit 默认 100，最大 200。
func ListClosedSupportConversationsBefore(cutoff int64, limit int) ([]SupportConversation, error) {
	if limit <= 0 {
		limit = supportClosedConversationCleanupLimit
	}
	if limit > supportClosedConversationCleanupMax {
		limit = supportClosedConversationCleanupMax
	}

	conversations := make([]SupportConversation, 0, limit)
	if cutoff <= 0 {
		return conversations, nil
	}
	err := DB.Where("status = ? AND closed_at > 0 AND closed_at < ?", SupportConversationStatusClosed, cutoff).
		Order("closed_at ASC").
		Limit(limit).
		Find(&conversations).Error
	if err != nil {
		return nil, err
	}
	return conversations, nil
}

// DeleteSupportConversationData 级联删除会话的附件、消息与会话本身，重复调用幂等。
func DeleteSupportConversationData(conversationId int) error {
	if conversationId <= 0 {
		return errors.New("invalid conversation id")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("conversation_id = ?", conversationId).
			Delete(&SupportMessageAttachment{}).Error; err != nil {
			return err
		}
		if err := tx.Where("conversation_id = ?", conversationId).
			Delete(&SupportMessage{}).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", conversationId).
			Delete(&SupportConversation{}).Error
	})
}
