package service

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seedHarnessLedgerChannel creates a channel whose ability exposes the pricing
// group for the model and registers that group as a procurement group with the
// given ratio, which is what harnessSettlementPricing resolves through.
func seedHarnessLedgerChannel(t *testing.T, modelName, group string, groupRatio, supplierTaxPercent float64) int {
	t.Helper()

	require.NoError(t, model.DB.AutoMigrate(&model.Channel{}, &model.Ability{}))

	originalRatioJSON := ratio_setting.GroupRatio2JSONString()
	t.Cleanup(func() { require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalRatioJSON)) })
	ratios := ratio_setting.GetGroupRatioCopy()
	ratios[group] = groupRatio
	encoded, err := common.Marshal(ratios)
	require.NoError(t, err)
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(string(encoded)))

	originalRouting := setting.GetIntelligentRoutingConfig()
	t.Cleanup(func() {
		// The validator rejects a nil global channel order, and an unconfigured
		// instance starts that way, so restore an empty non-nil list instead.
		restored := originalRouting
		if restored.GlobalChannelOrder == nil {
			restored.GlobalChannelOrder = []int{}
		}
		require.NoError(t, setting.UpdateIntelligentRoutingConfig(restored))
	})

	channel := &model.Channel{
		Name:          "ledger-tax-channel",
		Type:          1,
		Status:        common.ChannelStatusEnabled,
		Models:        modelName,
		Group:         group,
		OtherSettings: `{"supplier_tax_rate_percent":` + formatTestFloat(supplierTaxPercent) + `}`,
	}
	require.NoError(t, model.DB.Create(channel).Error)
	require.NoError(t, model.DB.Create(&model.Ability{
		Group:     group,
		Model:     modelName,
		ChannelId: channel.Id,
		Enabled:   true,
	}).Error)
	t.Cleanup(func() {
		model.DB.Exec("DELETE FROM channels WHERE id = ?", channel.Id)
		model.DB.Exec("DELETE FROM abilities WHERE channel_id = ?", channel.Id)
	})

	// The routing config validator requires a non-nil global channel order, so the
	// fixture registers its own channel there rather than leaving the list empty.
	routing := originalRouting
	routing.GlobalChannelOrder = []int{channel.Id}
	routing.PricingGroupOrder = append(append([]string{}, originalRouting.PricingGroupOrder...), group)
	require.NoError(t, setting.UpdateIntelligentRoutingConfig(routing))
	return channel.Id
}

// formatTestFloat renders a float for the JSON settings literal without an
// exponent or a trailing zero, which keeps the fixture readable.
func formatTestFloat(value float64) string {
	encoded, err := common.Marshal(value)
	if err != nil {
		return "0"
	}
	return string(encoded)
}

// normalizedDecimal trims insignificant trailing zeros so a fixed-precision
// decimal string can be compared against a readable expectation.
func normalizedDecimal(t *testing.T, value string) string {
	t.Helper()
	trimmed := strings.TrimRight(value, "0")
	trimmed = strings.TrimSuffix(trimmed, ".")
	if trimmed == "" {
		return "0"
	}
	return trimmed
}

// The tests here pin the three ledger invariants that a plain "the API returns
// data" test cannot catch: whose requests may enter the feed, that usage and cost
// corrections stay in their own stream, and that the cost amount follows the
// contract sample.

// TestLedgerSkipsRequestsThatDidNotComeFromHarness is the tenant-isolation
// guarantee. A settlement row is written for every relayed request because the
// operator needs the cost, but only platform-originated requests may appear in
// the ledger feed; otherwise Harness could read internal sk- users' usage and
// procurement cost.
func TestLedgerSkipsRequestsThatDidNotComeFromHarness(t *testing.T) {
	ledgerTestDB(t)

	originalQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })
	common.QuotaPerUnit = 500_000

	// No correlation ids: this is an internal sk- caller.
	internalSettlement := &model.HarnessSettlement{
		RequestId:    "gw-internal-sk",
		Status:       model.HarnessSettlementStatusSettled,
		ModelName:    "economy",
		PromptTokens: 5,
	}
	recordHarnessSettlement(internalSettlement,
		harnessSettlementMoneyForQuota(500_000, 0, "economy"),
		harnessSettlementCorrelation{UsageUnit: harnessUsageUnitToken, UsageSource: harnessUsageSourceUpstream})

	var leaked []model.HarnessLedgerEvent
	require.NoError(t, model.DB.Where("gateway_request_id = ?", "gw-internal-sk").Find(&leaked).Error)
	assert.Empty(t, leaked, "an internal request must not enter the platform ledger feed")

	// The snapshot still records the cost for the operator.
	var snapshots []model.HarnessSettlement
	require.NoError(t, model.DB.Where("request_id = ?", "gw-internal-sk").Find(&snapshots).Error)
	require.Len(t, snapshots, 1, "the operator still needs the cost snapshot")
	assert.Empty(t, snapshots[0].HarnessRequestId)

	// A platform request with an equivalent shape still produces both events.
	platformSettlement := &model.HarnessSettlement{
		RequestId:    "gw-platform",
		Status:       model.HarnessSettlementStatusSettled,
		ModelName:    "economy",
		PromptTokens: 5,
	}
	recordHarnessSettlement(platformSettlement,
		harnessSettlementMoneyForQuota(500_000, 0, "economy"),
		harnessSettlementCorrelation{HarnessRequestId: "hrq_platform", HarnessRunId: "run_platform", UsageUnit: harnessUsageUnitToken, UsageSource: harnessUsageSourceUpstream})

	var platformEvents []model.HarnessLedgerEvent
	require.NoError(t, model.DB.Where("gateway_request_id = ?", "gw-platform").Order("id ASC").Find(&platformEvents).Error)
	require.Len(t, platformEvents, 2, "a platform request emits one usage and one cost event")
}

