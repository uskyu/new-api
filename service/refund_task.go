package service

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/bytedance/gopkg/util/gopool"
)

const (
	refundWorkerTickInterval  = 2 * time.Second
	refundWorkerLeaseSeconds  = int64(60)
	refundWorkerChunksPerRun  = 20
	refundLogDeliveriesPerRun = 100
)

var (
	refundWorkerOnce    sync.Once
	refundWorkerRunning atomic.Bool
)

func StartRefundTask() {
	refundWorkerOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		hostname, _ := os.Hostname()
		workerId := fmt.Sprintf("%s-%d-%d", hostname, os.Getpid(), time.Now().UnixNano())
		if len(workerId) > 128 {
			workerId = workerId[:128]
		}
		gopool.Go(func() {
			logger.LogInfo(context.Background(), fmt.Sprintf("refund worker started: tick=%s", refundWorkerTickInterval))
			ticker := time.NewTicker(refundWorkerTickInterval)
			defer ticker.Stop()
			runRefundTaskOnce(workerId)
			for range ticker.C {
				runRefundTaskOnce(workerId)
			}
		})
	})
}

func runRefundTaskOnce(workerId string) {
	if !refundWorkerRunning.CompareAndSwap(false, true) {
		return
	}
	defer refundWorkerRunning.Store(false)

	for i := 0; i < refundWorkerChunksPerRun; i++ {
		worked, err := model.ProcessNextRefundBatchChunk(workerId, refundWorkerLeaseSeconds)
		if err != nil {
			logger.LogWarn(context.Background(), fmt.Sprintf("refund worker failed: %v", err))
			return
		}
		if !worked {
			break
		}
	}
	for i := 0; i < refundLogDeliveriesPerRun; i++ {
		worked, err := model.ProcessNextRefundLogDelivery(workerId, refundWorkerLeaseSeconds)
		if err != nil {
			logger.LogWarn(context.Background(), fmt.Sprintf("refund log delivery failed: %v", err))
			return
		}
		if !worked {
			return
		}
	}
}
