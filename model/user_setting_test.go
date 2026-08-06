package model

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

func TestUserSettingRecordIPDefaultsOn(t *testing.T) {
	for _, rawSetting := range []string{"", `{}`} {
		user := User{Setting: rawSetting}
		require.True(t, user.GetSetting().RecordIpLog)

		userBase := UserBase{Setting: rawSetting}
		require.True(t, userBase.GetSetting().RecordIpLog)
	}
}

func TestUserSettingRecordIPCanBeDisabled(t *testing.T) {
	user := User{Setting: `{"record_ip_log":false}`}
	require.False(t, user.GetSetting().RecordIpLog)

	user.SetSetting(dto.UserSetting{RecordIpLog: false})
	require.Contains(t, user.Setting, `"record_ip_log":false`)
	require.False(t, user.GetSetting().RecordIpLog)

	userBase := UserBase{Setting: user.Setting}
	require.False(t, userBase.GetSetting().RecordIpLog)
}
