package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/billing_setting"
)

const (
	upstreamSyncRequestTimeout     = 30 * time.Second
	upstreamSyncMaxResponseBytes   = 20 << 20
	upstreamSyncTriggerManual      = "manual"
	upstreamSyncTriggerScheduled   = "scheduled"
	upstreamSyncTieredExprBilling  = billing_setting.BillingModeTieredExpr
	upstreamSyncRatioBilling       = billing_setting.BillingModeRatio
	upstreamSyncBillingModeOption  = "billing_setting.billing_mode"
	upstreamSyncBillingExprOption  = "billing_setting.billing_expr"
	upstreamSyncMaxBaseURLDisplay  = 1020
	upstreamSyncMaxVersionDisplay  = 250
	upstreamSyncMaxIssueTextLength = 400
)

var upstreamSyncHTTPClient = &http.Client{Timeout: upstreamSyncRequestTimeout}

var upstreamSyncSecretPattern = regexp.MustCompile(
	`(?i)(api[_-]?key|access[_-]?token|authorization|secret|token|key)([=:]\s*)([^\s&,;]+)`,
)

// fingerprint of all pricing fields the unified sync is allowed to write. The
// plugin billing expression option is deliberately absent: upstream prices
// never overwrite per-plugin task expressions.
var upstreamSyncPricingFields = []string{
	"ModelRatio",
	"ModelPrice",
	"CompletionRatio",
	"CacheRatio",
	"CreateCacheRatio",
	"ImageRatio",
	"AudioRatio",
	"AudioCompletionRatio",
	"billing_setting.billing_mode",
	"billing_setting.billing_expr",
}

var upstreamSyncLegacyPricingFields = []string{
	"ModelRatio",
	"ModelPrice",
	"CompletionRatio",
	"CacheRatio",
	"CreateCacheRatio",
	"ImageRatio",
	"AudioRatio",
	"AudioCompletionRatio",
}

// UpstreamSyncSummary is the result returned to the system task framework and
// the root console. All timestamps stay out of the summary; the persisted log
// row carries them.
type UpstreamSyncSummary struct {
	Status          string                          `json:"status"`
	LogId           int64                           `json:"log_id"`
	BaseURLs        []string                        `json:"base_urls"`
	PricingVersion  string                          `json:"pricing_version"`
	GroupsChecked   int                             `json:"groups_checked"`
	ChannelsChanged int                             `json:"channels_changed"`
	AddedModels     []string                        `json:"added_models"`
	RemovedModels   []string                        `json:"removed_models"`
	PriceChanges    []model.UpstreamSyncPriceChange `json:"price_changes"`
	Errors          []model.UpstreamSyncIssue       `json:"errors"`
	Message         string                          `json:"message"`
}

// upstreamStringList accepts both the array form used by the gateway and a
// legacy comma-separated string so group matching never silently breaks.
type upstreamStringList []string

func (l *upstreamStringList) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "null" || trimmed == "" {
		*l = nil
		return nil
	}
	var list []string
	if err := common.Unmarshal(data, &list); err == nil {
		*l = list
		return nil
	}
	var single string
	if err := common.Unmarshal(data, &single); err != nil {
		return fmt.Errorf("invalid string list")
	}
	if strings.TrimSpace(single) == "" {
		*l = nil
		return nil
	}
	parts := strings.Split(single, ",")
	for _, part := range parts {
		if value := strings.TrimSpace(part); value != "" {
			list = append(list, value)
		}
	}
	*l = list
	return nil
}

// upstreamNumber preserves the distinction between an absent field and an
// explicit zero; a present zero is a real price, not a missing value.
type upstreamNumber struct {
	Value float64
	Set   bool
}

func (n *upstreamNumber) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "null" || trimmed == "" {
		n.Set = false
		n.Value = 0
		return nil
	}
	var number float64
	if err := common.Unmarshal(data, &number); err == nil {
		n.Value = number
		n.Set = true
		return nil
	}
	var text string
	if err := common.Unmarshal(data, &text); err != nil {
		return fmt.Errorf("invalid number")
	}
	parsed, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
	if err != nil {
		return fmt.Errorf("invalid number")
	}
	n.Value = parsed
	n.Set = true
	return nil
}

