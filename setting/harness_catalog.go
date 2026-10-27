package setting

import (
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
)

// HarnessCatalogOptionKey is the option row holding the Harness tier catalog.
const HarnessCatalogOptionKey = "HarnessCatalog"

// Model categories a resource pool can advertise, and the video billing modes.
//
// A pool carries exactly one kind, because the platform uses it to choose the
// calling convention: text pools serve synchronous chat/completions, video pools
// serve asynchronous tasks. Billing only means something for video, where the
// same kind of model is metered either per generated clip or per second of
// output; a text pool leaves it empty.
const (
	HarnessCatalogKindText  = "text"
	HarnessCatalogKindVideo = "video"

	HarnessCatalogBillingPerCall   = "per_call"
	HarnessCatalogBillingPerSecond = "per_second"
)

// HarnessCatalogTier is one platform-visible resource pool (a "档位").
//
// A tier is a routing policy, not a model, and it never maps onto one fixed
// upstream model: the pool it names is an Intelligent Routing group, whose
// channel priority decides the candidates, and whose channels jointly determine
// the real model ids the pool can serve. Those model ids are derived at request
// time from the group's channels rather than stored here, so the catalog can
// never advertise a model the pool cannot actually route.
//
// Kind and Billing are labels the operator sets on the pool, not facts derived
// from its contents: a pool is advertised even while it has no models yet, so the
// platform can see the full shape of the offering before the channels exist. The
// labels cost a consistency check — see validateHarnessCatalogTierTag.
type HarnessCatalogTier struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Kind        string `json:"kind,omitempty"`
	Billing     string `json:"billing,omitempty"`
}

// HarnessCatalogConfig is the persisted catalog snapshot. HarnessCatalogVersion
// and Fingerprint ride in the same option row as Tiers because the version is
// only meaningful together with the content fingerprint it was issued for.
//
// HarnessCatalogVersion is monotonic and advances only when the rendered catalog
// content actually changes. Comparing content fingerprints at read time is what
// keeps it correct: model/channel/pricing writes happen on more than twenty
// paths in this repository, so hooking every one of them would eventually miss
// one.
//
// DefaultTier names the pool a Harness call uses when it sends no routing
// header. It is configurable because which pool should be the default is an
// operating decision, not a property of the protocol.
type HarnessCatalogConfig struct {
	Tiers                 []HarnessCatalogTier `json:"tiers"`
	DefaultTier           string               `json:"default_tier,omitempty"`
	HarnessCatalogVersion int64                `json:"catalog_version"`
	Fingerprint           string               `json:"fingerprint"`
}

// defaultHarnessCatalogDefaultTier is the pool a Harness call falls back to when
// the operator has not chosen one.
const defaultHarnessCatalogDefaultTier = "economy"

// defaultHarnessCatalogTiers advertises the four pools: two text pools and the
// two video billing modes. The video pools carry no channels yet, and are listed
// anyway so the platform can integrate against the final shape.
func defaultHarnessCatalogTiers() []HarnessCatalogTier {
	return []HarnessCatalogTier{
		{ID: "economy", DisplayName: "经济档", Kind: HarnessCatalogKindText},
		{ID: "enterprise", DisplayName: "企业档", Kind: HarnessCatalogKindText},
		{ID: "video-per-call", DisplayName: "视频按次", Kind: HarnessCatalogKindVideo, Billing: HarnessCatalogBillingPerCall},
		{ID: "video-per-second", DisplayName: "视频按秒", Kind: HarnessCatalogKindVideo, Billing: HarnessCatalogBillingPerSecond},
	}
}

// NormalizeHarnessCatalogKind returns the canonical kind for a pool label, and
// reports whether the label is one the contract allows. A pool with no label is
// treated as a text pool: it is the shape every existing pool already has.
func NormalizeHarnessCatalogKind(value string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", HarnessCatalogKindText:
		return HarnessCatalogKindText, true
	case HarnessCatalogKindVideo:
		return HarnessCatalogKindVideo, true
	default:
		return "", false
	}
}

