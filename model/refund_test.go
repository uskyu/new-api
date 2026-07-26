package model

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

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

func TestValidateRefundFilterBoundaries(t *testing.T) {
	valid := RefundFilter{StartTime: 1, EndTime: 1, ChannelIds: []int{1}, Ratio: 1}
	require.NoError(t, validateRefundFilter(valid))
	valid.Ratio = 100
	require.NoError(t, validateRefundFilter(valid))
	for _, bad := range []RefundFilter{
		{StartTime: 0, EndTime: 1, ChannelIds: []int{1}, Ratio: 1},
		{StartTime: 2, EndTime: 1, ChannelIds: []int{1}, Ratio: 1},
		{StartTime: 1, EndTime: 1, Ratio: 1},
		{StartTime: 1, EndTime: 1, ChannelIds: []int{1}, Ratio: 0},
		{StartTime: 1, EndTime: 1, ChannelIds: []int{1}, Ratio: 101},
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

	preview, err := PreviewRefund(RefundFilter{StartTime: 1000, EndTime: 1000, ChannelIds: []int{1}, ModelNames: []string{"wanted"}, Ratio: 50})
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
}

func TestRefundMissingFiniteTokenRollsBackUserAdjustment(t *testing.T) {
	setupRefundTestDB(t)
	user, token := createRefundUserAndToken(t, true)
	createConsumeLog(t, user, token, 1, "m", 100, `{"billing_source":"wallet"}`)
	require.NoError(t, DB.Delete(&token).Error)

	batch, err := CreateAndRunRefund(RefundFilter{StartTime: 1000, EndTime: 1000, ChannelIds: []int{1}, Ratio: 100}, "missing-token", 1)
	require.NoError(t, err)
	require.Equal(t, RefundBatchFailed, batch.Status)
	var got User
	require.NoError(t, DB.First(&got, user.Id).Error)
	require.Equal(t, 100, got.Quota)
}
