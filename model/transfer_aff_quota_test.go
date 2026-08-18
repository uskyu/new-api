package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTransferAffQuotaToQuota_ZeroAmount(t *testing.T) {
	user := &User{Username: "transfer-zero", Password: "test-password", AffCode: "tz00", AffQuota: 1000}
	require.NoError(t, DB.Create(user).Error)
	t.Cleanup(func() { DB.Unscoped().Delete(user) })

	err := user.TransferAffQuotaToQuota(0)
	require.Error(t, err)
	require.Contains(t, err.Error(), "正整数")

	require.NoError(t, DB.First(user, user.Id).Error)
	require.Equal(t, 1000, user.AffQuota)
	require.Equal(t, 0, user.Quota)
}

func TestTransferAffQuotaToQuota_NegativeAmount(t *testing.T) {
	user := &User{Username: "transfer-neg", Password: "test-password", AffCode: "tz01", AffQuota: 1000}
	require.NoError(t, DB.Create(user).Error)
	t.Cleanup(func() { DB.Unscoped().Delete(user) })

	err := user.TransferAffQuotaToQuota(-100)
	require.Error(t, err)
	require.Contains(t, err.Error(), "正整数")

	require.NoError(t, DB.First(user, user.Id).Error)
	require.Equal(t, 1000, user.AffQuota)
	require.Equal(t, 0, user.Quota)
}

func TestTransferAffQuotaToQuota_SmallAmount(t *testing.T) {
	user := &User{Username: "transfer-small", Password: "test-password", AffCode: "tz02", AffQuota: 1000}
	require.NoError(t, DB.Create(user).Error)
	t.Cleanup(func() { DB.Unscoped().Delete(user) })

	err := user.TransferAffQuotaToQuota(1)
	require.NoError(t, err)

	require.NoError(t, DB.First(user, user.Id).Error)
	require.Equal(t, 999, user.AffQuota)
	require.Equal(t, 1, user.Quota)
}

func TestTransferAffQuotaToQuota_FullAmount(t *testing.T) {
	user := &User{Username: "transfer-full", Password: "test-password", AffCode: "tz03", AffQuota: 500}
	require.NoError(t, DB.Create(user).Error)
	t.Cleanup(func() { DB.Unscoped().Delete(user) })

	err := user.TransferAffQuotaToQuota(500)
	require.NoError(t, err)

	require.NoError(t, DB.First(user, user.Id).Error)
	require.Equal(t, 0, user.AffQuota)
	require.Equal(t, 500, user.Quota)
}

func TestTransferAffQuotaToQuota_ExceedAmount(t *testing.T) {
	user := &User{Username: "transfer-exceed", Password: "test-password", AffCode: "tz04", AffQuota: 300}
	require.NoError(t, DB.Create(user).Error)
	t.Cleanup(func() { DB.Unscoped().Delete(user) })

	err := user.TransferAffQuotaToQuota(301)
	require.Error(t, err)
	require.Contains(t, err.Error(), "不足")

	require.NoError(t, DB.First(user, user.Id).Error)
	require.Equal(t, 300, user.AffQuota)
	require.Equal(t, 0, user.Quota)
}

func TestTransferAffQuotaToQuota_PartialAmount(t *testing.T) {
	user := &User{Username: "transfer-partial", Password: "test-password", AffCode: "tz05", AffQuota: 1000}
	require.NoError(t, DB.Create(user).Error)
	t.Cleanup(func() { DB.Unscoped().Delete(user) })

	err := user.TransferAffQuotaToQuota(150)
	require.NoError(t, err)

	require.NoError(t, DB.First(user, user.Id).Error)
	require.Equal(t, 850, user.AffQuota)
	require.Equal(t, 150, user.Quota)
}

func TestTransferAffQuotaToQuota_AccumulatesQuota(t *testing.T) {
	user := &User{Username: "transfer-accum", Password: "test-password", AffCode: "tz06", AffQuota: 1000, Quota: 200}
	require.NoError(t, DB.Create(user).Error)
	t.Cleanup(func() { DB.Unscoped().Delete(user) })

	err := user.TransferAffQuotaToQuota(300)
	require.NoError(t, err)

	require.NoError(t, DB.First(user, user.Id).Error)
	require.Equal(t, 700, user.AffQuota)
	require.Equal(t, 500, user.Quota)
}

func TestTransferAffQuotaToQuota_AffQuotaExactlyOne(t *testing.T) {
	user := &User{Username: "transfer-one", Password: "test-password", AffCode: "tz07", AffQuota: 1}
	require.NoError(t, DB.Create(user).Error)
	t.Cleanup(func() { DB.Unscoped().Delete(user) })

	err := user.TransferAffQuotaToQuota(1)
	require.NoError(t, err)

	require.NoError(t, DB.First(user, user.Id).Error)
	require.Equal(t, 0, user.AffQuota)
	require.Equal(t, 1, user.Quota)
}
