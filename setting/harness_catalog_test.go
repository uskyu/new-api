package setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// textPoolFixture is the two text pools plus the two video billing modes. A pool
// carries its own labels; the models it serves are derived from its channels at
// request time, so nothing here lists model names.
func textPoolFixture() []HarnessCatalogTier {
	return []HarnessCatalogTier{
		{ID: "economy", DisplayName: "经济档", Kind: HarnessCatalogKindText},
		{ID: "enterprise", DisplayName: "企业档", Kind: HarnessCatalogKindText},
		{ID: "video-per-call", DisplayName: "视频按次", Kind: HarnessCatalogKindVideo, Billing: HarnessCatalogBillingPerCall},
		{ID: "video-per-second", DisplayName: "视频按秒", Kind: HarnessCatalogKindVideo, Billing: HarnessCatalogBillingPerSecond},
	}
}

func TestNormalizeHarnessCatalogKind(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		wantKind  string
		wantValid bool
	}{
		{name: "empty defaults to text", value: "", wantKind: HarnessCatalogKindText, wantValid: true},
		{name: "text", value: "text", wantKind: HarnessCatalogKindText, wantValid: true},
		{name: "text uppercase", value: "TEXT", wantKind: HarnessCatalogKindText, wantValid: true},
		{name: "text padded", value: "  text  ", wantKind: HarnessCatalogKindText, wantValid: true},
		{name: "video", value: "video", wantKind: HarnessCatalogKindVideo, wantValid: true},
		{name: "video uppercase", value: "ViDeO", wantKind: HarnessCatalogKindVideo, wantValid: true},
		{name: "unknown label is rejected", value: "audio", wantKind: "", wantValid: false},
		{name: "whitespace only is text", value: "   ", wantKind: HarnessCatalogKindText, wantValid: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			kind, valid := NormalizeHarnessCatalogKind(test.value)
			assert.Equal(t, test.wantKind, kind)
			assert.Equal(t, test.wantValid, valid)
		})
	}
}

func TestNormalizeHarnessCatalogBilling(t *testing.T) {
	tests := []struct {
		name        string
		kind        string
		billing     string
		wantBilling string
		wantValid   bool
	}{
		{
			name: "text pool declares no billing", kind: HarnessCatalogKindText, billing: "",
			wantBilling: "", wantValid: true,
		},
		{
			name: "text pool must not declare a billing mode", kind: HarnessCatalogKindText, billing: "per_call",
			wantBilling: "", wantValid: false,
		},
		{
			name: "video per call", kind: HarnessCatalogKindVideo, billing: "per_call",
			wantBilling: HarnessCatalogBillingPerCall, wantValid: true,
		},
		{
			name: "video per second", kind: HarnessCatalogKindVideo, billing: "per_second",
			wantBilling: HarnessCatalogBillingPerSecond, wantValid: true,
		},
		{
			name: "video uppercase", kind: HarnessCatalogKindVideo, billing: "PER_SECOND",
			wantBilling: HarnessCatalogBillingPerSecond, wantValid: true,
		},
		{
			name: "video must declare a billing mode", kind: HarnessCatalogKindVideo, billing: "",
			wantBilling: "", wantValid: false,
		},
		{
			name: "video with an unknown billing mode", kind: HarnessCatalogKindVideo, billing: "per_token",
			wantBilling: "", wantValid: false,
		},
		{
			name: "unknown kind rejects any billing", kind: "audio", billing: "per_call",
			wantBilling: "", wantValid: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			billing, valid := NormalizeHarnessCatalogBilling(test.kind, test.billing)
			assert.Equal(t, test.wantBilling, billing)
			assert.Equal(t, test.wantValid, valid)
		})
	}
}

func TestHarnessCatalogConfigResourcePoolIDs(t *testing.T) {
	config := HarnessCatalogConfig{Tiers: textPoolFixture()}

	assert.Equal(t, []string{"economy", "enterprise", "video-per-call", "video-per-second"},
		config.ResourcePoolIDs(), "catalog order is preserved")

	assert.True(t, config.IsResourcePool("economy"))
	assert.True(t, config.IsResourcePool("video-per-second"))
	assert.False(t, config.IsResourcePool("global"), "a pool may be missing from the catalog")
	assert.False(t, config.IsResourcePool(""), "an empty id is not a pool")

	empty := HarnessCatalogConfig{}
	assert.Empty(t, empty.ResourcePoolIDs())
	assert.False(t, empty.IsResourcePool("economy"))
}

