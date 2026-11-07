package controller

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/Calcium-Ion/go-epay/epay"
	"github.com/gin-gonic/gin"
)

const blindBoxMaxOrderTimeoutMinutes = 10080 // 7 天

type blindBoxPurchaseRequest struct {
	BoxId         int    `json:"box_id"`
	PaymentMethod string `json:"payment_method"`
}

type blindBoxCompleteRequest struct {
	TradeNo string `json:"trade_no"`
}

type blindBoxSettingRequest struct {
	Enabled             bool `json:"enabled"`
	OrderTimeoutMinutes int  `json:"order_timeout_minutes"`
}

type blindBoxUpsertRequest struct {
	Name         string          `json:"name"`
	Price        float64         `json:"price"`
	StockTotal   int             `json:"stock_total"`
	PerUserLimit int             `json:"per_user_limit"`
	Enabled      *bool           `json:"enabled"`
	Sort         int             `json:"sort"`
	Tiers        json.RawMessage `json:"tiers"`
	Remark       string          `json:"remark"`
}

type blindBoxAdminView struct {
	model.BlindBox
	Remaining int     `json:"remaining"`
	MinReward float64 `json:"min_reward"`
	MaxReward float64 `json:"max_reward"`
}

func newBlindBoxAdminView(box *model.BlindBox) *blindBoxAdminView {
	minReward, maxReward := model.GetBlindBoxRewardRange(box.Tiers, box.Price)
	return &blindBoxAdminView{
		BlindBox:  *box,
		Remaining: model.GetBlindBoxRemaining(box),
		MinReward: minReward,
		MaxReward: maxReward,
	}
}

func blindBoxPurchaseError(c *gin.Context, message string) {
	c.JSON(http.StatusOK, gin.H{"message": "error", "data": message})
}

// GetBlindBoxList 用户端盲盒列表。未开启时返回 enabled=false 与空列表。
func GetBlindBoxList(c *gin.Context) {
	setting := operation_setting.GetBlindBoxSetting()
	boxes := make([]*model.BlindBoxUserView, 0)
	if setting.Enabled {
		userId := c.GetInt("id")
		list, err := model.GetBlindBoxesForUser(userId, setting.CycleId)
		if err != nil {
			logger.LogError(c.Request.Context(), fmt.Sprintf("获取盲盒列表失败 user_id=%d error=%q", userId, err.Error()))
			common.ApiErrorMsg(c, "获取盲盒列表失败")
			return
		}
		boxes = list
	}
	common.ApiSuccess(c, gin.H{
		"enabled":  setting.Enabled,
		"cycle_id": setting.CycleId,
		"boxes":    boxes,
	})
}

