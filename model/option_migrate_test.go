package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestMigrateLegacyCheckinTiers(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&Option{}))
	t.Cleanup(func() { DB.Exec("DELETE FROM options") })
	// 测试环境未走 InitOptionMap,手动初始化内存缓存
	common.OptionMapRWMutex.Lock()
	common.OptionMap = make(map[string]string)
	common.OptionMapRWMutex.Unlock()

	// 1. 无旧键时：不产生新键，不报错
	migrateLegacyCheckinTiers()
	var count int64
	require.NoError(t, DB.Model(&Option{}).Where("key = ?", "checkin_setting.request_count_tiers").Count(&count).Error)
	require.Zero(t, count)

	// 2. 有旧键时：迁移到按次档位键并删除旧键
	legacy := `[{"threshold":20,"min_quota":5000,"max_quota":20000}]`
	require.NoError(t, UpdateOption("checkin_setting.bonus_tiers", legacy))
	require.NoError(t, DB.Model(&Option{}).Where("key = ?", "checkin_setting.request_count_tiers").Count(&count).Error)
	require.Zero(t, count)

	migrateLegacyCheckinTiers()

	var newVal string
	require.NoError(t, DB.Model(&Option{}).Where("key = ?", "checkin_setting.request_count_tiers").Pluck("value", &newVal).Error)
	require.Equal(t, legacy, newVal)
	require.NoError(t, DB.Model(&Option{}).Where("key = ?", "checkin_setting.bonus_tiers").Count(&count).Error)
	require.Zero(t, count)

	// 3. 幂等：新键已存在时旧键不再被迁移（保留原样）
	require.NoError(t, UpdateOption("checkin_setting.bonus_tiers", legacy))
	migrateLegacyCheckinTiers()
	require.NoError(t, DB.Model(&Option{}).Where("key = ?", "checkin_setting.bonus_tiers").Count(&count).Error)
	require.NotZero(t, count)
	require.NoError(t, DB.Model(&Option{}).Where("key = ?", "checkin_setting.request_count_tiers").Pluck("value", &newVal).Error)
	require.Equal(t, legacy, newVal)
}
