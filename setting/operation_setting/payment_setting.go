package operation_setting

import "github.com/QuantumNous/new-api/setting/config"

const (
	DefaultUserBillVisibleDays = 30
	MinUserBillVisibleDays     = 1
	MaxUserBillVisibleDays     = 3650
)

type PaymentSetting struct {
	AmountOptions       []int           `json:"amount_options"`
	AmountDiscount      map[int]float64 `json:"amount_discount"` // 充值金额对应的折扣，例如 100 元 0.9 表示 100 元充值享受 9 折优惠
	UserBillVisibleDays int             `json:"user_bill_visible_days"`
}

// 默认配置
var paymentSetting = PaymentSetting{
	AmountOptions:       []int{10, 20, 50, 100, 200, 500},
	AmountDiscount:      map[int]float64{},
	UserBillVisibleDays: DefaultUserBillVisibleDays,
}

func init() {
	// 注册到全局配置管理器
	config.GlobalConfig.Register("payment_setting", &paymentSetting)
}

func GetPaymentSetting() *PaymentSetting {
	return &paymentSetting
}

func GetUserBillVisibleDays() int {
	days := paymentSetting.UserBillVisibleDays
	if days < MinUserBillVisibleDays || days > MaxUserBillVisibleDays {
		return DefaultUserBillVisibleDays
	}
	return days
}