// PurchaseBlindBox 用户购买盲盒：创建 pending 订单并拉起易支付。
func PurchaseBlindBox(c *gin.Context) {
	userId := c.GetInt("id")
	var req blindBoxPurchaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		blindBoxPurchaseError(c, "参数错误")
		return
	}
	if req.BoxId <= 0 {
		blindBoxPurchaseError(c, "参数错误")
		return
	}
	paymentMethod := strings.TrimSpace(req.PaymentMethod)
	if paymentMethod == "" {
		blindBoxPurchaseError(c, "请选择支付方式")
		return
	}

	setting := operation_setting.GetBlindBoxSetting()
	if !setting.Enabled {
		blindBoxPurchaseError(c, "盲盒活动未开启")
		return
	}
	if !operation_setting.ContainsPayMethod(paymentMethod) {
		blindBoxPurchaseError(c, "支付方式不存在")
		return
	}

	box, err := model.GetBlindBoxById(req.BoxId)
	if err != nil {
		if errors.Is(err, model.ErrBlindBoxNotFound) {
			blindBoxPurchaseError(c, "盲盒不存在")
		} else {
			logger.LogError(c.Request.Context(), fmt.Sprintf("查询盲盒失败 user_id=%d box_id=%d error=%q", userId, req.BoxId, err.Error()))
			blindBoxPurchaseError(c, "获取盲盒信息失败")
		}
		return
	}
	if !box.Enabled {
		blindBoxPurchaseError(c, "盲盒未上架")
		return
	}
	if box.StockTotal > 0 && box.SoldCount >= box.StockTotal {
		blindBoxPurchaseError(c, "盲盒已售罄")
		return
	}
	if box.PerUserLimit > 0 {
		used, err := model.GetUserBlindBoxSuccessCount(userId, box.Id, setting.CycleId)
		if err != nil {
			logger.LogError(c.Request.Context(), fmt.Sprintf("查询盲盒限购次数失败 user_id=%d box_id=%d error=%q", userId, box.Id, err.Error()))
			blindBoxPurchaseError(c, "获取限购信息失败")
			return
		}
		if used >= int64(box.PerUserLimit) {
			blindBoxPurchaseError(c, "已达限购上限")
			return
		}
	}

	baseQuota := model.BlindBoxBaseQuota(box.Price)
	if baseQuota <= 0 {
		logger.LogError(c.Request.Context(), fmt.Sprintf("盲盒基础额度异常 box_id=%d price=%.2f base_quota=%d", box.Id, box.Price, baseQuota))
		blindBoxPurchaseError(c, "盲盒基础额度配置异常")
		return
	}

	client := GetEpayClient()
	if client == nil {
		blindBoxPurchaseError(c, "当前管理员未配置支付信息")
		return
	}

	tradeNo := fmt.Sprintf("BBX%d%s%d", userId, common.GetRandomString(12), time.Now().UnixNano())
	order := &model.BlindBoxOrder{
		TradeNo:         tradeNo,
		UserId:          userId,
		BoxId:           box.Id,
		BoxName:         box.Name,
		CycleId:         setting.CycleId,
		Price:           box.Price,
		BaseQuota:       baseQuota,
		TiersSnapshot:   box.Tiers,
		Status:          common.TopUpStatusPending,
		PaymentMethod:   paymentMethod,
		PaymentProvider: model.PaymentProviderEpay,
		CreateTime:      common.GetTimestamp(),
	}
	if err := model.CreateBlindBoxOrder(order); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("创建盲盒订单失败 user_id=%d box_id=%d trade_no=%s error=%q", userId, box.Id, tradeNo, err.Error()))
		blindBoxPurchaseError(c, "创建订单失败")
		return
	}

	callBackAddress := service.GetCallbackAddress()
	notifyUrl, err := url.Parse(callBackAddress + "/api/blind-box/epay/notify")
	if err != nil {
		_ = model.MarkBlindBoxOrderFailed(tradeNo)
		logger.LogError(c.Request.Context(), fmt.Sprintf("生成盲盒支付回调地址失败 trade_no=%s error=%q", tradeNo, err.Error()))
		blindBoxPurchaseError(c, "拉起支付失败")
		return
	}
	returnUrl, err := url.Parse(paymentReturnPath("/console/blind-box"))
	if err != nil {
		_ = model.MarkBlindBoxOrderFailed(tradeNo)
		logger.LogError(c.Request.Context(), fmt.Sprintf("生成盲盒支付返回地址失败 trade_no=%s error=%q", tradeNo, err.Error()))
		blindBoxPurchaseError(c, "拉起支付失败")
		return
	}

	uri, params, err := client.Purchase(&epay.PurchaseArgs{
		Type:           paymentMethod,
		ServiceTradeNo: tradeNo,
		Name:           fmt.Sprintf("BLINDBOX%d", box.Id),
		Money:          fmt.Sprintf("%.2f", box.Price),
		Device:         epay.PC,
		NotifyUrl:      notifyUrl,
		ReturnUrl:      returnUrl,
	})
	if err != nil {
		if updateErr := model.MarkBlindBoxOrderFailed(tradeNo); updateErr != nil {
			logger.LogError(c.Request.Context(), fmt.Sprintf("盲盒订单置为失败失败 trade_no=%s error=%q", tradeNo, updateErr.Error()))
		}
		logger.LogError(c.Request.Context(), fmt.Sprintf("盲盒 拉起支付失败 user_id=%d box_id=%d trade_no=%s payment_method=%s error=%q", userId, box.Id, tradeNo, paymentMethod, err.Error()))
		blindBoxPurchaseError(c, "拉起支付失败")
		return
	}
	logger.LogInfo(c.Request.Context(), fmt.Sprintf("盲盒订单创建成功 user_id=%d box_id=%d trade_no=%s payment_method=%s price=%.2f uri=%q params=%q", userId, box.Id, tradeNo, paymentMethod, box.Price, uri, common.GetJsonString(params)))
	c.JSON(http.StatusOK, gin.H{"message": "success", "data": params, "url": uri, "trade_no": tradeNo})
}

