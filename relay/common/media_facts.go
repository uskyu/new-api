package common

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	rootcommon "github.com/QuantumNous/new-api/common"
	"github.com/shopspring/decimal"
)

// Media facts describe what actually happened to one media request. They are
// deliberately separate from the billing result: the B3 billing_quantity is
// derived from these facts (or from the request) at settlement, but it is not a
// fact itself and must never be written back into this list.
const (
	MediaFactsVersion = 1

	MediaFactSourceUpstream    = "upstream"
	MediaFactSourceObserved    = "observed"
	MediaFactSourceRequest     = "request"
	MediaFactSourceEstimated   = "estimated"
	MediaFactSourceUnavailable = "unavailable"

	MediaDimensionImage = "image"
	MediaDimensionVideo = "video"

	MediaUnitImage       = "image"
	MediaUnitVideoSecond = "video_second"

	MediaFactNameImageCount       = "count"
	MediaFactNameRequestedSeconds = "requested_seconds"
	MediaFactNameInputSeconds     = "input_seconds"
	MediaFactNameOutputSeconds    = "output_seconds"
	MediaFactNameReferenceSeconds = "reference_seconds"
)

// MediaFact is one named quantity with its own source. A nil Quantity means the
// value is unknown; it must not be encoded as zero, which would claim a measured
// absence rather than a missing measurement.
type MediaFact struct {
	Name      string  `json:"name"`
	Dimension string  `json:"dimension"`
	Quantity  *string `json:"quantity"`
	Unit      string  `json:"unit"`
	Source    string  `json:"source"`
}

// MediaFacts is the versioned fact list attached to one B1 event. The list is
// sorted by dimension and name so two logically identical lists serialize
// identically; the ledger diff compares that serialized form.
type MediaFacts struct {
	Version int         `json:"version"`
	Facts   []MediaFact `json:"facts"`
}

func NewMediaFacts() MediaFacts {
	return MediaFacts{Version: MediaFactsVersion}
}

// Set inserts or replaces one fact. A repeated (dimension,name) pair is replaced
// rather than appended: a later observation of the same quantity is a
// correction, not a second usage entry.
func (f *MediaFacts) Set(fact MediaFact) error {
	normalized, err := normalizeMediaFact(fact)
	if err != nil {
		return err
	}
	if f.Version == 0 {
		f.Version = MediaFactsVersion
	}
	for i := range f.Facts {
		if f.Facts[i].Name == normalized.Name && f.Facts[i].Dimension == normalized.Dimension {
			f.Facts[i] = normalized
			f.sort()
			return nil
		}
	}
	f.Facts = append(f.Facts, normalized)
	f.sort()
	return nil
}

func (f *MediaFacts) SetQuantity(name, dimension, unit, source, quantity string) error {
	return f.Set(MediaFact{Name: name, Dimension: dimension, Unit: unit, Source: source, Quantity: &quantity})
}

func (f *MediaFacts) SetUnavailable(name, dimension, unit string) error {
	return f.Set(MediaFact{Name: name, Dimension: dimension, Unit: unit, Source: MediaFactSourceUnavailable})
}

func (f *MediaFacts) IsEmpty() bool {
	return f == nil || len(f.Facts) == 0
}

// CanonicalJSON is the persisted representation. It returns "" for an empty
// fact list, which keeps the feature switch off byte-for-byte identical to the
// previous ledger rows.
func (f *MediaFacts) CanonicalJSON() string {
	if f.IsEmpty() {
		return ""
	}
	if f.Version == 0 {
		f.Version = MediaFactsVersion
	}
	f.sort()
	encoded, err := rootcommon.Marshal(f)
	if err != nil {
		return ""
	}
	return string(encoded)
}

