package model

import (
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"gorm.io/gorm"
)

// Checkin 签到记录
type Checkin struct {
	Id           int    `json:"id" gorm:"primaryKey;autoIncrement"`
	UserId       int    `json:"user_id" gorm:"not null;uniqueIndex:idx_user_checkin_date"`
	CheckinDate  string `json:"checkin_date" gorm:"type:varchar(10);not null;uniqueIndex:idx_user_checkin_date"` // 格式: YYYY-MM-DD
	QuotaAwarded int    `json:"quota_awarded" gorm:"not null"`
	CreatedAt    int64  `json:"created_at" gorm:"bigint"`
}

// CheckinRecord 用于API返回的签到记录（不包含敏感字段）
type CheckinRecord struct {
	CheckinDate  string `json:"checkin_date"`
	QuotaAwarded int    `json:"quota_awarded"`
}

func (Checkin) TableName() string {
	return "checkins"
}

// GetUserCheckinRecords 获取用户在指定日期范围内的签到记录
func GetUserCheckinRecords(userId int, startDate, endDate string) ([]Checkin, error) {
	var records []Checkin
	err := DB.Where("user_id = ? AND checkin_date >= ? AND checkin_date <= ?",
		userId, startDate, endDate).
		Order("checkin_date DESC").
		Find(&records).Error
	return records, err
}

// HasCheckedInToday 检查用户今天是否已签到
func HasCheckedInToday(userId int) (bool, error) {
	today := time.Now().Format("2006-01-02")
	var count int64
	err := DB.Model(&Checkin{}).
		Where("user_id = ? AND checkin_date = ?", userId, today).
		Count(&count).Error
	return count > 0, err
}

// GetYesterdayCheckinUsage 统计用户昨日成功计费调用次数和消耗额度
func GetYesterdayCheckinUsage(userId int) (calls int64, quota int64, err error) {
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	start := startOfDay.Add(-24 * time.Hour).Unix()
	end := startOfDay.Unix()
	err = LOG_DB.Model(&Log{}).
		Where("user_id = ? AND type = ? AND created_at >= ? AND created_at < ?", userId, LogTypeConsume, start, end).
		Count(&calls).Error
	if err != nil {
		return 0, 0, err
	}
	err = LOG_DB.Model(&Log{}).
		Where("user_id = ? AND type = ? AND created_at >= ? AND created_at < ?", userId, LogTypeConsume, start, end).
		Select("COALESCE(SUM(quota), 0)").Scan(&quota).Error
	return calls, quota, err
}

// SelectCheckinTier 返回用户命中的最高活跃奖励档位
func SelectCheckinTier(tiers []operation_setting.CheckinBonusTier, metric int64) *operation_setting.CheckinBonusTier {
	var selected *operation_setting.CheckinBonusTier
	for i := range tiers {
		tier := &tiers[i]
		if int64(tier.Threshold) > metric {
			continue
		}
		if selected == nil || tier.Threshold > selected.Threshold {
			selected = tier
		}
	}
	return selected
}

func selectCheckinReward(setting *operation_setting.CheckinSetting, metric int64) int {
	quotaAwarded := setting.MinQuota
	if setting.MaxQuota > setting.MinQuota {
		quotaAwarded = setting.MinQuota + rand.Intn(setting.MaxQuota-setting.MinQuota+1)
	}
	if !setting.BonusEnabled {
		return quotaAwarded
	}
	tier := SelectCheckinTier(setting.ActiveTiers(), metric)
	if tier == nil || tier.MaxQuota < tier.MinQuota {
		return quotaAwarded
	}
	if tier.MinQuota == tier.MaxQuota {
		return tier.MinQuota
	}
	return tier.MinQuota + rand.Intn(tier.MaxQuota-tier.MinQuota+1)
}