type upstreamPricingItem struct {
	ModelName            string             `json:"model_name"`
	QuotaType            upstreamNumber     `json:"quota_type"`
	ModelRatio           upstreamNumber     `json:"model_ratio"`
	ModelPrice           upstreamNumber     `json:"model_price"`
	CompletionRatio      upstreamNumber     `json:"completion_ratio"`
	CacheRatio           upstreamNumber     `json:"cache_ratio"`
	CreateCacheRatio     upstreamNumber     `json:"create_cache_ratio"`
	ImageRatio           upstreamNumber     `json:"image_ratio"`
	AudioRatio           upstreamNumber     `json:"audio_ratio"`
	AudioCompletionRatio upstreamNumber     `json:"audio_completion_ratio"`
	EnableGroups         upstreamStringList `json:"enable_groups"`
	BillingMode          string             `json:"billing_mode"`
	BillingExpr          string             `json:"billing_expr"`
}

type upstreamPricingResponse struct {
	Success        *bool                 `json:"success"`
	Message        string                `json:"message"`
	PricingVersion string                `json:"pricing_version"`
	Data           []upstreamPricingItem `json:"data"`
	UsableGroup    map[string]any        `json:"usable_group"`
}

// upstreamPricingData is one base URL's validated pricing snapshot. GroupRatio
// is intentionally not modelled: the discounted group layer is local policy and
// must never be copied from upstream.
type upstreamPricingData struct {
	BaseURL     string
	Version     string
	Groups      map[string]map[string]*upstreamPricingItem
	Models      map[string]*upstreamPricingItem
	KnownGroups map[string]bool
}

type upstreamChannelModelPlan struct {
	Add      []string
	Remove   []string
	Final    []string
	Groups   int
	Warnings []model.UpstreamSyncIssue
}

type upstreamSyncRun struct {
	started         time.Time
	startedAt       int64
	trigger         string
	baseURLs        []string
	versions        []string
	groupsChecked   int
	channelsChanged int
	addedModels     []string
	removedModels   []string
	priceChanges    []model.UpstreamSyncPriceChange
	issues          []model.UpstreamSyncIssue
}

func normalizeUpstreamSyncTrigger(trigger string) string {
	trigger = strings.TrimSpace(trigger)
	if trigger == "" {
		return upstreamSyncTriggerManual
	}
	return trigger
}

func (r *upstreamSyncRun) addIssue(issue model.UpstreamSyncIssue) {
	if issue.Level == "" {
		issue.Level = model.UpstreamSyncIssueLevelError
	}
	if len(issue.Message) > upstreamSyncMaxIssueTextLength {
		issue.Message = issue.Message[:upstreamSyncMaxIssueTextLength]
	}
	r.issues = append(r.issues, issue)
}

func (r *upstreamSyncRun) hasErrors() bool {
	for _, issue := range r.issues {
		if issue.Level == "" || issue.Level == model.UpstreamSyncIssueLevelError {
			return true
		}
	}
	return false
}

func (r *upstreamSyncRun) status() string {
	if r.hasErrors() {
		return model.UpstreamSyncLogStatusPartial
	}
	return model.UpstreamSyncLogStatusSuccess
}

func (r *upstreamSyncRun) message(status string) string {
	message := fmt.Sprintf(
		"%s: checked %d group(s), changed %d channel(s), %d model(s) added, %d model(s) removed, %d price change(s)",
		status, r.groupsChecked, r.channelsChanged, len(r.addedModels), len(r.removedModels), len(r.priceChanges),
	)
	if r.hasErrors() {
		message += fmt.Sprintf(", %d error(s)", countUpstreamSyncErrors(r.issues))
	}
	return message
}

func countUpstreamSyncErrors(issues []model.UpstreamSyncIssue) int {
	count := 0
	for _, issue := range issues {
		if issue.Level == "" || issue.Level == model.UpstreamSyncIssueLevelError {
			count++
		}
	}
	return count
}

