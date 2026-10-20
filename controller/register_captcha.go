package controller

import (
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

// GetRegisterCaptcha 获取注册图形验证码
func GetRegisterCaptcha(c *gin.Context) {
	if !common.RegisterCaptchaEnabled {
		common.ApiErrorMsg(c, "注册验证码未启用")
		return
	}
	captchaId, imageBase64, err := service.GenerateRegisterCaptcha(common.RegisterCaptchaKind)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"captcha_id":   captchaId,
			"image_base64": imageBase64,
		},
	})
}
