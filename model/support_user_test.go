package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestSupportUserSearchRequiresKeywordAndReturnsInviter(t *testing.T) {
	truncateTables(t)

	inviter := &User{Id: 101, Username: "support-agent", Password: "password123", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default", AffCode: "support-agent-aff"}
	target := &User{Id: 102, Username: "target-user", DisplayName: "Target User", Password: "password123", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default", AffCode: "target-user-aff", InviterId: inviter.Id}
	admin := &User{Id: 103, Username: "hidden-admin", Password: "password123", Role: common.RoleAdminUser, Status: common.UserStatusEnabled, Group: "default", AffCode: "hidden-admin-aff"}
	require.NoError(t, DB.Create(inviter).Error)
	require.NoError(t, DB.Create(target).Error)
	require.NoError(t, DB.Create(admin).Error)

	pageInfo := &common.PageInfo{Page: 1, PageSize: 10}
	empty, total, err := SearchSupportManagedUsers("", pageInfo)
	require.NoError(t, err)
	assert.Empty(t, empty)
	assert.Zero(t, total)

	users, total, err := SearchSupportManagedUsers("target", pageInfo)
	require.NoError(t, err)
	require.Len(t, users, 1)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, target.Id, users[0].Id)
	assert.Equal(t, inviter.Id, users[0].InviterId)
	assert.Equal(t, inviter.Username, users[0].InviterUsername)

	wildcard, total, err := SearchSupportManagedUsers("%", pageInfo)
	require.NoError(t, err)
	assert.Empty(t, wildcard)
	assert.Zero(t, total)

	_, err = GetSupportManagedUserById(admin.Id)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}
