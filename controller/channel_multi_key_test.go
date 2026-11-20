package controller

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupMultiKeyChannelTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousUsingSQLite := common.UsingSQLite
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	common.UsingSQLite = true
	model.DB = db
	model.LOG_DB = db
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.UsingSQLite = previousUsingSQLite
	})
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}))
	return db
}

func TestMultiKeyEnableRestoresOnlyExhaustedChannels(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, cacheEnabled := range []bool{false, true} {
		for _, action := range []string{"enable_key", "enable_all_keys"} {
			for _, tc := range []struct {
				name           string
				initialStatus  int
				autoExhaust    bool
				manualOverride string
				wantStatus     int
			}{
				{name: "key exhaustion restores", initialStatus: common.ChannelStatusEnabled, wantStatus: common.ChannelStatusEnabled},
				{name: "manual disable is preserved", initialStatus: common.ChannelStatusManuallyDisabled, wantStatus: common.ChannelStatusManuallyDisabled},
				{name: "manual disable after exhaustion is preserved", initialStatus: common.ChannelStatusEnabled, manualOverride: "status", wantStatus: common.ChannelStatusManuallyDisabled},
				{name: "tag disable after exhaustion is preserved", initialStatus: common.ChannelStatusEnabled, manualOverride: "tag", wantStatus: common.ChannelStatusManuallyDisabled},
				{name: "auto key exhaustion restores", initialStatus: common.ChannelStatusEnabled, autoExhaust: true, wantStatus: common.ChannelStatusEnabled},
				{name: "auto disable for another reason is preserved", initialStatus: common.ChannelStatusAutoDisabled, wantStatus: common.ChannelStatusAutoDisabled},
				{name: "manual disable after auto exhaustion is preserved", initialStatus: common.ChannelStatusEnabled, autoExhaust: true, manualOverride: "status", wantStatus: common.ChannelStatusManuallyDisabled},
				{name: "tag disable after auto exhaustion is preserved", initialStatus: common.ChannelStatusEnabled, autoExhaust: true, manualOverride: "tag", wantStatus: common.ChannelStatusManuallyDisabled},
			} {
				t.Run(fmt.Sprintf("cache=%t/%s/%s", cacheEnabled, action, tc.name), func(t *testing.T) {
					database := setupMultiKeyChannelTestDB(t)
					common.MemoryCacheEnabled = cacheEnabled
					t.Cleanup(func() {
						common.MemoryCacheEnabled = false
					})

					tag := t.Name()
					channel := &model.Channel{
						Name:   t.Name(),
						Type:   1,
						Key:    "key-one\nkey-two",
						Status: tc.initialStatus,
						Models: "test-model",
						Group:  "default",
						Tag:    &tag,
						ChannelInfo: model.ChannelInfo{
							IsMultiKey:         true,
							MultiKeySize:       2,
							MultiKeyStatusList: map[int]int{1: common.ChannelStatusManuallyDisabled},
						},
					}
					require.NoError(t, channel.Insert())
					t.Cleanup(func() {
						_ = channel.Delete()
						model.InitChannelCache()
					})

					operations := []string{"disable_key", action}
					if tc.autoExhaust {
						// Key two is already disabled, so a request failure on key
						// one disables the whole channel automatically.
						model.InitChannelCache()
						require.True(t, model.UpdateChannelStatus(channel.Id, "key-one", common.ChannelStatusAutoDisabled, "provider rejected key"))
						exhausted, err := model.GetChannelById(channel.Id, true)
						require.NoError(t, err)
						require.Equal(t, common.ChannelStatusAutoDisabled, exhausted.Status)
						operations = []string{action}
					}
					for _, operation := range operations {
						if operation == action {
							if tc.manualOverride == "status" {
								model.UpdateChannelStatus(channel.Id, "", common.ChannelStatusManuallyDisabled, "manual operation")
							} else if tc.manualOverride == "tag" {
								require.NoError(t, model.DisableChannelByTag(tag))
							}
						}
						payload, err := common.Marshal(MultiKeyManageRequest{ChannelId: channel.Id, Action: operation, KeyIndex: common.GetPointer(0)})
						require.NoError(t, err)
						recorder := httptest.NewRecorder()
						c, _ := gin.CreateTestContext(recorder)
						c.Request = httptest.NewRequest(http.MethodPost, "/api/channel/multi_key", bytes.NewReader(payload))
						c.Request.Header.Set("Content-Type", "application/json")
						ManageMultiKeys(c)
						var result struct {
							Success bool `json:"success"`
						}
						require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &result))
						require.True(t, result.Success, recorder.Body.String())
					}

					loaded, err := model.GetChannelById(channel.Id, true)
					require.NoError(t, err)
					assert.Equal(t, tc.wantStatus, loaded.Status)
					if tc.wantStatus == common.ChannelStatusEnabled {
						assert.NotEqual(t, model.ChannelStatusReasonAllKeysDisabled, loaded.GetOtherInfo()["status_reason"])
						if tc.autoExhaust {
							assert.Equal(t, "", loaded.GetOtherInfo()["status_reason"])
						}
					}
					assert.NotContains(t, loaded.ChannelInfo.MultiKeyStatusList, 0)
					assert.NotContains(t, loaded.ChannelInfo.MultiKeyDisabledReason, 0)
					assert.NotContains(t, loaded.ChannelInfo.MultiKeyDisabledTime, 0)
					var ability model.Ability
					require.NoError(t, database.Where("channel_id = ?", channel.Id).First(&ability).Error)
					assert.Equal(t, tc.wantStatus == common.ChannelStatusEnabled, ability.Enabled)
				})
			}
		}
	}
}
