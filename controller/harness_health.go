package controller

import (
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// harnessUpstreamBaseline is the New API upstream commit the platform tracks for
// feature-compatibility questions. It is a fixed constant from the contract: the
// gateway reports the agreed baseline, never its own build commit.
const harnessUpstreamBaseline = "45fbb149"

// harnessInterfaceVersion is the frozen interface version of this contract.
const harnessInterfaceVersion = "v2"

// harnessHealthTimeFormat is RFC 3339 in UTC with exactly millisecond precision,
// as the contract fixes it. Fractional-second layout "000" keeps the trailing
// zeros instead of trimming them the way "999" would.
const harnessHealthTimeFormat = "2006-01-02T15:04:05.000Z"

// harnessHealthResponse is the B4 body. The contract fixes this literal shape,
// so it is written directly rather than wrapped in common.ApiSuccess: the
// platform polls liveness and must not receive a success envelope for a "down"
// gateway.
type harnessHealthResponse struct {
	Status           string `json:"status"`
	InterfaceVersion string `json:"interface_version"`
	UpstreamBaseline string `json:"upstream_baseline"`
	Time             string `json:"time"`
}

// GetHarnessHealth serves B4: liveness, the interface version, and the tracked
// upstream baseline.
//
// The two failure modes are deliberately different statuses. "degraded" is a 200
// because the gateway itself is healthy and the platform can keep its own
// bookkeeping going while reporting a servicing problem to operators; "down" is a
// 503 because nothing can be served at all. Both still return a complete body, so
// the platform can read the reason from the status field rather than from an
// error envelope.
//
// model.PingDB is the repository's existing database liveness check; it is
// throttled internally (it re-pings at most every ten seconds), so a polling
// platform does not turn health checks into a database load. An enabled ability is
// required for "ok" because a gateway with no enabled channel can accept a request
// but never serve it.
func GetHarnessHealth(c *gin.Context) {
	response := harnessHealthResponse{
		Status:           "ok",
		InterfaceVersion: harnessInterfaceVersion,
		UpstreamBaseline: harnessUpstreamBaseline,
		Time:             time.Now().UTC().Format(harnessHealthTimeFormat),
	}

	// The nil check comes first: model.PingDB dereferences model.DB, so an
	// uninitialized database would panic instead of reporting "down".
	dbReachable := model.DB != nil && model.PingDB() == nil

	status := http.StatusOK
	switch {
	case !dbReachable:
		response.Status = "down"
		status = http.StatusServiceUnavailable
	case len(model.GetAllEnableAbilities()) == 0:
		response.Status = "degraded"
	}

	c.JSON(status, response)
}
