package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	hosttypes "github.com/QuantumNous/new-api/types"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHarnessBaseAmountIgnoresCustomerGroupRatio pins the invariant this fix
// exists for: the procurement base must not move when only the caller's group
// ratio changes. Two callers with identical usage on the same channel therefore
// cost the operator the same money whatever group they belong to.
//
// The billing paths capture the amount before applying that ratio, so the ledger
// divides the pre-group amount by QuotaPerUnit instead of the charged quota.
func TestHarnessBaseAmountIgnoresCustomerGroupRatio(t *testing.T) {
	originalQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })
	common.QuotaPerUnit = 500_000

	channelId := seedHarnessLedgerChannel(t, "economy", "procurement-probe-group", 1.0, 0)

	// One usage, two customer group ratios. The billing path charges
	// pre-group * ratio, and reports the same pre-group amount to the ledger.
	preGroup := decimal.NewFromInt(1_000_000)

	atOne := harnessSettlementMoneyForQuota(hosttypes.NewProcurementBasis(preGroup, 1), 0, channelId, "economy")
	atTwo := harnessSettlementMoneyForQuota(hosttypes.NewProcurementBasis(preGroup, 2), 0, channelId, "economy")

	assert.Equal(t, "2", atOne.BaseAmount,
		"base_amount is the pre-group amount over QuotaPerUnit (1_000_000/500_000=2)")
	assert.Equal(t, atOne.BaseAmount, atTwo.BaseAmount,
		"doubling the customer group ratio must not change the procurement base")
	assert.Equal(t, atOne.TotalAmount, atTwo.TotalAmount,
		"and it must not change the procurement total either")
}

// TestHarnessBaseAmountStillAppliesProcurementRatio proves the fix did not strip
// the procurement discount, which is the operator's own supplier-side price and
// must keep working independently of the customer ratio.
func TestHarnessBaseAmountStillAppliesProcurementRatio(t *testing.T) {
	originalQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })
	common.QuotaPerUnit = 500_000

	channelId := seedHarnessLedgerChannel(t, "economy", "procurement-compose-group", 0.8, 0)

	// Customer ratio 2 on the charge, but the ledger is told the pre-group amount.
	preGroup := decimal.NewFromInt(1_000_000)
	money := harnessSettlementMoneyForQuota(hosttypes.NewProcurementBasis(preGroup, 2), 2_000_000, channelId, "economy")

	base, err := decimal.NewFromString(money.BaseAmount)
	require.NoError(t, err)
	total, err := decimal.NewFromString(money.TotalAmount)
	require.NoError(t, err)

	assert.True(t, base.Equal(decimal.NewFromInt(2)),
		"base is the pre-group amount, not the charged quota: got %s", money.BaseAmount)
	assert.True(t, total.Equal(decimal.NewFromInt(2).Mul(decimal.NewFromFloat(0.8))),
		"the procurement ratio still applies on top of that base, got %s", money.TotalAmount)
}

// TestHarnessBaseAmountFallsBackWithoutBasis keeps the defensive path honest: a
// request whose billing path reported no basis still gets a cost, but the row is
// tagged as a fallback so reconciliation can tell it from a corrected one.
func TestHarnessBaseAmountFallsBackWithoutBasis(t *testing.T) {
	originalQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })
	common.QuotaPerUnit = 500_000

	money := harnessSettlementMoneyForQuota(nil, 1_000_000, 0, "model-without-pricing-group")

	assert.Equal(t, "2", money.BaseAmount,
		"the fallback still prices the request instead of booking zero")
	assert.Equal(t, harnessCostCalculationFallback, money.CostCalculationVersion,
		"a missing basis must be visible in the row")
}

// TestHarnessBaseAmountRecordsPreGroupVersion pins the version tagging that lets
// reconciliation separate legacy rows from corrected ones.
func TestHarnessBaseAmountRecordsPreGroupVersion(t *testing.T) {
	originalQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })
	common.QuotaPerUnit = 500_000

	basis := hosttypes.NewProcurementBasis(decimal.NewFromInt(1_000_000), 1)
	money := harnessSettlementMoneyForQuota(basis, 1_000_000, 0, "model-without-pricing-group")

	assert.Equal(t, harnessCostCalculationPreGroup, money.CostCalculationVersion,
		"a corrected row must be distinguishable from a legacy one")
	assert.NotEqual(t, harnessCostCalculationLegacy, money.CostCalculationVersion)
}
