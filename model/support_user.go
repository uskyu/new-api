package model

import (
	"errors"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

type SupportManagedUser struct {
	Id                 int    `json:"id"`
	Username           string `json:"username"`
	DisplayName        string `json:"display_name"`
	Quota              int    `json:"quota"`
	UsedQuota          int    `json:"used_quota"`
	InviterId          int    `json:"inviter_id"`
	InviterUsername    string `json:"inviter_username"`
	InviterDisplayName string `json:"inviter_display_name"`
}

func SearchSupportManagedUsers(keyword string, pageInfo *common.PageInfo) ([]*SupportManagedUser, int64, error) {
	users := make([]*SupportManagedUser, 0)
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return users, 0, nil
	}

	pattern := strings.ReplaceAll(keyword, "!", "!!")
	pattern = strings.ReplaceAll(pattern, "%", "!%")
	pattern = strings.ReplaceAll(pattern, "_", "!_")
	pattern = "%" + pattern + "%"

	query := DB.Model(&User{}).
		Joins("LEFT JOIN users AS inviter ON inviter.id = users.inviter_id").
		Where("users.role = ?", common.RoleCommonUser)
	likeCondition := "users.username LIKE ? ESCAPE '!' OR users.email LIKE ? ESCAPE '!' OR users.display_name LIKE ? ESCAPE '!'"
	if id, err := strconv.Atoi(keyword); err == nil {
		query = query.Where("(users.id = ? OR "+likeCondition+")", id, pattern, pattern, pattern)
	} else {
		query = query.Where("("+likeCondition+")", pattern, pattern, pattern)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.
		Select("users.id, users.username, users.display_name, users.quota, users.used_quota, users.inviter_id, COALESCE(inviter.username, '') AS inviter_username, COALESCE(inviter.display_name, '') AS inviter_display_name").
		Order("users.id DESC").
		Limit(pageInfo.GetPageSize()).
		Offset(pageInfo.GetStartIdx()).
		Scan(&users).Error; err != nil {
		return nil, 0, err
	}
	return users, total, nil
}

func GetSupportManagedUserById(id int) (*SupportManagedUser, error) {
	if id <= 0 {
		return nil, errors.New("invalid user id")
	}
	user := &SupportManagedUser{}
	err := DB.Model(&User{}).
		Joins("LEFT JOIN users AS inviter ON inviter.id = users.inviter_id").
		Select("users.id, users.username, users.display_name, users.quota, users.used_quota, users.inviter_id, COALESCE(inviter.username, '') AS inviter_username, COALESCE(inviter.display_name, '') AS inviter_display_name").
		Where("users.id = ? AND users.role = ?", id, common.RoleCommonUser).
		First(user).Error
	return user, err
}