// GetUserEpayRechargeAmount 返回用户 EPAY 渠道累计成功充值金额，单位：元。
//
// 【amount 语义】top_ups.amount 恒为「金额单位」，在两种展示模式下都成立：
//   - 下单时 controller/topup.go 的 RequestEpay 会把 amount 写入订单，其中
//     QuotaDisplayTypeTokens 模式下 amount 被除以 QuotaPerUnit（见该文件
//     RequestEpay 中对 amount 的归一化），使落库值始终是金额而非 token 数；
//   - 支付成功回调 EpayNotify 用 quotaToAdd = amount * QuotaPerUnit 把额度
//     发给用户，这个乘法再次把 TOKENS 模式的除法抵消，说明库里存的就是金额。
//
// 因此这里直接 SUM(amount) 得到的就是「元」，无需再按 display type 换算。
//
// 【局限】该口径只统计 EPAY（boss 明确以 EPAY 为主要充值渠道）；其他渠道
// （stripe/creem/waffo/balance）的充值不计入。历史上若有订单是在
// QuotaDisplayType 切换之前写入的，其 amount 可能带有旧的舍入误差
// （IntPart() 截断），但不影响「是否充值过」的判定。
func GetUserEpayRechargeAmount(userId int) (float64, error) {
	if userId <= 0 {
		return 0, nil
	}
	var total float64
	err := DB.Model(&TopUp{}).
		Where("user_id = ? AND status = ? AND payment_provider = ?",
			userId, common.TopUpStatusSuccess, PaymentProviderEpay).
		Select("COALESCE(SUM(amount), 0)").
		Scan(&total).Error
	if err != nil {
		return 0, err
	}
	return total, nil
}

// GetUserLastApiAccessedTime 返回用户所有令牌中最新的 accessed_time；无令牌返回 0。
//
// 【数据源】tokens.accessed_time（tokens.user_id 有索引，单用户查询成本可接受）。
// 不新增表、不给 users 加列、不依赖 logs 表（boss 明确要求）。
//
// 【精度边界】accessed_time 只在「实际扣减/返还令牌额度」时刷新
// （increaseTokenQuota / decreaseTokenQuota）。以下情况不会刷新：
//   - playground 调用；
//   - 结算 delta == 0（未产生实际额度变动）；
//   - 计费前就失败（鉴权失败、额度不足、上游错误）。
//
// 另外 common.BatchUpdateEnabled = true 时走批量更新，最多滞后一个批次
// 间隔（默认 5s）。对本功能目标（识别从不调用的批量注册号）足够。
//
// 无令牌（用户删光了令牌）返回 0，视为「从未调用」。
func GetUserLastApiAccessedTime(userId int) (int64, error) {
	if userId <= 0 {
		return 0, nil
	}
	var lastAccessedAt int64
	err := DB.Model(&Token{}).
		Where("user_id = ?", userId).
		Select("COALESCE(MAX(accessed_time), 0)").
		Scan(&lastAccessedAt).Error
	if err != nil {
		return 0, err
	}
	return lastAccessedAt, nil
}

// 签到被拒的原因常量，供前端区分展示文案（对应 eligibility.reason）。
const (
	CheckinReasonNone             = ""
	CheckinReasonRechargeRequired = "recharge_required"
	CheckinReasonInactive         = "inactive"
)

// CheckinEligibility 是签到资格判定的完整结果，供 GetCheckinStatus 直接返回。
// 注意 LastApiUsedAt 仅在 InactiveDays > 0（启用不活跃限制）时才查询填充，
// 未启用时为 0，避免给「功能关闭」的默认部署增加一次无谓查询。
type CheckinEligibility struct {
	CanCheckin         bool    `json:"can_checkin"`
	Reason             string  `json:"reason"` // "" | "recharge_required" | "inactive"
	MinRechargeAmount  float64 `json:"min_recharge_amount"`
	UserRechargeAmount float64 `json:"user_recharge_amount"`
	InactiveDays       int     `json:"inactive_days"`
	LastApiUsedAt      int64   `json:"last_api_used_at"`
}

// GetCheckinEligibility 计算用户当前是否具备签到资格（只读，不产生副作用）。
func GetCheckinEligibility(userId int, setting *operation_setting.CheckinSetting) (*CheckinEligibility, error) {
	return buildCheckinEligibility(userId, setting, common.GetTimestamp())
}