// GetSelfBlindBoxOrders 用户自己的盲盒订单分页。
func GetSelfBlindBoxOrders(c *gin.Context) {
	userId := c.GetInt("id")
	pageInfo := common.GetPageQuery(c)
	orders, total, err := model.GetUserBlindBoxOrders(userId, pageInfo)
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("获取盲盒订单失败 user_id=%d error=%q", userId, err.Error()))
		common.ApiErrorMsg(c, "获取盲盒订单失败")
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(orders)
	common.ApiSuccess(c, pageInfo)
}

// BlindBoxEpayNotify 易支付盲盒回调。只有结算成功后才返回 success，否则返回 fail 以便易支付重试。
func BlindBoxEpayNotify(c *gin.Context) {
	if !isEpayWebhookEnabled() {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("盲盒易支付 webhook 被拒绝 reason=webhook_disabled path=%q client_ip=%s", c.Request.RequestURI, c.ClientIP()))
		writeBlindBoxEpayResult(c, "fail")
		return
	}

	var params map[string]string
	if c.Request.Method == http.MethodPost {
		if err := c.Request.ParseForm(); err != nil {
			logger.LogError(c.Request.Context(), fmt.Sprintf("盲盒易支付 webhook POST 表单解析失败 path=%q client_ip=%s error=%q", c.Request.RequestURI, c.ClientIP(), err.Error()))
			writeBlindBoxEpayResult(c, "fail")
			return
		}
		params = make(map[string]string, len(c.Request.PostForm))
		for key := range c.Request.PostForm {
			params[key] = c.Request.PostForm.Get(key)
		}
	} else {
		query := c.Request.URL.Query()
		params = make(map[string]string, len(query))
		for key := range query {
			params[key] = query.Get(key)
		}
	}
	if len(params) == 0 {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("盲盒易支付 webhook 参数为空 path=%q client_ip=%s", c.Request.RequestURI, c.ClientIP()))
		writeBlindBoxEpayResult(c, "fail")
		return
	}

	client := GetEpayClient()
	if client == nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("盲盒易支付 client 未初始化 path=%q client_ip=%s", c.Request.RequestURI, c.ClientIP()))
		writeBlindBoxEpayResult(c, "fail")
		return
	}
	verifyInfo, err := client.Verify(params)
	if err != nil || !verifyInfo.VerifyStatus {
		if err != nil {
			logger.LogWarn(c.Request.Context(), fmt.Sprintf("盲盒易支付 webhook 验签失败 path=%q client_ip=%s error=%q", c.Request.RequestURI, c.ClientIP(), err.Error()))
		} else {
			logger.LogWarn(c.Request.Context(), fmt.Sprintf("盲盒易支付 webhook 验签失败 path=%q client_ip=%s verify_status=false", c.Request.RequestURI, c.ClientIP()))
		}
		writeBlindBoxEpayResult(c, "fail")
		return
	}

	if verifyInfo.TradeStatus != epay.StatusTradeSuccess {
		logger.LogInfo(c.Request.Context(), fmt.Sprintf("盲盒易支付 webhook 忽略事件 trade_no=%s trade_status=%s client_ip=%s", verifyInfo.ServiceTradeNo, verifyInfo.TradeStatus, c.ClientIP()))
		writeBlindBoxEpayResult(c, "success")
		return
	}

	tradeNo := verifyInfo.ServiceTradeNo
	logger.LogInfo(c.Request.Context(), fmt.Sprintf("盲盒易支付 webhook 验签成功 trade_no=%s trade_status=%s client_ip=%s params=%q", tradeNo, verifyInfo.TradeStatus, c.ClientIP(), common.GetJsonString(params)))

	order, err := model.GetBlindBoxOrderByTradeNo(tradeNo)
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("盲盒易支付 webhook 订单不存在 trade_no=%s client_ip=%s error=%q", tradeNo, c.ClientIP(), err.Error()))
		writeBlindBoxEpayResult(c, "fail")
		return
	}
	if order.PaymentProvider != model.PaymentProviderEpay {
		logger.LogError(c.Request.Context(), fmt.Sprintf("盲盒易支付 webhook 支付渠道不匹配 trade_no=%s order_provider=%s client_ip=%s", tradeNo, order.PaymentProvider, c.ClientIP()))
		writeBlindBoxEpayResult(c, "fail")
		return
	}

	if money := strings.TrimSpace(verifyInfo.Money); money != "" {
		actualMoney, parseErr := strconv.ParseFloat(money, 64)
		if parseErr != nil {
			logger.LogError(c.Request.Context(), fmt.Sprintf("盲盒易支付 webhook 实付金额无法解析 trade_no=%s money=%q client_ip=%s error=%q", tradeNo, money, c.ClientIP(), parseErr.Error()))
			writeBlindBoxEpayResult(c, "fail")
			return
		}
		if math.Abs(actualMoney-order.Price) > 0.01 {
			logger.LogError(c.Request.Context(), fmt.Sprintf("盲盒易支付 webhook 实付金额不匹配 trade_no=%s order_price=%.2f actual_money=%.2f client_ip=%s", tradeNo, order.Price, actualMoney, c.ClientIP()))
			writeBlindBoxEpayResult(c, "fail")
			return
		}
	}

	settledOrder, alreadyProcessed, err := model.SettleBlindBoxOrder(tradeNo)
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("盲盒订单结算失败 trade_no=%s client_ip=%s error=%q", tradeNo, c.ClientIP(), err.Error()))
		writeBlindBoxEpayResult(c, "fail")
		return
	}
	if !alreadyProcessed {
		logger.LogInfo(c.Request.Context(), fmt.Sprintf("盲盒订单结算成功 trade_no=%s user_id=%d box_id=%d reward_quota=%d multiplier=%.4f oversold=%t over_limit=%t client_ip=%s", tradeNo, settledOrder.UserId, settledOrder.BoxId, settledOrder.RewardQuota, settledOrder.DrawnMultiplier, settledOrder.Oversold, settledOrder.OverLimit, c.ClientIP()))
	} else {
		logger.LogInfo(c.Request.Context(), fmt.Sprintf("盲盒订单重复回调已忽略 trade_no=%s client_ip=%s", tradeNo, c.ClientIP()))
	}
	writeBlindBoxEpayResult(c, "success")
}

