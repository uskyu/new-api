package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func blindBoxTestSetup(t *testing.T) {
	t.Helper()
	DB.Exec("DELETE FROM blind_boxes")
	DB.Exec("DELETE FROM blind_box_orders")
	DB.Exec("DELETE FROM users")
	t.Cleanup(func() {
		DB.Exec("DELETE FROM blind_boxes")
		DB.Exec("DELETE FROM blind_box_orders")
		DB.Exec("DELETE FROM users")
	})
}

func mustCreateBlindBox(t *testing.T, box *BlindBox) *BlindBox {
	t.Helper()
	require.NoError(t, CreateBlindBox(box))
	return box
}

func mustCreateUser(t *testing.T, name string) *User {
	t.Helper()
	user := User{Username: name, Password: "test-password"}
	require.NoError(t, DB.Create(&user).Error)
	return &user
}

func blindBoxBaseQuota(price float64) int {
	return BlindBoxBaseQuota(price)
}

func TestBlindBoxBaseQuota(t *testing.T) {
	// 售价是人民币；默认 1 USD = 7.3 CNY，1 USD = QuotaPerUnit 额度。
	assert.Equal(t, common.QuotaRound(common.QuotaPerUnit), BlindBoxBaseQuota(7.3))
	assert.Greater(t, BlindBoxBaseQuota(10), 0)
}

func TestValidateBlindBox(t *testing.T) {
	valid := &BlindBox{
		Name:         "  Lucky Box  ",
		Price:        50,
		StockTotal:   10,
		PerUserLimit: 2,
		Tiers:        `[{"min_multiplier":1,"max_multiplier":1.05,"probability":70},{"min_multiplier":1.05,"max_multiplier":1.2,"probability":30}]`,
	}
	require.NoError(t, ValidateBlindBox(valid))
	assert.Equal(t, "Lucky Box", valid.Name)

	cases := []struct {
		name string
		box  *BlindBox
	}{
		{
			name: "empty name",
			box:  &BlindBox{Name: " ", Price: 10, Tiers: `[{"min_multiplier":1,"max_multiplier":1,"probability":100}]`},
		},
		{
			name: "non positive price",
			box:  &BlindBox{Name: "box", Price: 0, Tiers: `[{"min_multiplier":1,"max_multiplier":1,"probability":100}]`},
		},
		{
			name: "probability not 100",
			box:  &BlindBox{Name: "box", Price: 10, Tiers: `[{"min_multiplier":1,"max_multiplier":1,"probability":60}]`},
		},
		{
			name: "max below min",
			box:  &BlindBox{Name: "box", Price: 10, Tiers: `[{"min_multiplier":2,"max_multiplier":1,"probability":100}]`},
		},
		{
			name: "negative stock",
			box:  &BlindBox{Name: "box", Price: 10, StockTotal: -1, Tiers: `[{"min_multiplier":1,"max_multiplier":1,"probability":100}]`},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Error(t, ValidateBlindBox(tc.box))
		})
	}
}

func TestDrawBlindBoxTierDistribution(t *testing.T) {
	tiers := []BlindBoxTier{
		{MinMultiplier: 1.0, MaxMultiplier: 1.05, Probability: 50},
		{MinMultiplier: 1.05, MaxMultiplier: 1.2, Probability: 30},
		{MinMultiplier: 1.2, MaxMultiplier: 1.5, Probability: 20},
	}
	counts := map[int]int{}
	const samples = 1000
	for i := 0; i < samples; i++ {
		tier, err := drawBlindBoxTier(tiers)
		require.NoError(t, err)
		for index := range tiers {
			if tier == &tiers[index] {
				counts[index]++
			}
		}
	}
	expected := []float64{50, 30, 20}
	for index, want := range expected {
		got := float64(counts[index]) * 100 / samples
		assert.InDelta(t, want, got, 8, "tier %d distribution", index)
	}
}

