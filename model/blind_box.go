package model

import (
	crand "crypto/rand"
	"errors"
	"fmt"
	"math"
	"math/big"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// 盲盒配置的边界值，防止异常输入写入数据库。
const (
	BlindBoxMaxPrice        = 1000000.0
	BlindBoxMaxStock        = 1000000
	BlindBoxMaxPerUserLimit = 1000000
	BlindBoxMaxTiers        = 100
	BlindBoxDefaultTimeout  = 15
)

var (
	ErrBlindBoxNotFound           = errors.New("盲盒不存在")
	ErrBlindBoxOrderNotFound      = errors.New("盲盒订单不存在")
	ErrBlindBoxOrderStatusInvalid = errors.New("盲盒订单状态错误")
)

// BlindBoxTier 盲盒倍率档位。
type BlindBoxTier struct {
	MinMultiplier float64 `json:"min_multiplier"`
	MaxMultiplier float64 `json:"max_multiplier"`
	Probability   float64 `json:"probability"` // 百分数 0~100，全部合计必须 = 100
}

// BlindBox 盲盒商品。
type BlindBox struct {
	Id           int     `json:"id"`
	Name         string  `json:"name" gorm:"type:varchar(64);not null"`
	Price        float64 `json:"price"` // 单位：元，保留两位
	StockTotal   int     `json:"stock_total"`
	SoldCount    int     `json:"sold_count"`
	PerUserLimit int     `json:"per_user_limit"`
	Enabled      bool    `json:"enabled"`
	Sort         int     `json:"sort"`
	Tiers        string  `json:"tiers" gorm:"type:text"` // JSON 数组
	Remark       string  `json:"remark" gorm:"type:text"`
	CreateTime   int64   `json:"create_time"`
	UpdateTime   int64   `json:"update_time"`
}

// BlindBoxOrder 盲盒订单。
type BlindBoxOrder struct {
	Id                int     `json:"id"`
	TradeNo           string  `json:"trade_no" gorm:"unique;type:varchar(255);index"`
	UserId            int     `json:"user_id" gorm:"index"`
	BoxId             int     `json:"box_id" gorm:"index"`
	BoxName           string  `json:"box_name" gorm:"type:varchar(64)"` // 下单快照
	CycleId           int64   `json:"cycle_id" gorm:"index"`
	Price             float64 `json:"price"`      // 下单快照
	BaseQuota         int     `json:"base_quota"` // 下单时 price × QuotaPerUnit
	TiersSnapshot     string  `json:"-" gorm:"type:text"`
	Status            string  `json:"status" gorm:"type:varchar(20)"`
	PaymentMethod     string  `json:"payment_method" gorm:"type:varchar(50)"`
	PaymentProvider   string  `json:"payment_provider" gorm:"type:varchar(50)"`
	CreateTime        int64   `json:"create_time"`
	CompleteTime      int64   `json:"complete_time"`
	DrawnMultiplier   float64 `json:"drawn_multiplier"`
	TierMinMultiplier float64 `json:"tier_min_multiplier"`
	TierMaxMultiplier float64 `json:"tier_max_multiplier"`
	RewardQuota       int     `json:"reward_quota"`
	Oversold          bool    `json:"oversold"`
	OverLimit         bool    `json:"over_limit"`
}

// BlindBoxUserView 返回给用户端的盲盒信息。
// 刻意不包含 Tiers（倍率区间与概率），用户端只能看到可得余额范围。
type BlindBoxUserView struct {
	Id           int     `json:"id"`
	Name         string  `json:"name"`
	Price        float64 `json:"price"` // 单位：元（真实支付金额）
	StockTotal   int     `json:"stock_total"`
	SoldCount    int     `json:"sold_count"`
	PerUserLimit int     `json:"per_user_limit"`
	Enabled      bool    `json:"enabled"`
	Sort         int     `json:"sort"`
	Remark       string  `json:"remark"`
	Remaining    int     `json:"remaining"`   // -1 = 不限量
	UserBought   int     `json:"user_bought"` // 当前周期已成功购买次数
	BaseQuota    int     `json:"base_quota"`  // 1 倍倍率对应的额度
	MinReward    int     `json:"min_reward"`  // 最低可得额度（quota）
	MaxReward    int     `json:"max_reward"`  // 最高可得额度（quota）
}

// BlindBoxOrderFilter 管理端盲盒订单筛选条件。
type BlindBoxOrderFilter struct {
	UserId         *int
	BoxId          *int
	Status         string
	Keyword        string
	StartTimestamp int64
	EndTimestamp   int64
}

// ValidateBlindBox 校验并规范化盲盒配置。保存前必须调用。
func ValidateBlindBox(box *BlindBox) error {
	if box == nil {
		return errors.New("盲盒配置不能为空")
	}
	name := strings.TrimSpace(box.Name)
	if name == "" {
		return errors.New("盲盒名称不能为空")
	}
	if utf8.RuneCountInString(name) > 64 {
		return errors.New("盲盒名称不能超过 64 个字符")
	}
	box.Name = name

	if math.IsNaN(box.Price) || math.IsInf(box.Price, 0) {
		return errors.New("盲盒售价无效")
	}
	if box.Price <= 0 {
		return errors.New("盲盒售价必须大于 0")
	}
	if box.Price > BlindBoxMaxPrice {
		return errors.New("盲盒售价不能超过 1000000 元")
	}
	box.Price = math.Round(box.Price*100) / 100
	if box.Price <= 0 {
		return errors.New("盲盒售价必须大于 0")
	}

	if box.StockTotal < 0 {
		return errors.New("库存总量不能为负数")
	}
	if box.StockTotal > BlindBoxMaxStock {
		return errors.New("库存总量不能超过 1000000")
	}
	if box.PerUserLimit < 0 {
		return errors.New("每人限购不能为负数")
	}
	if box.PerUserLimit > BlindBoxMaxPerUserLimit {
		return errors.New("每人限购不能超过 1000000")
	}

	tiers, err := parseBlindBoxTiers(box.Tiers)
	if err != nil {
		return err
	}
	if err := validateBlindBoxTiers(tiers); err != nil {
		return err
	}
	tiersJSON, err := common.Marshal(tiers)
	if err != nil {
		return errors.New("倍率档位序列化失败")
	}
	box.Tiers = string(tiersJSON)
	return nil
}

func parseBlindBoxTiers(raw string) ([]BlindBoxTier, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return nil, errors.New("请至少配置一个倍率档位")
	}
	var tiers []BlindBoxTier
	if err := common.Unmarshal([]byte(raw), &tiers); err != nil {
		return nil, errors.New("倍率档位格式错误")
	}
	return tiers, nil
}

