package common

import "testing"

func TestSupportRolePermissions(t *testing.T) {
	if !IsValidateRole(RoleSupportUser) {
		t.Fatalf("support role should be valid")
	}
	if RoleSupportUser >= RoleAdminUser {
		t.Fatalf("support role must stay below admin role")
	}
	if !RoleHasPermission(RoleSupportUser, PermissionRedemptionRead) {
		t.Fatalf("support role should read redemption codes")
	}
	if !RoleHasPermission(RoleSupportUser, PermissionRedemptionDisable) {
		t.Fatalf("support role should disable redemption codes")
	}
	if !RoleHasPermission(RoleSupportUser, PermissionRedemptionDelete) {
		t.Fatalf("support role should delete redemption codes")
	}
	if RoleHasPermission(RoleSupportUser, PermissionRedemptionManage) {
		t.Fatalf("support role should not fully manage redemption codes")
	}
	if !RoleHasPermission(RoleSupportUser, PermissionUserQuotaDecrease) {
		t.Fatalf("support role should decrease ordinary user quota")
	}
	if !RoleHasPermission(RoleSupportUser, PermissionAgentDownlineTransfer) {
		t.Fatalf("support role should transfer agent downline users")
	}
	if RoleHasPermission(RoleSupportUser, "admin.full_access") {
		t.Fatalf("support role should not receive unknown permissions")
	}
}
