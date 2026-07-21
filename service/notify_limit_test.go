package service

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

func TestQuotaExceedNotificationLimitUsesRolling24HourWindow(t *testing.T) {
	now := time.Date(2026, 7, 21, 15, 30, 0, 0, time.Local)
	rule := getNotificationLimitRule(123, dto.NotifyTypeQuotaExceed, now)

	if rule.Key != "123:quota_exceed" {
		t.Fatalf("expected stable quota notification key, got %q", rule.Key)
	}
	if rule.Limit != 1 {
		t.Fatalf("expected one quota notification per window, got %d", rule.Limit)
	}
	if rule.Duration != 24*time.Hour {
		t.Fatalf("expected 24 hour quota notification window, got %s", rule.Duration)
	}

	oneHourLater := getNotificationLimitRule(123, dto.NotifyTypeQuotaExceed, now.Add(time.Hour))
	if oneHourLater.Key != rule.Key {
		t.Fatalf("quota notification key changed at the hour boundary: %q != %q", oneHourLater.Key, rule.Key)
	}
}

func TestOtherNotificationLimitKeepsExistingHourlyRule(t *testing.T) {
	now := time.Date(2026, 7, 21, 15, 30, 0, 0, time.Local)
	rule := getNotificationLimitRule(123, dto.NotifyTypeChannelUpdate, now)

	if rule.Key != "123:channel_update:2026072115" {
		t.Fatalf("expected existing hourly notification key, got %q", rule.Key)
	}
	if rule.Duration != getDuration() {
		t.Fatalf("expected existing notification duration %s, got %s", getDuration(), rule.Duration)
	}
}

func TestQuotaExceedMemoryLimitAllowsOnlyOneConcurrentNotification(t *testing.T) {
	previousRedisEnabled := common.RedisEnabled
	common.RedisEnabled = false
	t.Cleanup(func() { common.RedisEnabled = previousRedisEnabled })

	const userID = 987654
	rule := getNotificationLimitRule(userID, dto.NotifyTypeQuotaExceed, time.Now())
	notifyLimitStore.Delete(rule.Key)
	t.Cleanup(func() { notifyLimitStore.Delete(rule.Key) })

	const workers = 32
	start := make(chan struct{})
	var allowed atomic.Int32
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			<-start
			canSend, err := CheckNotificationLimit(userID, dto.NotifyTypeQuotaExceed)
			if err != nil {
				t.Errorf("notification limit check failed: %v", err)
				return
			}
			if canSend {
				allowed.Add(1)
			}
		}()
	}
	close(start)
	wg.Wait()

	if got := allowed.Load(); got != 1 {
		t.Fatalf("expected exactly one allowed quota notification, got %d", got)
	}
}
