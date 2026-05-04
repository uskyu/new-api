package common

import "testing"

func TestSupportRolePermissions(t *testing.T) {
	if !IsValidateRole(RoleSupportUser) {
		t.Fatalf("support role should be valid")
	}
	if RoleSupportUser >= RoleAdminUser {
		t.Fatalf("support role must stay below admin role")
	}
	if !RoleHasPermission(RoleSupportUser, PermissionRedemptionManage) {
		t.Fatalf("support role should manage redemption codes")
	}
	if !RoleHasPermission(RoleSupportUser, PermissionUserQuotaDecrease) {
		t.Fatalf("support role should decrease ordinary user quota")
	}
	if RoleHasPermission(RoleSupportUser, "admin.full_access") {
		t.Fatalf("support role should not receive unknown permissions")
	}
}