func (r *upstreamSyncRun) finalize(status, message string) (*UpstreamSyncSummary, error) {
	log := &model.UpstreamSyncLog{
		StartedAt:       r.startedAt,
		FinishedAt:      common.GetTimestamp(),
		Trigger:         r.trigger,
		Status:          status,
		BaseUrl:         truncateUpstreamSyncText(strings.Join(r.baseURLs, ", "), upstreamSyncMaxBaseURLDisplay),
		PricingVersion:  truncateUpstreamSyncText(strings.Join(r.versions, ","), upstreamSyncMaxVersionDisplay),
		GroupsChecked:   r.groupsChecked,
		ChannelsChanged: r.channelsChanged,
		AddedModels:     marshalUpstreamSyncJSON(nonNilStrings(r.addedModels)),
		RemovedModels:   marshalUpstreamSyncJSON(nonNilStrings(r.removedModels)),
		PriceChanges:    marshalUpstreamSyncJSON(nonNilPriceChanges(r.priceChanges)),
		Errors:          marshalUpstreamSyncJSON(nonNilIssues(r.issues)),
		DurationMs:      time.Since(r.started).Milliseconds(),
		Message:         message,
	}
	logErr := model.CreateUpstreamSyncLog(log)
	if logErr == nil {
		_, _ = model.TrimUpstreamSyncLogs(0)
	}
	summary := &UpstreamSyncSummary{
		Status:          status,
		LogId:           log.Id,
		BaseURLs:        nonNilStrings(r.baseURLs),
		PricingVersion:  log.PricingVersion,
		GroupsChecked:   r.groupsChecked,
		ChannelsChanged: r.channelsChanged,
		AddedModels:     nonNilStrings(r.addedModels),
		RemovedModels:   nonNilStrings(r.removedModels),
		PriceChanges:    nonNilPriceChanges(r.priceChanges),
		Errors:          nonNilIssues(r.issues),
		Message:         message,
	}
	if logErr != nil {
		return summary, fmt.Errorf("persist upstream sync log: %s", sanitizeUpstreamSyncText(logErr.Error()))
	}
	return summary, nil
}

func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func nonNilPriceChanges(values []model.UpstreamSyncPriceChange) []model.UpstreamSyncPriceChange {
	if values == nil {
		return []model.UpstreamSyncPriceChange{}
	}
	return values
}

func nonNilIssues(values []model.UpstreamSyncIssue) []model.UpstreamSyncIssue {
	if values == nil {
		return []model.UpstreamSyncIssue{}
	}
	return values
}

func marshalUpstreamSyncJSON(value any) string {
	data, err := common.Marshal(value)
	if err != nil {
		return "[]"
	}
	return string(data)
}

func truncateUpstreamSyncText(value string, limit int) string {
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return value[:limit]
}

func sanitizeUpstreamSyncText(text string) string {
	text = strings.TrimSpace(text)
	text = upstreamSyncSecretPattern.ReplaceAllString(text, "$1$2[redacted]")
	if len(text) > upstreamSyncMaxIssueTextLength {
		text = text[:upstreamSyncMaxIssueTextLength]
	}
	return text
}

func sanitizeUpstreamSyncError(err error) string {
	if err == nil {
		return ""
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) && urlErr.Err != nil {
		err = urlErr.Err
	}
	return sanitizeUpstreamSyncText(err.Error())
}

