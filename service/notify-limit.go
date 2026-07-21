package service

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/bytedance/gopkg/util/gopool"
)

// notifyLimitStore is used for in-memory rate limiting when Redis is disabled
var (
	notifyLimitStore sync.Map
	notifyLimitMu    sync.Mutex
	cleanupOnce      sync.Once
)

type limitCount struct {
	Count     int
	Timestamp time.Time
	Duration  time.Duration
}

type notificationLimitRule struct {
	Key      string
	Limit    int
	Duration time.Duration
}

func getNotificationLimitRule(userId int, notifyType string, now time.Time) notificationLimitRule {
	if notifyType == dto.NotifyTypeQuotaExceed {
		return notificationLimitRule{
			Key:      fmt.Sprintf("%d:%s", userId, notifyType),
			Limit:    1,
			Duration: 24 * time.Hour,
		}
	}
	return notificationLimitRule{
		Key:      fmt.Sprintf("%d:%s:%s", userId, notifyType, now.Format("2006010215")),
		Limit:    constant.NotifyLimitCount,
		Duration: getDuration(),
	}
}

func getDuration() time.Duration {
	minute := constant.NotificationLimitDurationMinute
	return time.Duration(minute) * time.Minute
}

// startCleanupTask starts a background task to clean up expired entries
func startCleanupTask() {
	gopool.Go(func() {
		for {
			time.Sleep(time.Hour)
			notifyLimitMu.Lock()
			now := time.Now()
			notifyLimitStore.Range(func(key, value interface{}) bool {
				if limit, ok := value.(limitCount); ok {
					if now.Sub(limit.Timestamp) >= limit.Duration {
						notifyLimitStore.Delete(key)
					}
				}
				return true
			})
			notifyLimitMu.Unlock()
		}
	})
}

// CheckNotificationLimit checks if the user has exceeded their notification limit
// Returns true if the user can send notification, false if limit exceeded
func CheckNotificationLimit(userId int, notifyType string) (bool, error) {
	if common.RedisEnabled {
		return checkRedisLimit(userId, notifyType)
	}
	return checkMemoryLimit(userId, notifyType)
}

func checkRedisLimit(userId int, notifyType string) (bool, error) {
	rule := getNotificationLimitRule(userId, notifyType, time.Now())
	key := "notify_limit:" + rule.Key

	if notifyType == dto.NotifyTypeQuotaExceed {
		acquired, err := common.RDB.SetNX(context.Background(), key, "1", rule.Duration).Result()
		if err != nil {
			return false, fmt.Errorf("failed to reserve notification limit: %w", err)
		}
		return acquired, nil
	}

	// Get current count
	count, err := common.RedisGet(key)
	if err != nil && err.Error() != "redis: nil" {
		return false, fmt.Errorf("failed to get notification count: %w", err)
	}

	// If key doesn't exist, initialize it
	if count == "" {
		err = common.RedisSet(key, "1", rule.Duration)
		return true, err
	}

	currentCount, _ := strconv.Atoi(count)

	// Check if limit is already reached
	if currentCount >= rule.Limit {
		return false, nil
	}

	// Only increment if under limit
	err = common.RedisIncr(key, 1)
	if err != nil {
		return false, fmt.Errorf("failed to increment notification count: %w", err)
	}

	return true, nil
}

func checkMemoryLimit(userId int, notifyType string) (bool, error) {
	// Ensure cleanup task is started
	cleanupOnce.Do(startCleanupTask)

	notifyLimitMu.Lock()
	defer notifyLimitMu.Unlock()

	now := time.Now()
	rule := getNotificationLimitRule(userId, notifyType, now)
	key := rule.Key

	// Get current limit count or initialize new one
	var currentLimit limitCount
	if value, ok := notifyLimitStore.Load(key); ok {
		currentLimit = value.(limitCount)
		// Check if the entry has expired
		if now.Sub(currentLimit.Timestamp) >= rule.Duration {
			currentLimit = limitCount{Count: 0, Timestamp: now, Duration: rule.Duration}
		}
	} else {
		currentLimit = limitCount{Count: 0, Timestamp: now, Duration: rule.Duration}
	}

	// Increment count
	currentLimit.Count++

	// Store updated count
	notifyLimitStore.Store(key, currentLimit)

	return currentLimit.Count <= rule.Limit, nil
}
