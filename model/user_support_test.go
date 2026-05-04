package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestDecreaseUserQuotaBySupport(t *testing.T) {
	setupAgentTestDB(t, TestDBDialectSQLite)

	support := createAgentTestUser(t, "support_quota", "SUPPORT1")
	require.NoError(t, DB.Model(support).Update("role", common.RoleSupportUser).Error)
	user := createAgentTestUser(t, "common_quota", "COMMON1")
	require.NoError(t, DB.Model(user).Update("quota", 1000).Error)

	result, err := DecreaseUserQuotaBySupport(support.Id, common.RoleSupportUser, user.Id, 300, "service adjustment")
	require.NoError(t, err)
	require.Equal(t, user.Id, result.UserId)
	require.Equal(t, 300, result.QuotaDelta)
	require.Equal(t, 1000, result.QuotaBefore)
	require.Equal(t, 700, result.QuotaAfter)

	var updated User
	require.NoError(t, DB.First(&updated, user.Id).Error)
	require.Equal(t, 700, updated.Quota)
}

func TestDecreaseUserQuotaBySupportRejectsNonCommonUser(t *testing.T) {
	setupAgentTestDB(t, TestDBDialectSQLite)

	support := createAgentTestUser(t, "support_reject", "SUPPORT2")
	require.NoError(t, DB.Model(support).Update("role", common.RoleSupportUser).Error)
	admin := createAgentTestUser(t, "admin_reject", "ADMIN1")
	require.NoError(t, DB.Model(admin).Updates(map[string]interface{}{
		"role":  common.RoleAdminUser,
		"quota": 1000,
	}).Error)

	_, err := DecreaseUserQuotaBySupport(support.Id, common.RoleSupportUser, admin.Id, 100, "bad target")
	require.Error(t, err)

	var updated User
	require.NoError(t, DB.First(&updated, admin.Id).Error)
	require.Equal(t, 1000, updated.Quota)
}

func TestDecreaseUserQuotaBySupportRejectsInsufficientQuota(t *testing.T) {
	setupAgentTestDB(t, TestDBDialectSQLite)

	support := createAgentTestUser(t, "support_insufficient", "SUPPORT3")
	require.NoError(t, DB.Model(support).Update("role", common.RoleSupportUser).Error)
	user := createAgentTestUser(t, "common_insufficient", "COMMON3")
	require.NoError(t, DB.Model(user).Update("quota", 50).Error)

	_, err := DecreaseUserQuotaBySupport(support.Id, common.RoleSupportUser, user.Id, 100, "too much")
	require.Error(t, err)

	var updated User
	require.NoError(t, DB.First(&updated, user.Id).Error)
	require.Equal(t, 50, updated.Quota)
}
