package model

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	RefundBatchPending    = "pending"
	RefundBatchRunning    = "running"
	RefundBatchCompleted  = "completed"
	RefundBatchFailed     = "failed"
	RefundItemPending     = "pending"
	RefundItemSuccess     = "success"
	RefundItemSkipped     = "skipped"
	RefundItemFailed      = "failed"
	RefundScanPageSize    = 200
	RefundItemPageSize    = 100
	RefundItemMaxAttempts = 3
	RefundMaxRangeSeconds = 7 * 24 * 60 * 60
)

type RefundBatch struct {
	Id               int          `json:"id"`
	IdempotencyKey   string       `json:"idempotency_key" gorm:"type:varchar(128);uniqueIndex;not null"`
	StartTime        int64        `json:"start_time" gorm:"bigint;index"`
	EndTime          int64        `json:"end_time" gorm:"bigint;index"`
	ChannelIds       string       `json:"channel_ids" gorm:"type:text"`
	ModelNames       string       `json:"model_names" gorm:"type:text"`
	Ratio            int          `json:"ratio"`
	Reason           string       `json:"reason" gorm:"type:varchar(500)"`
	OperatorId       int          `json:"operator_id"`
	Status           string       `json:"status" gorm:"type:varchar(32);index"`
	TotalItems       int          `json:"total_items"`
	SuccessItems     int          `json:"success_items"`
	SkippedItems     int          `json:"skipped_items"`
	FailedItems      int          `json:"failed_items"`
	TotalQuota       int64        `json:"total_quota" gorm:"bigint"`
	RefundedQuota    int64        `json:"refunded_quota" gorm:"bigint"`
	Error            string       `json:"error" gorm:"type:text"`
	ScanCursor       int          `json:"scan_cursor" gorm:"index"`
	ScanCompleted    bool         `json:"scan_completed" gorm:"index"`
	ScanSkippedItems int          `json:"-"`
	LeaseOwner       string       `json:"-" gorm:"type:varchar(128);index"`
	LeaseExpiresAt   int64        `json:"-" gorm:"bigint;index"`
	CreatedAt        int64        `json:"created_at" gorm:"bigint;index"`
	UpdatedAt        int64        `json:"updated_at" gorm:"bigint"`
	Items            []RefundItem `json:"items,omitempty" gorm:"foreignKey:BatchId"`
	ItemPage         int          `json:"item_page" gorm:"-"`
	ItemPageSize     int          `json:"item_page_size" gorm:"-"`
	ItemTotal        int64        `json:"item_total" gorm:"-"`
}

type RefundItem struct {
	Id                int    `json:"id"`
	BatchId           int    `json:"batch_id" gorm:"uniqueIndex:idx_refund_batch_source,priority:1;index"`
	SourceLogId       int    `json:"source_log_id" gorm:"uniqueIndex:idx_refund_batch_source,priority:2;index"`
	UserId            int    `json:"user_id" gorm:"index"`
	ChannelId         int    `json:"channel_id"`
	ModelName         string `json:"model_name" gorm:"type:varchar(255)"`
	TokenId           int    `json:"token_id"`
	SourceQuota       int    `json:"source_quota"`
	RefundQuota       int    `json:"refund_quota"`
	Status            string `json:"status" gorm:"type:varchar(32);index"`
	Message           string `json:"message" gorm:"type:varchar(500)"`
	AttemptCount      int    `json:"attempt_count"`
	NextAttemptAt     int64  `json:"next_attempt_at" gorm:"bigint;index"`
	LogRecorded       bool   `json:"log_recorded" gorm:"index"`
	LogLeaseOwner     string `json:"-" gorm:"type:varchar(128);index"`
	LogLeaseExpiresAt int64  `json:"-" gorm:"bigint;index"`
	LogAttemptCount   int    `json:"-"`
	LogNextAttemptAt  int64  `json:"-" gorm:"bigint;index"`
	LogError          string `json:"-" gorm:"type:text"`
	CreatedAt         int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt         int64  `json:"updated_at" gorm:"bigint"`
}

