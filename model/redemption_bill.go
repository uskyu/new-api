package model

import (
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm/clause"
)

type RedemptionBill struct {
	Id           int    `json:"id"`
	UserId       int    `json:"user_id"`
	Name         string `json:"name"`
	Code         string `json:"code"`
	Quota        int    `json:"quota"`
	RedeemedTime int64  `json:"redeemed_time"`
}

func maskRedemptionBillCode(code string) string {
	if len(code) <= 4 {
		return "****"
	}
	return "****" + code[len(code)-4:]
}

func redemptionNamePrefix(keyword string) string {
	keyword = strings.ReplaceAll(keyword, "!", "!!")
	keyword = strings.ReplaceAll(keyword, "%", "!%")
	keyword = strings.ReplaceAll(keyword, "_", "!_")
	return keyword + "%"
}

func queryRedemptionBills(userId int, cutoff int64, keyword string, pageInfo *common.PageInfo) (bills []*RedemptionBill, total int64, err error) {
	tx := DB.Begin()
	if tx.Error != nil {
		return nil, 0, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Cleanup soft-deletes used codes, but those rows remain valid billing history.
	query := tx.Unscoped().Model(&Redemption{}).
		Where("used_user_id > 0 AND redeemed_time > 0")
	if userId > 0 {
		query = query.Where("used_user_id = ?", userId)
	}
	if cutoff > 0 {
		query = query.Where("redeemed_time >= ?", cutoff)
	}
	keyword = strings.TrimSpace(keyword)
	if keyword != "" {
		namePattern := redemptionNamePrefix(keyword)
		keywordQuery := tx.Where("name LIKE ? ESCAPE '!'", namePattern).
			Or(clause.Eq{Column: clause.Column{Name: "key"}, Value: keyword})
		if id, parseErr := strconv.Atoi(keyword); parseErr == nil {
			keywordQuery = keywordQuery.Or("id = ?", id)
		}
		query = query.Where(keywordQuery)
	}

	if err = query.Count(&total).Error; err != nil {
		tx.Rollback()
		return nil, 0, err
	}
	var rows []*Redemption
	if err = query.
		Order("redeemed_time desc, id desc").
		Limit(pageInfo.GetPageSize()).
		Offset(pageInfo.GetStartIdx()).
		Find(&rows).Error; err != nil {
		tx.Rollback()
		return nil, 0, err
	}
	if err = tx.Commit().Error; err != nil {
		return nil, 0, err
	}

	bills = make([]*RedemptionBill, 0, len(rows))
	for _, row := range rows {
		bills = append(bills, &RedemptionBill{
			Id:           row.Id,
			UserId:       row.UsedUserId,
			Name:         row.Name,
			Code:         maskRedemptionBillCode(row.Key),
			Quota:        row.Quota,
			RedeemedTime: row.RedeemedTime,
		})
	}
	return bills, total, nil
}

func GetRecentUserRedemptionBills(userId int, keyword string, pageInfo *common.PageInfo) ([]*RedemptionBill, int64, error) {
	return queryRedemptionBills(userId, topUpQueryCutoff(), keyword, pageInfo)
}

func GetAllRedemptionBills(keyword string, pageInfo *common.PageInfo) ([]*RedemptionBill, int64, error) {
	return queryRedemptionBills(0, 0, keyword, pageInfo)
}

func GetAllRedemptionBillsByUser(userId int, keyword string, pageInfo *common.PageInfo) ([]*RedemptionBill, int64, error) {
	return queryRedemptionBills(userId, 0, keyword, pageInfo)
}
