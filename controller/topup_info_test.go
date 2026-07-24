package controller

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetTopUpInfoReturnsWalletNotice(t *testing.T) {
	const notice = `<p>Scheduled maintenance at 02:00 UTC.</p>`

	common.OptionMapRWMutex.Lock()
	mapWasNil := common.OptionMap == nil
	if mapWasNil {
		common.OptionMap = make(map[string]string)
	}
	previous, existed := common.OptionMap["TopupNoticeHTML"]
	common.OptionMap["TopupNoticeHTML"] = notice
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		defer common.OptionMapRWMutex.Unlock()
		if mapWasNil {
			common.OptionMap = nil
			return
		}
		if existed {
			common.OptionMap["TopupNoticeHTML"] = previous
			return
		}
		delete(common.OptionMap, "TopupNoticeHTML")
	})

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	GetTopUpInfo(context)

	var response struct {
		Success bool `json:"success"`
		Data    struct {
			TopupNoticeHTML string `json:"topup_notice_html"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.True(t, response.Success)
	assert.Equal(t, notice, response.Data.TopupNoticeHTML)
}
