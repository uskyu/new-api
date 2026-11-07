package setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// textPoolFixture is the shipped four pools: the general chat pool, the single
// video pool, the reserved audio pool, and the plan pool. A pool carries its own
// labels; the models it serves are derived from its channels at request time, so
// nothing here lists model names.
func textPoolFixture() []HarnessCatalogTier {
	return []HarnessCatalogTier{
		{ID: "chat", DisplayName: "通用对话", Kind: HarnessCatalogKindText},
		{ID: "video", DisplayName: "视频", Kind: HarnessCatalogKindVideo},
		{ID: "image", DisplayName: "图片生成", Kind: HarnessCatalogKindImage},
		{ID: "audio", DisplayName: "语音", Kind: HarnessCatalogKindAudio},
		{ID: "plan", DisplayName: "畅用计划", Kind: HarnessCatalogKindVideo, Billing: HarnessCatalogBillingPerCall},
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
		{name: "audio", value: "audio", wantKind: HarnessCatalogKindAudio, wantValid: true},
		{name: "audio uppercase padded", value: "  AUDIO  ", wantKind: HarnessCatalogKindAudio, wantValid: true},
		{name: "image", value: "image", wantKind: HarnessCatalogKindImage, wantValid: true},
		{name: "image uppercase padded", value: "  IMAGE  ", wantKind: HarnessCatalogKindImage, wantValid: true},
		{name: "unknown label is rejected", value: "embedding", wantKind: "", wantValid: false},
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
			name: "audio pool declares no billing", kind: HarnessCatalogKindAudio, billing: "",
			wantBilling: "", wantValid: true,
		},
		{
			name: "audio pool must not declare a billing mode", kind: HarnessCatalogKindAudio, billing: "per_call",
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
			name: "video may leave billing empty", kind: HarnessCatalogKindVideo, billing: "",
			wantBilling: "", wantValid: true,
		},
		{
			name: "video with an unknown billing mode", kind: HarnessCatalogKindVideo, billing: "per_token",
			wantBilling: "", wantValid: false,
		},
		{
			name: "unknown kind rejects any billing", kind: "image", billing: "per_call",
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

	assert.Equal(t, []string{"chat", "video", "image", "audio", "plan"},
		config.ResourcePoolIDs(), "catalog order is preserved")

	assert.True(t, config.IsResourcePool("chat"))
	assert.True(t, config.IsResourcePool("plan"))
	assert.False(t, config.IsResourcePool("global"), "a pool may be missing from the catalog")
	assert.False(t, config.IsResourcePool(""), "an empty id is not a pool")

	empty := HarnessCatalogConfig{}
	assert.Empty(t, empty.ResourcePoolIDs())
	assert.False(t, empty.IsResourcePool("chat"))
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
			defaultTier: "plan",
			tiers:       textPoolFixture(),
			want:        "plan",
		},
		{
			name:        "unset default falls back to the first pool",
			defaultTier: "",
			tiers:       textPoolFixture(),
			want:        "chat",
		},
		{
			name:        "default naming a removed pool falls back to the first pool",
			defaultTier: "retired-pool",
			tiers:       textPoolFixture(),
			want:        "chat",
		},
		{
			name:        "no pools yields no default",
			defaultTier: "chat",
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

	assert.Equal(t, []string{"chat"}, config.TextResourcePoolIDs(),
		"only text pools are eligible for synchronous text routing")

	onlyVideo := HarnessCatalogConfig{Tiers: []HarnessCatalogTier{
		{ID: "video", Kind: HarnessCatalogKindVideo},
		{ID: "plan", Kind: HarnessCatalogKindVideo, Billing: HarnessCatalogBillingPerCall},
		{ID: "audio", Kind: HarnessCatalogKindAudio},
	}}
	assert.Empty(t, onlyVideo.TextResourcePoolIDs())
}

// A label pair that cannot describe a real pool is dropped rather than published,
// so the platform never has to interpret an incoherent tag.
func TestSanitizeHarnessCatalogTiersDropsIncoherentLabels(t *testing.T) {
	sanitized := sanitizeHarnessCatalogTiers([]HarnessCatalogTier{
		{ID: "chat", DisplayName: "通用对话", Kind: "text"},
		{ID: "chat", DisplayName: "duplicate id", Kind: "text"},
		{ID: "", DisplayName: "no id", Kind: "text"},
		{ID: "embedding", DisplayName: "unknown kind", Kind: "embedding"},
		{ID: "text-with-billing", DisplayName: "text cannot bill per call", Kind: "text", Billing: "per_call"},
		{ID: "audio-with-billing", DisplayName: "audio cannot bill per call", Kind: "audio", Billing: "per_call"},
		{ID: "video-bad-billing", DisplayName: "unknown billing", Kind: "video", Billing: "per_token"},
		{ID: "video", DisplayName: "视频", Kind: "video"},
		{ID: "image", DisplayName: "图片生成", Kind: "IMAGE"},
		{ID: "audio", DisplayName: "语音", Kind: "AUDIO"},
		{ID: "plan", DisplayName: "畅用计划", Kind: "VIDEO", Billing: "PER_CALL"},
		{ID: "unlabeled", DisplayName: "", Kind: ""},
	})

	ids := make([]string, 0, len(sanitized))
	for _, tier := range sanitized {
		ids = append(ids, tier.ID)
	}
	assert.Equal(t, []string{"chat", "video", "image", "audio", "plan", "unlabeled"}, ids)

	byID := make(map[string]HarnessCatalogTier, len(sanitized))
	for _, tier := range sanitized {
		byID[tier.ID] = tier
	}
	assert.Equal(t, HarnessCatalogKindText, byID["chat"].Kind)
	assert.Equal(t, HarnessCatalogBillingPerCall, byID["plan"].Billing,
		"labels are canonicalised to lower case")
	assert.Equal(t, HarnessCatalogKindVideo, byID["video"].Kind)
	assert.Empty(t, byID["video"].Billing, "a video pool may leave billing unset")
	assert.Equal(t, HarnessCatalogKindImage, byID["image"].Kind)
	assert.Empty(t, byID["image"].Billing, "an image pool carries no billing label")
	assert.Equal(t, HarnessCatalogKindAudio, byID["audio"].Kind)
	assert.Empty(t, byID["audio"].Billing, "an audio pool carries no billing label")
	assert.Equal(t, "unlabeled", byID["unlabeled"].DisplayName,
		"an empty display name falls back to the id")
	assert.Equal(t, HarnessCatalogKindText, byID["unlabeled"].Kind,
		"an unlabeled pool is a text pool")
}

// The shipped defaults must cover the full offering shape, in catalog order: the
// general chat pool, the single video pool, the reserved audio pool, and the plan
// pool billed per call, with every label already coherent.
func TestDefaultHarnessCatalogTiersCoverTheOffering(t *testing.T) {
	tiers := defaultHarnessCatalogTiers()
	require.Equal(t, []HarnessCatalogTier{
		{ID: "chat", DisplayName: "通用对话", Kind: HarnessCatalogKindText},
		{ID: "video", DisplayName: "视频", Kind: HarnessCatalogKindVideo},
		{ID: "image", DisplayName: "图片生成", Kind: HarnessCatalogKindImage},
		{ID: "audio", DisplayName: "语音", Kind: HarnessCatalogKindAudio},
		{ID: "plan", DisplayName: "畅用计划", Kind: HarnessCatalogKindVideo, Billing: HarnessCatalogBillingPerCall},
	}, tiers)

	for _, tier := range tiers {
		kind, ok := NormalizeHarnessCatalogKind(tier.Kind)
		require.Truef(t, ok, "default tier %q has an invalid kind", tier.ID)
		_, billingOk := NormalizeHarnessCatalogBilling(kind, tier.Billing)
		require.Truef(t, billingOk, "default tier %q has an incoherent label pair", tier.ID)
	}

	config := HarnessCatalogConfig{Tiers: tiers}
	assert.Equal(t, defaultHarnessCatalogDefaultTier, config.DefaultResourcePool(),
		"the shipped default pool must be one of the shipped pools")
	assert.Equal(t, "chat", defaultHarnessCatalogDefaultTier)
}