func validateBlindBoxTiers(tiers []BlindBoxTier) error {
	if len(tiers) == 0 {
		return errors.New("请至少配置一个倍率档位")
	}
	if len(tiers) > BlindBoxMaxTiers {
		return fmt.Errorf("倍率档位不能超过 %d 个", BlindBoxMaxTiers)
	}
	totalProbability := 0.0
	for i := range tiers {
		tier := &tiers[i]
		if math.IsNaN(tier.MinMultiplier) || math.IsInf(tier.MinMultiplier, 0) ||
			math.IsNaN(tier.MaxMultiplier) || math.IsInf(tier.MaxMultiplier, 0) {
			return fmt.Errorf("第 %d 档倍率无效", i+1)
		}
		if tier.MinMultiplier <= 0 {
			return fmt.Errorf("第 %d 档最低倍率必须大于 0", i+1)
		}
		if tier.MaxMultiplier < tier.MinMultiplier {
			return fmt.Errorf("第 %d 档最高倍率不能小于最低倍率", i+1)
		}
		if tier.MaxMultiplier > 100 {
			return fmt.Errorf("第 %d 档最高倍率不能超过 100", i+1)
		}
		if math.IsNaN(tier.Probability) || math.IsInf(tier.Probability, 0) {
			return fmt.Errorf("第 %d 档概率无效", i+1)
		}
		if tier.Probability <= 0 {
			return fmt.Errorf("第 %d 档概率必须大于 0", i+1)
		}
		if tier.Probability > 100 {
			return fmt.Errorf("第 %d 档概率不能超过 100", i+1)
		}
		totalProbability += tier.Probability
	}
	if math.Abs(totalProbability-100) > 0.01 {
		return fmt.Errorf("倍率档位概率合计必须为 100，当前为 %.2f", totalProbability)
	}
	return nil
}

func blindBoxRewardRange(tiers []BlindBoxTier, price float64) (minReward float64, maxReward float64) {
	if len(tiers) == 0 {
		return 0, 0
	}
	minMultiplier := tiers[0].MinMultiplier
	maxMultiplier := tiers[0].MaxMultiplier
	for _, tier := range tiers[1:] {
		if tier.MinMultiplier < minMultiplier {
			minMultiplier = tier.MinMultiplier
		}
		if tier.MaxMultiplier > maxMultiplier {
			maxMultiplier = tier.MaxMultiplier
		}
	}
	return price * minMultiplier, price * maxMultiplier
}

