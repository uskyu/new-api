package model

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	RefundBatchPending   = "pending"
	RefundBatchRunning   = "running"
	RefundBatchCompleted = "completed"
	RefundBatchFailed    = "failed"
	RefundItemPending    = "pending"
	RefundItemSuccess    = "success"
	RefundItemSkipped    = "skipped"
	RefundItemFailed     = "failed"
	RefundScanLimit      = 10000
)

type RefundBatch struct {
	Id             int          `json:"id"`
	IdempotencyKey string       `json:"idempotency_key" gorm:"type:varchar(128);uniqueIndex;not null"`
	StartTime      int64        `json:"start_time" gorm:"bigint;index"`
	EndTime        int64        `json:"end_time" gorm:"bigint;index"`
	ChannelIds     string       `json:"channel_ids" gorm:"type:text"`
	ModelNames     string       `json:"model_names" gorm:"type:text"`
	Ratio          int          `json:"ratio"`
	Reason         string       `json:"reason" gorm:"type:varchar(500)"`
	OperatorId     int          `json:"operator_id"`
	Status         string       `json:"status" gorm:"type:varchar(32);index"`
	TotalItems     int          `json:"total_items"`
	SuccessItems   int          `json:"success_items"`
	SkippedItems   int          `json:"skipped_items"`
	FailedItems    int          `json:"failed_items"`
	TotalQuota     int64        `json:"total_quota" gorm:"bigint"`
	RefundedQuota  int64        `json:"refunded_quota" gorm:"bigint"`
	Error          string       `json:"error" gorm:"type:text"`
	CreatedAt      int64        `json:"created_at" gorm:"bigint;index"`
	UpdatedAt      int64        `json:"updated_at" gorm:"bigint"`
	Items          []RefundItem `json:"items,omitempty" gorm:"foreignKey:BatchId"`
}

