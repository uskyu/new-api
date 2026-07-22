package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSupportRoleHasOnlyExpectedOperationalPermissions(t *testing.T) {
	for _, permission := range []string{
		PermissionUserQuotaDecrease,
		PermissionAgentDownlineTransfer,
	} {
		assert.True(t, RoleHasPermission(RoleSupportUser, permission), permission)
	}
	for _, permission := range []string{
		PermissionAgentDownlineAssign,
		PermissionAgentBalanceAdjust,
	} {
		assert.False(t, RoleHasPermission(RoleSupportUser, permission), permission)
	}
	assert.False(t, RoleHasPermission(RoleSupportUser, "admin.full_access"))
}