type RefundFilter struct {
	StartTime  int64    `json:"start_time"`
	EndTime    int64    `json:"end_time"`
	ChannelIds []int    `json:"channel_ids"`
	ModelNames []string `json:"model_names"`
	Ratio      int      `json:"ratio"`
	Reason     string   `json:"reason"`
}

type RefundPreview struct {
	MatchedItems int                        `json:"matched_items"`
	SourceQuota  int64                      `json:"source_quota"`
	RefundQuota  int64                      `json:"refund_quota"`
	Skipped      int                        `json:"skipped"`
	ByChannel    map[int]RefundBreakdown    `json:"by_channel"`
	ByModel      map[string]RefundBreakdown `json:"by_model"`
}

type RefundBreakdown struct {
	Items       int   `json:"items"`
	SourceQuota int64 `json:"source_quota"`
	RefundQuota int64 `json:"refund_quota"`
}

type RefundOptions struct {
	Channels []RefundOptionChannel `json:"channels"`
	Models   []string              `json:"models"`
}
type RefundOptionChannel struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
}

func validateRefundFilter(f RefundFilter) error {
	if f.StartTime <= 0 || f.EndTime <= 0 || f.StartTime > f.EndTime {
		return errors.New("invalid refund time range")
	}
	if f.EndTime-f.StartTime > RefundMaxRangeSeconds {
		return errors.New("refund time range must not exceed 7 days")
	}
	if len(f.ChannelIds) == 0 {
		return errors.New("channel_ids is required")
	}
	if f.Ratio < 1 || f.Ratio > 100 {
		return errors.New("ratio must be between 1 and 100")
	}
	reason := strings.TrimSpace(f.Reason)
	if reason == "" {
		return errors.New("refund reason is required")
	}
	if utf8.RuneCountInString(reason) > 500 {
		return errors.New("refund reason must not exceed 500 characters")
	}
	return nil
}

func refundSourceEligible(log *Log) bool {
	var other map[string]interface{}
	if common.UnmarshalJsonStr(log.Other, &other) != nil {
		return false
	}
	source, _ := other["billing_source"].(string)
	return source == "wallet"
}

func queryRefundLogsPage(f RefundFilter, afterId int, limit int) ([]*Log, error) {
	if err := validateRefundFilter(f); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = RefundScanPageSize
	}
	tx := LOG_DB.Where("id > ? AND type = ? AND created_at >= ? AND created_at <= ? AND channel_id IN ? AND quota > 0", afterId, LogTypeConsume, f.StartTime, f.EndTime, f.ChannelIds)
	if len(f.ModelNames) > 0 {
		tx = tx.Where("model_name IN ?", f.ModelNames)
	}
	var logs []*Log
	if err := tx.Order("id asc").Limit(limit).Find(&logs).Error; err != nil {
		return nil, err
	}
	return logs, nil
}

func filterEligibleRefundLogs(logs []*Log) ([]*Log, int) {
	eligible := make([]*Log, 0, len(logs))
	skipped := 0
	for _, log := range logs {
		// First wallet-only release is deliberately fail-closed. Modern wallet logs
		// explicitly carry billing_source=wallet; missing/unknown may be subscription.
		if refundSourceEligible(log) {
			eligible = append(eligible, log)
		} else {
			skipped++
		}
	}
	return eligible, skipped
}

func calcRefundAmountForLogs(logs []*Log, ratio int) (map[int]int, error) {
	amounts := make(map[int]int, len(logs))
	if len(logs) == 0 {
		return amounts, nil
	}
	sourceIds := make([]int, 0, len(logs))
	for _, l := range logs {
		sourceIds = append(sourceIds, l.Id)
	}
	var refundedRows []struct {
		SourceLogId int
		Refunded    int64
	}
	if err := DB.Model(&RefundItem{}).
		Select("source_log_id, COALESCE(SUM(refund_quota),0) as refunded").
		Where("source_log_id IN ? AND status = ?", sourceIds, RefundItemSuccess).
		Group("source_log_id").
		Scan(&refundedRows).Error; err != nil {
		return nil, err
	}
	refunded := make(map[int]int64, len(refundedRows))
	for _, row := range refundedRows {
		refunded[row.SourceLogId] = row.Refunded
	}
	for _, l := range logs {
		remaining := int64(l.Quota) - refunded[l.Id]
		if remaining <= 0 {
			amounts[l.Id] = 0
			continue
		}
		requested := int64(l.Quota) * int64(ratio) / 100
		if requested > remaining {
			requested = remaining
		}
		if requested > int64(^uint(0)>>1) {
			return nil, errors.New("refund amount overflow")
		}
		amounts[l.Id] = int(requested)
	}
	return amounts, nil
}

