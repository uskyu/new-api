package setting

import (
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
)

// HarnessCatalogOptionKey is the option row holding the Harness tier catalog.
const HarnessCatalogOptionKey = "HarnessCatalog"

// HarnessCatalogTier is one platform-visible tier. Models lists the gateway
// model names that back the tier: a tier is advertised only while at least one
// of those names is present in model.GetPricing(), so an unroutable tier is
// never offered.
type HarnessCatalogTier struct {
	ID          string   `json:"id"`
	DisplayName string   `json:"display_name"`
	Models      []string `json:"models"`
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
type HarnessCatalogConfig struct {
	Tiers                 []HarnessCatalogTier `json:"tiers"`
	HarnessCatalogVersion int64                `json:"catalog_version"`
	Fingerprint           string               `json:"fingerprint"`
}

// defaultHarnessCatalogTiers advertises the two agreed tiers. Each tier is
// backed by a model of the same name, which is how the gateway resolves it
// through the ordinary ability / model-mapping path.
func defaultHarnessCatalogTiers() []HarnessCatalogTier {
	return []HarnessCatalogTier{
		{ID: "economy", DisplayName: "经济档", Models: []string{"economy"}},
		{ID: "enterprise", DisplayName: "企业档", Models: []string{"enterprise"}},
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
// empty or duplicated id, or a tier with no backing model.
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
		models := make([]string, 0, len(tier.Models))
		for _, modelName := range tier.Models {
			if name := strings.TrimSpace(modelName); name != "" {
				models = append(models, name)
			}
		}
		if len(models) == 0 {
			continue
		}
		seen[id] = struct{}{}
		displayName := strings.TrimSpace(tier.DisplayName)
		if displayName == "" {
			displayName = id
		}
		sanitized = append(sanitized, HarnessCatalogTier{ID: id, DisplayName: displayName, Models: models})
	}
	return sanitized
}

func cloneHarnessCatalogConfig(config HarnessCatalogConfig) HarnessCatalogConfig {
	tiers := make([]HarnessCatalogTier, len(config.Tiers))
	for i, tier := range config.Tiers {
		tiers[i] = HarnessCatalogTier{
			ID:          tier.ID,
			DisplayName: tier.DisplayName,
			Models:      append([]string(nil), tier.Models...),
		}
	}
	return HarnessCatalogConfig{
		Tiers:                 tiers,
		HarnessCatalogVersion: config.HarnessCatalogVersion,
		Fingerprint:           config.Fingerprint,
	}
}