// TestLedgerCorrectionChainsStayInTheirOwnStream guards the supersedes link: a
// usage correction must point at the previous usage event, never at a cost event,
// because the platform follows each stream independently.
func TestLedgerCorrectionChainsStayInTheirOwnStream(t *testing.T) {
	ledgerTestDB(t)

	originalQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })
	common.QuotaPerUnit = 500_000

	correlation := harnessSettlementCorrelation{
		HarnessRequestId: "hrq_chain",
		HarnessRunId:     "run_chain",
		UsageUnit:        harnessUsageUnitToken,
		UsageSource:      harnessUsageSourceUpstream,
	}

	settled := &model.HarnessSettlement{RequestId: "gw-chain", Status: model.HarnessSettlementStatusSettled, ModelName: "economy"}
	recordHarnessSettlement(settled, harnessSettlementMoneyForQuota(1_000_000, 0, "economy"), correlation)

	// A refund appends a correction in both streams.
	refunded := &model.HarnessSettlement{RequestId: "gw-chain", Status: model.HarnessSettlementStatusRefunded, ModelName: "economy"}
	recordHarnessSettlement(refunded, harnessSettlementMoneyForQuota(0, 0, "economy"), correlation)

	var usageEvents, costEvents []model.HarnessLedgerEvent
	require.NoError(t, model.DB.Where("gateway_request_id = ? AND stream = ?", "gw-chain", model.HarnessLedgerStreamUsage).Order("id ASC").Find(&usageEvents).Error)
	require.NoError(t, model.DB.Where("gateway_request_id = ? AND stream = ?", "gw-chain", model.HarnessLedgerStreamCost).Order("id ASC").Find(&costEvents).Error)
	require.Len(t, usageEvents, 2)
	require.Len(t, costEvents, 2)

	require.NotNil(t, usageEvents[1].SupersedesEventId)
	assert.Equal(t, usageEvents[0].EventId, *usageEvents[1].SupersedesEventId,
		"a usage correction must supersede the previous usage event")
	assert.Contains(t, *usageEvents[1].SupersedesEventId, model.HarnessLedgerStreamUsage+"_",
		"the superseded id must belong to the usage stream")
	assert.Equal(t, 2, usageEvents[1].RecordVersion)

	require.NotNil(t, costEvents[1].SupersedesEventId)
	assert.Equal(t, costEvents[0].EventId, *costEvents[1].SupersedesEventId,
		"a cost correction must supersede the previous cost event")
	assert.Contains(t, *costEvents[1].SupersedesEventId, model.HarnessLedgerStreamCost+"_",
		"the superseded id must belong to the cost stream")
	assert.Equal(t, 2, costEvents[1].RecordVersion)

	// The two chains advance independently: both settle at version 2 here, and a
	// third cost event must not be pushed to version 3 by a usage-only change.
	assert.Equal(t, usageEvents[1].RecordVersion, costEvents[1].RecordVersion)
}

// TestLedgerTotalAmountIncludesSupplierTax pins the cost formula against the
// contract sample: base 0.011500, group ratio 0.8 and 6.5% tax must yield 0.0098,
// not the untaxed 0.0092.
func TestLedgerTotalAmountIncludesSupplierTax(t *testing.T) {
	ledgerTestDB(t)

	originalQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })
	// quota 5750 / 500000 = 0.0115 base, matching the contract sample.
	common.QuotaPerUnit = 500_000

	channelId := seedHarnessLedgerChannel(t, "economy", "ledger-tax-group", 0.8, 6.5)

	money := harnessSettlementMoneyForQuota(5750, channelId, "economy")

	assert.Equal(t, "0.0115", normalizedDecimal(t, money.BaseAmount))
	assert.Equal(t, 0.8, money.GroupRatio)
	assert.Equal(t, 6.5, money.TaxRatePercent)
	assert.Equal(t, "0.009798", normalizedDecimal(t, money.TotalAmount),
		"total_amount must include the supplier tax, matching the contract sample")
}