func TestSettleBlindBoxOrderIdempotent(t *testing.T) {
	blindBoxTestSetup(t)
	user := mustCreateUser(t, "blind-box-user-1")
	box := mustCreateBlindBox(t, &BlindBox{
		Name:         "One Tier",
		Price:        10,
		StockTotal:   1,
		PerUserLimit: 1,
		Enabled:      true,
		Tiers:        `[{"min_multiplier":1,"max_multiplier":1.5,"probability":100}]`,
	})
	baseQuota := blindBoxBaseQuota(box.Price)
	order := &BlindBoxOrder{
		TradeNo:         "BBX-IDEMPOTENT",
		UserId:          user.Id,
		BoxId:           box.Id,
		BoxName:         box.Name,
		CycleId:         1001,
		Price:           box.Price,
		BaseQuota:       baseQuota,
		TiersSnapshot:   box.Tiers,
		Status:          common.TopUpStatusPending,
		PaymentProvider: PaymentProviderEpay,
	}
	require.NoError(t, CreateBlindBoxOrder(order))

	settled, already, err := SettleBlindBoxOrder(order.TradeNo)
	require.NoError(t, err)
	assert.False(t, already)
	assert.Equal(t, common.TopUpStatusSuccess, settled.Status)
	assert.True(t, settled.DrawnMultiplier >= 1.0 && settled.DrawnMultiplier <= 1.5)
	assert.Equal(t, common.QuotaRound(float64(baseQuota)*settled.DrawnMultiplier), settled.RewardQuota)

	var afterFirst User
	require.NoError(t, DB.First(&afterFirst, user.Id).Error)
	assert.Equal(t, settled.RewardQuota, afterFirst.Quota)
	var storedBox BlindBox
	require.NoError(t, DB.First(&storedBox, box.Id).Error)
	assert.Equal(t, 1, storedBox.SoldCount)

	_, already, err = SettleBlindBoxOrder(order.TradeNo)
	require.NoError(t, err)
	assert.True(t, already)

	var afterSecond User
	require.NoError(t, DB.First(&afterSecond, user.Id).Error)
	assert.Equal(t, afterFirst.Quota, afterSecond.Quota)
	var storedBoxAgain BlindBox
	require.NoError(t, DB.First(&storedBoxAgain, box.Id).Error)
	assert.Equal(t, 1, storedBoxAgain.SoldCount)
}

func TestSettleBlindBoxOversellAndOverLimit(t *testing.T) {
	blindBoxTestSetup(t)
	user := mustCreateUser(t, "blind-box-user-2")
	box := mustCreateBlindBox(t, &BlindBox{
		Name:         "Limited",
		Price:        10,
		StockTotal:   1,
		PerUserLimit: 1,
		Enabled:      true,
		Tiers:        `[{"min_multiplier":1,"max_multiplier":1,"probability":100}]`,
	})
	for _, tradeNo := range []string{"BBX-FIRST", "BBX-SECOND"} {
		require.NoError(t, CreateBlindBoxOrder(&BlindBoxOrder{
			TradeNo:       tradeNo,
			UserId:        user.Id,
			BoxId:         box.Id,
			BoxName:       box.Name,
			CycleId:       2002,
			Price:         box.Price,
			BaseQuota:     blindBoxBaseQuota(box.Price),
			TiersSnapshot: box.Tiers,
			Status:        common.TopUpStatusPending,
		}))
	}

	first, _, err := SettleBlindBoxOrder("BBX-FIRST")
	require.NoError(t, err)
	assert.False(t, first.Oversold)
	assert.False(t, first.OverLimit)

	second, _, err := SettleBlindBoxOrder("BBX-SECOND")
	require.NoError(t, err)
	assert.True(t, second.Oversold)
	assert.True(t, second.OverLimit)

	var storedBox BlindBox
	require.NoError(t, DB.First(&storedBox, box.Id).Error)
	assert.Equal(t, 2, storedBox.SoldCount)
}

func TestExpirePendingBlindBoxOrders(t *testing.T) {
	blindBoxTestSetup(t)
	now := common.GetTimestamp()
	require.NoError(t, CreateBlindBoxOrder(&BlindBoxOrder{
		TradeNo:    "BBX-OLD",
		UserId:     1,
		BoxId:      1,
		Status:     common.TopUpStatusPending,
		CreateTime: now - 20*60,
	}))
	require.NoError(t, CreateBlindBoxOrder(&BlindBoxOrder{
		TradeNo:    "BBX-NEW",
		UserId:     1,
		BoxId:      1,
		Status:     common.TopUpStatusPending,
		CreateTime: now - 60,
	}))

	expired, err := ExpirePendingBlindBoxOrders(15)
	require.NoError(t, err)
	assert.Equal(t, int64(1), expired)

	var oldOrder BlindBoxOrder
	require.NoError(t, DB.Where("trade_no = ?", "BBX-OLD").First(&oldOrder).Error)
	assert.Equal(t, common.TopUpStatusExpired, oldOrder.Status)

	var newOrder BlindBoxOrder
	require.NoError(t, DB.Where("trade_no = ?", "BBX-NEW").First(&newOrder).Error)
	assert.Equal(t, common.TopUpStatusPending, newOrder.Status)
}