// GetBlindBoxRewardRange 根据档位快照返回可得余额范围（元）。
func GetBlindBoxRewardRange(tiersJSON string, price float64) (minReward float64, maxReward float64) {
	tiers, err := parseBlindBoxTiers(tiersJSON)
	if err != nil {
		return 0, 0
	}
	return blindBoxRewardRange(tiers, price)
}

// GetBlindBoxRemaining 返回剩余库存，-1 表示不限量。
func GetBlindBoxRemaining(box *BlindBox) int {
	if box == nil || box.StockTotal <= 0 {
		return -1
	}
	remaining := box.StockTotal - box.SoldCount
	if remaining < 0 {
		return 0
	}
	return remaining
}

// BlindBoxBaseQuota 将人民币售价转换为 1 倍倍率对应的内部额度。
// 售价是真实支付的人民币金额，而 QuotaPerUnit 以美元为单位，所以需要按汇率折算。
func BlindBoxBaseQuota(price float64) int {
	rate := operation_setting.USDExchangeRate
	if rate <= 0 {
		rate = 7.3
	}
	return common.QuotaFromDecimal(
		decimal.NewFromFloat(price).
			Div(decimal.NewFromFloat(rate)).
			Mul(decimal.NewFromFloat(common.QuotaPerUnit)),
	)
}

// blindBoxQuotaRange 返回档位快照对应的最低/最高可得额度。
func blindBoxQuotaRange(tiers []BlindBoxTier, baseQuota int) (int, int) {
	if len(tiers) == 0 || baseQuota <= 0 {
		return baseQuota, baseQuota
	}
	minMultiplier := tiers[0].MinMultiplier
	maxMultiplier := tiers[0].MaxMultiplier
	for _, tier := range tiers[1:] {
		if tier.MinMultiplier < minMultiplier {
			minMultiplier = tier.MinMultiplier
		}
		if tier.MaxMultiplier > maxMultiplier {
			maxMultiplier = tier.MaxMultiplier
		}
	}
	return common.QuotaRound(float64(baseQuota) * minMultiplier),
		common.QuotaRound(float64(baseQuota) * maxMultiplier)
}

// CreateBlindBoxOrder 创建盲盒订单。
func CreateBlindBoxOrder(order *BlindBoxOrder) error {
	if order == nil {
		return errors.New("盲盒订单不能为空")
	}
	if order.CreateTime == 0 {
		order.CreateTime = common.GetTimestamp()
	}
	return DB.Create(order).Error
}

// MarkBlindBoxOrderFailed 将待支付订单标记为失败（仅 pending 可变更）。
func MarkBlindBoxOrderFailed(tradeNo string) error {
	if strings.TrimSpace(tradeNo) == "" {
		return ErrBlindBoxOrderNotFound
	}
	result := DB.Model(&BlindBoxOrder{}).
		Where("trade_no = ? AND status = ?", tradeNo, common.TopUpStatusPending).
		Update("status", common.TopUpStatusFailed)
	return result.Error
}

// GetBlindBoxOrderByTradeNo 按订单号查询盲盒订单。
func GetBlindBoxOrderByTradeNo(tradeNo string) (*BlindBoxOrder, error) {
	if strings.TrimSpace(tradeNo) == "" {
		return nil, ErrBlindBoxOrderNotFound
	}
	var order BlindBoxOrder
	if err := DB.Where("trade_no = ?", tradeNo).First(&order).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrBlindBoxOrderNotFound
		}
		return nil, err
	}
	return &order, nil
}