func PreviewRefund(f RefundFilter) (*RefundPreview, error) {
	if err := validateRefundFilter(f); err != nil {
		return nil, err
	}
	p := &RefundPreview{ByChannel: map[int]RefundBreakdown{}, ByModel: map[string]RefundBreakdown{}}
	cursor := 0
	for {
		rawLogs, err := queryRefundLogsPage(f, cursor, RefundScanPageSize)
		if err != nil {
			return nil, err
		}
		if len(rawLogs) == 0 {
			break
		}
		cursor = rawLogs[len(rawLogs)-1].Id
		logs, skipped := filterEligibleRefundLogs(rawLogs)
		p.Skipped += skipped
		amounts, err := calcRefundAmountForLogs(logs, f.Ratio)
		if err != nil {
			return nil, err
		}
		for _, l := range logs {
			r := amounts[l.Id]
			if r <= 0 {
				p.Skipped++
				continue
			}
			p.MatchedItems++
			p.SourceQuota += int64(l.Quota)
			p.RefundQuota += int64(r)
			bc := p.ByChannel[l.ChannelId]
			bc.Items++
			bc.SourceQuota += int64(l.Quota)
			bc.RefundQuota += int64(r)
			p.ByChannel[l.ChannelId] = bc
			bm := p.ByModel[l.ModelName]
			bm.Items++
			bm.SourceQuota += int64(l.Quota)
			bm.RefundQuota += int64(r)
			p.ByModel[l.ModelName] = bm
		}
		if len(rawLogs) < RefundScanPageSize {
			break
		}
	}
	return p, nil
}