// NormalizeHarnessCatalogBilling returns the canonical billing mode for a pool
// label, and reports whether the pair is coherent: only a video pool may declare
// one, and a video pool must declare exactly one so the platform knows how the
// model is metered.
func NormalizeHarnessCatalogBilling(kind, billing string) (string, bool) {
	normalized := strings.ToLower(strings.TrimSpace(billing))
	switch kind {
	case HarnessCatalogKindText:
		return "", normalized == ""
	case HarnessCatalogKindVideo:
		switch normalized {
		case HarnessCatalogBillingPerCall, HarnessCatalogBillingPerSecond:
			return normalized, true
		default:
			return "", false
		}
	default:
		return "", false
	}
}

var (
	harnessCatalogMu sync.RWMutex
	// harnessCatalogConfig is the in-memory snapshot of the option row.
	harnessCatalogConfig = HarnessCatalogConfig{
		Tiers:                 defaultHarnessCatalogTiers(),
		HarnessCatalogVersion: 1,
	}
	// harnessCatalogRaw is the option value the snapshot was built from, so a
	// value written by the admin option API is picked up without a hook in
	// model/option.go.
	harnessCatalogRaw string
	// harnessCatalogVersionIssued records that the current version already
	// describes some fingerprint, so the next content change must advance it.
	harnessCatalogVersionIssued bool
)

// GetHarnessCatalogConfig returns a deep copy of the current catalog config,
// first adopting the option row when it changed since the last read.
func GetHarnessCatalogConfig() HarnessCatalogConfig {
	raw := harnessCatalogRawOption()

	harnessCatalogMu.RLock()
	if raw == harnessCatalogRaw {
		config := cloneHarnessCatalogConfig(harnessCatalogConfig)
		harnessCatalogMu.RUnlock()
		return config
	}
	harnessCatalogMu.RUnlock()

	harnessCatalogMu.Lock()
	defer harnessCatalogMu.Unlock()
	harnessCatalogApplyOptionLocked(raw)
	return cloneHarnessCatalogConfig(harnessCatalogConfig)
}

// AdvanceHarnessCatalogVersion returns the version that now describes
// fingerprint, advancing the counter when the content changed since the version
// was issued. The second return value reports whether the snapshot changed and
// must be persisted. The very first fingerprint is adopted without a bump: there
// is no earlier content to call a change, and starting at the configured version
// keeps the counter honest.
func AdvanceHarnessCatalogVersion(fingerprint string) (HarnessCatalogConfig, bool) {
	raw := harnessCatalogRawOption()

	harnessCatalogMu.Lock()
	defer harnessCatalogMu.Unlock()
	harnessCatalogApplyOptionLocked(raw)

	if harnessCatalogConfig.Fingerprint == fingerprint {
		return cloneHarnessCatalogConfig(harnessCatalogConfig), false
	}
	harnessCatalogConfig.Fingerprint = fingerprint
	if harnessCatalogVersionIssued {
		harnessCatalogConfig.HarnessCatalogVersion++
	}
	harnessCatalogVersionIssued = true
	return cloneHarnessCatalogConfig(harnessCatalogConfig), true
}