func writeBlindBoxEpayResult(c *gin.Context, result string) {
	if _, err := c.Writer.Write([]byte(result)); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("盲盒易支付回调响应写入失败 path=%q client_ip=%s error=%q", c.Request.RequestURI, c.ClientIP(), err.Error()))
	}
}

// AdminGetBlindBoxes 管理端盲盒列表（含禁用）。
func AdminGetBlindBoxes(c *gin.Context) {
	boxes, err := model.GetAllBlindBoxes()
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("管理端获取盲盒列表失败 error=%q", err.Error()))
		common.ApiErrorMsg(c, "获取盲盒列表失败")
		return
	}
	views := make([]*blindBoxAdminView, 0, len(boxes))
	for _, box := range boxes {
		views = append(views, newBlindBoxAdminView(box))
	}
	common.ApiSuccess(c, views)
}

// AdminCreateBlindBox 管理端新建盲盒。
func AdminCreateBlindBox(c *gin.Context) {
	var req blindBoxUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	tiers, err := parseBlindBoxTiersInput(req.Tiers)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	enabled := false
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	box := &model.BlindBox{
		Name:         req.Name,
		Price:        req.Price,
		StockTotal:   req.StockTotal,
		PerUserLimit: req.PerUserLimit,
		Enabled:      enabled,
		Sort:         req.Sort,
		Tiers:        tiers,
		Remark:       req.Remark,
	}
	if err := model.ValidateBlindBox(box); err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	if err := model.CreateBlindBox(box); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("管理端创建盲盒失败 error=%q", err.Error()))
		common.ApiErrorMsg(c, "创建盲盒失败")
		return
	}
	logger.LogInfo(c.Request.Context(), fmt.Sprintf("管理端创建盲盒成功 box_id=%d name=%q price=%.2f", box.Id, box.Name, box.Price))
	common.ApiSuccess(c, newBlindBoxAdminView(box))
}

