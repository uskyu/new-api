package service

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestIOCopyBytesGracefullyFiltersManagedHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Writer.Header().Set(common.RequestIdKey, "local-request-id")

	src := &http.Response{
		StatusCode: http.StatusAccepted,
		Header: http.Header{
			"Content-Length":      {"999"},
			common.RequestIdKey:   {"upstream-request-id"},
			"X-Upstream-Response": {"copied"},
		},
	}

	IOCopyBytesGracefully(c, src, []byte("body"))

	require.Equal(t, http.StatusAccepted, recorder.Code)
	require.Equal(t, "4", recorder.Header().Get("Content-Length"))
	require.Equal(t, "local-request-id", recorder.Header().Get(common.RequestIdKey))
	require.Equal(t, "copied", recorder.Header().Get("X-Upstream-Response"))
	require.Equal(t, "upstream-request-id", c.GetString(common.UpstreamRequestIdKey))
}
