package controller

import (
	"fmt"
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
)

// GetCheckinStatus 获取用户签到状态和历史记录
func GetCheckinStatus(c *gin.Context) {
	setting := operation_setting.GetCheckinSetting()
	if !setting.Enabled {
		common.ApiErrorMsg(c, "签到功能未启用")
		return
	}
	userId := c.GetInt("id")
	// 获取月份参数，默认为当前月份
	month := c.DefaultQuery("month", time.Now().Format("2006-01"))

	stats, err := model.GetUserCheckinStats(userId, month)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	// bonus_enabled 由是否存在档位决定，旧开关（checkin_setting.bonus_enabled）已不再参与计算。
	data := gin.H{
		"enabled":         setting.Enabled,
		"captcha_enabled": setting.CaptchaEnabled,
		"captcha_kind":    setting.CaptchaKind,
		"bonus_enabled":   len(setting.ActiveTiers()) > 0,
		"bonus_metric":    setting.BonusMetric,
		"bonus_tiers":     operation_setting.SortedUniqueTiers(setting.ActiveTiers()),
		"stats":           stats,
	}
	if len(setting.ActiveTiers()) > 0 {
		calls, consumedQuota, err := model.GetYesterdayCheckinUsage(userId)
		if err == nil {
			data["yesterday_calls"] = calls
			data["yesterday_quota"] = consumedQuota
			metric := calls
			if setting.BonusMetric == "quota_consumed" {
				metric = consumedQuota
			}
			if tier := model.SelectCheckinTier(setting.ActiveTiers(), metric); tier != nil {
				data["bonus_tier"] = gin.H{
					"threshold": tier.Threshold,
					"min_quota": tier.MinQuota,
					"max_quota": tier.MaxQuota,
				}
			}
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    data,
	})
}

// GetCheckinCaptcha 获取签到图形验证码
func GetCheckinCaptcha(c *gin.Context) {
	setting := operation_setting.GetCheckinSetting()
	if !setting.Enabled || !setting.CaptchaEnabled {
		common.ApiErrorMsg(c, "签到验证码未启用")
		return
	}
	captchaId, imageBase64, err := service.GenerateCheckinCaptcha(setting.CaptchaKind)
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

// DoCheckin 执行用户签到
func DoCheckin(c *gin.Context) {
	setting := operation_setting.GetCheckinSetting()
	if !setting.Enabled {
		common.ApiErrorMsg(c, "签到功能未启用")
		return
	}

	if setting.CaptchaEnabled {
		if err := service.VerifyCheckinCaptcha(c.Query("captcha_id"), c.Query("captcha_answer")); err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	}

	userId := c.GetInt("id")

	checkin, err := model.UserCheckin(userId)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	if checkin.QuotaAwarded > 0 {
		model.RecordLog(userId, model.LogTypeSystem, fmt.Sprintf("用户签到，获得额度 %s", logger.LogQuota(checkin.QuotaAwarded)))
	} else {
		model.RecordLog(userId, model.LogTypeSystem, "用户签到，昨日活跃度未达标，本次无额度奖励")
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "签到成功",
		"data": gin.H{
			"quota_awarded": checkin.QuotaAwarded,
			"checkin_date":  checkin.CheckinDate},
	})
}
