package controller

import (
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/gin-gonic/gin"
)

// GetChatWidget exposes the public LiveHelperChat bootstrap configuration.
// It never returns an invalid or unsafe configuration: callers receive
// enabled=false and no widget URL when the setting is missing, disabled, or
// cannot be validated.
func GetChatWidget(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    system_setting.GetLiveHelperChatPublicConfig(common.DebugEnabled),
	})
}
