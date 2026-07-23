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

func TestSupportUserSearchMarksEnabledAgents(t *testing.T) {
	setupAgentBackendTest(t)
	agent := createAgentBackendTestUser(t, "support-search-agent")
	require.NoError(t, DB.Create(&AgentProfile{UserId: agent.Id, Status: AgentStatusEnabled}).Error)

	users, total, err := SearchSupportManagedUsers(agent.Username, &common.PageInfo{Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Len(t, users, 1)
	assert.Equal(t, int64(1), total)
	assert.True(t, users[0].IsAgent)
}

func TestAgentAssignedUsersLoadsWithoutKeywordAndFiltersByAgent(t *testing.T) {
	setupAgentBackendTest(t)
	firstAgent := createAgentBackendTestUser(t, "assignment-first-agent")
	secondAgent := createAgentBackendTestUser(t, "assignment-second-agent")
	firstUser := createAgentBackendTestUser(t, "assignment-first-user")
	secondUser := createAgentBackendTestUser(t, "assignment-second-user")
	unassigned := createAgentBackendTestUser(t, "assignment-unassigned")
	require.NoError(t, DB.Model(firstUser).Update("inviter_id", firstAgent.Id).Error)
	require.NoError(t, DB.Model(secondUser).Update("inviter_id", secondAgent.Id).Error)

	pageInfo := &common.PageInfo{Page: 1, PageSize: 10}
	users, total, err := GetAgentAssignedUsers("", 0, pageInfo)
	require.NoError(t, err)
	require.Len(t, users, 2)
	assert.Equal(t, int64(2), total)
	for _, user := range users {
		assert.NotEqual(t, unassigned.Id, user.Id)
	}

	users, total, err = GetAgentAssignedUsers("", firstAgent.Id, pageInfo)
	require.NoError(t, err)
	require.Len(t, users, 1)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, firstUser.Id, users[0].Id)
	assert.Equal(t, firstAgent.Username, users[0].InviterUsername)

	users, total, err = GetAgentAssignedUsers(secondAgent.Username, 0, pageInfo)
	require.NoError(t, err)
	require.Len(t, users, 1)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, secondUser.Id, users[0].Id)
}

func TestSupportQuotaDecreaseIsAtomicAndCannotOverdraw(t *testing.T) {
	setupAgentBackendTest(t)
	support := createAgentBackendTestUser(t, "quota-support")
	require.NoError(t, DB.Model(support).Update("role", common.RoleSupportUser).Error)
	target := createAgentBackendTestUser(t, "quota-target")
	require.NoError(t, DB.Model(target).Update("quota", 1000).Error)

	result, err := DecreaseUserQuotaBySupport(support.Id, common.RoleSupportUser, target.Id, 300, "service correction")
	require.NoError(t, err)
	assert.Equal(t, -300, result.QuotaDelta)
	assert.Equal(t, 1000, result.QuotaBefore)
	assert.Equal(t, 700, result.QuotaAfter)

	_, err = DecreaseUserQuotaBySupport(support.Id, common.RoleSupportUser, target.Id, 701, "invalid overdraw")
	require.Error(t, err)
	var updated User
	require.NoError(t, DB.First(&updated, target.Id).Error)
	assert.Equal(t, 700, updated.Quota)
}