type RefundItem struct {
	Id          int    `json:"id"`
	BatchId     int    `json:"batch_id" gorm:"uniqueIndex:idx_refund_batch_source,priority:1;index"`
	SourceLogId int    `json:"source_log_id" gorm:"uniqueIndex:idx_refund_batch_source,priority:2;index"`
	UserId      int    `json:"user_id" gorm:"index"`
	ChannelId   int    `json:"channel_id"`
	ModelName   string `json:"model_name" gorm:"type:varchar(255)"`
	TokenId     int    `json:"token_id"`
	SourceQuota int    `json:"source_quota"`
	RefundQuota int    `json:"refund_quota"`
	Status      string `json:"status" gorm:"type:varchar(32);index"`
	Message     string `json:"message" gorm:"type:varchar(500)"`
	CreatedAt   int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt   int64  `json:"updated_at" gorm:"bigint"`
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
	if len(f.ChannelIds) == 0 {
		return errors.New("channel_ids is required")
	}
	if f.Ratio < 1 || f.Ratio > 100 {
		return errors.New("ratio must be between 1 and 100")
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

func queryRefundLogs(f RefundFilter) ([]*Log, int, error) {
	if err := validateRefundFilter(f); err != nil {
		return nil, 0, err
	}
	tx := LOG_DB.Where("type = ? AND created_at >= ? AND created_at <= ? AND channel_id IN ? AND quota > 0", LogTypeConsume, f.StartTime, f.EndTime, f.ChannelIds)
	if len(f.ModelNames) > 0 {
		tx = tx.Where("model_name IN ?", f.ModelNames)
	}
	var logs []*Log
	if err := tx.Order("id asc").Limit(RefundScanLimit + 1).Find(&logs).Error; err != nil {
		return nil, 0, err
	}
	if len(logs) > RefundScanLimit {
		return nil, 0, fmt.Errorf("too many matching logs (limit %d), narrow the filters", RefundScanLimit)
	}
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
	return eligible, skipped, nil
}

func PreviewRefund(f RefundFilter) (*RefundPreview, error) {
	logs, skipped, err := queryRefundLogs(f)
	if err != nil {
		return nil, err
	}
	p := &RefundPreview{Skipped: skipped, ByChannel: map[int]RefundBreakdown{}, ByModel: map[string]RefundBreakdown{}}
	for _, l := range logs {
		r := int(int64(l.Quota) * int64(f.Ratio) / 100)
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
	return p, nil
}

func GetRefundOptions(start, end int64) (*RefundOptions, error) {
	if start <= 0 || end <= 0 || start > end {
		return nil, errors.New("invalid refund time range")
	}
	var logs []*Log
	if err := LOG_DB.Where("type = ? AND created_at >= ? AND created_at <= ? AND quota > 0", LogTypeConsume, start, end).Order("id asc").Limit(RefundScanLimit + 1).Find(&logs).Error; err != nil {
		return nil, err
	}
	if len(logs) > RefundScanLimit {
		return nil, fmt.Errorf("too many logs (limit %d), narrow the time range", RefundScanLimit)
	}
	channelSet := map[int]bool{}
	modelSet := map[string]bool{}
	for _, l := range logs {
		if refundSourceEligible(l) {
			channelSet[l.ChannelId] = true
			if l.ModelName != "" {
				modelSet[l.ModelName] = true
			}
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

func CreateAndRunRefund(f RefundFilter, key string, operatorId int) (*RefundBatch, error) {
	if err := validateRefundFilter(f); err != nil {
		return nil, err
	}
	key = strings.TrimSpace(key)
	if key == "" || len(key) > 128 {
		return nil, errors.New("valid idempotency_key is required")
	}
	var existing RefundBatch
	if err := DB.Where("idempotency_key = ?", key).First(&existing).Error; err == nil {
		return getRefundBatchAndRepairLogs(existing.Id)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	logs, skipped, err := queryRefundLogs(f)
	if err != nil {
		return nil, err
	}
	channels, _ := common.Marshal(f.ChannelIds)
	models, _ := common.Marshal(f.ModelNames)
	now := common.GetTimestamp()
	batch := RefundBatch{IdempotencyKey: key, StartTime: f.StartTime, EndTime: f.EndTime, ChannelIds: string(channels), ModelNames: string(models), Ratio: f.Ratio, Reason: strings.TrimSpace(f.Reason), OperatorId: operatorId, Status: RefundBatchPending, SkippedItems: skipped, CreatedAt: now, UpdatedAt: now}
	err = DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&batch).Error; err != nil {
			return err
		}
		items := make([]RefundItem, 0, len(logs))
		for _, l := range logs {
			q := int(int64(l.Quota) * int64(f.Ratio) / 100)
			if q <= 0 {
				batch.SkippedItems++
				continue
			}
			items = append(items, RefundItem{BatchId: batch.Id, SourceLogId: l.Id, UserId: l.UserId, ChannelId: l.ChannelId, ModelName: l.ModelName, TokenId: l.TokenId, SourceQuota: l.Quota, RefundQuota: q, Status: RefundItemPending, CreatedAt: now, UpdatedAt: now})
			batch.TotalQuota += int64(l.Quota)
		}
		batch.TotalItems = len(items)
		if len(items) > 0 {
			if err := tx.Create(&items).Error; err != nil {
				return err
			}
		}
		return tx.Model(&batch).Updates(map[string]interface{}{"total_items": batch.TotalItems, "skipped_items": batch.SkippedItems, "total_quota": batch.TotalQuota}).Error
	})
	if err != nil {
		if e := DB.Where("idempotency_key = ?", key).First(&existing).Error; e == nil {
			return getRefundBatchAndRepairLogs(existing.Id)
		}
		return nil, err
	}
	return RunRefundBatch(batch.Id)
}

func RunRefundBatch(id int) (*RefundBatch, error) {
	var batch RefundBatch
	if err := DB.First(&batch, id).Error; err != nil {
		return nil, err
	}
	if batch.Status != RefundBatchPending {
		return getRefundBatchAndRepairLogs(id)
	}
	claim := DB.Model(&RefundBatch{}).Where("id = ? AND status = ?", id, RefundBatchPending).Updates(map[string]interface{}{"status": RefundBatchRunning, "updated_at": common.GetTimestamp()})
	if claim.Error != nil {
		return nil, claim.Error
	}
	if claim.RowsAffected != 1 {
		return getRefundBatchAndRepairLogs(id)
	}
	var items []RefundItem
	if err := DB.Where("batch_id = ? AND status = ?", id, RefundItemPending).Order("id asc").Find(&items).Error; err != nil {
		return nil, err
	}
	for i := range items {
		executeRefundItem(&batch, &items[i])
	}
	var counts []struct {
		Status string
		Count  int
	}
	DB.Model(&RefundItem{}).Select("status, count(*) as count").Where("batch_id = ?", id).Group("status").Scan(&counts)
	updates := map[string]interface{}{"status": RefundBatchCompleted, "updated_at": common.GetTimestamp()}
	failed := 0
	for _, c := range counts {
		switch c.Status {
		case RefundItemSuccess:
			updates["success_items"] = c.Count
		case RefundItemSkipped:
			updates["skipped_items"] = batch.SkippedItems + c.Count
		case RefundItemFailed:
			failed = c.Count
			updates["failed_items"] = c.Count
		}
	}
	if failed > 0 {
		updates["status"] = RefundBatchFailed
	}
	var refunded int64
	DB.Model(&RefundItem{}).Where("batch_id = ? AND status = ?", id, RefundItemSuccess).Select("COALESCE(SUM(refund_quota),0)").Scan(&refunded)
	updates["refunded_quota"] = refunded
	DB.Model(&RefundBatch{}).Where("id = ?", id).Updates(updates)
	DB.Preload("Items").First(&batch, id)
	return &batch, nil
}

func executeRefundItem(batch *RefundBatch, item *RefundItem) {
	message := ""
	status := RefundItemSuccess
	err := DB.Transaction(func(tx *gorm.DB) error {
		var locked RefundItem
		if err := lockForUpdate(tx).First(&locked, item.Id).Error; err != nil {
			return err
		}
		if locked.Status == RefundItemSuccess || locked.Status == RefundItemSkipped {
			return nil
		}
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
			return tx.Model(&locked).Updates(map[string]interface{}{"status": status, "refund_quota": 0, "message": message, "updated_at": common.GetTimestamp()}).Error
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
		if err := tx.Model(&locked).Updates(map[string]interface{}{"status": RefundItemSuccess, "refund_quota": amount, "message": "", "updated_at": common.GetTimestamp()}).Error; err != nil {
			return err
		}
		item.RefundQuota = int(amount)
		return nil
	})
	if err != nil {
		status = RefundItemFailed
		message = err.Error()
		DB.Model(item).Updates(map[string]interface{}{"status": status, "message": message, "updated_at": common.GetTimestamp()})
		return
	}
	if status == RefundItemSuccess {
		recordRefundLog(batch, item)
	}
}

func recordRefundLog(batch *RefundBatch, item *RefundItem) {
	requestId := fmt.Sprintf("refund-item-%d", item.Id)
	var count int64
	if LOG_DB.Model(&Log{}).Where("request_id = ? AND type = ?", requestId, LogTypeRefund).Count(&count).Error == nil && count > 0 {
		return
	}
	username, _ := GetUsernameById(item.UserId, false)
	log := Log{UserId: item.UserId, Username: username, CreatedAt: common.GetTimestamp(), Type: LogTypeRefund, Content: fmt.Sprintf("Quick refund: %s", batch.Reason), ModelName: item.ModelName, Quota: item.RefundQuota, ChannelId: item.ChannelId, TokenId: item.TokenId, RequestId: requestId, Other: common.MapToJsonStr(map[string]interface{}{"batch_id": batch.Id, "source_log_id": item.SourceLogId, "source": "wallet", "ratio": batch.Ratio, "reason": batch.Reason, "admin_info": map[string]interface{}{"operator_id": batch.OperatorId}})}
	if e := LOG_DB.Create(&log).Error; e != nil {
		common.SysLog("failed to record quick refund log: " + e.Error())
	}
}

// getRefundBatchAndRepairLogs never executes quota mutations. Repeated
// idempotency keys may safely repair a log write that failed after the wallet
// transaction committed, without crediting the user a second time.
func getRefundBatchAndRepairLogs(id int) (*RefundBatch, error) {
	var batch RefundBatch
	if err := DB.Preload("Items").First(&batch, id).Error; err != nil {
		return nil, err
	}
	for i := range batch.Items {
		if batch.Items[i].Status == RefundItemSuccess {
			recordRefundLog(&batch, &batch.Items[i])
		}
	}
	return &batch, nil
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
func GetRefundBatch(id int) (*RefundBatch, error) {
	var b RefundBatch
	err := DB.Preload("Items").First(&b, id).Error
	return &b, err
}
