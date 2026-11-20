package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

func TestGlobalAPIRateLimitSeparatesSupportPolling(t *testing.T) {
	previousEnable := common.GlobalApiRateLimitEnable
	previousNum := common.GlobalApiRateLimitNum
	previousDuration := common.GlobalApiRateLimitDuration
	previousRedisEnabled := common.RedisEnabled
	common.GlobalApiRateLimitEnable = true
	common.GlobalApiRateLimitNum = 2
	common.GlobalApiRateLimitDuration = 180
	common.RedisEnabled = false
	t.Cleanup(func() {
		common.GlobalApiRateLimitEnable = previousEnable
		common.GlobalApiRateLimitNum = previousNum
		common.GlobalApiRateLimitDuration = previousDuration
		common.RedisEnabled = previousRedisEnabled
	})

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(GlobalAPIRateLimit())
	router.GET("/api/status", func(c *gin.Context) { c.Status(http.StatusOK) })
	router.GET("/api/support/admin/unread", func(c *gin.Context) { c.Status(http.StatusOK) })

	doRequest := func(path string, remoteAddr string) int {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.RemoteAddr = remoteAddr
		router.ServeHTTP(recorder, request)
		return recorder.Code
	}

	const generalIP = "10.10.10.1:1234"
	if code := doRequest("/api/status", generalIP); code != http.StatusOK {
		t.Fatalf("first general request status = %d", code)
	}
	if code := doRequest("/api/status", generalIP); code != http.StatusOK {
		t.Fatalf("second general request status = %d", code)
	}
	if code := doRequest("/api/status", generalIP); code != http.StatusTooManyRequests {
		t.Fatalf("third general request status = %d, want 429", code)
	}

	const supportIP = "10.10.10.2:1234"
	for i := 0; i < 5; i++ {
		if code := doRequest("/api/support/admin/unread", supportIP); code != http.StatusOK {
			t.Fatalf("support request %d status = %d, want 200", i+1, code)
		}
	}

	// 客服轮询不应消耗普通 API 的额度，也不应被普通限流拦截。
	if code := doRequest("/api/status", supportIP); code != http.StatusOK {
		t.Fatalf("general request after support polling status = %d, want 200", code)
	}
}