// buildCheckinEligibility 是 GetCheckinEligibility 的实现，now 显式传入便于测试。
func buildCheckinEligibility(userId int, setting *operation_setting.CheckinSetting, now int64) (*CheckinEligibility, error) {
	if setting == nil {
		setting = operation_setting.GetCheckinSetting()
	}
	eligibility := &CheckinEligibility{
		CanCheckin:        true,
		Reason:            CheckinReasonNone,
		MinRechargeAmount: setting.MinRechargeAmount,
		InactiveDays:      setting.InactiveDays,
	}

	// 累计 EPAY 充值金额：同时用于「充值门槛」与「不活跃解锁」两个判断。
	rechargeAmount, err := GetUserEpayRechargeAmount(userId)
	if err != nil {
		return nil, err
	}
	eligibility.UserRechargeAmount = rechargeAmount

	// 1) 充值门槛：MinRechargeAmount > 0 时才限制。
	//    恰好等于门槛视为达成（用 < 判断）。
	if setting.MinRechargeAmount > 0 && rechargeAmount < setting.MinRechargeAmount {
		eligibility.CanCheckin = false
		eligibility.Reason = CheckinReasonRechargeRequired
		return eligibility, nil
	}

	// 2) 不活跃限制：InactiveDays > 0 时生效。
	if setting.InactiveDays <= 0 {
		return eligibility, nil
	}

	lastAccessedAt, err := GetUserLastApiAccessedTime(userId)
	if err != nil {
		return nil, err
	}
	eligibility.LastApiUsedAt = lastAccessedAt

	// 恢复规则（boss 原话）：
	//   - "他充值过之后才能够解锁签到功能" → 累计充值过（> 0）即视为已解锁，
	//     不受不活跃限制。注意这是「累计充值过」而非「最近 N 天内充值」，
	//     充值是一次性解锁凭证。
	//   - "如果签到功能又被封禁了，就需要调用一次 API 才能够恢复" → 最近 N 天内
	//     调用过 API 也可通过（lastAccessedAt 在阈值内）。
	// 两者任一满足即通过。
	if rechargeAmount > 0 {
		return eligibility, nil
	}

	// 从未充值：要求最近 N 天内有 API 调用记录。
	// lastAccessedAt == 0（从未调用 / 无令牌）必然超期，会被判为不活跃。
	inactiveCutoff := now - int64(setting.InactiveDays)*86400
	if lastAccessedAt < inactiveCutoff {
		eligibility.CanCheckin = false
		eligibility.Reason = CheckinReasonInactive
	}
	return eligibility, nil
}

// checkCheckinEligibility 返回 nil 表示可签到；否则返回用户可读错误。
// 依次判断：充值门槛 → 不活跃。
func checkCheckinEligibility(userId int, setting *operation_setting.CheckinSetting) error {
	if setting == nil {
		setting = operation_setting.GetCheckinSetting()
	}
	eligibility, err := buildCheckinEligibility(userId, setting, common.GetTimestamp())
	if err != nil {
		return err
	}
	switch eligibility.Reason {
	case CheckinReasonRechargeRequired:
		shortfall := setting.MinRechargeAmount - eligibility.UserRechargeAmount
		if shortfall < 0 {
			shortfall = 0
		}
		return fmt.Errorf("签到需先充值满 %s 元，当前累计充值 %s 元，还差 %s 元",
			formatCheckinAmount(setting.MinRechargeAmount),
			formatCheckinAmount(eligibility.UserRechargeAmount),
			formatCheckinAmount(shortfall))
	case CheckinReasonInactive:
		return fmt.Errorf("已超过 %d 天未调用 API，签到已暂停；调用一次 API 或充值后即可恢复",
			setting.InactiveDays)
	default:
		return nil
	}
}

// formatCheckinAmount 把金额格式化为最多两位小数、去掉无意义的尾随零，
// 便于错误提示阅读（如 10 → "10"，0.5 → "0.5"）。
func formatCheckinAmount(amount float64) string {
	return strconv.FormatFloat(amount, 'f', -1, 64)
}

