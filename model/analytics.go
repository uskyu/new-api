package model

import (
	"errors"
	"sort"
	"strconv"

	"github.com/QuantumNous/new-api/common"
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
	TopupAmount     float64 `json:"topup_amount"`
	ConsumeQuota    int64   `json:"consume_quota"`
	CallCount       int64   `json:"call_count"`
	ActiveUserCount int64   `json:"active_user_count"`
}

type AnalyticsDistributions struct {
	ModelConsumption []AnalyticsModelConsumptionShare `json:"model_consumption"`
	ChannelUsage     []AnalyticsChannelUsageShare     `json:"channel_usage"`
	PaymentMethod    []AnalyticsPaymentMethodShare    `json:"payment_method"`
	BalanceBuckets   []AnalyticsBalanceBucket         `json:"balance_buckets"`
}

type AnalyticsModelConsumptionShare struct {
	ModelName        string  `json:"model_name"`
	Quota            int64   `json:"quota"`
	CallCount        int64   `json:"call_count"`
	ActiveUserCount  int64   `json:"active_user_count"`
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
	TotalTokens      int64   `json:"total_tokens"`
	AverageQuota     float64 `json:"average_quota"`
	AverageUseTime   float64 `json:"average_use_time"`
	Ratio            float64 `json:"ratio"`
}

