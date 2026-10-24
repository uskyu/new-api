package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHarnessSettlementCurrencyIsAlwaysUsd pins the ledger currency. The agreed
// contract fixes the procurement cost currency to USD, so the ledger must never
// follow the site display currency: doing so would label USD base amounts as CNY
// once the dashboard is switched, which is exactly the defect this locks out.
func TestHarnessSettlementCurrencyIsAlwaysUsd(t *testing.T) {
	assert.Equal(t, "USD", harnessSettlementCurrency)

	// A zero-amount row is the only path reachable without a database, and it
	// still carries the currency, which is what B3 reports for an unsettled row.
	money := harnessSettlementMoneyForQuota(0, 0, "model-without-pricing-group")
	assert.Equal(t, "USD", money.Currency)
	assert.Equal(t, "0", money.BaseAmount)
	assert.Equal(t, "0", money.TotalAmount)
}

// TestHarnessSettlementMoneyAppliesGroupRatioOnly documents the amount formula
// the ledger guarantees: base is quota divided by QuotaPerUnit, and the only
// multiplier folded into the total is the procurement group ratio. The supplier
// tax is reported separately and never charged, and no display-currency rate is
// applied, so total == base when no pricing group resolves.
//
// This asserts the arithmetic invariants on the numbers the function returns
// rather than on channel/routing fixtures, because the pricing-group lookup is
// covered by model-level tests.
func TestHarnessSettlementMoneyAppliesGroupRatioOnly(t *testing.T) {
	originalQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })
	common.QuotaPerUnit = 500_000

	// No procurement group resolves for an unknown channel/model, so the total
	// must track the base: no discount, no tax, no conversion.
	money := harnessSettlementMoneyForQuota(1_000_000, 0, "model-without-pricing-group")

	base, err := decimal.NewFromString(money.BaseAmount)
	require.NoError(t, err)
	total, err := decimal.NewFromString(money.TotalAmount)
	require.NoError(t, err)

	assert.True(t, base.Equal(decimal.NewFromInt(2)),
		"base must be quota/QuotaPerUnit (1_000_000/500_000=2), got %s", money.BaseAmount)
	assert.True(t, total.Equal(base),
		"without a procurement group the total must equal the base with no tax or conversion, got %s", money.TotalAmount)
	assert.Equal(t, 0.0, money.GroupRatio)
	assert.Equal(t, 0.0, money.TaxRatePercent)
}

// TestHarnessSettlementMoneyRejectsNegativeQuota keeps the defensive branch: a
// negative quota can only come from a caller bug, and the ledger must report
// zero rather than a negative cost.
func TestHarnessSettlementMoneyRejectsNegativeQuota(t *testing.T) {
	money := harnessSettlementMoneyForQuota(-1, 0, "model-without-pricing-group")

	assert.Equal(t, "USD", money.Currency)
	assert.Equal(t, "0", money.BaseAmount)
	assert.Equal(t, "0", money.TotalAmount)
}
