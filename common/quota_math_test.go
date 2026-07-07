package common

import (
	"math"
	"testing"

	"github.com/shopspring/decimal"
)

func TestQuotaFromFloatSaturates(t *testing.T) {
	tests := []struct {
		name string
		in   float64
		want int
	}{
		{name: "normal", in: 123.9, want: 123},
		{name: "nan", in: math.NaN(), want: 0},
		{name: "overflow", in: float64(math.MaxInt32) * 2, want: math.MaxInt32},
		{name: "underflow", in: float64(math.MinInt32) * 2, want: math.MinInt32},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := QuotaFromFloat(tt.in); got != tt.want {
				t.Fatalf("QuotaFromFloat(%v) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}

func TestQuotaFromDecimalRoundsAndSaturates(t *testing.T) {
	if got := QuotaFromDecimal(decimal.NewFromFloat(12.5)); got != 13 {
		t.Fatalf("QuotaFromDecimal(12.5) = %d, want 13", got)
	}
	if got := QuotaFromDecimal(decimal.NewFromInt(math.MaxInt64)); got != math.MaxInt32 {
		t.Fatalf("QuotaFromDecimal(MaxInt64) = %d, want %d", got, math.MaxInt32)
	}
}
