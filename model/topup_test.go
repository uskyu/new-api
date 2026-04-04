package model

import (
	"fmt"
	"os"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func runManualCompleteTopUpIdempotentTest(t *testing.T, dialect TestDBDialect) {
	t.Helper()
	setupAgentTestDB(t, dialect)

	uniqueSuffix := common.GetRandomString(6)
	user := createAgentTestUser(
		t,
		fmt.Sprintf("tp_%s_%s", dialect, uniqueSuffix),
		fmt.Sprintf("AFF_%s_%s", dialect, uniqueSuffix),
	)
	tradeNo := fmt.Sprintf("trade_%s_manual_complete_%s", dialect, uniqueSuffix)
	topup := &TopUp{
		UserId:        user.Id,
		Amount:        12,
		Money:         12,
		TradeNo:       tradeNo,
		PaymentMethod: "epay",
		CreateTime:    common.GetTimestamp(),
		Status:        common.TopUpStatusPending,
	}
	require.NoError(t, DB.Create(topup).Error)

	before, err := GetUserById(user.Id, false)
	require.NoError(t, err)
	require.NotNil(t, before)

	require.NoError(t, ManualCompleteTopUp(tradeNo))
	require.NoError(t, ManualCompleteTopUp(tradeNo))

	after, err := GetUserById(user.Id, false)
	require.NoError(t, err)
	require.NotNil(t, after)

	updatedTopUp := GetTopUpByTradeNo(tradeNo)
	require.NotNil(t, updatedTopUp)
	require.Equal(t, common.TopUpStatusSuccess, updatedTopUp.Status)
	require.NotZero(t, updatedTopUp.CompleteTime)

	expectedQuotaDelta := int(float64(topup.Amount) * common.QuotaPerUnit)
	require.Equal(t, before.Quota+expectedQuotaDelta, after.Quota)
}

func runRechargeIdempotentTest(t *testing.T, dialect TestDBDialect) {
	t.Helper()
	setupAgentTestDB(t, dialect)

	uniqueSuffix := common.GetRandomString(6)
	customerID := "cust_" + uniqueSuffix
	user := createAgentTestUser(
		t,
		fmt.Sprintf("rc_%s_%s", dialect, uniqueSuffix),
		fmt.Sprintf("RFC_%s_%s", dialect, uniqueSuffix),
	)
	referenceID := fmt.Sprintf("trade_%s_recharge_%s", dialect, uniqueSuffix)
	topup := &TopUp{
		UserId:        user.Id,
		Amount:        0,
		Money:         9.5,
		TradeNo:       referenceID,
		PaymentMethod: "stripe",
		CreateTime:    common.GetTimestamp(),
		Status:        common.TopUpStatusPending,
	}
	require.NoError(t, DB.Create(topup).Error)

	before, err := GetUserById(user.Id, false)
	require.NoError(t, err)
	require.NotNil(t, before)

	require.NoError(t, Recharge(referenceID, customerID))
	require.NoError(t, Recharge(referenceID, customerID))

	after, err := GetUserById(user.Id, false)
	require.NoError(t, err)
	require.NotNil(t, after)

	updatedTopUp := GetTopUpByTradeNo(referenceID)
	require.NotNil(t, updatedTopUp)
	require.Equal(t, common.TopUpStatusSuccess, updatedTopUp.Status)
	require.NotZero(t, updatedTopUp.CompleteTime)
	require.Equal(t, customerID, after.StripeCustomer)

	expectedQuotaDelta := int(topup.Money * common.QuotaPerUnit)
	require.Equal(t, before.Quota+expectedQuotaDelta, after.Quota)
}

func TestManualCompleteTopUpIdempotentSQLite(t *testing.T) {
	runManualCompleteTopUpIdempotentTest(t, TestDBDialectSQLite)
}

func TestManualCompleteTopUpIdempotentMySQL(t *testing.T) {
	if os.Getenv("TEST_MYSQL_DSN") == "" {
		t.Skip("TEST_MYSQL_DSN is not set")
	}
	runManualCompleteTopUpIdempotentTest(t, TestDBDialectMySQL)
}

func TestManualCompleteTopUpIdempotentPostgres(t *testing.T) {
	if os.Getenv("TEST_POSTGRES_DSN") == "" {
		t.Skip("TEST_POSTGRES_DSN is not set")
	}
	runManualCompleteTopUpIdempotentTest(t, TestDBDialectPostgres)
}

func TestRechargeIdempotentSQLite(t *testing.T) {
	runRechargeIdempotentTest(t, TestDBDialectSQLite)
}

func TestRechargeIdempotentMySQL(t *testing.T) {
	if os.Getenv("TEST_MYSQL_DSN") == "" {
		t.Skip("TEST_MYSQL_DSN is not set")
	}
	runRechargeIdempotentTest(t, TestDBDialectMySQL)
}

func TestRechargeIdempotentPostgres(t *testing.T) {
	if os.Getenv("TEST_POSTGRES_DSN") == "" {
		t.Skip("TEST_POSTGRES_DSN is not set")
	}
	runRechargeIdempotentTest(t, TestDBDialectPostgres)
}
