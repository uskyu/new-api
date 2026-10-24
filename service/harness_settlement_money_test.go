package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	hosttypes "github.com/QuantumNous/new-api/types"
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
	money := harnessSettlementMoneyForQuota(nil, 0, 0, "model-without-pricing-group")
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
	money := harnessSettlementMoneyForQuota(nil, 1_000_000, 0, "model-without-pricing-group")

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
	money := harnessSettlementMoneyForQuota(nil, -1, 0, "model-without-pricing-group")

	assert.Equal(t, "USD", money.Currency)
	assert.Equal(t, "0", money.BaseAmount)
	assert.Equal(t, "0", money.TotalAmount)
}

// TestHarnessProcurementBasisSurvivesEveryCustomerGroupRatio is the acceptance
// invariant of the procurement-basis work: with one usage quantity and one
// procurement configuration, changing only the customer's group ratio must not
// move either ledger amount. A zero ratio matters most -- it used to make the
// whole cost vanish, and it is the value that makes dividing by the ratio
// impossible, which is why the amount is captured instead of reconstructed.
func TestHarnessProcurementBasisSurvivesEveryCustomerGroupRatio(t *testing.T) {
	originalQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })
	common.QuotaPerUnit = 500_000

	channelId := seedHarnessLedgerChannel(t, "economy", "invariant-group", 0.8, 6.5)
	preGroup := decimal.NewFromInt(1_000_000)

	// Expected procurement cost for this one usage, independent of the caller.
	wantBase := "2"
	wantTotal := "1.704"

	for _, ratio := range []float64{1, 2, 0.9, 0.1, 0} {
		money := harnessSettlementMoneyForQuota(hosttypes.NewProcurementBasis(preGroup, ratio), 0, channelId, "economy")

		assert.Equal(t, wantBase, money.BaseAmount,
			"customer group ratio %v must not change the procurement base", ratio)
		assert.Equal(t, wantTotal, money.TotalAmount,
			"customer group ratio %v must not change the procurement total", ratio)
		assert.Equal(t, ratio, hosttypes.NewProcurementBasis(preGroup, ratio).CustomerGroupRatio,
			"the ratio stays recorded for reconciliation")
	}
}

// TestHarnessProcurementBasisKeepsProcurementInputsIndependent proves the fix
// stripped only the customer multiplier. The procurement group ratio, the
// channel tax rate and the pre-group amount must each still drive the result.
func TestHarnessProcurementBasisKeepsProcurementInputsIndependent(t *testing.T) {
	originalQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })
	common.QuotaPerUnit = 500_000

	preGroup := decimal.NewFromInt(1_000_000)

	// A doubled pre-group amount doubles both amounts: the model price still
	// reaches the ledger.
	doubled := harnessSettlementMoneyForQuota(
		hosttypes.NewProcurementBasis(decimal.NewFromInt(2_000_000), 1), 0, 0, "no-group-model")
	assert.Equal(t, "4", doubled.BaseAmount, "the pre-group amount must drive the base")

	// The procurement ratio and tax apply on their own, with no customer ratio.
	discounted := seedHarnessLedgerChannel(t, "economy", "procurement-only-group", 0.5, 10)
	money := harnessSettlementMoneyForQuota(
		hosttypes.NewProcurementBasis(preGroup, 1), 0, discounted, "economy")
	assert.Equal(t, "2", money.BaseAmount)
	assert.Equal(t, "1.1", money.TotalAmount,
		"0.5 procurement ratio with 10%% tax must still produce 1.1")
	assert.Equal(t, 0.5, money.GroupRatio)
	assert.Equal(t, 10.0, money.TaxRatePercent)
}
