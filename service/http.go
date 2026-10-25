package service

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/setting"

	"github.com/gin-gonic/gin"
)

func CloseResponseBodyGracefully(httpResponse *http.Response) {
	if httpResponse == nil || httpResponse.Body == nil {
		return
	}
	err := httpResponse.Body.Close()
	if err != nil {
		common.SysError(common.LogText("failed to close response body: %s", err.Error()))
	}
}

// newAPIRequestIdHeader is the gateway's own request id header. It is declared
// here as well as in middleware because service must not import middleware (that
// would be an import cycle), and the two must stay identical: this one guards the
// write path against an upstream overwrite, the other names the header the
// engine stamps on every response.
const newAPIRequestIdHeader = "X-NewAPI-Request-Id"

// ShouldCopyUpstreamHeader checks whether a given upstream response header
// should be copied to the client response. It returns false for Content-Length
// (managed separately) and X-Oneapi-Request-Id (to preserve the local instance
// ID). When the upstream header is X-Oneapi-Request-Id, the value is captured
// into the Gin context for later logging.
func isIntelligentRoutingResponseHeader(name string) bool {
	return strings.EqualFold(name, setting.IntelligentRoutingGroupHeader) ||
		strings.EqualFold(name, setting.IntelligentRoutingChannelIDHeader) ||
		strings.EqualFold(name, setting.IntelligentRoutingGroupRatioHeader) ||
		strings.EqualFold(name, setting.IntelligentRoutingTaxRatePercentHeader)
}

func ShouldCopyUpstreamHeader(c *gin.Context, k string, v []string) bool {
	if strings.EqualFold(k, "Content-Length") || isIntelligentRoutingResponseHeader(k) {
		return false
	}
	// The gateway's own request id identifies this call to the platform, so an
	// upstream must never be able to replace it: a hostile or misconfigured
	// upstream echoing the header would make the platform attribute the cost to a
	// request id the gateway never issued.
	if strings.EqualFold(k, newAPIRequestIdHeader) {
		return false
	}
	if strings.EqualFold(k, common.RequestIdKey) {
		if c != nil && len(v) > 0 {
			c.Set(common.UpstreamRequestIdKey, v[0])
		}
		return false
	}
	return true
}

func IOCopyBytesGracefully(c *gin.Context, src *http.Response, data []byte) {
	if c.Writer == nil {
		return
	}

	body := io.NopCloser(bytes.NewBuffer(data))

	// We shouldn't set the header before we parse the response body, because the parse part may fail.
	// And then we will have to send an error response, but in this case, the header has already been set.
	// So the httpClient will be confused by the response.
	// For example, Postman will report error, and we cannot check the response at all.
	if src != nil {
		for k, v := range src.Header {
			if !ShouldCopyUpstreamHeader(c, k, v) {
				continue
			}
			c.Writer.Header().Set(k, v[0])
		}
	}

	// set Content-Length header manually BEFORE calling WriteHeader
	c.Writer.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))

	// Write header with status code (this sends the headers)
	if src != nil {
		c.Writer.WriteHeader(src.StatusCode)
	} else {
		c.Writer.WriteHeader(http.StatusOK)
	}

	_, err := io.Copy(c.Writer, body)
	if err != nil {
		logger.LogError(c, common.LogText("failed to copy response body: %s", err.Error()))
	}
	c.Writer.Flush()
}
