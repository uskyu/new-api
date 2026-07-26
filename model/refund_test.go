package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type legacyRefundBatch struct {
	Id             int
	IdempotencyKey string `gorm:"type:varchar(128);uniqueIndex;not null"`
	Status         string `gorm:"type:varchar(32);index"`
	TotalItems     int
	SkippedItems   int
	CreatedAt      int64 `gorm:"bigint;index"`
	UpdatedAt      int64 `gorm:"bigint"`
}

func (legacyRefundBatch) TableName() string {
	return "refund_batches"
}

type legacyRefundItem struct {
	Id          int
	BatchId     int `gorm:"uniqueIndex:idx_refund_batch_source,priority:1;index"`
	SourceLogId int `gorm:"uniqueIndex:idx_refund_batch_source,priority:2;index"`
	UserId      int `gorm:"index"`
	RefundQuota int
	Status      string `gorm:"type:varchar(32);index"`
	CreatedAt   int64  `gorm:"bigint"`
	UpdatedAt   int64  `gorm:"bigint"`
}

func (legacyRefundItem) TableName() string {
	return "refund_items"
}

func setupRefundTestDB(t *testing.T) {
	t.Helper()
	dsn := fmt.Sprintf("file:refund-%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&User{}, &Token{}, &Log{}, &Channel{}, &RefundBatch{}, &RefundItem{}))
	oldDB, oldLogDB := DB, LOG_DB
	DB, LOG_DB = db, db
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	t.Cleanup(func() {
		DB, LOG_DB = oldDB, oldLogDB
		sqlDB, _ := db.DB()
		_ = sqlDB.Close()
	})
}

func createRefundUserAndToken(t *testing.T, finite bool) (User, Token) {
	t.Helper()
	user := User{Username: "refund-user", Password: "test-password", Quota: 100}
	require.NoError(t, DB.Create(&user).Error)
	token := Token{UserId: user.Id, Key: "refund-token", Name: "refund", RemainQuota: 20, UsedQuota: 100, UnlimitedQuota: !finite}
	require.NoError(t, DB.Create(&token).Error)
	return user, token
}

func createConsumeLog(t *testing.T, user User, token Token, channel int, modelName string, quota int, other string) Log {
	t.Helper()
	row := Log{UserId: user.Id, Username: user.Username, CreatedAt: 1000, Type: LogTypeConsume, ChannelId: channel, ModelName: modelName, TokenId: token.Id, Quota: quota, Other: other}
	require.NoError(t, LOG_DB.Create(&row).Error)
	return row
}

func createConsumeLogs(t *testing.T, user User, token Token, count int, quota int) {
	t.Helper()
	rows := make([]Log, count)
	for i := range rows {
		rows[i] = Log{UserId: user.Id, Username: user.Username, CreatedAt: 1000, Type: LogTypeConsume, ChannelId: 1, ModelName: "m", TokenId: token.Id, Quota: quota, Other: `{"billing_source":"wallet"}`}
	}
	require.NoError(t, LOG_DB.CreateInBatches(&rows, 100).Error)
}

func drainRefundLogDeliveries(t *testing.T) {
	t.Helper()
	for i := 0; i < 1000; i++ {
		worked, err := ProcessNextRefundLogDelivery("test-log-worker", 30)
		require.NoError(t, err)
		if !worked {
			return
		}
	}
	t.Fatal("refund log delivery did not become idle")
}

func TestRefundAutoMigratePreservesLegacyBatchData(t *testing.T) {
	dsn := fmt.Sprintf("file:refund-migration-%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&legacyRefundBatch{}, &legacyRefundItem{}))

	legacyBatch := legacyRefundBatch{IdempotencyKey: "legacy-batch", Status: RefundBatchRunning, TotalItems: 1, CreatedAt: 100, UpdatedAt: 100}
	require.NoError(t, db.Create(&legacyBatch).Error)
	legacyItem := legacyRefundItem{BatchId: legacyBatch.Id, SourceLogId: 123, UserId: 7, RefundQuota: 50, Status: RefundItemSuccess, CreatedAt: 100, UpdatedAt: 100}
	require.NoError(t, db.Create(&legacyItem).Error)

	require.NoError(t, db.AutoMigrate(&RefundBatch{}, &RefundItem{}))

	var migratedBatch RefundBatch
	require.NoError(t, db.First(&migratedBatch, legacyBatch.Id).Error)
	require.Equal(t, "legacy-batch", migratedBatch.IdempotencyKey)
	require.Equal(t, 1, migratedBatch.TotalItems)
	require.False(t, migratedBatch.ScanCompleted)
	require.True(t, db.Migrator().HasColumn(&RefundBatch{}, "lease_owner"))
	require.True(t, db.Migrator().HasColumn(&RefundItem{}, "log_recorded"))

	var migratedItem RefundItem
	require.NoError(t, db.First(&migratedItem, legacyItem.Id).Error)
	require.Equal(t, 123, migratedItem.SourceLogId)
	require.Equal(t, RefundItemSuccess, migratedItem.Status)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
}

func TestValidateRefundFilterBoundaries(t *testing.T) {
	valid := RefundFilter{StartTime: 1, EndTime: 1, ChannelIds: []int{1}, Ratio: 1, Reason: "incident"}
	require.NoError(t, validateRefundFilter(valid))
	valid.Ratio = 100
	require.NoError(t, validateRefundFilter(valid))
	for _, bad := range []RefundFilter{
		{StartTime: 0, EndTime: 1, ChannelIds: []int{1}, Ratio: 1, Reason: "incident"},
		{StartTime: 2, EndTime: 1, ChannelIds: []int{1}, Ratio: 1, Reason: "incident"},
		{StartTime: 1, EndTime: 1 + RefundMaxRangeSeconds + 1, ChannelIds: []int{1}, Ratio: 1, Reason: "incident"},
		{StartTime: 1, EndTime: 1, Ratio: 1, Reason: "incident"},
		{StartTime: 1, EndTime: 1, ChannelIds: []int{1}, Ratio: 0, Reason: "incident"},
		{StartTime: 1, EndTime: 1, ChannelIds: []int{1}, Ratio: 101, Reason: "incident"},
		{StartTime: 1, EndTime: 1, ChannelIds: []int{1}, Ratio: 1},
		{StartTime: 1, EndTime: 1, ChannelIds: []int{1}, Ratio: 1, Reason: strings.Repeat("界", 501)},
	} {
		require.Error(t, validateRefundFilter(bad))
	}
}

func TestPreviewRefundFiltersAndRequiresExplicitWallet(t *testing.T) {
	setupRefundTestDB(t)
	user, token := createRefundUserAndToken(t, true)
	createConsumeLog(t, user, token, 1, "wanted", 101, `{"billing_source":"wallet"}`)
	createConsumeLog(t, user, token, 1, "wanted", 100, `{}`)
	createConsumeLog(t, user, token, 1, "wanted", 100, `{"billing_source":"subscription"}`)
	createConsumeLog(t, user, token, 2, "wanted", 100, `{"billing_source":"wallet"}`)
	createConsumeLog(t, user, token, 1, "other", 100, `{"billing_source":"wallet"}`)
	zero := createConsumeLog(t, user, token, 1, "wanted", 1, `{"billing_source":"wallet"}`)
	_ = zero

	preview, err := PreviewRefund(RefundFilter{StartTime: 1000, EndTime: 1000, ChannelIds: []int{1}, ModelNames: []string{"wanted"}, Ratio: 50, Reason: "incident"})
	require.NoError(t, err)
	require.Equal(t, 1, preview.MatchedItems)
	require.Equal(t, int64(101), preview.SourceQuota)
	require.Equal(t, int64(50), preview.RefundQuota)
	require.Equal(t, 3, preview.Skipped) // missing source, subscription, and rounded-to-zero
	require.Equal(t, 1, preview.ByChannel[1].Items)
	require.Equal(t, 1, preview.ByModel["wanted"].Items)

	options, err := GetRefundOptions(1000, 1000)
	require.NoError(t, err)
	require.Len(t, options.Channels, 2)
	require.Equal(t, []string{"other", "wanted"}, options.Models)
}

func TestRefundIdempotencyCumulativeCapAndFiniteTokenAdjustment(t *testing.T) {
	setupRefundTestDB(t)
	user, token := createRefundUserAndToken(t, true)
	source := createConsumeLog(t, user, token, 1, "m", 100, `{"billing_source":"wallet"}`)
	filter := RefundFilter{StartTime: 1000, EndTime: 1000, ChannelIds: []int{1}, ModelNames: []string{"m"}, Ratio: 60, Reason: "incident"}

	first, err := CreateAndRunRefund(filter, "same-key", 99)
	require.NoError(t, err)
	require.Equal(t, int64(60), first.RefundedQuota)
	_, err = CreateAndRunRefund(filter, "same-key", 99)
	require.NoError(t, err)

	preview, err := PreviewRefund(RefundFilter{StartTime: 1000, EndTime: 1000, ChannelIds: []int{1}, ModelNames: []string{"m"}, Ratio: 100, Reason: "incident"})
	require.NoError(t, err)
	require.Equal(t, 1, preview.MatchedItems)
	require.Equal(t, int64(40), preview.RefundQuota)

	var gotUser User
	var gotToken Token
	require.NoError(t, DB.First(&gotUser, user.Id).Error)
	require.NoError(t, DB.First(&gotToken, token.Id).Error)
	require.Equal(t, 160, gotUser.Quota)
	require.Equal(t, 80, gotToken.RemainQuota)
	require.Equal(t, 40, gotToken.UsedQuota)

	second, err := CreateAndRunRefund(filter, "second-key", 99)
	require.NoError(t, err)
	require.Equal(t, int64(40), second.RefundedQuota)
	require.NoError(t, DB.First(&gotUser, user.Id).Error)
	require.NoError(t, DB.First(&gotToken, token.Id).Error)
	require.Equal(t, 200, gotUser.Quota)
	require.Equal(t, 120, gotToken.RemainQuota)
	require.Equal(t, 0, gotToken.UsedQuota)

	var refunded int64
	require.NoError(t, DB.Model(&RefundItem{}).Where("source_log_id = ? AND status = ?", source.Id, RefundItemSuccess).Select("COALESCE(SUM(refund_quota),0)").Scan(&refunded).Error)
	require.Equal(t, int64(100), refunded)

	drainRefundLogDeliveries(t)
	var logs []Log
	require.NoError(t, LOG_DB.Where("type = ?", LogTypeRefund).Find(&logs).Error)
	require.Len(t, logs, 2)
	parsed, err := common.StrToMap(logs[0].Other)
	require.NoError(t, err)
	require.Contains(t, parsed, "admin_info")
	require.NotContains(t, parsed, "operator_id")
	formatUserLogs([]*Log{&logs[0]}, 0)
	parsed, err = common.StrToMap(logs[0].Other)
	require.NoError(t, err)
	require.NotContains(t, parsed, "admin_info")

	var logItem RefundItem
	require.NoError(t, DB.Where("source_log_id = ? AND status = ?", source.Id, RefundItemSuccess).First(&logItem).Error)
	require.NoError(t, DB.Model(&RefundItem{}).Where("id = ?", logItem.Id).Update("log_recorded", false).Error)
	drainRefundLogDeliveries(t)
	require.NoError(t, LOG_DB.Where("type = ?", LogTypeRefund).Find(&logs).Error)
	require.Len(t, logs, 2)
}

func TestRefundMissingFiniteTokenRollsBackUserAdjustment(t *testing.T) {
	setupRefundTestDB(t)
	user, token := createRefundUserAndToken(t, true)
	createConsumeLog(t, user, token, 1, "m", 100, `{"billing_source":"wallet"}`)
	require.NoError(t, DB.Delete(&token).Error)

	batch, err := CreateAndRunRefund(RefundFilter{StartTime: 1000, EndTime: 1000, ChannelIds: []int{1}, Ratio: 100, Reason: "incident"}, "missing-token", 1)
	require.NoError(t, err)
	require.Equal(t, RefundBatchFailed, batch.Status)
	var got User
	require.NoError(t, DB.First(&got, user.Id).Error)
	require.Equal(t, 100, got.Quota)
}

func TestRefundPreviewScansMoreThanOneThousandLogs(t *testing.T) {
	setupRefundTestDB(t)
	user, token := createRefundUserAndToken(t, true)
	createConsumeLogs(t, user, token, 1001, 1)

	preview, err := PreviewRefund(RefundFilter{StartTime: 1000, EndTime: 1000, ChannelIds: []int{1}, Ratio: 100, Reason: "large incident"})
	require.NoError(t, err)
	require.Equal(t, 1001, preview.MatchedItems)
	require.Equal(t, int64(1001), preview.RefundQuota)
}

func TestRefundWorkerResumesWithoutCreditingCompletedItemsTwice(t *testing.T) {
	setupRefundTestDB(t)
	user, token := createRefundUserAndToken(t, true)
	createConsumeLogs(t, user, token, 150, 1)
	filter := RefundFilter{StartTime: 1000, EndTime: 1000, ChannelIds: []int{1}, Ratio: 100, Reason: "worker restart"}

	batch, err := CreateRefundBatch(filter, "background-restart", 99)
	require.NoError(t, err)
	require.Equal(t, RefundBatchPending, batch.Status)

	var before User
	require.NoError(t, DB.First(&before, user.Id).Error)
	require.Equal(t, 100, before.Quota)

	worked, err := ProcessNextRefundBatchChunk("worker-before-restart", 60)
	require.NoError(t, err)
	require.True(t, worked)
	var firstPassSuccess int64
	require.NoError(t, DB.Model(&RefundItem{}).Where("batch_id = ? AND status = ?", batch.Id, RefundItemSuccess).Count(&firstPassSuccess).Error)
	require.Equal(t, int64(RefundItemPageSize), firstPassSuccess)

	require.NoError(t, DB.Model(&RefundBatch{}).Where("id = ?", batch.Id).Updates(map[string]interface{}{
		"status": RefundBatchRunning, "lease_owner": "stopped-worker", "lease_expires_at": common.GetTimestamp() - 1,
	}).Error)

	for i := 0; i < 20; i++ {
		worked, err = ProcessNextRefundBatchChunk("replacement-worker", 60)
		require.NoError(t, err)
		var current RefundBatch
		require.NoError(t, DB.First(&current, batch.Id).Error)
		if current.Status == RefundBatchCompleted {
			break
		}
		require.True(t, worked)
	}

	var completed RefundBatch
	require.NoError(t, DB.First(&completed, batch.Id).Error)
	require.Equal(t, RefundBatchCompleted, completed.Status)
	require.Equal(t, 150, completed.SuccessItems)
	require.Equal(t, int64(150), completed.RefundedQuota)

	var gotUser User
	var gotToken Token
	require.NoError(t, DB.First(&gotUser, user.Id).Error)
	require.NoError(t, DB.First(&gotToken, token.Id).Error)
	require.Equal(t, 250, gotUser.Quota)
	require.Equal(t, 170, gotToken.RemainQuota)
	require.Equal(t, 0, gotToken.UsedQuota)

	worked, err = ProcessNextRefundBatchChunk("replacement-worker", 60)
	require.NoError(t, err)
	require.False(t, worked)
	require.NoError(t, DB.First(&gotUser, user.Id).Error)
	require.Equal(t, 250, gotUser.Quota)
}

func TestRefundBatchDetailsAreServerPaginated(t *testing.T) {
	setupRefundTestDB(t)
	user, token := createRefundUserAndToken(t, true)
	createConsumeLogs(t, user, token, 45, 1)

	batch, err := CreateAndRunRefund(RefundFilter{StartTime: 1000, EndTime: 1000, ChannelIds: []int{1}, Ratio: 100, Reason: "pagination"}, "pagination", 99)
	require.NoError(t, err)
	page, err := GetRefundBatchPage(batch.Id, 2, 20)
	require.NoError(t, err)
	require.Equal(t, 2, page.ItemPage)
	require.Equal(t, 20, page.ItemPageSize)
	require.Equal(t, int64(45), page.ItemTotal)
	require.Len(t, page.Items, 20)
}

func TestRefundLogDeliveryRetriesWhenBatchIsMissing(t *testing.T) {
	setupRefundTestDB(t)
	item := RefundItem{
		BatchId: 999, SourceLogId: 123, UserId: 7, RefundQuota: 50,
		Status: RefundItemSuccess, CreatedAt: 100, UpdatedAt: 100,
	}
	require.NoError(t, DB.Create(&item).Error)

	worked, err := ProcessNextRefundLogDelivery("missing-batch-worker", 30)
	require.Error(t, err)
	require.True(t, worked)

	var persisted RefundItem
	require.NoError(t, DB.First(&persisted, item.Id).Error)
	require.False(t, persisted.LogRecorded)
	require.NotEmpty(t, persisted.LogError)
	require.Greater(t, persisted.LogNextAttemptAt, int64(0))
}