// RunUpstreamSync refreshes enabled channels' model lists and the local base
// prices from every distinct upstream /api/pricing endpoint. It never copies
// upstream GroupRatio values: local group ratios are the discount layer.
func RunUpstreamSync(ctx context.Context, trigger string, report func(processed, total int)) (*UpstreamSyncSummary, error) {
	started := time.Now()
	run := &upstreamSyncRun{
		started:   started,
		startedAt: started.Unix(),
		trigger:   normalizeUpstreamSyncTrigger(trigger),
	}
	if ctx == nil {
		ctx = context.Background()
	}

	channels, err := loadEnabledUpstreamSyncChannels()
	if err != nil {
		run.addIssue(model.UpstreamSyncIssue{
			Level:   model.UpstreamSyncIssueLevelError,
			Scope:   "database",
			Message: "load enabled channels failed: " + sanitizeUpstreamSyncText(err.Error()),
		})
		summary, logErr := run.finalize(model.UpstreamSyncLogStatusFailed, "load enabled channels failed")
		if logErr != nil {
			return summary, logErr
		}
		return summary, errors.New(summary.Message)
	}

	total := len(channels)
	if report != nil {
		report(0, total)
	}
	if total == 0 {
		summary, logErr := run.finalize(model.UpstreamSyncLogStatusSuccess, "no enabled channels")
		if logErr != nil {
			return summary, logErr
		}
		return summary, nil
	}

	// Deduplicate base URLs and keep the channel-to-base mapping. Invalid or
	// missing base URLs are reported but never touched.
	channelsByBase := make(map[string][]*model.Channel)
	invalidChannels := make(map[*model.Channel]bool)
	var baseURLs []string
	for _, channel := range channels {
		normalized, normalizeErr := normalizeUpstreamBaseURL(channel.GetBaseURL())
		if normalizeErr != nil {
			invalidChannels[channel] = true
			run.addIssue(model.UpstreamSyncIssue{
				Level:       model.UpstreamSyncIssueLevelError,
				Scope:       "channel",
				ChannelID:   channel.Id,
				ChannelName: channel.Name,
				Message:     "invalid channel base_url: " + sanitizeUpstreamSyncText(normalizeErr.Error()),
			})
			continue
		}
		if _, exists := channelsByBase[normalized]; !exists {
			baseURLs = append(baseURLs, normalized)
		}
		channelsByBase[normalized] = append(channelsByBase[normalized], channel)
	}
	sort.Strings(baseURLs)
	run.baseURLs = baseURLs

	validPricing := make(map[string]*upstreamPricingData)
	for _, baseURL := range baseURLs {
		if err := ctx.Err(); err != nil {
			run.addIssue(model.UpstreamSyncIssue{
				Level:   model.UpstreamSyncIssueLevelError,
				Scope:   "sync",
				BaseURL: baseURL,
				Message: "upstream sync canceled: " + sanitizeUpstreamSyncText(err.Error()),
			})
			summary, logErr := run.finalize(model.UpstreamSyncLogStatusFailed, "upstream sync canceled")
			if logErr != nil {
				return summary, logErr
			}
			return summary, err
		}
		data, fetchErr := fetchUpstreamPricing(ctx, baseURL)
		if fetchErr != nil {
			run.addIssue(model.UpstreamSyncIssue{
				Level:   model.UpstreamSyncIssueLevelError,
				Scope:   "base_url",
				BaseURL: baseURL,
				Message: "fetch upstream pricing failed: " + sanitizeUpstreamSyncText(fetchErr.Error()),
			})
			continue
		}
		validPricing[baseURL] = data
		if data.Version != "" {
			run.versions = appendUnique(run.versions, data.Version)
		}
	}

	if len(validPricing) == 0 {
		message := "no valid upstream pricing response"
		for _, issue := range run.issues {
			if issue.Scope == "base_url" && issue.Level == model.UpstreamSyncIssueLevelError {
				message = issue.Message
				if issue.BaseURL != "" {
					message = issue.BaseURL + ": " + message
				}
				break
			}
		}
		summary, logErr := run.finalize(model.UpstreamSyncLogStatusFailed, message)
		if logErr != nil {
			return summary, logErr
		}
		return summary, errors.New(summary.Message)
	}

	// Group ratio from upstream is ignored on purpose; the local discount layer
	// stays authoritative.
	pricesByModel := make(map[string]*upstreamPricingItem)
	for _, baseURL := range baseURLs {
		data := validPricing[baseURL]
		if data == nil {
			continue
		}
		for name, item := range data.Models {
			if _, exists := pricesByModel[name]; !exists {
				pricesByModel[name] = item
			}
		}
	}

	finalModels := make(map[string]bool)
	processed := 0
	now := common.GetTimestamp()
	for _, channel := range channels {
		processed++
		if invalidChannels[channel] {
			if report != nil {
				report(processed, total)
			}
			continue
		}
		baseURL, normalizeErr := normalizeUpstreamBaseURL(channel.GetBaseURL())
		if normalizeErr != nil {
			if report != nil {
				report(processed, total)
			}
			continue
		}
		data := validPricing[baseURL]
		if data == nil {
			// Already reported as a base URL fetch error; leave this channel alone.
			if report != nil {
				report(processed, total)
			}
			continue
		}

		plan := planUpstreamChannelModels(channel, data)
		for _, warning := range plan.Warnings {
			run.addIssue(warning)
		}
		run.groupsChecked += plan.Groups
		modelsChanged := len(plan.Add) > 0 || len(plan.Remove) > 0
		if err := persistUpstreamChannelPlan(channel, plan, now); err != nil {
			run.addIssue(model.UpstreamSyncIssue{
				Level:       model.UpstreamSyncIssueLevelError,
				Scope:       "channel",
				BaseURL:     baseURL,
				ChannelID:   channel.Id,
				ChannelName: channel.Name,
				Message:     "persist channel model changes failed: " + sanitizeUpstreamSyncText(err.Error()),
			})
			for _, modelName := range normalizeSyncModelNames(channel.GetModels()) {
				finalModels[modelName] = true
			}
			if report != nil {
				report(processed, total)
			}
			continue
		}
		if modelsChanged {
			run.channelsChanged++
			run.addedModels = appendUniqueStrings(run.addedModels, plan.Add...)
			run.removedModels = appendUniqueStrings(run.removedModels, plan.Remove...)
		}
		for _, modelName := range plan.Final {
			finalModels[modelName] = true
		}
		if report != nil {
			report(processed, total)
		}
	}

	sort.Strings(run.addedModels)
	sort.Strings(run.removedModels)

	priceChanges, priceIssues := applyUpstreamPricing(finalModels, pricesByModel)
	for _, issue := range priceIssues {
		run.addIssue(issue)
	}
	run.priceChanges = priceChanges
	if len(priceChanges) > 0 {
		run.priceChanges = nonNilPriceChanges(run.priceChanges)
	}

	model.InitChannelCache()
	model.InvalidatePricingCache()

	status := run.status()
	message := run.message(status)
	summary, logErr := run.finalize(status, message)
	if report != nil {
		report(total, total)
	}
	if logErr != nil {
		return summary, logErr
	}
	return summary, nil
}