type AnalyticsChannelUsageShare struct {
	ChannelId       int     `json:"channel_id"`
	ChannelName     string  `json:"channel_name"`
	Quota           int64   `json:"quota"`
	CallCount       int64   `json:"call_count"`
	ActiveUserCount int64   `json:"active_user_count"`
	AverageQuota    float64 `json:"average_quota"`
	AverageUseTime  float64 `json:"average_use_time"`
	Ratio           float64 `json:"ratio"`
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
	UserTopups        []AnalyticsUserTopupRank         `json:"user_topups"`
	UserConsumptions  []AnalyticsUserConsumptionRank   `json:"user_consumptions"`
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

type analyticsUserSummary struct {
	Id          int
	Username    string
	DisplayName string
}

type analyticsTopUpRow struct {
	UserId        int
	Money         float64
	PaymentMethod string
	CompleteTime  int64
}

type analyticsConsumeRow struct {
	UserId           int
	CreatedAt        int64
	ModelName        string
	Quota            int
	PromptTokens     int
	CompletionTokens int
	UseTime          int
	ChannelId        int
}

type analyticsTopupAccumulator struct {
	amount float64
	count  int64
}

type analyticsConsumptionAccumulator struct {
	quota            int64
	count            int64
	promptTokens     int64
	completionTokens int64
	useTime          int64
	activeUsers      map[int]struct{}
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
	if options.Range.Start <= 0 || options.Range.End <= 0 || options.Range.End < options.Range.Start {
		return nil, errors.New("invalid analytics time range")
	}
	if options.Limit <= 0 {
		options.Limit = 10
	}
	if options.Limit > 100 {
		options.Limit = 100
	}

	step := analyticsTrendStep(options.Range.Start, options.Range.End)
	options.Range.Step = step
	overview := &AnalyticsOverview{
		Range:  options.Range,
		Trends: initAnalyticsTrendPoints(options.Range.Start, options.Range.End, step),
	}

	if err := fillAnalyticsUserBalance(&overview.Metrics); err != nil {
		return nil, err
	}

	topupRows, err := loadAnalyticsTopups(options.Range.Start, options.Range.End)
	if err != nil {
		return nil, err
	}
	consumeRows, err := loadAnalyticsConsumes(options.Range.Start, options.Range.End)
	if err != nil {
		return nil, err
	}

	userTopups, paymentMethods := applyAnalyticsTopups(overview, topupRows)
	userConsumptions, modelConsumption, channelConsumption := applyAnalyticsConsumes(overview, consumeRows)
	overview.Distributions.PaymentMethod = buildAnalyticsPaymentShares(paymentMethods, overview.Metrics.SuccessfulTopupAmount, options.Limit)
	overview.Distributions.ModelConsumption = buildAnalyticsModelShares(modelConsumption, overview.Metrics.ConsumeQuota, options.Limit)
	overview.Distributions.ChannelUsage = buildAnalyticsChannelShares(channelConsumption, overview.Metrics.ConsumeQuota, options.Limit)
	overview.Distributions.BalanceBuckets, err = buildAnalyticsBalanceBuckets()
	if err != nil {
		return nil, err
	}

	overview.Rankings.AgentContribution, err = buildAnalyticsAgentContributionRanks(options.Range.Start, options.Range.End, options.Limit)
	if err != nil {
		return nil, err
	}
	overview.Rankings.UserTopups = buildAnalyticsUserTopupRanks(userTopups, options.Limit)
	overview.Rankings.UserConsumptions = buildAnalyticsUserConsumptionRanks(userConsumptions, options.Limit)

	fillAnalyticsRankingUsers(overview)
	return overview, nil
}

func fillAnalyticsUserBalance(metrics *AnalyticsMetrics) error {
	var stats struct {
		UserCount        int64 `gorm:"column:user_count"`
		UserBalanceQuota int64 `gorm:"column:user_balance_quota"`
		UserUsedQuota    int64 `gorm:"column:user_used_quota"`
	}
	err := DB.Model(&User{}).
		Select("COUNT(*) AS user_count, COALESCE(SUM(quota), 0) AS user_balance_quota, COALESCE(SUM(used_quota), 0) AS user_used_quota").
		Scan(&stats).Error
	if err != nil {
		return err
	}
	metrics.UserCount = stats.UserCount
	metrics.UserBalanceQuota = stats.UserBalanceQuota
	metrics.UserUsedQuota = stats.UserUsedQuota
	return nil
}

func loadAnalyticsTopups(start int64, end int64) ([]analyticsTopUpRow, error) {
	var rows []analyticsTopUpRow
	err := DB.Model(&TopUp{}).
		Select("user_id, money, payment_method, complete_time").
		Where("status = ? AND complete_time >= ? AND complete_time <= ?", common.TopUpStatusSuccess, start, end).
		Find(&rows).Error
	return rows, err
}

func loadAnalyticsConsumes(start int64, end int64) ([]analyticsConsumeRow, error) {
	var rows []analyticsConsumeRow
	err := LOG_DB.Model(&Log{}).
		Select("user_id, created_at, model_name, quota, prompt_tokens, completion_tokens, use_time, channel_id").
		Where("type = ? AND created_at >= ? AND created_at <= ?", LogTypeConsume, start, end).
		Find(&rows).Error
	return rows, err
}

func applyAnalyticsTopups(overview *AnalyticsOverview, rows []analyticsTopUpRow) (map[int]analyticsTopupAccumulator, map[string]analyticsTopupAccumulator) {
	userTopups := map[int]analyticsTopupAccumulator{}
	paymentMethods := map[string]analyticsTopupAccumulator{}
	userCounts := map[int]int64{}

	for _, row := range rows {
		overview.Metrics.SuccessfulTopupAmount += row.Money
		overview.Metrics.SuccessfulTopupCount++
		userCounts[row.UserId]++

		userAcc := userTopups[row.UserId]
		userAcc.amount += row.Money
		userAcc.count++
		userTopups[row.UserId] = userAcc

		method := row.PaymentMethod
		if method == "" {
			method = "unknown"
		}
		methodAcc := paymentMethods[method]
		methodAcc.amount += row.Money
		methodAcc.count++
		paymentMethods[method] = methodAcc

		if idx := analyticsTrendIndex(overview.Range.Start, overview.Range.Step, len(overview.Trends), row.CompleteTime); idx >= 0 {
			overview.Trends[idx].TopupAmount += row.Money
		}
	}

	for _, count := range userCounts {
		overview.Metrics.TopupUserCount++
		if count > 1 {
			overview.Metrics.RepeatTopupUserCount++
		}
	}
	overview.Metrics.RepurchaseRate = analyticsRatio(float64(overview.Metrics.RepeatTopupUserCount), float64(overview.Metrics.TopupUserCount))
	return userTopups, paymentMethods
}

func applyAnalyticsConsumes(overview *AnalyticsOverview, rows []analyticsConsumeRow) (map[int]analyticsConsumptionAccumulator, map[string]analyticsConsumptionAccumulator, map[int]analyticsConsumptionAccumulator) {
	userConsumptions := map[int]analyticsConsumptionAccumulator{}
	modelConsumption := map[string]analyticsConsumptionAccumulator{}
	channelConsumption := map[int]analyticsConsumptionAccumulator{}
	activeUsers := map[int]struct{}{}
	trendActiveUsers := make([]map[int]struct{}, len(overview.Trends))

	for _, row := range rows {
		quota := int64(row.Quota)
		overview.Metrics.ConsumeQuota += quota
		overview.Metrics.CallCount++
		activeUsers[row.UserId] = struct{}{}

		userConsumptions[row.UserId] = addAnalyticsConsumption(userConsumptions[row.UserId], row)

		modelName := row.ModelName
		if modelName == "" {
			modelName = "unknown"
		}
		modelConsumption[modelName] = addAnalyticsConsumption(modelConsumption[modelName], row)
		channelConsumption[row.ChannelId] = addAnalyticsConsumption(channelConsumption[row.ChannelId], row)

		if idx := analyticsTrendIndex(overview.Range.Start, overview.Range.Step, len(overview.Trends), row.CreatedAt); idx >= 0 {
			overview.Trends[idx].ConsumeQuota += quota
			overview.Trends[idx].CallCount++
			if trendActiveUsers[idx] == nil {
				trendActiveUsers[idx] = map[int]struct{}{}
			}
			trendActiveUsers[idx][row.UserId] = struct{}{}
		}
	}

	overview.Metrics.ActiveUserCount = int64(len(activeUsers))
	for idx := range overview.Trends {
		overview.Trends[idx].ActiveUserCount = int64(len(trendActiveUsers[idx]))
	}
	return userConsumptions, modelConsumption, channelConsumption
}

func addAnalyticsConsumption(acc analyticsConsumptionAccumulator, row analyticsConsumeRow) analyticsConsumptionAccumulator {
	acc.quota += int64(row.Quota)
	acc.count++
	acc.promptTokens += int64(row.PromptTokens)
	acc.completionTokens += int64(row.CompletionTokens)
	acc.useTime += int64(row.UseTime)
	if acc.activeUsers == nil {
		acc.activeUsers = map[int]struct{}{}
	}
	acc.activeUsers[row.UserId] = struct{}{}
	return acc
}

func buildAnalyticsPaymentShares(items map[string]analyticsTopupAccumulator, total float64, limit int) []AnalyticsPaymentMethodShare {
	shares := make([]AnalyticsPaymentMethodShare, 0, len(items))
	for method, acc := range items {
		shares = append(shares, AnalyticsPaymentMethodShare{
			PaymentMethod: method,
			TopupAmount:   acc.amount,
			TopupCount:    acc.count,
			Ratio:         analyticsRatio(acc.amount, total),
		})
	}
	sort.Slice(shares, func(i, j int) bool {
		if shares[i].TopupAmount == shares[j].TopupAmount {
			return shares[i].PaymentMethod < shares[j].PaymentMethod
		}
		return shares[i].TopupAmount > shares[j].TopupAmount
	})
	return limitAnalyticsSlice(shares, limit)
}

func buildAnalyticsModelShares(items map[string]analyticsConsumptionAccumulator, total int64, limit int) []AnalyticsModelConsumptionShare {
	shares := make([]AnalyticsModelConsumptionShare, 0, len(items))
	for modelName, acc := range items {
		shares = append(shares, AnalyticsModelConsumptionShare{
			ModelName:        modelName,
			Quota:            acc.quota,
			CallCount:        acc.count,
			ActiveUserCount:  int64(len(acc.activeUsers)),
			PromptTokens:     acc.promptTokens,
			CompletionTokens: acc.completionTokens,
			TotalTokens:      acc.promptTokens + acc.completionTokens,
			AverageQuota:     analyticsRatio(float64(acc.quota), float64(acc.count)),
			AverageUseTime:   analyticsRatio(float64(acc.useTime), float64(acc.count)),
			Ratio:            analyticsRatio(float64(acc.quota), float64(total)),
		})
	}
	sort.Slice(shares, func(i, j int) bool {
		if shares[i].Quota == shares[j].Quota {
			return shares[i].ModelName < shares[j].ModelName
		}
		return shares[i].Quota > shares[j].Quota
	})
	return limitAnalyticsSlice(shares, limit)
}

func buildAnalyticsChannelShares(items map[int]analyticsConsumptionAccumulator, total int64, limit int) []AnalyticsChannelUsageShare {
	channelNames := loadAnalyticsChannelNameMap(items)
	shares := make([]AnalyticsChannelUsageShare, 0, len(items))
	for channelId, acc := range items {
		channelName := channelNames[channelId]
		if channelName == "" {
			if channelId == 0 {
				channelName = "unknown"
			} else {
				channelName = "channel #" + strconv.Itoa(channelId)
			}
		}
		shares = append(shares, AnalyticsChannelUsageShare{
			ChannelId:       channelId,
			ChannelName:     channelName,
			Quota:           acc.quota,
			CallCount:       acc.count,
			ActiveUserCount: int64(len(acc.activeUsers)),
			AverageQuota:    analyticsRatio(float64(acc.quota), float64(acc.count)),
			AverageUseTime:  analyticsRatio(float64(acc.useTime), float64(acc.count)),
			Ratio:           analyticsRatio(float64(acc.quota), float64(total)),
		})
	}
	sort.Slice(shares, func(i, j int) bool {
		if shares[i].Quota == shares[j].Quota {
			return shares[i].ChannelId < shares[j].ChannelId
		}
		return shares[i].Quota > shares[j].Quota
	})
	return limitAnalyticsSlice(shares, limit)
}

func buildAnalyticsBalanceBuckets() ([]AnalyticsBalanceBucket, error) {
	unit := int64(common.QuotaPerUnit)
	if unit <= 0 {
		unit = 1
	}
	unit10 := unit * 10
	unit100 := unit * 100
	buckets := []AnalyticsBalanceBucket{
		newAnalyticsBalanceBucket("<=0", 0, int64Ptr(0)),
		newAnalyticsBalanceBucket("0-1 unit", 1, int64Ptr(unit)),
		newAnalyticsBalanceBucket("1-10 units", unit+1, int64Ptr(unit10)),
		newAnalyticsBalanceBucket("10-100 units", unit10+1, int64Ptr(unit100)),
		newAnalyticsBalanceBucket(">100 units", unit100+1, nil),
	}

	var users []struct {
		Quota int64 `gorm:"column:quota"`
	}
	if err := DB.Model(&User{}).Select("quota").Find(&users).Error; err != nil {
		return nil, err
	}

	for _, user := range users {
		idx := len(buckets) - 1
		for i, bucket := range buckets {
			if bucket.MaxQuota == nil {
				if user.Quota >= bucket.MinQuota {
					idx = i
					break
				}
				continue
			}
			if user.Quota >= bucket.MinQuota && user.Quota <= *bucket.MaxQuota {
				idx = i
				break
			}
		}
		buckets[idx].UserCount++
	}

	total := float64(len(users))
	for i := range buckets {
		buckets[i].Ratio = analyticsRatio(float64(buckets[i].UserCount), total)
	}
	return buckets, nil
}

func buildAnalyticsAgentContributionRanks(start int64, end int64, limit int) ([]AnalyticsAgentContributionRank, error) {
	var topupRows []analyticsAgentContributionRow
	if err := DB.Model(&AgentRebateRecord{}).
		Select("agent_user_id, COUNT(*) AS topup_count, COALESCE(SUM(pay_amount), 0) AS pay_amount, COALESCE(SUM(rebate_amount), 0) AS rebate_amount").
		Where("status = ? AND settled_at >= ? AND settled_at <= ?", AgentRebateRecordSettled, start, end).
		Group("agent_user_id").
		Scan(&topupRows).Error; err != nil {
		return nil, err
	}

	var redemptionRows []analyticsAgentContributionRow
	if err := DB.Model(&AgentRedemptionRebateRecord{}).
		Select("agent_user_id, COUNT(*) AS topup_count, COALESCE(SUM(pay_amount), 0) AS pay_amount, COALESCE(SUM(rebate_amount), 0) AS rebate_amount").
		Where("status = ? AND settled_at >= ? AND settled_at <= ?", AgentRebateRecordSettled, start, end).
		Group("agent_user_id").
		Scan(&redemptionRows).Error; err != nil {
		return nil, err
	}

	rankMap := map[int]AnalyticsAgentContributionRank{}
	mergeRows := func(rows []analyticsAgentContributionRow) {
		for _, row := range rows {
			rank := rankMap[row.AgentUserId]
			rank.AgentUserId = row.AgentUserId
			rank.TopupCount += row.TopupCount
			rank.PayAmount += row.PayAmount
			rank.RebateAmount += row.RebateAmount
			rankMap[row.AgentUserId] = rank
		}
	}
	mergeRows(topupRows)
	mergeRows(redemptionRows)

	ranks := make([]AnalyticsAgentContributionRank, 0, len(rankMap))
	for _, rank := range rankMap {
		ranks = append(ranks, rank)
	}
	sort.Slice(ranks, func(i, j int) bool {
		if ranks[i].PayAmount == ranks[j].PayAmount {
			return ranks[i].AgentUserId < ranks[j].AgentUserId
		}
		return ranks[i].PayAmount > ranks[j].PayAmount
	})
	return limitAnalyticsSlice(ranks, limit), nil
}

func buildAnalyticsUserTopupRanks(items map[int]analyticsTopupAccumulator, limit int) []AnalyticsUserTopupRank {
	ranks := make([]AnalyticsUserTopupRank, 0, len(items))
	for userId, acc := range items {
		ranks = append(ranks, AnalyticsUserTopupRank{
			UserId:      userId,
			TopupAmount: acc.amount,
			TopupCount:  acc.count,
		})
	}
	sort.Slice(ranks, func(i, j int) bool {
		if ranks[i].TopupAmount == ranks[j].TopupAmount {
			return ranks[i].UserId < ranks[j].UserId
		}
		return ranks[i].TopupAmount > ranks[j].TopupAmount
	})
	return limitAnalyticsSlice(ranks, limit)
}

func buildAnalyticsUserConsumptionRanks(items map[int]analyticsConsumptionAccumulator, limit int) []AnalyticsUserConsumptionRank {
	ranks := make([]AnalyticsUserConsumptionRank, 0, len(items))
	for userId, acc := range items {
		ranks = append(ranks, AnalyticsUserConsumptionRank{
			UserId:       userId,
			ConsumeQuota: acc.quota,
			CallCount:    acc.count,
		})
	}
	sort.Slice(ranks, func(i, j int) bool {
		if ranks[i].ConsumeQuota == ranks[j].ConsumeQuota {
			return ranks[i].UserId < ranks[j].UserId
		}
		return ranks[i].ConsumeQuota > ranks[j].ConsumeQuota
	})
	return limitAnalyticsSlice(ranks, limit)
}

func fillAnalyticsRankingUsers(overview *AnalyticsOverview) {
	idSet := map[int]struct{}{}
	for _, rank := range overview.Rankings.AgentContribution {
		idSet[rank.AgentUserId] = struct{}{}
	}
	for _, rank := range overview.Rankings.UserTopups {
		idSet[rank.UserId] = struct{}{}
	}
	for _, rank := range overview.Rankings.UserConsumptions {
		idSet[rank.UserId] = struct{}{}
	}

	ids := make([]int, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}
	userMap := loadAnalyticsUserSummaryMap(ids)
	for i := range overview.Rankings.AgentContribution {
		if user, ok := userMap[overview.Rankings.AgentContribution[i].AgentUserId]; ok {
			overview.Rankings.AgentContribution[i].Username = user.Username
			overview.Rankings.AgentContribution[i].DisplayName = user.DisplayName
		}
	}
	for i := range overview.Rankings.UserTopups {
		if user, ok := userMap[overview.Rankings.UserTopups[i].UserId]; ok {
			overview.Rankings.UserTopups[i].Username = user.Username
			overview.Rankings.UserTopups[i].DisplayName = user.DisplayName
		}
	}
	for i := range overview.Rankings.UserConsumptions {
		if user, ok := userMap[overview.Rankings.UserConsumptions[i].UserId]; ok {
			overview.Rankings.UserConsumptions[i].Username = user.Username
			overview.Rankings.UserConsumptions[i].DisplayName = user.DisplayName
		}
	}
}

func loadAnalyticsUserSummaryMap(ids []int) map[int]analyticsUserSummary {
	users := map[int]analyticsUserSummary{}
	if len(ids) == 0 {
		return users
	}
	var rows []analyticsUserSummary
	if err := DB.Unscoped().Model(&User{}).Select("id, username, display_name").Where("id IN ?", ids).Find(&rows).Error; err != nil {
		common.SysLog("failed to load analytics users: " + err.Error())
		return users
	}
	for _, row := range rows {
		users[row.Id] = row
	}
	return users
}

func loadAnalyticsChannelNameMap(items map[int]analyticsConsumptionAccumulator) map[int]string {
	channelNames := map[int]string{}
	ids := make([]int, 0, len(items))
	for id := range items {
		if id > 0 {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return channelNames
	}

	var rows []struct {
		Id   int
		Name string
	}
	if err := DB.Model(&Channel{}).Select("id, name").Where("id IN ?", ids).Find(&rows).Error; err != nil {
		common.SysLog("failed to load analytics channels: " + err.Error())
		return channelNames
	}
	for _, row := range rows {
		channelNames[row.Id] = row.Name
	}
	return channelNames
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
		points = append(points, AnalyticsTrendPoint{
			Start: cursor,
			End:   pointEnd,
		})
	}
	return points
}

func analyticsTrendIndex(start int64, step int64, bucketCount int, timestamp int64) int {
	if step <= 0 || bucketCount == 0 || timestamp < start {
		return -1
	}
	idx := int((timestamp - start) / step)
	if idx < 0 || idx >= bucketCount {
		return -1
	}
	return idx
}

func newAnalyticsBalanceBucket(label string, minQuota int64, maxQuota *int64) AnalyticsBalanceBucket {
	return AnalyticsBalanceBucket{
		Label:    label,
		MinQuota: minQuota,
		MaxQuota: maxQuota,
	}
}

func int64Ptr(value int64) *int64 {
	return &value
}

func analyticsRatio(part float64, total float64) float64 {
	if total <= 0 {
		return 0
	}
	return part / total
}

func limitAnalyticsSlice[T any](items []T, limit int) []T {
	if limit <= 0 || len(items) <= limit {
		return items
	}
	return items[:limit]
}