func GetRefundOptions(start, end int64) (*RefundOptions, error) {
	if start <= 0 || end <= 0 || start > end {
		return nil, errors.New("invalid refund time range")
	}
	if end-start > RefundMaxRangeSeconds {
		return nil, errors.New("refund time range must not exceed 7 days")
	}
	channelSet := map[int]bool{}
	modelSet := map[string]bool{}
	cursor := 0
	for {
		var logs []*Log
		if err := LOG_DB.Where("id > ? AND type = ? AND created_at >= ? AND created_at <= ? AND quota > 0", cursor, LogTypeConsume, start, end).Order("id asc").Limit(RefundScanPageSize).Find(&logs).Error; err != nil {
			return nil, err
		}
		if len(logs) == 0 {
			break
		}
		cursor = logs[len(logs)-1].Id
		for _, l := range logs {
			if refundSourceEligible(l) {
				channelSet[l.ChannelId] = true
				if l.ModelName != "" {
					modelSet[l.ModelName] = true
				}
			}
		}
		if len(logs) < RefundScanPageSize {
			break
		}
	}
	ids := make([]int, 0, len(channelSet))
	for id := range channelSet {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	names := map[int]string{}
	var channels []Channel
	if len(ids) > 0 {
		_ = DB.Select("id", "name").Where("id IN ?", ids).Find(&channels).Error
	}
	for _, c := range channels {
		names[c.Id] = c.Name
	}
	out := &RefundOptions{}
	for _, id := range ids {
		out.Channels = append(out.Channels, RefundOptionChannel{Id: id, Name: names[id]})
	}
	for m := range modelSet {
		out.Models = append(out.Models, m)
	}
	sort.Strings(out.Models)
	return out, nil
}

func CreateRefundBatch(f RefundFilter, key string, operatorId int) (*RefundBatch, error) {
	if err := validateRefundFilter(f); err != nil {
		return nil, err
	}
	key = strings.TrimSpace(key)
	if key == "" || len(key) > 128 {
		return nil, errors.New("valid idempotency_key is required")
	}
	var existing RefundBatch
	if err := DB.Where("idempotency_key = ?", key).First(&existing).Error; err == nil {
		return getRefundBatchSummary(existing.Id)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	channels, _ := common.Marshal(f.ChannelIds)
	models, _ := common.Marshal(f.ModelNames)
	now := common.GetTimestamp()
	batch := RefundBatch{IdempotencyKey: key, StartTime: f.StartTime, EndTime: f.EndTime, ChannelIds: string(channels), ModelNames: string(models), Ratio: f.Ratio, Reason: strings.TrimSpace(f.Reason), OperatorId: operatorId, Status: RefundBatchPending, CreatedAt: now, UpdatedAt: now}
	err := DB.Create(&batch).Error
	if err != nil {
		if e := DB.Where("idempotency_key = ?", key).First(&existing).Error; e == nil {
			return getRefundBatchSummary(existing.Id)
		}
		return nil, err
	}
	return &batch, nil
}

// CreateAndRunRefund is retained for callers and tests that require a blocking
// execution. HTTP handlers should use CreateRefundBatch so the durable worker
// owns execution after the request returns.
func CreateAndRunRefund(f RefundFilter, key string, operatorId int) (*RefundBatch, error) {
	batch, err := CreateRefundBatch(f, key, operatorId)
	if err != nil {
		return nil, err
	}
	if batch.Status == RefundBatchPending || batch.Status == RefundBatchRunning {
		return RunRefundBatch(batch.Id)
	}
	return batch, nil
}

func refundFilterFromBatch(batch *RefundBatch) (RefundFilter, error) {
	f := RefundFilter{StartTime: batch.StartTime, EndTime: batch.EndTime, Ratio: batch.Ratio, Reason: batch.Reason}
	if err := common.UnmarshalJsonStr(batch.ChannelIds, &f.ChannelIds); err != nil {
		return f, fmt.Errorf("invalid refund batch channels: %w", err)
	}
	if strings.TrimSpace(batch.ModelNames) != "" && strings.TrimSpace(batch.ModelNames) != "null" {
		if err := common.UnmarshalJsonStr(batch.ModelNames, &f.ModelNames); err != nil {
			return f, fmt.Errorf("invalid refund batch models: %w", err)
		}
	}
	return f, validateRefundFilter(f)
}

func normalizeLegacyRefundBatch(batch *RefundBatch) error {
	if batch.ScanCompleted || batch.ScanCursor != 0 || batch.TotalItems == 0 {
		return nil
	}
	updates := map[string]interface{}{
		"scan_completed":     true,
		"scan_skipped_items": batch.SkippedItems,
		"updated_at":         common.GetTimestamp(),
	}
	if err := DB.Model(&RefundBatch{}).Where("id = ? AND scan_cursor = ? AND scan_completed = ?", batch.Id, 0, false).Updates(updates).Error; err != nil {
		return err
	}
	batch.ScanCompleted = true
	batch.ScanSkippedItems = batch.SkippedItems
	return nil
}

func scanRefundBatchPage(batch *RefundBatch) error {
	if batch.ScanCompleted {
		return nil
	}
	f, err := refundFilterFromBatch(batch)
	if err != nil {
		return err
	}
	rawLogs, err := queryRefundLogsPage(f, batch.ScanCursor, RefundScanPageSize)
	if err != nil {
		return err
	}
	done := len(rawLogs) < RefundScanPageSize
	nextCursor := batch.ScanCursor
	if len(rawLogs) > 0 {
		nextCursor = rawLogs[len(rawLogs)-1].Id
	}
	logs, skipped := filterEligibleRefundLogs(rawLogs)
	amounts, err := calcRefundAmountForLogs(logs, f.Ratio)
	if err != nil {
		return err
	}
	now := common.GetTimestamp()
	items := make([]RefundItem, 0, len(logs))
	for _, log := range logs {
		amount := amounts[log.Id]
		if amount <= 0 {
			skipped++
			continue
		}
		items = append(items, RefundItem{
			BatchId: batch.Id, SourceLogId: log.Id, UserId: log.UserId,
			ChannelId: log.ChannelId, ModelName: log.ModelName, TokenId: log.TokenId,
			SourceQuota: log.Quota, RefundQuota: amount, Status: RefundItemPending,
			CreatedAt: now, UpdatedAt: now,
		})
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var current RefundBatch
		if err := lockForUpdate(tx).First(&current, batch.Id).Error; err != nil {
			return err
		}
		if current.ScanCompleted || current.ScanCursor != batch.ScanCursor {
			return nil
		}
		inserted := 0
		var sourceQuota int64
		for i := range items {
			result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&items[i])
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 1 {
				inserted++
				sourceQuota += int64(items[i].SourceQuota)
			}
		}
		updates := map[string]interface{}{
			"scan_cursor":        nextCursor,
			"scan_completed":     done,
			"scan_skipped_items": gorm.Expr("scan_skipped_items + ?", skipped),
			"skipped_items":      gorm.Expr("skipped_items + ?", skipped),
			"total_items":        gorm.Expr("total_items + ?", inserted),
			"total_quota":        gorm.Expr("total_quota + ?", sourceQuota),
			"error":              "",
			"updated_at":         now,
		}
		return tx.Model(&RefundBatch{}).Where("id = ?", batch.Id).Updates(updates).Error
	})
}

