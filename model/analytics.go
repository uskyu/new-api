package model

import (
	"errors"
	"fmt"
	"sort"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

type AnalyticsRange struct {
	Range string `json:"range"`
	Start int64  `json:"start"`
	End   int64  `json:"end"`
	Step  int64  `json:"step"`
}

type AnalyticsOverviewOptions struct {
	Range AnalyticsRange
	Limit int
}

type AnalyticsOverview struct {
	Range         AnalyticsRange         `json:"range"`
	Metrics       AnalyticsMetrics       `json:"metrics"`
	Trends        []AnalyticsTrendPoint  `json:"trends"`
	Distributions AnalyticsDistributions `json:"distributions"`
	Rankings      AnalyticsRankings      `json:"rankings"`
}

type AnalyticsMetrics struct {
	UserCount             int64   `json:"user_count"`
	NewUserCount          int64   `json:"new_user_count"`
	UserBalanceQuota      int64   `json:"user_balance_quota"`
	UserUsedQuota         int64   `json:"user_used_quota"`
	SuccessfulTopupAmount float64 `json:"successful_topup_amount"`
	SuccessfulTopupCount  int64   `json:"successful_topup_count"`
	TopupUserCount        int64   `json:"topup_user_count"`
	RepeatTopupUserCount  int64   `json:"repeat_topup_user_count"`
	RepurchaseRate        float64 `json:"repurchase_rate"`
	ConsumeQuota          int64   `json:"consume_quota"`
	CallCount             int64   `json:"call_count"`
	ActiveUserCount       int64   `json:"active_user_count"`
}

type AnalyticsTrendPoint struct {
	Start           int64   `json:"start"`
	End             int64   `json:"end"`
	NewUserCount    int64   `json:"new_user_count"`
	TopupAmount     float64 `json:"topup_amount"`
	ConsumeQuota    int64   `json:"consume_quota"`
	CallCount       int64   `json:"call_count"`
	ActiveUserCount int64   `json:"active_user_count"`
}

type AnalyticsDistributions struct {
	PaymentMethod  []AnalyticsPaymentMethodShare `json:"payment_method"`
	BalanceBuckets []AnalyticsBalanceBucket      `json:"balance_buckets"`
}

type AnalyticsPaymentMethodShare struct {
	PaymentMethod string  `json:"payment_method"`
	TopupAmount   float64 `json:"topup_amount"`
	TopupCount    int64   `json:"topup_count"`
	Ratio         float64 `json:"ratio"`
}

type AnalyticsBalanceBucket struct {
	Label     string  `json:"label"`
	MinQuota  int64   `json:"min_quota"`
	MaxQuota  *int64  `json:"max_quota,omitempty"`
	UserCount int64   `json:"user_count"`
	Ratio     float64 `json:"ratio"`
}

type AnalyticsRankings struct {
	AgentContribution []AnalyticsAgentContributionRank `json:"agent_contribution"`
	ModelUsage        []AnalyticsModelUsageRank        `json:"model_usage"`
	UserTopups        []AnalyticsUserTopupRank         `json:"user_topups"`
	UserConsumptions  []AnalyticsUserConsumptionRank   `json:"user_consumptions"`
}

type AnalyticsModelUsageRank struct {
	ModelName       string `json:"model_name"`
	ConsumeQuota    int64  `json:"consume_quota"`
	CallCount       int64  `json:"call_count"`
	ActiveUserCount int64  `json:"active_user_count"`
}

type AnalyticsAgentContributionRank struct {
	AgentUserId  int    `json:"agent_user_id"`
	Username     string `json:"username"`
	DisplayName  string `json:"display_name"`
	TopupCount   int64  `json:"topup_count"`
	PayAmount    int64  `json:"pay_amount"`
	RebateAmount int64  `json:"rebate_amount"`
}

type AnalyticsUserTopupRank struct {
	UserId      int     `json:"user_id"`
	Username    string  `json:"username"`
	DisplayName string  `json:"display_name"`
	TopupAmount float64 `json:"topup_amount"`
	TopupCount  int64   `json:"topup_count"`
}

type AnalyticsUserConsumptionRank struct {
	UserId       int    `json:"user_id"`
	Username     string `json:"username"`
	DisplayName  string `json:"display_name"`
	ConsumeQuota int64  `json:"consume_quota"`
	CallCount    int64  `json:"call_count"`
}

type analyticsBucketRow struct {
	Bucket          int     `gorm:"column:bucket"`
	NewUserCount    int64   `gorm:"column:new_user_count"`
	TopupAmount     float64 `gorm:"column:topup_amount"`
	ConsumeQuota    int64   `gorm:"column:consume_quota"`
	CallCount       int64   `gorm:"column:call_count"`
	ActiveUserCount int64   `gorm:"column:active_user_count"`
}

type analyticsUserSummary struct {
	Id          int
	Username    string
	DisplayName string
}

type analyticsAgentContributionRow struct {
	AgentUserId  int   `gorm:"column:agent_user_id"`
	TopupCount   int64 `gorm:"column:topup_count"`
	PayAmount    int64 `gorm:"column:pay_amount"`
	RebateAmount int64 `gorm:"column:rebate_amount"`
}

func GetAdminAnalyticsOverview(options AnalyticsOverviewOptions) (*AnalyticsOverview, error) {
	if DB == nil || LOG_DB == nil {
		return nil, errors.New("database is not initialized")
	}
	if options.Range.Start <= 0 || options.Range.End < options.Range.Start {
		return nil, errors.New("invalid analytics time range")
	}
	if options.Limit <= 0 {
		options.Limit = 10
	}
	if options.Limit > 50 {
		options.Limit = 50
	}

	options.Range.Step = analyticsTrendStep(options.Range.Start, options.Range.End)
	overview := &AnalyticsOverview{
		Range:  options.Range,
		Trends: initAnalyticsTrendPoints(options.Range.Start, options.Range.End, options.Range.Step),
	}
	if err := fillAnalyticsUserMetrics(&overview.Metrics, options.Range); err != nil {
		return nil, err
	}
	if err := fillAnalyticsTopupMetrics(&overview.Metrics, options.Range); err != nil {
		return nil, err
	}
	if err := fillAnalyticsConsumeMetrics(&overview.Metrics, options.Range); err != nil {
		return nil, err
	}

	trendLoaders := []func(*AnalyticsOverview) error{
		fillAnalyticsUserTrend,
		fillAnalyticsTopupTrend,
		fillAnalyticsConsumeTrend,
	}
	for _, loader := range trendLoaders {
		if err := loader(overview); err != nil {
			return nil, err
		}
	}

	var err error
	overview.Distributions.PaymentMethod, err = buildAnalyticsPaymentShares(options.Range, overview.Metrics.SuccessfulTopupAmount, options.Limit)
	if err != nil {
		return nil, err
	}
	overview.Distributions.BalanceBuckets, err = buildAnalyticsBalanceBuckets()
	if err != nil {
		return nil, err
	}
	overview.Rankings.UserTopups, err = buildAnalyticsUserTopupRanks(options.Range, options.Limit)
	if err != nil {
		return nil, err
	}
	overview.Rankings.UserConsumptions, err = buildAnalyticsUserConsumptionRanks(options.Range, options.Limit)
	if err != nil {
		return nil, err
	}
	overview.Rankings.ModelUsage, err = buildAnalyticsModelUsageRanks(options.Range, options.Limit)
	if err != nil {
		return nil, err
	}
	overview.Rankings.AgentContribution, err = buildAnalyticsAgentContributionRanks(options.Range, options.Limit)
	if err != nil {
		return nil, err
	}
	fillAnalyticsRankingUsers(overview)
	return overview, nil
}

func fillAnalyticsUserMetrics(metrics *AnalyticsMetrics, timeRange AnalyticsRange) error {
	var totals struct {
		UserCount        int64 `gorm:"column:user_count"`
		UserBalanceQuota int64 `gorm:"column:user_balance_quota"`
		UserUsedQuota    int64 `gorm:"column:user_used_quota"`
	}
	if err := DB.Model(&User{}).
		Select("COUNT(*) AS user_count, COALESCE(SUM(quota), 0) AS user_balance_quota, COALESCE(SUM(used_quota), 0) AS user_used_quota").
		Scan(&totals).Error; err != nil {
		return err
	}
	metrics.UserCount = totals.UserCount
	metrics.UserBalanceQuota = totals.UserBalanceQuota
	metrics.UserUsedQuota = totals.UserUsedQuota
	return DB.Model(&User{}).
		Where("created_at >= ? AND created_at <= ?", timeRange.Start, timeRange.End).
		Count(&metrics.NewUserCount).Error
}

func fillAnalyticsTopupMetrics(metrics *AnalyticsMetrics, timeRange AnalyticsRange) error {
	var totals struct {
		Amount    float64 `gorm:"column:amount"`
		Count     int64   `gorm:"column:count"`
		UserCount int64   `gorm:"column:user_count"`
	}
	query := DB.Model(&TopUp{}).
		Where("status = ? AND complete_time >= ? AND complete_time <= ?", common.TopUpStatusSuccess, timeRange.Start, timeRange.End)
	if err := query.Session(&gorm.Session{}).
		Select("COALESCE(SUM(money), 0) AS amount, COUNT(*) AS count, COUNT(DISTINCT user_id) AS user_count").
		Scan(&totals).Error; err != nil {
		return err
	}
	metrics.SuccessfulTopupAmount = totals.Amount
	metrics.SuccessfulTopupCount = totals.Count
	metrics.TopupUserCount = totals.UserCount

	repeatedUsers := query.Session(&gorm.Session{}).Select("user_id").Group("user_id").Having("COUNT(*) > 1")
	if err := DB.Table("(?) AS repeated_topup_users", repeatedUsers).Count(&metrics.RepeatTopupUserCount).Error; err != nil {
		return err
	}
	metrics.RepurchaseRate = analyticsRatio(float64(metrics.RepeatTopupUserCount), float64(metrics.TopupUserCount))
	return nil
}

func fillAnalyticsConsumeMetrics(metrics *AnalyticsMetrics, timeRange AnalyticsRange) error {
	var totals struct {
		ConsumeQuota    int64 `gorm:"column:consume_quota"`
		CallCount       int64 `gorm:"column:call_count"`
		ActiveUserCount int64 `gorm:"column:active_user_count"`
	}
	err := LOG_DB.Model(&Log{}).
		Where("type = ? AND created_at >= ? AND created_at <= ?", LogTypeConsume, timeRange.Start, timeRange.End).
		Select("COALESCE(SUM(quota), 0) AS consume_quota, COUNT(*) AS call_count, COUNT(DISTINCT user_id) AS active_user_count").
		Scan(&totals).Error
	if err != nil {
		return err
	}
	metrics.ConsumeQuota = totals.ConsumeQuota
	metrics.CallCount = totals.CallCount
	metrics.ActiveUserCount = totals.ActiveUserCount
	return nil
}

func fillAnalyticsUserTrend(overview *AnalyticsOverview) error {
	expression := analyticsBucketExpression("created_at", overview.Range.Start, overview.Range.Step, common.MainDatabaseType())
	var rows []analyticsBucketRow
	if err := DB.Model(&User{}).
		Select(expression+" AS bucket, COUNT(*) AS new_user_count").
		Where("created_at >= ? AND created_at <= ?", overview.Range.Start, overview.Range.End).
		Group(expression).
		Scan(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		if row.Bucket >= 0 && row.Bucket < len(overview.Trends) {
			overview.Trends[row.Bucket].NewUserCount = row.NewUserCount
		}
	}
	return nil
}

func fillAnalyticsTopupTrend(overview *AnalyticsOverview) error {
	expression := analyticsBucketExpression("complete_time", overview.Range.Start, overview.Range.Step, common.MainDatabaseType())
	var rows []analyticsBucketRow
	if err := DB.Model(&TopUp{}).
		Select(expression+" AS bucket, COALESCE(SUM(money), 0) AS topup_amount").
		Where("status = ? AND complete_time >= ? AND complete_time <= ?", common.TopUpStatusSuccess, overview.Range.Start, overview.Range.End).
		Group(expression).
		Scan(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		if row.Bucket >= 0 && row.Bucket < len(overview.Trends) {
			overview.Trends[row.Bucket].TopupAmount = row.TopupAmount
		}
	}
	return nil
}

func fillAnalyticsConsumeTrend(overview *AnalyticsOverview) error {
	expression := analyticsBucketExpression("created_at", overview.Range.Start, overview.Range.Step, common.LogDatabaseType())
	var rows []analyticsBucketRow
	if err := LOG_DB.Model(&Log{}).
		Select(expression+" AS bucket, COALESCE(SUM(quota), 0) AS consume_quota, COUNT(*) AS call_count, COUNT(DISTINCT user_id) AS active_user_count").
		Where("type = ? AND created_at >= ? AND created_at <= ?", LogTypeConsume, overview.Range.Start, overview.Range.End).
		Group(expression).
		Scan(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		if row.Bucket >= 0 && row.Bucket < len(overview.Trends) {
			overview.Trends[row.Bucket].ConsumeQuota = row.ConsumeQuota
			overview.Trends[row.Bucket].CallCount = row.CallCount
			overview.Trends[row.Bucket].ActiveUserCount = row.ActiveUserCount
		}
	}
	return nil
}

func analyticsBucketExpression(column string, start int64, step int64, databaseType common.DatabaseType) string {
	if step <= 0 {
		step = 1
	}
	switch databaseType {
	case common.DatabaseTypeSQLite:
		return fmt.Sprintf("CAST((%s - %d) / %d AS INTEGER)", column, start, step)
	case common.DatabaseTypeClickHouse:
		return fmt.Sprintf("intDiv(%s - %d, %d)", column, start, step)
	default:
		return fmt.Sprintf("FLOOR((%s - %d) / %d)", column, start, step)
	}
}

func buildAnalyticsPaymentShares(timeRange AnalyticsRange, total float64, limit int) ([]AnalyticsPaymentMethodShare, error) {
	var rows []AnalyticsPaymentMethodShare
	if err := DB.Model(&TopUp{}).
		Select("payment_method, COALESCE(SUM(money), 0) AS topup_amount, COUNT(*) AS topup_count").
		Where("status = ? AND complete_time >= ? AND complete_time <= ?", common.TopUpStatusSuccess, timeRange.Start, timeRange.End).
		Group("payment_method").
		Order("topup_amount DESC").
		Limit(limit).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for i := range rows {
		if rows[i].PaymentMethod == "" {
			rows[i].PaymentMethod = "unknown"
		}
		rows[i].Ratio = analyticsRatio(rows[i].TopupAmount, total)
	}
	return rows, nil
}

func buildAnalyticsBalanceBuckets() ([]AnalyticsBalanceBucket, error) {
	unit := int64(common.QuotaPerUnit)
	if unit <= 0 {
		unit = 1
	}
	var totals struct {
		Total    int64 `gorm:"column:total"`
		Empty    int64 `gorm:"column:empty_count"`
		Unit     int64 `gorm:"column:unit_count"`
		TenUnits int64 `gorm:"column:ten_unit_count"`
		Hundred  int64 `gorm:"column:hundred_unit_count"`
		Above    int64 `gorm:"column:above_count"`
	}
	selectClause := fmt.Sprintf(`COUNT(*) AS total,
		COALESCE(SUM(CASE WHEN quota <= 0 THEN 1 ELSE 0 END), 0) AS empty_count,
		COALESCE(SUM(CASE WHEN quota > 0 AND quota <= %d THEN 1 ELSE 0 END), 0) AS unit_count,
		COALESCE(SUM(CASE WHEN quota > %d AND quota <= %d THEN 1 ELSE 0 END), 0) AS ten_unit_count,
		COALESCE(SUM(CASE WHEN quota > %d AND quota <= %d THEN 1 ELSE 0 END), 0) AS hundred_unit_count,
		COALESCE(SUM(CASE WHEN quota > %d THEN 1 ELSE 0 END), 0) AS above_count`, unit, unit, unit*10, unit*10, unit*100, unit*100)
	if err := DB.Model(&User{}).Select(selectClause).Scan(&totals).Error; err != nil {
		return nil, err
	}
	buckets := []AnalyticsBalanceBucket{
		{Label: "<=0", MinQuota: 0, MaxQuota: int64Pointer(0), UserCount: totals.Empty},
		{Label: "0-1 unit", MinQuota: 1, MaxQuota: int64Pointer(unit), UserCount: totals.Unit},
		{Label: "1-10 units", MinQuota: unit + 1, MaxQuota: int64Pointer(unit * 10), UserCount: totals.TenUnits},
		{Label: "10-100 units", MinQuota: unit*10 + 1, MaxQuota: int64Pointer(unit * 100), UserCount: totals.Hundred},
		{Label: ">100 units", MinQuota: unit*100 + 1, UserCount: totals.Above},
	}
	for i := range buckets {
		buckets[i].Ratio = analyticsRatio(float64(buckets[i].UserCount), float64(totals.Total))
	}
	return buckets, nil
}

func buildAnalyticsUserTopupRanks(timeRange AnalyticsRange, limit int) ([]AnalyticsUserTopupRank, error) {
	var rows []AnalyticsUserTopupRank
	if err := DB.Model(&TopUp{}).
		Select("user_id, COALESCE(SUM(money), 0) AS topup_amount, COUNT(*) AS topup_count").
		Where("status = ? AND complete_time >= ? AND complete_time <= ?", common.TopUpStatusSuccess, timeRange.Start, timeRange.End).
		Group("user_id").
		Order("topup_amount DESC").
		Limit(limit).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func buildAnalyticsUserConsumptionRanks(timeRange AnalyticsRange, limit int) ([]AnalyticsUserConsumptionRank, error) {
	var rows []AnalyticsUserConsumptionRank
	if err := LOG_DB.Model(&Log{}).
		Select("user_id, COALESCE(SUM(quota), 0) AS consume_quota, COUNT(*) AS call_count").
		Where("type = ? AND created_at >= ? AND created_at <= ?", LogTypeConsume, timeRange.Start, timeRange.End).
		Group("user_id").
		Order("consume_quota DESC").
		Limit(limit).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func buildAnalyticsModelUsageRanks(timeRange AnalyticsRange, limit int) ([]AnalyticsModelUsageRank, error) {
	var rows []AnalyticsModelUsageRank
	if err := LOG_DB.Model(&Log{}).
		Select("model_name, COALESCE(SUM(quota), 0) AS consume_quota, COUNT(*) AS call_count, COUNT(DISTINCT user_id) AS active_user_count").
		Where("type = ? AND created_at >= ? AND created_at <= ?", LogTypeConsume, timeRange.Start, timeRange.End).
		Where("model_name <> ?", "").
		Group("model_name").
		Order("consume_quota DESC").
		Order("call_count DESC").
		Order("model_name ASC").
		Limit(limit).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func buildAnalyticsAgentContributionRanks(timeRange AnalyticsRange, limit int) ([]AnalyticsAgentContributionRank, error) {
	topupRows, err := loadAnalyticsAgentContributionRows(&AgentRebateRecord{}, timeRange)
	if err != nil {
		return nil, err
	}
	redemptionRows, err := loadAnalyticsAgentContributionRows(&AgentRedemptionRebateRecord{}, timeRange)
	if err != nil {
		return nil, err
	}
	rankMap := make(map[int]AnalyticsAgentContributionRank)
	for _, rows := range [][]analyticsAgentContributionRow{topupRows, redemptionRows} {
		for _, row := range rows {
			rank := rankMap[row.AgentUserId]
			rank.AgentUserId = row.AgentUserId
			rank.TopupCount += row.TopupCount
			rank.PayAmount += row.PayAmount
			rank.RebateAmount += row.RebateAmount
			rankMap[row.AgentUserId] = rank
		}
	}
	ranks := make([]AnalyticsAgentContributionRank, 0, len(rankMap))
	for _, rank := range rankMap {
		ranks = append(ranks, rank)
	}
	sort.Slice(ranks, func(i int, j int) bool {
		if ranks[i].PayAmount == ranks[j].PayAmount {
			return ranks[i].AgentUserId < ranks[j].AgentUserId
		}
		return ranks[i].PayAmount > ranks[j].PayAmount
	})
	if len(ranks) > limit {
		ranks = ranks[:limit]
	}
	return ranks, nil
}

func loadAnalyticsAgentContributionRows(value any, timeRange AnalyticsRange) ([]analyticsAgentContributionRow, error) {
	var rows []analyticsAgentContributionRow
	err := DB.Model(value).
		Select("agent_user_id, COUNT(*) AS topup_count, COALESCE(SUM(pay_amount), 0) AS pay_amount, COALESCE(SUM(rebate_amount), 0) AS rebate_amount").
		Where("status = ? AND settled_at >= ? AND settled_at <= ?", AgentRebateRecordSettled, timeRange.Start, timeRange.End).
		Group("agent_user_id").
		Scan(&rows).Error
	return rows, err
}

func fillAnalyticsRankingUsers(overview *AnalyticsOverview) {
	ids := make([]int, 0)
	seen := make(map[int]struct{})
	appendId := func(id int) {
		if id <= 0 {
			return
		}
		if _, exists := seen[id]; exists {
			return
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	for _, rank := range overview.Rankings.AgentContribution {
		appendId(rank.AgentUserId)
	}
	for _, rank := range overview.Rankings.UserTopups {
		appendId(rank.UserId)
	}
	for _, rank := range overview.Rankings.UserConsumptions {
		appendId(rank.UserId)
	}
	if len(ids) == 0 {
		return
	}
	var users []analyticsUserSummary
	if err := DB.Unscoped().Model(&User{}).Select("id, username, display_name").Where("id IN ?", ids).Find(&users).Error; err != nil {
		common.SysLog("failed to load analytics users: " + err.Error())
		return
	}
	userMap := make(map[int]analyticsUserSummary, len(users))
	for _, user := range users {
		userMap[user.Id] = user
	}
	for i := range overview.Rankings.AgentContribution {
		user := userMap[overview.Rankings.AgentContribution[i].AgentUserId]
		overview.Rankings.AgentContribution[i].Username = user.Username
		overview.Rankings.AgentContribution[i].DisplayName = user.DisplayName
	}
	for i := range overview.Rankings.UserTopups {
		user := userMap[overview.Rankings.UserTopups[i].UserId]
		overview.Rankings.UserTopups[i].Username = user.Username
		overview.Rankings.UserTopups[i].DisplayName = user.DisplayName
	}
	for i := range overview.Rankings.UserConsumptions {
		user := userMap[overview.Rankings.UserConsumptions[i].UserId]
		overview.Rankings.UserConsumptions[i].Username = user.Username
		overview.Rankings.UserConsumptions[i].DisplayName = user.DisplayName
	}
}

func analyticsTrendStep(start int64, end int64) int64 {
	duration := end - start + 1
	if duration <= 2*24*3600 {
		return 3600
	}
	if duration <= 35*24*3600 {
		return 24 * 3600
	}
	step := duration / 30
	if step <= 0 {
		return 24 * 3600
	}
	return step
}

func initAnalyticsTrendPoints(start int64, end int64, step int64) []AnalyticsTrendPoint {
	if step <= 0 {
		step = 24 * 3600
	}
	points := make([]AnalyticsTrendPoint, 0)
	for cursor := start; cursor <= end; cursor += step {
		pointEnd := cursor + step - 1
		if pointEnd > end {
			pointEnd = end
		}
		points = append(points, AnalyticsTrendPoint{Start: cursor, End: pointEnd})
	}
	return points
}

func analyticsRatio(part float64, total float64) float64 {
	if total <= 0 {
		return 0
	}
	return part / total
}

func int64Pointer(value int64) *int64 {
	return &value
}
