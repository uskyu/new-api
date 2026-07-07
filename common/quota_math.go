package common

import (
	"math"

	"github.com/shopspring/decimal"
)

// Quota columns (user/token/log) are 32-bit integers in the database, so an
// oversized product must clamp to the int32 range instead of wrapping around
// and turning a charge into a credit.
const (
	MaxQuota = math.MaxInt32
	MinQuota = math.MinInt32
)

// QuotaFromFloat converts a computed quota value to int, truncating toward
// zero, with saturation. Use for float products of prices, ratios, and
// user-controlled multipliers (image n, video seconds, resolution ratios).
func QuotaFromFloat(value float64) int {
	if math.IsNaN(value) {
		return 0
	}
	if value >= MaxQuota {
		return MaxQuota
	}
	if value <= MinQuota {
		return MinQuota
	}
	return int(value)
}

// QuotaRound converts a float64 quota value to int using half-away-from-zero
// rounding, with saturation. Every tiered billing path (pre-consume,
// settlement, breakdown validation, log fields) MUST use this function to
// avoid +-1 discrepancies.
func QuotaRound(value float64) int {
	r := math.Round(value)
	if math.IsNaN(r) {
		return 0
	}
	if r >= MaxQuota {
		return MaxQuota
	}
	if r <= MinQuota {
		return MinQuota
	}
	return int(r)
}

// QuotaFromDecimal converts a computed quota decimal to int with saturation.
// The decimal is rounded (half away from zero) before conversion.
func QuotaFromDecimal(d decimal.Decimal) int {
	f, _ := d.Round(0).Float64()
	return QuotaRound(f)
}