func processRefundItemsPage(batch *RefundBatch, ignoreRetrySchedule bool) (int, error) {
	now := common.GetTimestamp()
	query := DB.Where("batch_id = ? AND status = ? AND attempt_count < ?", batch.Id, RefundItemPending, RefundItemMaxAttempts)
	if !ignoreRetrySchedule {
		query = query.Where("next_attempt_at = 0 OR next_attempt_at <= ?", now)
	}
	var items []RefundItem
	if err := query.Order("id asc").Limit(RefundItemPageSize).Find(&items).Error; err != nil {
		return 0, err
	}
	for i := range items {
		executeRefundItem(&items[i])
	}
	return len(items), nil
}

func finalizeRefundBatchIfDone(id int) (bool, error) {
	var batch RefundBatch
	if err := DB.First(&batch, id).Error; err != nil {
		return false, err
	}
	if !batch.ScanCompleted {
		return false, nil
	}
	var pending int64
	if err := DB.Model(&RefundItem{}).Where("batch_id = ? AND status = ?", id, RefundItemPending).Count(&pending).Error; err != nil {
		return false, err
	}
	if pending > 0 {
		return false, nil
	}
	var counts []struct {
		Status string
		Count  int
	}
	if err := DB.Model(&RefundItem{}).Select("status, count(*) as count").Where("batch_id = ?", id).Group("status").Scan(&counts).Error; err != nil {
		return false, err
	}
	updates := map[string]interface{}{
		"status": RefundBatchCompleted, "success_items": 0, "failed_items": 0,
		"skipped_items": batch.ScanSkippedItems, "refunded_quota": 0,
		"error": "", "updated_at": common.GetTimestamp(),
	}
	failed := 0
	for _, count := range counts {
		switch count.Status {
		case RefundItemSuccess:
			updates["success_items"] = count.Count
		case RefundItemSkipped:
			updates["skipped_items"] = batch.ScanSkippedItems + count.Count
		case RefundItemFailed:
			failed = count.Count
			updates["failed_items"] = count.Count
		}
	}
	if failed > 0 {
		updates["status"] = RefundBatchFailed
		updates["error"] = fmt.Sprintf("%d refund item(s) failed after retries", failed)
	}
	var refunded int64
	if err := DB.Model(&RefundItem{}).Where("batch_id = ? AND status = ?", id, RefundItemSuccess).Select("COALESCE(SUM(refund_quota),0)").Scan(&refunded).Error; err != nil {
		return false, err
	}
	updates["refunded_quota"] = refunded
	if err := DB.Model(&RefundBatch{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return false, err
	}
	_, err := getRefundBatchSummary(id)
	return true, err
}

func processRefundBatchChunk(id int, ignoreRetrySchedule bool) (bool, error) {
	var batch RefundBatch
	if err := DB.First(&batch, id).Error; err != nil {
		return false, err
	}
	if batch.Status == RefundBatchCompleted || batch.Status == RefundBatchFailed {
		return true, nil
	}
	if err := normalizeLegacyRefundBatch(&batch); err != nil {
		return false, err
	}
	if !batch.ScanCompleted {
		if err := scanRefundBatchPage(&batch); err != nil {
			return false, err
		}
	}
	if _, err := processRefundItemsPage(&batch, ignoreRetrySchedule); err != nil {
		return false, err
	}
	return finalizeRefundBatchIfDone(id)
}

// ProcessNextRefundBatchChunk claims one durable batch lease and advances it by
// one scan/item page. Expired leases make interrupted work resumable by another
// process without re-crediting completed refund items.
func ProcessNextRefundBatchChunk(workerId string, leaseSeconds int64) (bool, error) {
	if strings.TrimSpace(workerId) == "" {
		return false, errors.New("refund worker id is required")
	}
	if leaseSeconds < 5 {
		leaseSeconds = 30
	}
	now := common.GetTimestamp()
	var candidate RefundBatch
	err := DB.Where("status IN ? AND (lease_expires_at = 0 OR lease_expires_at <= ? OR lease_owner = ?)", []string{RefundBatchPending, RefundBatchRunning}, now, workerId).Order("id asc").First(&candidate).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	claim := DB.Model(&RefundBatch{}).
		Where("id = ? AND status IN ? AND (lease_expires_at = 0 OR lease_expires_at <= ? OR lease_owner = ?)", candidate.Id, []string{RefundBatchPending, RefundBatchRunning}, now, workerId).
		Updates(map[string]interface{}{"status": RefundBatchRunning, "lease_owner": workerId, "lease_expires_at": now + leaseSeconds, "updated_at": now})
	if claim.Error != nil {
		return false, claim.Error
	}
	if claim.RowsAffected != 1 {
		return true, nil
	}
	_, processErr := processRefundBatchChunk(candidate.Id, false)
	release := map[string]interface{}{"lease_owner": "", "lease_expires_at": 0, "updated_at": common.GetTimestamp()}
	if processErr != nil {
		release["error"] = processErr.Error()
	}
	if err := DB.Model(&RefundBatch{}).Where("id = ? AND lease_owner = ?", candidate.Id, workerId).Updates(release).Error; err != nil && processErr == nil {
		processErr = err
	}
	return true, processErr
}

func RunRefundBatch(id int) (*RefundBatch, error) {
	if err := DB.Model(&RefundBatch{}).Where("id = ? AND status = ?", id, RefundBatchPending).Updates(map[string]interface{}{"status": RefundBatchRunning, "updated_at": common.GetTimestamp()}).Error; err != nil {
		return nil, err
	}
	for {
		done, err := processRefundBatchChunk(id, true)
		if err != nil {
			return nil, err
		}
		if done {
			return getRefundBatchSummary(id)
		}
	}
}

func executeRefundItem(item *RefundItem) {
	message := ""
	status := RefundItemSuccess
	attemptCount := item.AttemptCount + 1
	alreadyFinished := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		var locked RefundItem
		if err := lockForUpdate(tx).First(&locked, item.Id).Error; err != nil {
			return err
		}
		if locked.Status == RefundItemSuccess || locked.Status == RefundItemSkipped {
			alreadyFinished = true
			status = locked.Status
			return nil
		}
		attemptCount = locked.AttemptCount + 1
		// Serialize refunds for a user before checking the cumulative source cap.
		var user User
		if err := lockForUpdate(tx).Select("id").First(&user, locked.UserId).Error; err != nil {
			return err
		}
		var already int64
		if err := tx.Model(&RefundItem{}).Where("source_log_id = ? AND status = ?", locked.SourceLogId, RefundItemSuccess).Select("COALESCE(SUM(refund_quota),0)").Scan(&already).Error; err != nil {
			return err
		}
		remaining := int64(locked.SourceQuota) - already
		amount := int64(locked.RefundQuota)
		if amount > remaining {
			amount = remaining
		}
		if amount <= 0 {
			status = RefundItemSkipped
			message = "source consumption already fully refunded"
			return tx.Model(&locked).Updates(map[string]interface{}{"status": status, "refund_quota": 0, "message": message, "attempt_count": attemptCount, "next_attempt_at": 0, "updated_at": common.GetTimestamp()}).Error
		}
		userUpdate := tx.Model(&User{}).Where("id = ?", locked.UserId).UpdateColumn("quota", gorm.Expr("quota + ?", amount))
		if userUpdate.Error != nil {
			return userUpdate.Error
		}
		if userUpdate.RowsAffected != 1 {
			return errors.New("refund user no longer exists")
		}
		if locked.TokenId > 0 {
			var token Token
			if err := lockForUpdate(tx).Select("id", "unlimited_quota").Where("id = ? AND user_id = ?", locked.TokenId, locked.UserId).First(&token).Error; err != nil {
				return fmt.Errorf("refund source token unavailable: %w", err)
			}
			if !token.UnlimitedQuota {
				tokenUpdate := tx.Model(&Token{}).Where("id = ? AND user_id = ? AND unlimited_quota = ?", locked.TokenId, locked.UserId, false).Updates(map[string]interface{}{
					"remain_quota": gorm.Expr("remain_quota + ?", amount),
					"used_quota": gorm.Expr(
						"CASE WHEN used_quota >= ? THEN used_quota - ? ELSE ? END",
						amount, amount, 0,
					),
				})
				if tokenUpdate.Error != nil {
					return tokenUpdate.Error
				}
				if tokenUpdate.RowsAffected != 1 {
					return errors.New("finite refund token was not adjusted")
				}
			}
		}
		locked.RefundQuota = int(amount)
		if err := tx.Model(&locked).Updates(map[string]interface{}{"status": RefundItemSuccess, "refund_quota": amount, "message": "", "attempt_count": attemptCount, "next_attempt_at": 0, "updated_at": common.GetTimestamp()}).Error; err != nil {
			return err
		}
		item.RefundQuota = int(amount)
		return nil
	})
	if err != nil {
		status = RefundItemPending
		nextAttemptAt := common.GetTimestamp() + int64(attemptCount*2)
		if attemptCount >= RefundItemMaxAttempts {
			status = RefundItemFailed
			nextAttemptAt = 0
		}
		message = err.Error()
		DB.Model(item).Updates(map[string]interface{}{"status": status, "message": message, "attempt_count": attemptCount, "next_attempt_at": nextAttemptAt, "updated_at": common.GetTimestamp()})
		return
	}
	if status == RefundItemSuccess && !alreadyFinished {
		_ = InvalidateUserCache(item.UserId)
		_ = InvalidateUserTokensCache(item.UserId)
	}
}