// GetBlindBoxesForUser 返回用户端可见的盲盒列表。
func GetBlindBoxesForUser(userId int, cycleId int64) ([]*BlindBoxUserView, error) {
	boxes := make([]*BlindBox, 0)
	if err := DB.Where("enabled = ?", true).Order("sort desc, id desc").Find(&boxes).Error; err != nil {
		return nil, err
	}

	boughtCounts := make(map[int]int)
	if len(boxes) > 0 {
		type boxCountRow struct {
			BoxId int
			Total int64
		}
		rows := make([]boxCountRow, 0)
		if err := DB.Model(&BlindBoxOrder{}).
			Select("box_id, COUNT(*) AS total").
			Where("user_id = ? AND cycle_id = ? AND status = ?", userId, cycleId, common.TopUpStatusSuccess).
			Group("box_id").
			Scan(&rows).Error; err != nil {
			return nil, err
		}
		for _, row := range rows {
			boughtCounts[row.BoxId] = int(row.Total)
		}
	}

	views := make([]*BlindBoxUserView, 0, len(boxes))
	for _, box := range boxes {
		baseQuota := BlindBoxBaseQuota(box.Price)
		minReward, maxReward := baseQuota, baseQuota
		if tiers, err := parseBlindBoxTiers(box.Tiers); err == nil {
			minReward, maxReward = blindBoxQuotaRange(tiers, baseQuota)
		}
		views = append(views, &BlindBoxUserView{
			Id:           box.Id,
			Name:         box.Name,
			Price:        box.Price,
			StockTotal:   box.StockTotal,
			SoldCount:    box.SoldCount,
			PerUserLimit: box.PerUserLimit,
			Enabled:      box.Enabled,
			Sort:         box.Sort,
			Remark:       box.Remark,
			Remaining:    GetBlindBoxRemaining(box),
			UserBought:   boughtCounts[box.Id],
			BaseQuota:    baseQuota,
			MinReward:    minReward,
			MaxReward:    maxReward,
		})
	}

	// 售罄的盲盒排到最后，避免占住用户页最显眼的位置。
	sort.SliceStable(views, func(i, j int) bool {
		iSoldOut := views[i].Remaining == 0
		jSoldOut := views[j].Remaining == 0
		if iSoldOut != jSoldOut {
			return !iSoldOut
		}
		if views[i].Sort != views[j].Sort {
			return views[i].Sort > views[j].Sort
		}
		return views[i].Id > views[j].Id
	})
	return views, nil
}

// GetUserBlindBoxOrders 分页查询用户自己的盲盒订单。
func GetUserBlindBoxOrders(userId int, pageInfo *common.PageInfo) ([]*BlindBoxOrder, int64, error) {
	if pageInfo == nil {
		pageInfo = &common.PageInfo{Page: 1, PageSize: common.ItemsPerPage}
	}
	orders := make([]*BlindBoxOrder, 0)
	var total int64

	tx := DB.Begin()
	if tx.Error != nil {
		return nil, 0, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Model(&BlindBoxOrder{}).Where("user_id = ?", userId).Count(&total).Error; err != nil {
		tx.Rollback()
		return nil, 0, err
	}
	if err := tx.Where("user_id = ?", userId).Order("id desc").
		Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).
		Find(&orders).Error; err != nil {
		tx.Rollback()
		return nil, 0, err
	}
	if err := tx.Commit().Error; err != nil {
		return nil, 0, err
	}
	return orders, total, nil
}

// AdminGetBlindBoxOrders 管理员按条件分页查询盲盒订单。
func AdminGetBlindBoxOrders(filter BlindBoxOrderFilter, pageInfo *common.PageInfo) ([]*BlindBoxOrder, int64, error) {
	if pageInfo == nil {
		pageInfo = &common.PageInfo{Page: 1, PageSize: common.ItemsPerPage}
	}

	query := DB.Model(&BlindBoxOrder{})
	if filter.UserId != nil && *filter.UserId > 0 {
		query = query.Where("user_id = ?", *filter.UserId)
	}
	if filter.BoxId != nil && *filter.BoxId > 0 {
		query = query.Where("box_id = ?", *filter.BoxId)
	}
	if filter.Status != "" {
		switch filter.Status {
		case common.TopUpStatusPending, common.TopUpStatusSuccess, common.TopUpStatusFailed, common.TopUpStatusExpired:
			query = query.Where("status = ?", filter.Status)
		default:
			return nil, 0, errors.New("订单状态参数错误")
		}
	}
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		pattern, err := sanitizeLikePattern("%" + keyword + "%")
		if err != nil {
			return nil, 0, err
		}
		query = query.Where("trade_no LIKE ? ESCAPE '!'", pattern)
	}
	if filter.StartTimestamp > 0 {
		query = query.Where("create_time >= ?", filter.StartTimestamp)
	}
	if filter.EndTimestamp > 0 {
		query = query.Where("create_time <= ?", filter.EndTimestamp)
	}

	orders := make([]*BlindBoxOrder, 0)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Order("id desc").
		Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).
		Find(&orders).Error; err != nil {
		return nil, 0, err
	}
	return orders, total, nil
}