// AdminUpdateBlindBox 管理端更新盲盒，不修改 sold_count。
func AdminUpdateBlindBox(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	var req blindBoxUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	tiers, err := parseBlindBoxTiersInput(req.Tiers)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}

	existing, err := model.GetBlindBoxById(id)
	if err != nil {
		if errors.Is(err, model.ErrBlindBoxNotFound) {
			common.ApiErrorMsg(c, "盲盒不存在")
		} else {
			logger.LogError(c.Request.Context(), fmt.Sprintf("管理端查询盲盒失败 box_id=%d error=%q", id, err.Error()))
			common.ApiErrorMsg(c, "获取盲盒信息失败")
		}
		return
	}

	candidate := *existing
	candidate.Name = req.Name
	candidate.Price = req.Price
	candidate.StockTotal = req.StockTotal
	candidate.PerUserLimit = req.PerUserLimit
	candidate.Sort = req.Sort
	candidate.Tiers = tiers
	candidate.Remark = req.Remark
	if req.Enabled != nil {
		candidate.Enabled = *req.Enabled
	}
	if err := model.ValidateBlindBox(&candidate); err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}

	updates := map[string]interface{}{
		"name":           candidate.Name,
		"price":          candidate.Price,
		"stock_total":    candidate.StockTotal,
		"per_user_limit": candidate.PerUserLimit,
		"enabled":        candidate.Enabled,
		"sort":           candidate.Sort,
		"tiers":          candidate.Tiers,
		"remark":         candidate.Remark,
	}
	if err := model.UpdateBlindBox(id, updates); err != nil {
		if errors.Is(err, model.ErrBlindBoxNotFound) {
			common.ApiErrorMsg(c, "盲盒不存在")
		} else {
			logger.LogError(c.Request.Context(), fmt.Sprintf("管理端更新盲盒失败 box_id=%d error=%q", id, err.Error()))
			common.ApiErrorMsg(c, "更新盲盒失败")
		}
		return
	}

	updated, err := model.GetBlindBoxById(id)
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("管理端更新后查询盲盒失败 box_id=%d error=%q", id, err.Error()))
		common.ApiErrorMsg(c, "更新盲盒失败")
		return
	}
	logger.LogInfo(c.Request.Context(), fmt.Sprintf("管理端更新盲盒成功 box_id=%d name=%q price=%.2f", updated.Id, updated.Name, updated.Price))
	common.ApiSuccess(c, newBlindBoxAdminView(updated))
}

// AdminGetBlindBoxOrders 管理端盲盒订单分页与筛选。
func AdminGetBlindBoxOrders(c *gin.Context) {
	userId, err := parseOptionalPositiveInt(c.Query("user_id"))
	if err != nil {
		common.ApiErrorMsg(c, "user_id 参数错误")
		return
	}
	boxId, err := parseOptionalPositiveInt(c.Query("box_id"))
	if err != nil {
		common.ApiErrorMsg(c, "box_id 参数错误")
		return
	}
	startTimestamp, err := parseOptionalTimestamp(c.Query("start_timestamp"))
	if err != nil {
		common.ApiErrorMsg(c, "start_timestamp 参数错误")
		return
	}
	endTimestamp, err := parseOptionalTimestamp(c.Query("end_timestamp"))
	if err != nil {
		common.ApiErrorMsg(c, "end_timestamp 参数错误")
		return
	}

	pageInfo := common.GetPageQuery(c)
	filter := model.BlindBoxOrderFilter{
		UserId:         userId,
		BoxId:          boxId,
		Status:         strings.TrimSpace(c.Query("status")),
		Keyword:        c.Query("keyword"),
		StartTimestamp: startTimestamp,
		EndTimestamp:   endTimestamp,
	}
	orders, total, err := model.AdminGetBlindBoxOrders(filter, pageInfo)
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("管理端获取盲盒订单失败 error=%q", err.Error()))
		common.ApiErrorMsg(c, err.Error())
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(orders)
	common.ApiSuccess(c, pageInfo)
}