func loadEnabledUpstreamSyncChannels() ([]*model.Channel, error) {
	var channels []*model.Channel
	err := model.DB.Where("status = ?", common.ChannelStatusEnabled).Find(&channels).Error
	return channels, err
}

func normalizeUpstreamBaseURL(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", errors.New("base_url is empty")
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", errors.New("base_url is invalid")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("base_url must use http or https")
	}
	if parsed.Host == "" {
		return "", errors.New("base_url host is empty")
	}
	// Credentials, query strings and fragments can contain gateway keys and are
	// never needed by the public pricing endpoint.
	parsed.User = nil
	parsed.RawQuery = ""
	parsed.Fragment = ""
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	return parsed.String(), nil
}

func fetchUpstreamPricing(ctx context.Context, baseURL string) (*upstreamPricingData, error) {
	endpoint := strings.TrimRight(baseURL, "/") + "/api/pricing"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build pricing request: %w", err)
	}
	request.Header.Set("Accept", "application/json")

	response, err := upstreamSyncHTTPClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upstream pricing returned HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, upstreamSyncMaxResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read upstream pricing response: %w", err)
	}
	if len(body) > upstreamSyncMaxResponseBytes {
		return nil, errors.New("upstream pricing response is too large")
	}

	var payload upstreamPricingResponse
	if err := common.Unmarshal(body, &payload); err != nil {
		return nil, errors.New("upstream pricing response is not valid JSON")
	}
	if payload.Success != nil && !*payload.Success {
		message := strings.TrimSpace(payload.Message)
		if message == "" {
			message = "upstream rejected the pricing request"
		}
		return nil, errors.New(message)
	}
	if len(payload.Data) == 0 {
		return nil, errors.New("upstream pricing response contains no models")
	}

	data := &upstreamPricingData{
		BaseURL:     baseURL,
		Version:     strings.TrimSpace(payload.PricingVersion),
		Groups:      make(map[string]map[string]*upstreamPricingItem),
		Models:      make(map[string]*upstreamPricingItem),
		KnownGroups: make(map[string]bool),
	}
	for group := range payload.UsableGroup {
		if name := strings.TrimSpace(group); name != "" {
			data.KnownGroups[name] = true
		}
	}

	for index := range payload.Data {
		item := &payload.Data[index]
		name := strings.TrimSpace(item.ModelName)
		if name == "" {
			return nil, errors.New("upstream pricing response contains a model without a name")
		}
		item.ModelName = name
		groups := normalizeSyncModelNames([]string(item.EnableGroups))

		if existing, ok := data.Models[name]; ok {
			for _, group := range groups {
				data.KnownGroups[group] = true
				if data.Groups[group] == nil {
					data.Groups[group] = make(map[string]*upstreamPricingItem)
				}
				data.Groups[group][name] = existing
			}
			continue
		}
		data.Models[name] = item
		for _, group := range groups {
			data.KnownGroups[group] = true
			if data.Groups[group] == nil {
				data.Groups[group] = make(map[string]*upstreamPricingItem)
			}
			data.Groups[group][name] = item
		}
	}
	return data, nil
}

