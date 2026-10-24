package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"

	"github.com/gin-gonic/gin"
)

// SetHarnessInternalRouter registers the platform-facing read APIs under
// /internal/v1/gateway. These paths are server-to-server only: they require a
// verified Harness JWT, never a dashboard session or an sk- token.
//
// Route shape note (gin specific): "usage-events:batchGet" is NOT a literal
// segment. gin splits the path at the first ':' anywhere, so the prefix ends at
// ".../usage-events" and ":batchGet" becomes a wildcard parameter whose value
// includes the colon. That means:
//   - the plain list route and the colon route can coexist on this prefix;
//   - a mistyped sibling such as "/usage-eventsXYZ" also matches the colon route,
//     so the verb guard below rejects anything but the exact capture;
//   - adding a second colon route on the same prefix panics at startup
//     ("conflicts with existing wildcard"), so never register another
//     ".../usage-events:<verb>" here.
func SetHarnessInternalRouter(router *gin.Engine) {
	router.Handle("GET", "/internal/v1/gateway/usage-events",
		harnessReadChain(controller.GetHarnessUsageEvents)...)
	router.Handle("POST", "/internal/v1/gateway/usage-events:batchGet",
		harnessBatchGetChain(":batchGet", controller.BatchGetHarnessUsageEvents)...)
	router.Handle("GET", "/internal/v1/gateway/cost-records",
		harnessReadChain(controller.GetHarnessCostRecords)...)
	router.Handle("POST", "/internal/v1/gateway/cost-records:batchGet",
		harnessBatchGetChain(":batchGet", controller.BatchGetHarnessCostRecords)...)
	router.Handle("GET", "/internal/v1/gateway/health",
		harnessReadChain(controller.GetHarnessHealth)...)
}

// harnessReadChain is the auth gate for a plain read route.
func harnessReadChain(handler gin.HandlerFunc) []gin.HandlerFunc {
	return []gin.HandlerFunc{middleware.HarnessJWT(), middleware.RequireHarnessPrincipal(), handler}
}

// harnessBatchGetChain puts the verb guard ahead of auth. A mistyped path must
// answer 404 without disclosing whether credentials would have been accepted, and
// it must not consume an authentication attempt.
func harnessBatchGetChain(wantParam string, handler gin.HandlerFunc) []gin.HandlerFunc {
	return []gin.HandlerFunc{harnessVerbGuard(wantParam), middleware.HarnessJWT(), middleware.RequireHarnessPrincipal(), handler}
}

// harnessVerbGuard rejects a colon-style route whose captured verb is not the
// expected one. gin turns ".../usage-events:batchGet" into the prefix
// ".../usage-events" plus the wildcard ":batchGet", so without this guard any
// single-segment suffix (including a typo such as ".../usage-eventsXYZ") would be
// served by the batchGet handler instead of answering 404.
func harnessVerbGuard(wantParam string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Param("batchGet") != wantParam {
			c.JSON(404, gin.H{"error": gin.H{
				"code":    "REQUEST_NOT_FOUND",
				"message": "unknown gateway operation",
			}})
			c.Abort()
			return
		}
		c.Next()
	}
}