func recordRefundLog(batch *RefundBatch, item *RefundItem) error {
	requestId := fmt.Sprintf("refund-item-%d", item.Id)
	var count int64
	if err := LOG_DB.Model(&Log{}).Where("request_id = ? AND type = ?", requestId, LogTypeRefund).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	username, _ := GetUsernameById(item.UserId, false)
	log := Log{UserId: item.UserId, Username: username, CreatedAt: common.GetTimestamp(), Type: LogTypeRefund, Content: fmt.Sprintf("Quick refund: %s", batch.Reason), ModelName: item.ModelName, Quota: item.RefundQuota, ChannelId: item.ChannelId, TokenId: item.TokenId, RequestId: requestId, Other: common.MapToJsonStr(map[string]interface{}{"batch_id": batch.Id, "source_log_id": item.SourceLogId, "source": "wallet", "ratio": batch.Ratio, "reason": batch.Reason, "admin_info": map[string]interface{}{"operator_id": batch.OperatorId}})}
	return LOG_DB.Create(&log).Error
}

func getRefundBatchSummary(id int) (*RefundBatch, error) {
	var batch RefundBatch
	if err := DB.First(&batch, id).Error; err != nil {
		return nil, err
	}
	return &batch, nil
}

// ProcessNextRefundLogDelivery durably delivers one successful refund into
// LOG_DB. A crashed delivery is reclaimed after its lease expires; the stable
// request_id check prevents a second visible log if the crash happened after
// LOG_DB committed but before LogRecorded was persisted in the main database.
func ProcessNextRefundLogDelivery(workerId string, leaseSeconds int64) (bool, error) {
	if strings.TrimSpace(workerId) == "" {
		return false, errors.New("refund log worker id is required")
	}
	if leaseSeconds < 5 {
		leaseSeconds = 30
	}
	now := common.GetTimestamp()
	var candidate RefundItem
	err := DB.Where("status = ? AND log_recorded = ? AND (log_next_attempt_at = 0 OR log_next_attempt_at <= ?) AND (log_lease_expires_at = 0 OR log_lease_expires_at <= ? OR log_lease_owner = ?)", RefundItemSuccess, false, now, now, workerId).Order("id asc").First(&candidate).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	claim := DB.Model(&RefundItem{}).
		Where("id = ? AND status = ? AND log_recorded = ? AND (log_next_attempt_at = 0 OR log_next_attempt_at <= ?) AND (log_lease_expires_at = 0 OR log_lease_expires_at <= ? OR log_lease_owner = ?)", candidate.Id, RefundItemSuccess, false, now, now, workerId).
		Updates(map[string]interface{}{"log_lease_owner": workerId, "log_lease_expires_at": now + leaseSeconds})
	if claim.Error != nil {
		return false, claim.Error
	}
	if claim.RowsAffected != 1 {
		return true, nil
	}
	var batch RefundBatch
	deliveryErr := DB.First(&batch, candidate.BatchId).Error
	if deliveryErr == nil {
		deliveryErr = recordRefundLog(&batch, &candidate)
	}
	updates := map[string]interface{}{
		"log_lease_owner": "", "log_lease_expires_at": 0,
		"log_attempt_count": gorm.Expr("log_attempt_count + ?", 1),
	}
	if deliveryErr == nil {
		updates["log_recorded"] = true
		updates["log_error"] = ""
		updates["log_next_attempt_at"] = 0
	} else {
		updates["log_error"] = deliveryErr.Error()
		delay := int64((candidate.LogAttemptCount + 1) * 10)
		if delay > 300 {
			delay = 300
		}
		updates["log_next_attempt_at"] = now + delay
	}
	releaseErr := DB.Model(&RefundItem{}).Where("id = ? AND log_lease_owner = ?", candidate.Id, workerId).Updates(updates).Error
	if deliveryErr != nil {
		return true, deliveryErr
	}
	return true, releaseErr
}

func ListRefundBatches(offset, limit int) ([]RefundBatch, int64, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	var total int64
	var rows []RefundBatch
	err := DB.Model(&RefundBatch{}).Count(&total).Error
	if err == nil {
		err = DB.Order("id desc").Offset(offset).Limit(limit).Find(&rows).Error
	}
	return rows, total, err
}
func GetRefundBatchPage(id, page, pageSize int) (*RefundBatch, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	var b RefundBatch
	if err := DB.First(&b, id).Error; err != nil {
		return &b, err
	}
	if err := DB.Model(&RefundItem{}).Where("batch_id = ?", id).Count(&b.ItemTotal).Error; err != nil {
		return &b, err
	}
	if err := DB.Where("batch_id = ?", id).Order("id asc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&b.Items).Error; err != nil {
		return &b, err
	}
	b.ItemPage = page
	b.ItemPageSize = pageSize
	return &b, nil
}

func GetRefundBatch(id int) (*RefundBatch, error) {
	return GetRefundBatchPage(id, 1, 20)
}
