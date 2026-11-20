package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupSupportCleanupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB := model.DB
	previousLogDB := model.LOG_DB
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(
		&model.SupportConversation{},
		&model.SupportMessage{},
		&model.SupportMessageAttachment{},
	))
	t.Cleanup(func() {
		sqlDB, closeErr := db.DB()
		if closeErr == nil {
			_ = sqlDB.Close()
		}
		model.DB = previousDB
		model.LOG_DB = previousLogDB
	})
	return db
}

func setSupportCleanupTestRetentionDays(t *testing.T, days int) {
	t.Helper()
	cfg, ok := config.GlobalConfig.Get("support_chat").(*system_setting.SupportChatSettings)
	require.True(t, ok)
	previous := cfg.RetentionDays
	cfg.RetentionDays = days
	t.Cleanup(func() {
		cfg.RetentionDays = previous
	})
}

func TestCleanupSupportAttachmentsRemovesExpiredClosedConversations(t *testing.T) {
	db := setupSupportCleanupTestDB(t)
	setupSupportAttachmentTestDir(t)
	setSupportCleanupTestRetentionDays(t, 30)

	oldConversation, err := model.GetOrCreateSupportConversation(1501)
	require.NoError(t, err)
	stored := saveSupportTestImage(t, oldConversation.Id, "old.png", encodeSupportTestPNG(t, 40, 30))
	oldMessage := &model.SupportMessage{
		ConversationId: oldConversation.Id,
		SenderType:     model.SupportSenderTypeUser,
		SenderId:       1501,
		SenderName:     "user1501",
		Content:        "old message",
	}
	require.NoError(t, model.CreateSupportMessageWithAttachments(oldMessage, []model.SupportMessageAttachment{
		{
			ConversationId:    oldConversation.Id,
			FileName:          stored.FileName,
			StorageKey:        stored.StorageKey,
			ThumbnailKey:      stored.ThumbnailKey,
			ThumbnailMimeType: stored.ThumbnailMimeType,
			MimeType:          stored.MimeType,
			SizeBytes:         stored.SizeBytes,
			Width:             stored.Width,
			Height:            stored.Height,
		},
	}))
	require.NoError(t, model.CloseSupportConversation(oldConversation.Id))
	require.NoError(t, db.Model(&model.SupportConversation{}).
		Where("id = ?", oldConversation.Id).
		Update("closed_at", time.Now().Unix()-31*86400).Error)

	recentConversation, err := model.GetOrCreateSupportConversation(1502)
	require.NoError(t, err)
	recentMessage := &model.SupportMessage{
		ConversationId: recentConversation.Id,
		SenderType:     model.SupportSenderTypeUser,
		SenderId:       1502,
		SenderName:     "user1502",
		Content:        "recent message",
	}
	require.NoError(t, model.CreateSupportMessage(recentMessage))
	require.NoError(t, model.CloseSupportConversation(recentConversation.Id))
	require.NoError(t, db.Model(&model.SupportConversation{}).
		Where("id = ?", recentConversation.Id).
		Update("closed_at", time.Now().Unix()-10*86400).Error)

	openConversation, err := model.GetOrCreateSupportConversation(1503)
	require.NoError(t, err)
	require.NoError(t, db.Model(&model.SupportConversation{}).
		Where("id = ?", openConversation.Id).
		Update("created_at", time.Now().Unix()-60*86400).Error)
	require.NoError(t, db.Model(&model.SupportConversation{}).
		Where("id = ?", openConversation.Id).
		Update("last_message_at", time.Now().Unix()-60*86400).Error)

	require.NoError(t, CleanupSupportAttachments())

	var count int64
	require.NoError(t, db.Model(&model.SupportConversation{}).Where("id = ?", oldConversation.Id).Count(&count).Error)
	assert.Zero(t, count)
	require.NoError(t, db.Model(&model.SupportMessage{}).Where("conversation_id = ?", oldConversation.Id).Count(&count).Error)
	assert.Zero(t, count)
	require.NoError(t, db.Model(&model.SupportMessageAttachment{}).Where("conversation_id = ?", oldConversation.Id).Count(&count).Error)
	assert.Zero(t, count)
	_, statErr := os.Stat(filepath.Join(GetSupportAttachmentRootDir(), fmt.Sprintf("%d", oldConversation.Id)))
	assert.True(t, os.IsNotExist(statErr))

	require.NoError(t, db.Model(&model.SupportConversation{}).Where("id = ?", recentConversation.Id).Count(&count).Error)
	assert.Equal(t, int64(1), count)
	require.NoError(t, db.Model(&model.SupportConversation{}).Where("id = ?", openConversation.Id).Count(&count).Error)
	assert.Equal(t, int64(1), count)

	// 0 表示永久保留，不执行清理。
	setSupportCleanupTestRetentionDays(t, 0)
	require.NoError(t, CleanupSupportAttachments())
	require.NoError(t, db.Model(&model.SupportConversation{}).Where("id = ?", recentConversation.Id).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}
