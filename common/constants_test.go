package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSupportRoleHasOnlyExpectedOperationalPermissions(t *testing.T) {
	for _, permission := range []string{
		PermissionRedemptionRead,
		PermissionRedemptionDisable,
		PermissionRedemptionDelete,
		PermissionUserQuotaDecrease,
		PermissionAgentDownlineTransfer,
		PermissionAgentBalanceAdjust,
	} {
		assert.True(t, RoleHasPermission(RoleSupportUser, permission), permission)
	}
	for _, permission := range []string{
		PermissionAgentDownlineAssign,
		PermissionRedemptionManage,
	} {
		assert.False(t, RoleHasPermission(RoleSupportUser, permission), permission)
	}
	assert.False(t, RoleHasPermission(RoleSupportUser, "admin.full_access"))
}