// ParseMediaFacts validates and canonicalizes a stored fact list. A malformed
// row is rejected instead of being published with a fabricated source.
func ParseMediaFacts(raw string) (*MediaFacts, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var facts MediaFacts
	if err := rootcommon.Unmarshal([]byte(raw), &facts); err != nil {
		return nil, fmt.Errorf("invalid media facts JSON: %w", err)
	}
	if facts.Version == 0 {
		facts.Version = MediaFactsVersion
	}
	if facts.Version != MediaFactsVersion {
		return nil, fmt.Errorf("unsupported media facts version %d", facts.Version)
	}
	if len(facts.Facts) == 0 {
		return nil, nil
	}
	normalized := make([]MediaFact, 0, len(facts.Facts))
	for _, fact := range facts.Facts {
		value, err := normalizeMediaFact(fact)
		if err != nil {
			return nil, err
		}
		normalized = append(normalized, value)
	}
	seen := make(map[string]struct{}, len(normalized))
	for _, fact := range normalized {
		key := fact.Dimension + "\x00" + fact.Name
		if _, exists := seen[key]; exists {
			return nil, fmt.Errorf("duplicate media fact %s/%s", fact.Dimension, fact.Name)
		}
		seen[key] = struct{}{}
	}
	facts.Facts = normalized
	facts.sort()
	return &facts, nil
}

func (f *MediaFacts) sort() {
	slices.SortFunc(f.Facts, func(a, b MediaFact) int {
		return cmp.Or(
			cmp.Compare(a.Dimension, b.Dimension),
			cmp.Compare(a.Name, b.Name),
		)
	})
}

func normalizeMediaFact(fact MediaFact) (MediaFact, error) {
	fact.Name = strings.TrimSpace(fact.Name)
	fact.Dimension = strings.TrimSpace(fact.Dimension)
	fact.Unit = strings.TrimSpace(fact.Unit)
	fact.Source = strings.TrimSpace(fact.Source)
	if fact.Name == "" {
		return MediaFact{}, fmt.Errorf("media fact name is required")
	}
	if fact.Source == "" {
		return MediaFact{}, fmt.Errorf("media fact %s has no source", fact.Name)
	}
	switch fact.Dimension {
	case MediaDimensionImage:
		if fact.Unit != MediaUnitImage {
			return MediaFact{}, fmt.Errorf("media fact %s has unit %q, want %q", fact.Name, fact.Unit, MediaUnitImage)
		}
	case MediaDimensionVideo:
		if fact.Unit != MediaUnitVideoSecond {
			return MediaFact{}, fmt.Errorf("media fact %s has unit %q, want %q", fact.Name, fact.Unit, MediaUnitVideoSecond)
		}
	default:
		return MediaFact{}, fmt.Errorf("media fact %s has unknown dimension %q", fact.Name, fact.Dimension)
	}
	switch fact.Source {
	case MediaFactSourceUpstream, MediaFactSourceObserved, MediaFactSourceRequest, MediaFactSourceEstimated:
		if fact.Quantity == nil || strings.TrimSpace(*fact.Quantity) == "" {
			return MediaFact{}, fmt.Errorf("media fact %s from %s has no quantity", fact.Name, fact.Source)
		}
	case MediaFactSourceUnavailable:
		if fact.Quantity != nil {
			return MediaFact{}, fmt.Errorf("media fact %s is unavailable but carries quantity %q", fact.Name, *fact.Quantity)
		}
	default:
		return MediaFact{}, fmt.Errorf("media fact %s has unknown source %q", fact.Name, fact.Source)
	}
	if fact.Quantity != nil {
		value, err := decimal.NewFromString(strings.TrimSpace(*fact.Quantity))
		if err != nil {
			return MediaFact{}, fmt.Errorf("media fact %s has invalid quantity %q", fact.Name, *fact.Quantity)
		}
		if value.IsNegative() {
			return MediaFact{}, fmt.Errorf("media fact %s has negative quantity %q", fact.Name, *fact.Quantity)
		}
		// The frozen B1 contract types image counts as integers. A fractional
		// image quantity is a producer bug and must be rejected here instead of
		// leaking a string into a field the platform expects as a number.
		if fact.Dimension == MediaDimensionImage && !value.IsInteger() {
			return MediaFact{}, fmt.Errorf("media fact %s has non-integer image quantity %q", fact.Name, *fact.Quantity)
		}
		canonical := value.String()
		fact.Quantity = &canonical
	}
	return fact, nil
}
