package controller

import (
	"errors"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func bindRefundFilter(c *gin.Context) (model.RefundFilter, bool) {
	var f model.RefundFilter
	if err := common.DecodeJson(c.Request.Body, &f); err != nil {
		common.ApiError(c, errors.New("invalid request: "+err.Error()))
		return f, false
	}
	return f, true
}

func GetRefundOptions(c *gin.Context) {
	start, e1 := strconv.ParseInt(c.Query("start_time"), 10, 64)
	end, e2 := strconv.ParseInt(c.Query("end_time"), 10, 64)
	if e1 != nil || e2 != nil {
		common.ApiError(c, errors.New("start_time and end_time are required"))
		return
	}
	data, err := model.GetRefundOptions(start, end)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, data)
}
func PreviewRefund(c *gin.Context) {
	f, ok := bindRefundFilter(c)
	if !ok {
		return
	}
	data, err := model.PreviewRefund(f)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, data)
}

type createRefundRequest struct {
	model.RefundFilter
	IdempotencyKey string `json:"idempotency_key"`
}

func CreateRefundBatch(c *gin.Context) {
	var req createRefundRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiError(c, errors.New("invalid request: "+err.Error()))
		return
	}
	key := strings.TrimSpace(req.IdempotencyKey)
	if key == "" {
		key = strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	}
	data, err := model.CreateAndRunRefund(req.RefundFilter, key, c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, data)
}
func ListRefundBatches(c *gin.Context) {
	p := common.GetPageQuery(c)
	rows, total, err := model.ListRefundBatches(p.GetStartIdx(), p.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	p.SetTotal(int(total))
	p.SetItems(rows)
	common.ApiSuccess(c, p)
}
func GetRefundBatch(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiError(c, errors.New("invalid batch id"))
		return
	}
	data, err := model.GetRefundBatch(id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		common.ApiError(c, errors.New("refund batch not found"))
		return
	}
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, data)
}