func planUpstreamChannelModels(channel *model.Channel, data *upstreamPricingData) upstreamChannelModelPlan {
	plan := upstreamChannelModelPlan{Final: []string{}}
	current := normalizeSyncModelNames(channel.GetModels())
	currentSet := make(map[string]bool, len(current))
	for _, name := range current {
		currentSet[name] = true
	}

	settings := channel.GetOtherSettings()
	ignored := buildUpstreamSyncIgnoredMatcher(settings.UpstreamModelUpdateIgnoredModels)

	groups := channel.GetGroups()
	plan.Groups = len(groups)
	upstreamSet := make(map[string]bool)
	var upstreamOrdered []string
	for _, group := range groups {
		group = strings.TrimSpace(group)
		if group == "" {
			continue
		}
		groupModels := data.Groups[group]
		if len(groupModels) == 0 {
			if data.KnownGroups[group] {
				plan.Warnings = append(plan.Warnings, model.UpstreamSyncIssue{
					Level:       model.UpstreamSyncIssueLevelWarning,
					Scope:       "group",
					BaseURL:     data.BaseURL,
					ChannelID:   channel.Id,
					ChannelName: channel.Name,
					Group:       group,
					Message:     "group_empty: upstream exposes the group but has no models in it",
				})
			} else {
				plan.Warnings = append(plan.Warnings, model.UpstreamSyncIssue{
					Level:       model.UpstreamSyncIssueLevelWarning,
					Scope:       "group",
					BaseURL:     data.BaseURL,
					ChannelID:   channel.Id,
					ChannelName: channel.Name,
					Group:       group,
					Message:     "group_missing: upstream pricing response does not contain this group",
				})
			}
			continue
		}
		names := make([]string, 0, len(groupModels))
		for name := range groupModels {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			if upstreamSet[name] {
				continue
			}
			upstreamSet[name] = true
			upstreamOrdered = append(upstreamOrdered, name)
		}
	}

	for _, name := range current {
		if ignored(name) || upstreamSet[name] {
			plan.Final = append(plan.Final, name)
			continue
		}
		plan.Remove = append(plan.Remove, name)
	}
	for _, name := range upstreamOrdered {
		if currentSet[name] || ignored(name) {
			continue
		}
		plan.Final = append(plan.Final, name)
		plan.Add = append(plan.Add, name)
	}
	plan.Final = normalizeSyncModelNames(plan.Final)
	return plan
}

func buildUpstreamSyncIgnoredMatcher(ignoredModels []string) func(string) bool {
	normalized := normalizeSyncModelNames(ignoredModels)
	return func(modelName string) bool {
		for _, ignored := range normalized {
			if body, ok := strings.CutPrefix(ignored, "regex:"); ok {
				matched, err := regexp.MatchString(strings.TrimSpace(body), modelName)
				if err == nil && matched {
					return true
				}
				continue
			}
			if ignored == modelName {
				return true
			}
		}
		return false
	}
}

func persistUpstreamChannelPlan(channel *model.Channel, plan upstreamChannelModelPlan, now int64) error {
	settings := channel.GetOtherSettings()
	settings.UpstreamModelUpdateLastCheckTime = now
	settings.UpstreamModelUpdateLastDetectedModels = nonNilStrings(plan.Add)
	settings.UpstreamModelUpdateLastRemovedModels = nonNilStrings(plan.Remove)
	channel.SetOtherSettings(settings)

	updates := map[string]any{"settings": channel.OtherSettings}
	modelsChanged := len(plan.Add) > 0 || len(plan.Remove) > 0
	if modelsChanged {
		channel.Models = strings.Join(plan.Final, ",")
		updates["models"] = channel.Models
	}
	if err := model.DB.Model(&model.Channel{}).Where("id = ?", channel.Id).Updates(updates).Error; err != nil {
		return err
	}
	if modelsChanged {
		if err := channel.UpdateAbilities(nil); err != nil {
			return fmt.Errorf("update abilities: %w", err)
		}
	}
	return nil
}

