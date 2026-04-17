package service

import (
	"fmt"
	"sync/atomic"

	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
)

var aiImageMemoryReservedBytes int64
var aiImageInFlight int64

func estimateAIImageReservation(rawBytes int64) int64 {
	if rawBytes <= 0 {
		return 0
	}
	// 原始二进制 + base64 膨胀 + 中间对象拷贝，留足一定安全余量
	reserved := rawBytes * 3
	if reserved < rawBytes {
		return rawBytes
	}
	return reserved
}

func TryReserveAIImageMemory(c *gin.Context, rawBytes int64) (int64, error) {
	reserved := estimateAIImageReservation(rawBytes)
	if reserved <= 0 {
		return 0, nil
	}

	maxConcurrency := constant.AIImageMaxConcurrency
	if maxConcurrency > 0 {
		current := atomic.LoadInt64(&aiImageInFlight)
		if current >= int64(maxConcurrency) {
			return 0, fmt.Errorf("ai image requests are too busy, please retry later")
		}
	}

	maxMemoryMB := constant.AIImageMaxMemoryMB
	if maxMemoryMB > 0 {
		limitBytes := int64(maxMemoryMB) << 20
		for {
			current := atomic.LoadInt64(&aiImageMemoryReservedBytes)
			if current+reserved > limitBytes {
				return 0, fmt.Errorf("ai image temporary memory budget exceeded, please retry later")
			}
			if atomic.CompareAndSwapInt64(&aiImageMemoryReservedBytes, current, current+reserved) {
				break
			}
		}
	}

	atomic.AddInt64(&aiImageInFlight, 1)
	if c != nil {
		c.Set(string(constant.ContextKeyAIImageMemoryReservation), reserved)
	}
	return reserved, nil
}

func ReleaseAIImageMemory(c *gin.Context) {
	if c == nil {
		return
	}
	value, exists := c.Get(string(constant.ContextKeyAIImageMemoryReservation))
	if !exists || value == nil {
		return
	}
	reserved, ok := value.(int64)
	if !ok || reserved <= 0 {
		c.Set(string(constant.ContextKeyAIImageMemoryReservation), nil)
		return
	}
	if atomic.AddInt64(&aiImageMemoryReservedBytes, -reserved) < 0 {
		atomic.StoreInt64(&aiImageMemoryReservedBytes, 0)
	}
	if atomic.AddInt64(&aiImageInFlight, -1) < 0 {
		atomic.StoreInt64(&aiImageInFlight, 0)
	}
	c.Set(string(constant.ContextKeyAIImageMemoryReservation), nil)
}
