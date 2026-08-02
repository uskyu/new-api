package service

import (
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

const quickLoginCleanupInterval = time.Hour

func StartQuickLoginCleanup() {
	if !common.IsMasterNode {
		return
	}
	go func() {
		cleanupExpiredQuickLoginFlows()
		ticker := time.NewTicker(quickLoginCleanupInterval)
		defer ticker.Stop()
		for range ticker.C {
			cleanupExpiredQuickLoginFlows()
		}
	}()
}

func cleanupExpiredQuickLoginFlows() {
	if err := model.DeleteExpiredAuthFlows(time.Now()); err != nil {
		common.SysError("failed to delete expired quick login flows: " + err.Error())
	}
}