// AdminCompleteBlindBoxOrder 管理员手动补单：pending/expired 订单正常结算。
func AdminCompleteBlindBoxOrder(c *gin.Context) {
	var req blindBoxCompleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	tradeNo := strings.TrimSpace(req.TradeNo)
	if tradeNo == "" {
		common.ApiErrorMsg(c, "请提供订单号")
		return
	}
	order, alreadyProcessed, err := model.SettleBlindBoxOrder(tradeNo)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	if !alreadyProcessed {
		logger.LogInfo(c.Request.Context(), fmt.Sprintf("管理员手动补单盲盒成功 trade_no=%s user_id=%d box_id=%d reward_quota=%d multiplier=%.4f oversold=%t over_limit=%t", tradeNo, order.UserId, order.BoxId, order.RewardQuota, order.DrawnMultiplier, order.Oversold, order.OverLimit))
	}
	common.ApiSuccess(c, order)
}

// AdminGetBlindBoxSetting 管理端读取盲盒全局设置。
func AdminGetBlindBoxSetting(c *gin.Context) {
	common.ApiSuccess(c, operation_setting.GetBlindBoxSetting())
}

// AdminUpdateBlindBoxSetting 管理端保存盲盒全局设置，并在关闭->开启时生成新的周期 ID。
func AdminUpdateBlindBoxSetting(c *gin.Context) {
	var req blindBoxSettingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	timeoutMinutes := req.OrderTimeoutMinutes
	if timeoutMinutes <= 0 {
		timeoutMinutes = 15
	}
	if timeoutMinutes > blindBoxMaxOrderTimeoutMinutes {
		common.ApiErrorMsg(c, "订单超时时间不能超过 10080 分钟")
		return
	}

	current := operation_setting.GetBlindBoxSetting()
	cycleId := current.CycleId
	if req.Enabled && (!current.Enabled || current.CycleId <= 0) {
		cycleId = time.Now().UnixMilli()
	}
	values := map[string]string{
		"blind_box_setting.enabled":               strconv.FormatBool(req.Enabled),
		"blind_box_setting.cycle_id":              strconv.FormatInt(cycleId, 10),
		"blind_box_setting.order_timeout_minutes": strconv.Itoa(timeoutMinutes),
	}
	if err := model.UpdateOptionsBulk(values); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("保存盲盒设置失败 error=%q", err.Error()))
		common.ApiErrorMsg(c, "保存设置失败")
		return
	}
	logger.LogInfo(c.Request.Context(), fmt.Sprintf("管理端保存盲盒设置成功 enabled=%t cycle_id=%d order_timeout_minutes=%d", req.Enabled, cycleId, timeoutMinutes))
	common.ApiSuccess(c, operation_setting.GetBlindBoxSetting())
}

// parseBlindBoxTiersInput 同时兼容 tiers 为数组或 JSON 字符串两种请求格式。
func parseBlindBoxTiersInput(raw json.RawMessage) (string, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return "", errors.New("请至少配置一个倍率档位")
	}
	if trimmed[0] == '"' {
		var encoded string
		if err := common.Unmarshal(trimmed, &encoded); err != nil {
			return "", errors.New("倍率档位格式错误")
		}
		trimmed = bytes.TrimSpace([]byte(encoded))
	}
	var tiers []model.BlindBoxTier
	if err := common.Unmarshal(trimmed, &tiers); err != nil {
		return "", errors.New("倍率档位格式错误")
	}
	data, err := common.Marshal(tiers)
	if err != nil {
		return "", errors.New("倍率档位序列化失败")
	}
	return string(data), nil
}

func parseOptionalPositiveInt(value string) (*int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return nil, errors.New("参数错误")
	}
	return &parsed, nil
}

func parseOptionalTimestamp(value string) (int64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		return 0, errors.New("参数错误")
	}
	return parsed, nil
}
