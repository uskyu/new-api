package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/bytedance/gopkg/util/gopool"
)

const blindBoxExpireTickInterval = time.Minute

var blindBoxExpireTaskOnce sync.Once

// StartBlindBoxExpireTask 启动盲盒待支付订单过期任务，每分钟检查一次。
func StartBlindBoxExpireTask() {
	blindBoxExpireTaskOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		gopool.Go(func() {
			logger.LogInfo(context.Background(), fmt.Sprintf("blind box expire task started: tick=%s", blindBoxExpireTickInterval))
			ticker := time.NewTicker(blindBoxExpireTickInterval)
			defer ticker.Stop()
			runBlindBoxExpireTaskOnce()
			for range ticker.C {
				runBlindBoxExpireTaskOnce()
			}
		})
	})
}

func runBlindBoxExpireTaskOnce() {
	setting := operation_setting.GetBlindBoxSetting()
	timeoutMinutes := setting.OrderTimeoutMinutes
	if timeoutMinutes <= 0 {
		timeoutMinutes = model.BlindBoxDefaultTimeout
	}
	expired, err := model.ExpirePendingBlindBoxOrders(timeoutMinutes)
	if err != nil {
		logger.LogWarn(context.Background(), fmt.Sprintf("blind box expire task failed: %v", err))
		return
	}
	if expired > 0 {
		logger.LogInfo(context.Background(), fmt.Sprintf("blind box expire task marked %d pending order(s) as expired", expired))
	}
}