// HarnessCatalogConfigJSON serializes a snapshot for the option row.
func HarnessCatalogConfigJSON(config HarnessCatalogConfig) (string, error) {
	data, err := common.Marshal(config)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// harnessCatalogRawOption reads the option value without holding the catalog
// lock, so the common option lock is never nested under it.
func harnessCatalogRawOption() string {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()
	return common.OptionMap[HarnessCatalogOptionKey]
}

// harnessCatalogApplyOptionLocked adopts raw when it differs from the value the
// snapshot was built from. A malformed or empty option falls back to the default
// tiers instead of withdrawing the catalog, and the version never regresses.
func harnessCatalogApplyOptionLocked(raw string) {
	if raw == harnessCatalogRaw {
		return
	}
	harnessCatalogRaw = raw

	config := HarnessCatalogConfig{
		Tiers:                 defaultHarnessCatalogTiers(),
		DefaultTier:           defaultHarnessCatalogDefaultTier,
		HarnessCatalogVersion: harnessCatalogConfig.HarnessCatalogVersion,
		// The fingerprint is inherited rather than cleared: an operator editing
		// only display names or tier order rewrites this option without one, and
		// dropping it would advance the version for content that did not change.
		// The rendered content is compared against the inherited value below,
		// which still detects a real change.
		Fingerprint: harnessCatalogConfig.Fingerprint,
	}
	if strings.TrimSpace(raw) != "" {
		var stored HarnessCatalogConfig
		if err := common.UnmarshalJsonStr(raw, &stored); err != nil {
			common.SysError(common.LogText("failed to parse HarnessCatalog option: %s", err.Error()))
		} else {
			if tiers := sanitizeHarnessCatalogTiers(stored.Tiers); len(tiers) > 0 {
				config.Tiers = tiers
			}
			if defaultTier := strings.TrimSpace(stored.DefaultTier); defaultTier != "" {
				config.DefaultTier = defaultTier
			}
			if stored.HarnessCatalogVersion > config.HarnessCatalogVersion {
				config.HarnessCatalogVersion = stored.HarnessCatalogVersion
			}
			// A stored fingerprint means a version was already issued for some
			// content, so the next different fingerprint is a real change.
			if stored.Fingerprint != "" {
				config.Fingerprint = stored.Fingerprint
				harnessCatalogVersionIssued = true
			}
		}
	}
	if config.HarnessCatalogVersion < 1 {
		config.HarnessCatalogVersion = 1
	}
	harnessCatalogConfig = config
}

// sanitizeHarnessCatalogTiers drops entries the platform could not address: an
// empty or duplicated id, or a label pair that cannot describe a real pool. A
// rejected label is dropped rather than published, so the platform never has to
// guess what an incoherent tag means. Whether the pool can actually serve
// anything is not decided here, because that depends on live channel
// configuration and is resolved when the catalog is rendered.
func sanitizeHarnessCatalogTiers(tiers []HarnessCatalogTier) []HarnessCatalogTier {
	seen := make(map[string]struct{}, len(tiers))
	sanitized := make([]HarnessCatalogTier, 0, len(tiers))
	for _, tier := range tiers {
		id := strings.TrimSpace(tier.ID)
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		kind, ok := NormalizeHarnessCatalogKind(tier.Kind)
		if !ok {
			continue
		}
		billing, ok := NormalizeHarnessCatalogBilling(kind, tier.Billing)
		if !ok {
			continue
		}
		seen[id] = struct{}{}
		displayName := strings.TrimSpace(tier.DisplayName)
		if displayName == "" {
			displayName = id
		}
		sanitized = append(sanitized, HarnessCatalogTier{
			ID:          id,
			DisplayName: displayName,
			Kind:        kind,
			Billing:     billing,
		})
	}
	return sanitized
}

// ResourcePoolIDs lists the pool ids the catalog currently advertises, in
// catalog order, and reports whether a specific pool is still part of it.
func (config HarnessCatalogConfig) ResourcePoolIDs() []string {
	ids := make([]string, 0, len(config.Tiers))
	for _, tier := range config.Tiers {
		ids = append(ids, tier.ID)
	}
	return ids
}

// IsResourcePool reports whether poolID is one the catalog advertises.
func (config HarnessCatalogConfig) IsResourcePool(poolID string) bool {
	for _, tier := range config.Tiers {
		if tier.ID == poolID {
			return true
		}
	}
	return false
}

// DefaultResourcePool returns the pool a Harness call uses when it sends no
// routing header. An operator-set default wins when it names a pool that is
// still advertised; otherwise the first advertised pool is used, so the gateway
// never falls back to a pool the catalog does not offer.
func (config HarnessCatalogConfig) DefaultResourcePool() string {
	if config.IsResourcePool(config.DefaultTier) {
		return config.DefaultTier
	}
	for _, tier := range config.Tiers {
		return tier.ID
	}
	return ""
}

func cloneHarnessCatalogConfig(config HarnessCatalogConfig) HarnessCatalogConfig {
	tiers := make([]HarnessCatalogTier, len(config.Tiers))
	copy(tiers, config.Tiers)
	return HarnessCatalogConfig{
		Tiers:                 tiers,
		DefaultTier:           config.DefaultTier,
		HarnessCatalogVersion: config.HarnessCatalogVersion,
		Fingerprint:           config.Fingerprint,
	}
}

// TextResourcePoolIDs lists only the pools the platform uses for synchronous
// text calls, in catalog order, so routing never applies a video pool to a text
// request.
func (config HarnessCatalogConfig) TextResourcePoolIDs() []string {
	ids := make([]string, 0, len(config.Tiers))
	for _, tier := range config.Tiers {
		if tier.Kind == HarnessCatalogKindText {
			ids = append(ids, tier.ID)
		}
	}
	return ids
}
