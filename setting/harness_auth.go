package setting

import (
	"math"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
)

const (
	HarnessJWKSURLOptionKey         = "HarnessJWKSURL"
	HarnessJWTIssuerOptionKey       = "HarnessJWTIssuer"
	HarnessJWTAudienceOptionKey     = "HarnessJWTAudience"
	HarnessSubjectTokenMapOptionKey = "HarnessSubjectTokenMap"

	HarnessJWTDefaultIssuer   = "harness"
	HarnessJWTDefaultAudience = "new-api"
)

// HarnessSubject is the internal service token a verified Harness subject maps
// to. The option accepts either a token id ({"harness-gateway": 12}) or a token
// name ({"harness-gateway": "harness gateway"}).
type HarnessSubject struct {
	TokenID   int
	TokenName string
}

// HarnessAuthConfig is the Harness JWT authentication snapshot. An empty JWKS
// URL disables the feature entirely.
type HarnessAuthConfig struct {
	JWKSURL  string
	Issuer   string
	Audience string
	Subjects map[string]HarnessSubject
}

// Enabled reports whether Harness JWT authentication is configured.
func (c HarnessAuthConfig) Enabled() bool {
	return c.JWKSURL != ""
}

var (
	harnessAuthConfigMu       sync.RWMutex
	harnessAuthConfigSnapshot = HarnessAuthConfig{Issuer: HarnessJWTDefaultIssuer, Audience: HarnessJWTDefaultAudience}
	harnessAuthConfigRawKey   string
)

// GetHarnessAuthConfig returns the current snapshot. The option map is the
// source of truth, but the subject map is parsed once per option change because
// this runs on every relayed request. Callers must treat the result as
// read-only; the snapshot is replaced, never mutated in place.
func GetHarnessAuthConfig() HarnessAuthConfig {
	raw := readHarnessAuthOptions()
	snapshotKey := strings.Join(raw[:], "\x00")

	harnessAuthConfigMu.RLock()
	cached, cachedKey := harnessAuthConfigSnapshot, harnessAuthConfigRawKey
	harnessAuthConfigMu.RUnlock()
	if cachedKey == snapshotKey {
		return cached
	}

	config := HarnessAuthConfig{
		JWKSURL:  strings.TrimSpace(raw[0]),
		Issuer:   strings.TrimSpace(raw[1]),
		Audience: strings.TrimSpace(raw[2]),
		Subjects: parseHarnessSubjectTokenMap(raw[3]),
	}
	if config.Issuer == "" {
		config.Issuer = HarnessJWTDefaultIssuer
	}
	if config.Audience == "" {
		config.Audience = HarnessJWTDefaultAudience
	}

	harnessAuthConfigMu.Lock()
	harnessAuthConfigSnapshot = config
	harnessAuthConfigRawKey = snapshotKey
	harnessAuthConfigMu.Unlock()
	return config
}

func readHarnessAuthOptions() [4]string {
	var raw [4]string
	common.OptionMapRWMutex.RLock()
	if common.OptionMap != nil {
		raw[0] = common.OptionMap[HarnessJWKSURLOptionKey]
		raw[1] = common.OptionMap[HarnessJWTIssuerOptionKey]
		raw[2] = common.OptionMap[HarnessJWTAudienceOptionKey]
		raw[3] = common.OptionMap[HarnessSubjectTokenMapOptionKey]
	}
	common.OptionMapRWMutex.RUnlock()
	return raw
}

// parseHarnessSubjectTokenMap keeps only entries that resolve to a usable token
// reference, so a malformed option denies access instead of widening it.
func parseHarnessSubjectTokenMap(raw string) map[string]HarnessSubject {
	raw = strings.TrimSpace(raw)
	subjects := make(map[string]HarnessSubject)
	if raw == "" {
		return subjects
	}
	var values map[string]any
	if err := common.UnmarshalJsonStr(raw, &values); err != nil {
		return subjects
	}
	for subject, value := range values {
		subject = strings.TrimSpace(subject)
		if subject == "" {
			continue
		}
		switch typed := value.(type) {
		case float64:
			if typed <= 0 || typed > math.MaxInt32 || typed != math.Trunc(typed) {
				continue
			}
			subjects[subject] = HarnessSubject{TokenID: int(typed)}
		case string:
			if name := strings.TrimSpace(typed); name != "" {
				subjects[subject] = HarnessSubject{TokenName: name}
			}
		}
	}
	return subjects
}