// SettleBlindBoxOrder 结算盲盒订单：标记成功、扣库存、开奖并给用户加额度。
// 返回 alreadyProcessed=true 表示订单此前已成功结算，本次未做任何修改。
func SettleBlindBoxOrder(tradeNo string) (order *BlindBoxOrder, alreadyProcessed bool, err error) {
	if strings.TrimSpace(tradeNo) == "" {
		return nil, false, ErrBlindBoxOrderNotFound
	}

	txErr := DB.Transaction(func(tx *gorm.DB) error {
		current := &BlindBoxOrder{}
		if err := LockForUpdate(tx).Where("trade_no = ?", tradeNo).First(current).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrBlindBoxOrderNotFound
			}
			return err
		}
		order = current

		if current.Status == common.TopUpStatusSuccess {
			alreadyProcessed = true
			return nil
		}
		if current.Status == common.TopUpStatusFailed {
			return errors.New("盲盒订单已失败，无法发放奖励")
		}
		if current.Status != common.TopUpStatusPending && current.Status != common.TopUpStatusExpired {
			return ErrBlindBoxOrderStatusInvalid
		}

		box := &BlindBox{}
		if err := tx.Where("id = ?", current.BoxId).First(box).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrBlindBoxNotFound
			}
			return err
		}

		// 限购仅用于标记，不阻止发奖。
		current.OverLimit = false
		if box.PerUserLimit > 0 {
			var used int64
			if err := tx.Model(&BlindBoxOrder{}).
				Where("user_id = ? AND box_id = ? AND cycle_id = ? AND status = ?",
					current.UserId, current.BoxId, current.CycleId, common.TopUpStatusSuccess).
				Count(&used).Error; err != nil {
				return err
			}
			if used >= int64(box.PerUserLimit) {
				current.OverLimit = true
			}
		}

		// 库存扣减：支付成功后才扣；并发超卖时仍发奖并标记。
		current.Oversold = false
		if box.StockTotal > 0 {
			result := tx.Model(&BlindBox{}).
				Where("id = ? AND sold_count < stock_total", box.Id).
				UpdateColumn("sold_count", gorm.Expr("sold_count + 1"))
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				result = tx.Model(&BlindBox{}).Where("id = ?", box.Id).
					UpdateColumn("sold_count", gorm.Expr("sold_count + 1"))
				if result.Error != nil {
					return result.Error
				}
				if result.RowsAffected == 0 {
					return ErrBlindBoxNotFound
				}
				current.Oversold = true
			}
		} else {
			result := tx.Model(&BlindBox{}).Where("id = ?", box.Id).
				UpdateColumn("sold_count", gorm.Expr("sold_count + 1"))
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return ErrBlindBoxNotFound
			}
		}

		tiers, err := parseBlindBoxTiers(current.TiersSnapshot)
		if err != nil {
			return fmt.Errorf("盲盒档位快照无效: %w", err)
		}
		if err := validateBlindBoxTiers(tiers); err != nil {
			return fmt.Errorf("盲盒档位快照无效: %w", err)
		}
		selectedTier, err := drawBlindBoxTier(tiers)
		if err != nil {
			return err
		}
		multiplier, err := drawBlindBoxMultiplier(selectedTier)
		if err != nil {
			return err
		}

		rewardQuota := common.QuotaRound(float64(current.BaseQuota) * multiplier)
		if rewardQuota <= 0 {
			return errors.New("抽中奖励额度无效，无法发放")
		}

		current.Status = common.TopUpStatusSuccess
		current.CompleteTime = common.GetTimestamp()
		current.DrawnMultiplier = multiplier
		current.TierMinMultiplier = selectedTier.MinMultiplier
		current.TierMaxMultiplier = selectedTier.MaxMultiplier
		current.RewardQuota = rewardQuota
		if err := tx.Save(current).Error; err != nil {
			return err
		}

		result := tx.Model(&User{}).Where("id = ?", current.UserId).
			Update("quota", gorm.Expr("quota + ?", rewardQuota))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return errors.New("用户不存在，无法发放盲盒奖励")
		}
		return nil
	})
	if txErr != nil {
		return nil, false, txErr
	}
	return order, alreadyProcessed, nil
}