func applyUpstreamPricing(
	finalModels map[string]bool,
	pricesByModel map[string]*upstreamPricingItem,
) ([]model.UpstreamSyncPriceChange, []model.UpstreamSyncIssue) {
	names := make([]string, 0, len(finalModels))
	for name := range finalModels {
		if pricesByModel[name] != nil {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	if len(names) == 0 {
		return []model.UpstreamSyncPriceChange{}, nil
	}

	for attempt := 0; attempt < 2; attempt++ {
		snapshot, err := model.GetModelPricingSnapshot(names)
		if err != nil {
			return nil, []model.UpstreamSyncIssue{{
				Level:   model.UpstreamSyncIssueLevelError,
				Scope:   "pricing",
				Message: "read model pricing snapshot failed: " + sanitizeUpstreamSyncText(err.Error()),
			}}
		}
		configured := make(map[string]model.PricingValues, len(snapshot.Entries))
		versions := make(map[string]string, len(snapshot.Entries))
		for _, entry := range snapshot.Entries {
			configured[entry.ModelName] = entry.Configured
			versions[entry.ModelName] = entry.Version
		}

		var changes []model.ModelPricingChange
		var recorded []model.UpstreamSyncPriceChange
		var issues []model.UpstreamSyncIssue
		for _, name := range names {
			draft, deriveErr := deriveUpstreamPricing(configured[name], pricesByModel[name])
			if deriveErr != nil {
				issues = append(issues, model.UpstreamSyncIssue{
					Level:   model.UpstreamSyncIssueLevelError,
					Scope:   "pricing",
					Message: sanitizeUpstreamSyncText(deriveErr.Error()),
				})
				continue
			}
			fields, before, after := diffUpstreamPricing(configured[name], draft)
			if len(fields) == 0 {
				continue
			}
			changes = append(changes, model.ModelPricingChange{
				ModelName:       name,
				ExpectedVersion: versions[name],
				Pricing:         draft,
			})
			recorded = append(recorded, model.UpstreamSyncPriceChange{
				ModelName: name,
				Fields:    fields,
				Before:    before,
				After:     after,
			})
		}
		if len(changes) == 0 {
			return recorded, issues
		}

		err = model.UpdateModelPricing(changes)
		if err == nil {
			return recorded, issues
		}
		if errors.Is(err, model.ErrModelPricingConflict) && attempt == 0 {
			continue
		}
		// One model with an upstream expression this gateway cannot validate must
		// not block every other model's price. Retry each change separately and
		// keep only the per-model failures in the sync log.
		perModelIssues := issues
		applied := make([]model.UpstreamSyncPriceChange, 0, len(recorded))
		for index, change := range changes {
			if perErr := model.UpdateModelPricing([]model.ModelPricingChange{change}); perErr != nil {
				perModelIssues = append(perModelIssues, model.UpstreamSyncIssue{
					Level:   model.UpstreamSyncIssueLevelError,
					Scope:   "pricing",
					Message: "model " + change.ModelName + ": " + sanitizeUpstreamSyncText(perErr.Error()),
				})
				continue
			}
			applied = append(applied, recorded[index])
		}
		if len(perModelIssues) == len(issues) {
			return applied, issues
		}
		return applied, perModelIssues
	}
	return nil, []model.UpstreamSyncIssue{{
		Level:   model.UpstreamSyncIssueLevelError,
		Scope:   "pricing",
		Message: "update model pricing conflict persisted after retry",
	}}
}

func deriveUpstreamPricing(base model.PricingValues, item *upstreamPricingItem) (model.PricingValues, error) {
	draft := model.PricingValues{}
	for key, value := range base {
		draft[key] = value
	}
	billingMode := strings.ToLower(strings.TrimSpace(item.BillingMode))
	if billingMode == upstreamSyncTieredExprBilling {
		expression := strings.TrimSpace(item.BillingExpr)
		if expression == "" {
			return nil, fmt.Errorf("model %s: upstream tiered expression is empty", item.ModelName)
		}
		// The expression is authoritative; drop any stale legacy ratio so a
		// later mode change cannot silently resurrect an old price.
		for _, key := range upstreamSyncLegacyPricingFields {
			delete(draft, key)
		}
		draft[upstreamSyncBillingModeOption] = upstreamSyncTieredExprBilling
		draft[upstreamSyncBillingExprOption] = expression
		return draft, nil
	}

	draft[upstreamSyncBillingModeOption] = upstreamSyncRatioBilling
	// A tiered expression from a previous sync must not keep shadowing the new
	// per-token/per-call price.
	delete(draft, upstreamSyncBillingExprOption)

	quotaType := 0
	if item.QuotaType.Set {
		quotaType = int(math.Round(item.QuotaType.Value))
	}
	wroteBasePrice := false
	if quotaType == 1 {
		// A per-call model must not retain an old token ratio that would make
		// the effective price flip depending on which field billing reads first.
		delete(draft, "ModelRatio")
		delete(draft, "CompletionRatio")
		if item.ModelPrice.Set {
			draft["ModelPrice"] = item.ModelPrice.Value
			wroteBasePrice = true
		}
	} else {
		delete(draft, "ModelPrice")
		if item.ModelRatio.Set {
			draft["ModelRatio"] = item.ModelRatio.Value
			wroteBasePrice = true
		}
		if item.CompletionRatio.Set {
			draft["CompletionRatio"] = item.CompletionRatio.Value
			wroteBasePrice = true
		}
	}
	if !wroteBasePrice {
		// Be tolerant of upstreams that omit quota_type: accept whichever base
		// price they actually sent.
		if item.ModelPrice.Set {
			draft["ModelPrice"] = item.ModelPrice.Value
			wroteBasePrice = true
		} else if item.ModelRatio.Set {
			draft["ModelRatio"] = item.ModelRatio.Value
			wroteBasePrice = true
		}
	}
	if !wroteBasePrice {
		return nil, fmt.Errorf("model %s: upstream response has no base price", item.ModelName)
	}

	for key, number := range map[string]upstreamNumber{
		"CacheRatio":           item.CacheRatio,
		"CreateCacheRatio":     item.CreateCacheRatio,
		"ImageRatio":           item.ImageRatio,
		"AudioRatio":           item.AudioRatio,
		"AudioCompletionRatio": item.AudioCompletionRatio,
	} {
		if number.Set {
			draft[key] = number.Value
		}
	}
	return draft, nil
}

func diffUpstreamPricing(before, after model.PricingValues) ([]string, map[string]any, map[string]any) {
	beforeValues := make(map[string]any)
	afterValues := make(map[string]any)
	var fields []string
	for _, key := range upstreamSyncPricingFields {
		beforeValue, beforeSet := before[key]
		afterValue, afterSet := after[key]
		if !beforeSet && !afterSet {
			continue
		}
		if beforeSet && afterSet && upstreamSyncPricingValueEqual(beforeValue, afterValue) {
			continue
		}
		fields = append(fields, key)
		if beforeSet {
			beforeValues[key] = beforeValue
		} else {
			beforeValues[key] = nil
		}
		if afterSet {
			afterValues[key] = afterValue
		} else {
			afterValues[key] = nil
		}
	}
	if len(fields) == 0 {
		return nil, nil, nil
	}
	return fields, beforeValues, afterValues
}

func upstreamSyncPricingValueEqual(left, right any) bool {
	leftFloat, leftIsFloat := upstreamSyncFloat(left)
	rightFloat, rightIsFloat := upstreamSyncFloat(right)
	if leftIsFloat && rightIsFloat {
		return math.Abs(leftFloat-rightFloat) < 1e-9
	}
	return left == right
}

func upstreamSyncFloat(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case int32:
		return float64(typed), true
	default:
		return 0, false
	}
}

func normalizeSyncModelNames(models []string) []string {
	seen := make(map[string]bool, len(models))
	result := make([]string, 0, len(models))
	for _, modelName := range models {
		trimmed := strings.TrimSpace(modelName)
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		result = append(result, trimmed)
	}
	return result
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func appendUniqueStrings(values []string, additions ...string) []string {
	for _, addition := range additions {
		values = appendUnique(values, addition)
	}
	return values
}
