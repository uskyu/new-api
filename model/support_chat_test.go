package model

import (
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupSupportChatTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := openTestDB(t, TestDBDialectSQLite)
	require.NoError(t, db.AutoMigrate(&SupportConversation{}, &SupportMessage{}))
	DB = db
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
		DB = nil
	})
	return db
}

func reloadSupportConversation(t *testing.T, id int) *SupportConversation {
	t.Helper()
	conversation, err := GetSupportConversationById(id)
	require.NoError(t, err)
	require.NotNil(t, conversation)
	return conversation
}

func TestGetOrCreateSupportConversationIdempotentAndIsolated(t *testing.T) {
	setupSupportChatTestDB(t)

	first, err := GetOrCreateSupportConversation(101)
	require.NoError(t, err)
	require.NotNil(t, first)
	assert.Equal(t, SupportConversationStatusOpen, first.Status)
	assert.Equal(t, 101, first.UserId)

	second, err := GetOrCreateSupportConversation(101)
	require.NoError(t, err)
	require.NotNil(t, second)
	assert.Equal(t, first.Id, second.Id)

	other, err := GetOrCreateSupportConversation(202)
	require.NoError(t, err)
	require.NotNil(t, other)
	assert.NotEqual(t, first.Id, other.Id)

	var count int64
	require.NoError(t, DB.Model(&SupportConversation{}).
		Where("user_id = ? AND status = ?", 101, SupportConversationStatusOpen).
		Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestGetOrCreateSupportConversationConcurrent(t *testing.T) {
	setupSupportChatTestDB(t)

	const goroutines = 8
	ids := make([]int, goroutines)
	errs := make([]error, goroutines)
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			conversation, err := GetOrCreateSupportConversation(303)
			errs[idx] = err
			if conversation != nil {
				ids[idx] = conversation.Id
			}
		}(i)
	}
	wg.Wait()

	for i := 0; i < goroutines; i++ {
		require.NoError(t, errs[i])
		require.NotZero(t, ids[i])
		assert.Equal(t, ids[0], ids[i])
	}

	var count int64
	require.NoError(t, DB.Model(&SupportConversation{}).
		Where("user_id = ? AND status = ?", 303, SupportConversationStatusOpen).
		Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestCreateSupportMessageUnreadPreviewAndAfterId(t *testing.T) {
	setupSupportChatTestDB(t)

	conversation, err := GetOrCreateSupportConversation(404)
	require.NoError(t, err)

	first := &SupportMessage{
		ConversationId: conversation.Id,
		SenderType:     SupportSenderTypeUser,
		SenderId:       404,
		SenderName:     "user404",
		Content:        "hello support",
	}
	require.NoError(t, CreateSupportMessage(first))
	require.NotZero(t, first.Id)

	updated := reloadSupportConversation(t, conversation.Id)
	assert.Equal(t, 0, updated.UserUnread)
	assert.Equal(t, 1, updated.AdminUnread)
	assert.Equal(t, "hello support", updated.LastMessagePreview)
	assert.NotZero(t, updated.LastMessageAt)
	assert.Equal(t, 0, updated.AssignedAdminId)

	longContent := strings.Repeat("中", 210)
	second := &SupportMessage{
		ConversationId: conversation.Id,
		SenderType:     SupportSenderTypeUser,
		SenderId:       404,
		SenderName:     "user404",
		Content:        longContent,
	}
	require.NoError(t, CreateSupportMessage(second))
	updated = reloadSupportConversation(t, conversation.Id)
	assert.Equal(t, 2, updated.AdminUnread)
	assert.Equal(t, 200, len([]rune(updated.LastMessagePreview)))

	adminMessage := &SupportMessage{
		ConversationId: conversation.Id,
		SenderType:     SupportSenderTypeAdmin,
		SenderId:       909,
		SenderName:     "agent909",
		Content:        "admin reply",
	}
	require.NoError(t, CreateSupportMessage(adminMessage))
	updated = reloadSupportConversation(t, conversation.Id)
	assert.Equal(t, 1, updated.UserUnread)
	assert.Equal(t, 2, updated.AdminUnread)
	assert.Equal(t, 909, updated.AssignedAdminId)
	assert.Equal(t, "admin reply", updated.LastMessagePreview)

	all, err := ListSupportMessages(conversation.Id, 0, 100)
	require.NoError(t, err)
	require.Len(t, all, 3)
	assert.Equal(t, first.Id, all[0].Id)
	assert.Equal(t, second.Id, all[1].Id)
	assert.Equal(t, adminMessage.Id, all[2].Id)

	after, err := ListSupportMessages(conversation.Id, first.Id, 100)
	require.NoError(t, err)
	require.Len(t, after, 2)
	assert.Equal(t, second.Id, after[0].Id)
	assert.Equal(t, adminMessage.Id, after[1].Id)

	limited, err := ListSupportMessages(conversation.Id, 0, 2)
	require.NoError(t, err)
	require.Len(t, limited, 2)
	assert.Equal(t, second.Id, limited[0].Id)
	assert.Equal(t, adminMessage.Id, limited[1].Id)
}

func TestMarkSupportConversationRead(t *testing.T) {
	setupSupportChatTestDB(t)

	conversation, err := GetOrCreateSupportConversation(505)
	require.NoError(t, err)

	userMessage := &SupportMessage{
		ConversationId: conversation.Id,
		SenderType:     SupportSenderTypeUser,
		SenderId:       505,
		SenderName:     "user505",
		Content:        "question",
	}
	require.NoError(t, CreateSupportMessage(userMessage))

	require.NoError(t, MarkSupportConversationRead(conversation.Id, SupportSenderTypeAdmin, 0))
	updated := reloadSupportConversation(t, conversation.Id)
	assert.Equal(t, 0, updated.AdminUnread)
	assert.Equal(t, userMessage.Id, updated.AdminLastReadId)

	adminMessage := &SupportMessage{
		ConversationId: conversation.Id,
		SenderType:     SupportSenderTypeAdmin,
		SenderId:       808,
		SenderName:     "agent808",
		Content:        "answer",
	}
	require.NoError(t, CreateSupportMessage(adminMessage))
	updated = reloadSupportConversation(t, conversation.Id)
	assert.Equal(t, 1, updated.UserUnread)

	require.NoError(t, MarkSupportConversationRead(conversation.Id, SupportSenderTypeUser, 0))
	updated = reloadSupportConversation(t, conversation.Id)
	assert.Equal(t, 0, updated.UserUnread)
	assert.Equal(t, adminMessage.Id, updated.UserLastReadId)

	err = MarkSupportConversationRead(conversation.Id, "invalid", 1)
	require.Error(t, err)
}

func TestClaimSupportConversationOnlyUnassigned(t *testing.T) {
	setupSupportChatTestDB(t)

	conversation, err := GetOrCreateSupportConversation(606)
	require.NoError(t, err)

	claimed, err := ClaimSupportConversation(conversation.Id, 11)
	require.NoError(t, err)
	assert.True(t, claimed)
	assert.Equal(t, 11, reloadSupportConversation(t, conversation.Id).AssignedAdminId)

	claimed, err = ClaimSupportConversation(conversation.Id, 12)
	require.NoError(t, err)
	assert.False(t, claimed)
	assert.Equal(t, 11, reloadSupportConversation(t, conversation.Id).AssignedAdminId)

	claimed, err = ClaimSupportConversation(999999, 11)
	require.NoError(t, err)
	assert.False(t, claimed)

	require.NoError(t, CloseSupportConversation(conversation.Id))
	claimed, err = ClaimSupportConversation(conversation.Id, 13)
	require.NoError(t, err)
	assert.False(t, claimed)
}

func TestCloseSupportConversationIdempotent(t *testing.T) {
	setupSupportChatTestDB(t)

	conversation, err := GetOrCreateSupportConversation(707)
	require.NoError(t, err)

	require.NoError(t, CloseSupportConversation(conversation.Id))
	closed := reloadSupportConversation(t, conversation.Id)
	assert.Equal(t, SupportConversationStatusClosed, closed.Status)
	assert.NotZero(t, closed.ClosedAt)
	closedAt := closed.ClosedAt

	require.NoError(t, CloseSupportConversation(conversation.Id))
	closedAgain := reloadSupportConversation(t, conversation.Id)
	assert.Equal(t, SupportConversationStatusClosed, closedAgain.Status)
	assert.Equal(t, closedAt, closedAgain.ClosedAt)

	current, err := GetOpenSupportConversationByUserId(707)
	require.NoError(t, err)
	assert.Nil(t, current)

	reopened, err := GetOrCreateSupportConversation(707)
	require.NoError(t, err)
	require.NotNil(t, reopened)
	assert.NotEqual(t, conversation.Id, reopened.Id)
	assert.Equal(t, SupportConversationStatusOpen, reopened.Status)

	err = CloseSupportConversation(999999)
	require.Error(t, err)
}

func TestListSupportConversationsOrderFilterAndUnreadCount(t *testing.T) {
	setupSupportChatTestDB(t)

	first, err := GetOrCreateSupportConversation(801)
	require.NoError(t, err)
	second, err := GetOrCreateSupportConversation(802)
	require.NoError(t, err)
	third, err := GetOrCreateSupportConversation(803)
	require.NoError(t, err)

	for _, conversation := range []*SupportConversation{first, second, third} {
		require.NoError(t, CreateSupportMessage(&SupportMessage{
			ConversationId: conversation.Id,
			SenderType:     SupportSenderTypeUser,
			SenderId:       conversation.UserId,
			SenderName:     "user",
			Content:        "message",
		}))
	}

	require.NoError(t, DB.Model(&SupportConversation{}).Where("id = ?", first.Id).Update("last_message_at", 100).Error)
	require.NoError(t, DB.Model(&SupportConversation{}).Where("id = ?", second.Id).Update("last_message_at", 200).Error)
	require.NoError(t, DB.Model(&SupportConversation{}).Where("id = ?", third.Id).Update("last_message_at", 300).Error)

	items, total, err := ListSupportConversations(SupportConversationStatusOpen, 0, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	require.Len(t, items, 3)
	assert.Equal(t, third.Id, items[0].Id)
	assert.Equal(t, second.Id, items[1].Id)
	assert.Equal(t, first.Id, items[2].Id)

	count, err := CountSupportUnreadForAgent(11)
	require.NoError(t, err)
	assert.Equal(t, int64(3), count)

	require.NoError(t, CloseSupportConversation(second.Id))

	openItems, total, err := ListSupportConversations(SupportConversationStatusOpen, 0, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	require.Len(t, openItems, 2)

	closedItems, total, err := ListSupportConversations(SupportConversationStatusClosed, 0, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, closedItems, 1)
	assert.Equal(t, second.Id, closedItems[0].Id)

	count, err = CountSupportUnreadForAgent(11)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)

	require.NoError(t, MarkSupportConversationRead(third.Id, SupportSenderTypeAdmin, 0))
	count, err = CountSupportUnreadForAgent(11)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
}