// UserCheckin 执行用户签到
// MySQL 和 PostgreSQL 使用事务保证原子性
// SQLite 不支持嵌套事务，使用顺序操作 + 手动回滚
func UserCheckin(userId int) (*Checkin, error) {
	setting := operation_setting.GetCheckinSetting()
	if !setting.Enabled {
		return nil, errors.New("签到功能未启用")
	}

	// 检查今天是否已签到
	hasChecked, err := HasCheckedInToday(userId)
	if err != nil {
		return nil, err
	}
	if hasChecked {
		return nil, errors.New("今日已签到")
	}

	// 签到资格门槛（充值门槛 + 不活跃停签到），必须在写入任何签到记录之前判定。
	if err := checkCheckinEligibility(userId, setting); err != nil {
		return nil, err
	}

	// 计算随机额度奖励，命中活跃阶梯时使用对应奖励区间
	quotaAwarded := setting.MinQuota
	if setting.MaxQuota > setting.MinQuota {
		quotaAwarded = setting.MinQuota + rand.Intn(setting.MaxQuota-setting.MinQuota+1)
	}
	if setting.BonusEnabled && len(setting.ActiveTiers()) > 0 {
		calls, consumedQuota, usageErr := GetYesterdayCheckinUsage(userId)
		if usageErr == nil {
			metric := calls
			if setting.BonusMetric == "quota_consumed" {
				metric = consumedQuota
			}
			quotaAwarded = selectCheckinReward(setting, metric)
		}
	}

	today := time.Now().Format("2006-01-02")
	checkin := &Checkin{
		UserId:       userId,
		CheckinDate:  today,
		QuotaAwarded: quotaAwarded,
		CreatedAt:    time.Now().Unix(),
	}

	// 根据数据库类型选择不同的策略
	if common.UsingSQLite {
		// SQLite 不支持嵌套事务，使用顺序操作 + 手动回滚
		return userCheckinWithoutTransaction(checkin, userId, quotaAwarded)
	}

	// MySQL 和 PostgreSQL 支持事务，使用事务保证原子性
	return userCheckinWithTransaction(checkin, userId, quotaAwarded)
}

// userCheckinWithTransaction 使用事务执行签到（适用于 MySQL 和 PostgreSQL）
func userCheckinWithTransaction(checkin *Checkin, userId int, quotaAwarded int) (*Checkin, error) {
	err := DB.Transaction(func(tx *gorm.DB) error {
		// 步骤1: 创建签到记录
		// 数据库有唯一约束 (user_id, checkin_date)，可以防止并发重复签到
		if err := tx.Create(checkin).Error; err != nil {
			return errors.New("签到失败，请稍后重试")
		}

		// 步骤2: 在事务中增加用户额度
		if err := tx.Model(&User{}).Where("id = ?", userId).
			Update("quota", gorm.Expr("quota + ?", quotaAwarded)).Error; err != nil {
			return errors.New("签到失败：更新额度出错")
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// 事务成功后，异步更新缓存
	go func() {
		_ = cacheIncrUserQuota(userId, int64(quotaAwarded))
	}()

	return checkin, nil
}

// userCheckinWithoutTransaction 不使用事务执行签到（适用于 SQLite）
func userCheckinWithoutTransaction(checkin *Checkin, userId int, quotaAwarded int) (*Checkin, error) {
	// 步骤1: 创建签到记录
	// 数据库有唯一约束 (user_id, checkin_date)，可以防止并发重复签到
	if err := DB.Create(checkin).Error; err != nil {
		return nil, errors.New("签到失败，请稍后重试")
	}

	// 步骤2: 增加用户额度
	// 使用 db=true 强制直接写入数据库，不使用批量更新
	if err := IncreaseUserQuota(userId, quotaAwarded, true); err != nil {
		// 如果增加额度失败，需要回滚签到记录
		DB.Delete(checkin)
		return nil, errors.New("签到失败：更新额度出错")
	}

	return checkin, nil
}

// GetUserCheckinStats 获取用户签到统计信息
func GetUserCheckinStats(userId int, month string) (map[string]interface{}, error) {
	// 获取指定月份的所有签到记录
	startDate := month + "-01"
	endDate := month + "-31"

	records, err := GetUserCheckinRecords(userId, startDate, endDate)
	if err != nil {
		return nil, err
	}

	// 转换为不包含敏感字段的记录
	checkinRecords := make([]CheckinRecord, len(records))
	for i, r := range records {
		checkinRecords[i] = CheckinRecord{
			CheckinDate:  r.CheckinDate,
			QuotaAwarded: r.QuotaAwarded,
		}
	}

	// 检查今天是否已签到
	hasCheckedToday, _ := HasCheckedInToday(userId)

	// 获取用户所有时间的签到统计
	var totalCheckins int64
	var totalQuota int64
	DB.Model(&Checkin{}).Where("user_id = ?", userId).Count(&totalCheckins)
	DB.Model(&Checkin{}).Where("user_id = ?", userId).Select("COALESCE(SUM(quota_awarded), 0)").Scan(&totalQuota)

	return map[string]interface{}{
		"total_quota":      totalQuota,      // 所有时间累计获得的额度
		"total_checkins":   totalCheckins,   // 所有时间累计签到次数
		"checkin_count":    len(records),    // 本月签到次数
		"checked_in_today": hasCheckedToday, // 今天是否已签到
		"records":          checkinRecords,  // 本月签到记录详情（不含id和user_id）
	}, nil
}
