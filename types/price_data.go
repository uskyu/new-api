package types

import (
	"fmt"
	"math"

	"github.com/shopspring/decimal"
)

type GroupRatioInfo struct {
	GroupRatio        float64
	GroupSpecialRatio float64
	HasSpecialRatio   bool
}

// ProcurementBasis is the billing amount captured before the customer-side
// group ratio is applied. The procurement ledger must not inherit that ratio:
// the same model, usage and procurement configuration must cost the operator the
// same money whoever the caller is. The billing paths therefore expose the
// pre-group decimal instead of letting the ledger divide a rounded integer by
// the ratio, which would lose precision to quota rounding and break outright on
// a zero ratio.
//
// The amount deliberately still carries model price, unit ratios (cache, image,
// audio), the caller-supplied request multipliers (n, seconds, size) and the
// tiered expression result: those describe the purchase itself. It excludes the
// customer group ratio and the tool-call surcharge, which the operator sets as
// customer-facing pricing rather than a supplier cost.
type ProcurementBasis struct {
	// PreGroupQuota is the billable amount before the customer group ratio, in
	// quota units, as a decimal string. A string keeps the full precision the
	// ledger stores; float64 would round at 1e-16.
	PreGroupQuota string `json:"pre_group_quota"`
	// CustomerGroupRatio is the ratio that was lifted out of PreGroupQuota. It is
	// recorded for reconciliation only and never multiplies a procurement amount.
	CustomerGroupRatio float64 `json:"customer_group_ratio"`
}

// NewProcurementBasis records the pre-group amount produced by a billing path.
// The decimal is kept as its exact string form so no precision is lost between
// the calculation and the ledger write.
func NewProcurementBasis(preGroupQuota decimal.Decimal, customerGroupRatio float64) *ProcurementBasis {
	return &ProcurementBasis{
		PreGroupQuota:      preGroupQuota.String(),
		CustomerGroupRatio: customerGroupRatio,
	}
}

// PreGroupQuotaDecimal parses PreGroupQuota. ok is false when the basis is
// absent or carries an unparseable amount, so a caller falls back instead of
// booking a zero cost.
func (b *ProcurementBasis) PreGroupQuotaDecimal() (decimal.Decimal, bool) {
	if b == nil || b.PreGroupQuota == "" {
		return decimal.Zero, false
	}
	value, err := decimal.NewFromString(b.PreGroupQuota)
	if err != nil {
		return decimal.Zero, false
	}
	return value, true
}

type PriceData struct {
	FreeModel            bool
	ModelPrice           float64
	ModelRatio           float64
	CompletionRatio      float64
	CacheRatio           float64
	CacheCreationRatio   float64
	CacheCreation5mRatio float64
	CacheCreation1hRatio float64
	ImageRatio           float64
	AudioRatio           float64
	AudioCompletionRatio float64
	otherRatios          map[string]float64
	UsePrice             bool
	Quota                int // 按次计费的最终额度（MJ / Task）
	QuotaToPreConsume    int // 按量计费的预消耗额度
	GroupRatioInfo       GroupRatioInfo
}

func (p *PriceData) AddOtherRatio(key string, ratio float64) {
	if !isValidOtherRatio(ratio) {
		return
	}
	if p.otherRatios == nil {
		p.otherRatios = make(map[string]float64)
	}
	p.otherRatios[key] = ratio
}

func (p *PriceData) ReplaceOtherRatios(ratios map[string]float64) bool {
	p.otherRatios = nil
	for key, ratio := range ratios {
		p.AddOtherRatio(key, ratio)
	}
	return len(p.otherRatios) > 0
}

func (p *PriceData) HasOtherRatio(key string) bool {
	ratio, ok := p.otherRatios[key]
	return ok && isValidOtherRatio(ratio)
}

func (p *PriceData) OtherRatios() map[string]float64 {
	if len(p.otherRatios) == 0 {
		return nil
	}
	ratios := make(map[string]float64, len(p.otherRatios))
	for key, ratio := range p.otherRatios {
		if isValidOtherRatio(ratio) {
			ratios[key] = ratio
		}
	}
	if len(ratios) == 0 {
		return nil
	}
	return ratios
}

func (p *PriceData) OtherRatioMultiplier() float64 {
	multiplier := 1.0
	for _, ratio := range p.otherRatios {
		if isValidOtherRatio(ratio) && ratio != 1.0 {
			multiplier *= ratio
		}
	}
	return multiplier
}

func (p *PriceData) ApplyOtherRatiosToFloat(value float64) float64 {
	return value * p.OtherRatioMultiplier()
}

func (p *PriceData) ApplyOtherRatiosToDecimal(value decimal.Decimal) decimal.Decimal {
	for _, ratio := range p.otherRatios {
		if isValidOtherRatio(ratio) && ratio != 1.0 {
			value = value.Mul(decimal.NewFromFloat(ratio))
		}
	}
	return value
}

func (p *PriceData) RemoveOtherRatiosFromFloat(value float64) float64 {
	for _, ratio := range p.otherRatios {
		if isValidOtherRatio(ratio) && ratio != 1.0 {
			value /= ratio
		}
	}
	return value
}

func isValidOtherRatio(ratio float64) bool {
	return ratio > 0 && !math.IsInf(ratio, 1)
}

func (p *PriceData) ToSetting() string {
	return fmt.Sprintf("ModelPrice: %f, ModelRatio: %f, CompletionRatio: %f, CacheRatio: %f, GroupRatio: %f, UsePrice: %t, CacheCreationRatio: %f, CacheCreation5mRatio: %f, CacheCreation1hRatio: %f, QuotaToPreConsume: %d, ImageRatio: %f, AudioRatio: %f, AudioCompletionRatio: %f", p.ModelPrice, p.ModelRatio, p.CompletionRatio, p.CacheRatio, p.GroupRatioInfo.GroupRatio, p.UsePrice, p.CacheCreationRatio, p.CacheCreation5mRatio, p.CacheCreation1hRatio, p.QuotaToPreConsume, p.ImageRatio, p.AudioRatio, p.AudioCompletionRatio)
}