// drawBlindBoxTier 按概率选中一个档位。使用 crypto/rand 保证不可预测。
func drawBlindBoxTier(tiers []BlindBoxTier) (*BlindBoxTier, error) {
	random, err := cryptoRandFloat64()
	if err != nil {
		return nil, fmt.Errorf("生成随机数失败: %w", err)
	}
	target := random * 100
	cumulative := 0.0
	for i := range tiers {
		cumulative += tiers[i].Probability
		if target < cumulative {
			return &tiers[i], nil
		}
	}
	// 概率合计存在浮点误差时兜底最后一个档位。
	return &tiers[len(tiers)-1], nil
}

// drawBlindBoxMultiplier 在档位区间内均匀取倍率，四舍五入到 4 位小数。
func drawBlindBoxMultiplier(tier *BlindBoxTier) (float64, error) {
	random, err := cryptoRandFloat64()
	if err != nil {
		return 0, fmt.Errorf("生成随机数失败: %w", err)
	}
	multiplier := tier.MinMultiplier + random*(tier.MaxMultiplier-tier.MinMultiplier)
	multiplier = math.Round(multiplier*10000) / 10000
	if multiplier < tier.MinMultiplier {
		multiplier = tier.MinMultiplier
	}
	if multiplier > tier.MaxMultiplier {
		multiplier = tier.MaxMultiplier
	}
	return multiplier, nil
}

// cryptoRandFloat64 返回 [0, 1) 区间的均匀随机浮点数。
func cryptoRandFloat64() (float64, error) {
	const max = int64(1) << 53
	value, err := crand.Int(crand.Reader, big.NewInt(max))
	if err != nil {
		return 0, err
	}
	return float64(value.Int64()) / float64(max), nil
}

// ExpirePendingBlindBoxOrders 将超时未支付的订单置为 expired，只更新状态不删除订单。
func ExpirePendingBlindBoxOrders(timeoutMinutes int) (int64, error) {
	if timeoutMinutes <= 0 {
		timeoutMinutes = BlindBoxDefaultTimeout
	}
	cutoff := common.GetTimestamp() - int64(timeoutMinutes)*60
	result := DB.Model(&BlindBoxOrder{}).
		Where("status = ? AND create_time < ?", common.TopUpStatusPending, cutoff).
		Update("status", common.TopUpStatusExpired)
	return result.RowsAffected, result.Error
}

// GetUserBlindBoxSuccessCount 统计用户在指定周期内成功购买某个盲盒的次数。
func GetUserBlindBoxSuccessCount(userId, boxId int, cycleId int64) (int64, error) {
	var count int64
	err := DB.Model(&BlindBoxOrder{}).
		Where("user_id = ? AND box_id = ? AND cycle_id = ? AND status = ?",
			userId, boxId, cycleId, common.TopUpStatusSuccess).
		Count(&count).Error
	return count, err
}

// CreateBlindBox 新建盲盒。
func CreateBlindBox(box *BlindBox) error {
	if box == nil {
		return errors.New("盲盒配置不能为空")
	}
	now := common.GetTimestamp()
	if box.CreateTime == 0 {
		box.CreateTime = now
	}
	box.UpdateTime = now
	return DB.Create(box).Error
}

// GetBlindBoxById 按 ID 查询盲盒。
func GetBlindBoxById(id int) (*BlindBox, error) {
	if id <= 0 {
		return nil, ErrBlindBoxNotFound
	}
	var box BlindBox
	if err := DB.Where("id = ?", id).First(&box).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrBlindBoxNotFound
		}
		return nil, err
	}
	return &box, nil
}

// GetAllBlindBoxes 返回全部盲盒（包含禁用），管理端使用。
func GetAllBlindBoxes() ([]*BlindBox, error) {
	boxes := make([]*BlindBox, 0)
	if err := DB.Order("sort desc, id desc").Find(&boxes).Error; err != nil {
		return nil, err
	}
	return boxes, nil
}

// UpdateBlindBox 只更新允许修改的字段，sold_count 不允许通过此函数修改。
func UpdateBlindBox(id int, updates map[string]interface{}) error {
	if id <= 0 {
		return ErrBlindBoxNotFound
	}
	if updates == nil {
		return errors.New("没有需要更新的字段")
	}
	delete(updates, "id")
	delete(updates, "sold_count")
	delete(updates, "create_time")
	updates["update_time"] = common.GetTimestamp()
	result := DB.Model(&BlindBox{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		// MySQL 默认在字段值未变化时返回 RowsAffected=0，此时需额外确认记录是否存在。
		if _, err := GetBlindBoxById(id); err != nil {
			return err
		}
	}
	return nil
}