func TestHarnessCatalogConfigDefaultResourcePool(t *testing.T) {
	tests := []struct {
		name        string
		defaultTier string
		tiers       []HarnessCatalogTier
		want        string
	}{
		{
			name:        "configured default wins",
			defaultTier: "enterprise",
			tiers:       textPoolFixture(),
			want:        "enterprise",
		},
		{
			name:        "unset default falls back to the first pool",
			defaultTier: "",
			tiers:       textPoolFixture(),
			want:        "economy",
		},
		{
			name:        "default naming a removed pool falls back to the first pool",
			defaultTier: "retired-pool",
			tiers:       textPoolFixture(),
			want:        "economy",
		},
		{
			name:        "no pools yields no default",
			defaultTier: "economy",
			tiers:       nil,
			want:        "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := HarnessCatalogConfig{Tiers: test.tiers, DefaultTier: test.defaultTier}
			assert.Equal(t, test.want, config.DefaultResourcePool())
		})
	}
}

func TestHarnessCatalogConfigTextResourcePoolIDs(t *testing.T) {
	config := HarnessCatalogConfig{Tiers: textPoolFixture()}

	assert.Equal(t, []string{"economy", "enterprise"}, config.TextResourcePoolIDs(),
		"only text pools are eligible for synchronous text routing")

	onlyVideo := HarnessCatalogConfig{Tiers: []HarnessCatalogTier{
		{ID: "video-per-call", Kind: HarnessCatalogKindVideo, Billing: HarnessCatalogBillingPerCall},
	}}
	assert.Empty(t, onlyVideo.TextResourcePoolIDs())
}

// A label pair that cannot describe a real pool is dropped rather than published,
// so the platform never has to interpret an incoherent tag.
func TestSanitizeHarnessCatalogTiersDropsIncoherentLabels(t *testing.T) {
	sanitized := sanitizeHarnessCatalogTiers([]HarnessCatalogTier{
		{ID: "economy", DisplayName: "经济档", Kind: "text"},
		{ID: "economy", DisplayName: "duplicate id", Kind: "text"},
		{ID: "", DisplayName: "no id", Kind: "text"},
		{ID: "audio", DisplayName: "unknown kind", Kind: "audio"},
		{ID: "text-with-billing", DisplayName: "text cannot bill per call", Kind: "text", Billing: "per_call"},
		{ID: "video-no-billing", DisplayName: "video needs a billing mode", Kind: "video"},
		{ID: "video-bad-billing", DisplayName: "unknown billing", Kind: "video", Billing: "per_token"},
		{ID: "video-per-call", DisplayName: "视频按次", Kind: "VIDEO", Billing: "PER_CALL"},
		{ID: "unlabeled", DisplayName: "", Kind: ""},
	})

	ids := make([]string, 0, len(sanitized))
	for _, tier := range sanitized {
		ids = append(ids, tier.ID)
	}
	assert.Equal(t, []string{"economy", "video-per-call", "unlabeled"}, ids)

	byID := make(map[string]HarnessCatalogTier, len(sanitized))
	for _, tier := range sanitized {
		byID[tier.ID] = tier
	}
	assert.Equal(t, HarnessCatalogKindText, byID["economy"].Kind)
	assert.Equal(t, HarnessCatalogBillingPerCall, byID["video-per-call"].Billing,
		"labels are canonicalised to lower case")
	assert.Equal(t, "unlabeled", byID["unlabeled"].DisplayName,
		"an empty display name falls back to the id")
	assert.Equal(t, HarnessCatalogKindText, byID["unlabeled"].Kind,
		"an unlabeled pool is a text pool")
}

// The shipped defaults must cover the full offering shape: two text pools and
// both video billing modes, with every label already coherent.
func TestDefaultHarnessCatalogTiersCoverTheOffering(t *testing.T) {
	tiers := defaultHarnessCatalogTiers()
	assert.Len(t, tiers, 4)

	byID := make(map[string]HarnessCatalogTier, len(tiers))
	for _, tier := range tiers {
		byID[tier.ID] = tier
		kind, ok := NormalizeHarnessCatalogKind(tier.Kind)
		assert.Truef(t, ok, "default tier %q has an invalid kind", tier.ID)
		_, billingOk := NormalizeHarnessCatalogBilling(kind, tier.Billing)
		assert.Truef(t, billingOk, "default tier %q has an incoherent label pair", tier.ID)
	}

	assert.Equal(t, HarnessCatalogKindText, byID["economy"].Kind)
	assert.Equal(t, HarnessCatalogKindText, byID["enterprise"].Kind)
	assert.Equal(t, HarnessCatalogBillingPerCall, byID["video-per-call"].Billing)
	assert.Equal(t, HarnessCatalogBillingPerSecond, byID["video-per-second"].Billing)

	config := HarnessCatalogConfig{Tiers: tiers}
	assert.Equal(t, defaultHarnessCatalogDefaultTier, config.DefaultResourcePool(),
		"the shipped default pool must be one of the shipped pools")
}
