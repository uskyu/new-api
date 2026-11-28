package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMediaFactsCanonicalizesAndSorts(t *testing.T) {
	var facts MediaFacts
	require.NoError(t, facts.SetQuantity(
		MediaFactNameOutputSeconds, MediaDimensionVideo, MediaUnitVideoSecond, MediaFactSourceUpstream, "8.50"))
	require.NoError(t, facts.SetQuantity(
		MediaFactNameRequestedSeconds, MediaDimensionVideo, MediaUnitVideoSecond, MediaFactSourceRequest, "10.0"))
	require.NoError(t, facts.SetQuantity(
		MediaFactNameImageCount, MediaDimensionImage, MediaUnitImage, MediaFactSourceObserved, "1"))

	require.Len(t, facts.Facts, 3)
	assert.Equal(t, MediaFactNameImageCount, facts.Facts[0].Name, "facts sort by dimension then name")
	assert.Equal(t, MediaFactNameOutputSeconds, facts.Facts[1].Name)
	assert.Equal(t, MediaFactNameRequestedSeconds, facts.Facts[2].Name)

	raw := facts.CanonicalJSON()
	require.NotEmpty(t, raw)
	assert.Contains(t, raw, `"quantity":"8.5"`, "decimal strings are canonicalized")
	assert.Contains(t, raw, `"quantity":"10"`)
	assert.Contains(t, raw, `"version":1`)

	parsed, err := ParseMediaFacts(raw)
	require.NoError(t, err)
	require.NotNil(t, parsed)
	assert.Equal(t, facts.Facts, parsed.Facts)
}

func TestMediaFactsUnknownStaysNull(t *testing.T) {
	var facts MediaFacts
	require.NoError(t, facts.SetUnavailable(
		MediaFactNameOutputSeconds, MediaDimensionVideo, MediaUnitVideoSecond))

	raw := facts.CanonicalJSON()
	assert.Contains(t, raw, `"quantity":null`, "unknown is null, never a measured zero")
	assert.Contains(t, raw, `"source":"unavailable"`)

	parsed, err := ParseMediaFacts(raw)
	require.NoError(t, err)
	require.NotNil(t, parsed)
	require.Len(t, parsed.Facts, 1)
	assert.Nil(t, parsed.Facts[0].Quantity)
}

func TestMediaFactsRejectsInvalidFacts(t *testing.T) {
	quantity := "1"
	fractional := "1.5"
	empty := ""
	negative := "-1"
	bad := "1.2.3"
	unavailableQuantity := "0"

	tests := []struct {
		name string
		fact MediaFact
	}{
		{name: "missing name", fact: MediaFact{Dimension: MediaDimensionImage, Unit: MediaUnitImage, Source: MediaFactSourceObserved, Quantity: &quantity}},
		{name: "unknown dimension", fact: MediaFact{Name: MediaFactNameImageCount, Dimension: "audio", Unit: MediaUnitImage, Source: MediaFactSourceObserved, Quantity: &quantity}},
		{name: "wrong unit", fact: MediaFact{Name: MediaFactNameImageCount, Dimension: MediaDimensionImage, Unit: MediaUnitVideoSecond, Source: MediaFactSourceObserved, Quantity: &quantity}},
		{name: "missing quantity", fact: MediaFact{Name: MediaFactNameImageCount, Dimension: MediaDimensionImage, Unit: MediaUnitImage, Source: MediaFactSourceObserved}},
		{name: "empty quantity", fact: MediaFact{Name: MediaFactNameImageCount, Dimension: MediaDimensionImage, Unit: MediaUnitImage, Source: MediaFactSourceObserved, Quantity: &empty}},
		{name: "negative quantity", fact: MediaFact{Name: MediaFactNameImageCount, Dimension: MediaDimensionImage, Unit: MediaUnitImage, Source: MediaFactSourceObserved, Quantity: &negative}},
		{name: "invalid decimal", fact: MediaFact{Name: MediaFactNameImageCount, Dimension: MediaDimensionImage, Unit: MediaUnitImage, Source: MediaFactSourceObserved, Quantity: &bad}},
		{name: "fractional image count", fact: MediaFact{Name: MediaFactNameImageCount, Dimension: MediaDimensionImage, Unit: MediaUnitImage, Source: MediaFactSourceObserved, Quantity: &fractional}},
		{name: "unknown source", fact: MediaFact{Name: MediaFactNameImageCount, Dimension: MediaDimensionImage, Unit: MediaUnitImage, Source: "vendor", Quantity: &quantity}},
		{name: "unavailable with quantity", fact: MediaFact{Name: MediaFactNameOutputSeconds, Dimension: MediaDimensionVideo, Unit: MediaUnitVideoSecond, Source: MediaFactSourceUnavailable, Quantity: &unavailableQuantity}},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			var facts MediaFacts
			require.Error(t, facts.Set(testCase.fact))
		})
	}
}

func TestParseMediaFactsRejectsCorruption(t *testing.T) {
	_, err := ParseMediaFacts(`{"version":1,"facts":[{"name":"count","dimension":"image","quantity":"1","unit":"image","source":"observed"},{"name":"count","dimension":"image","quantity":"2","unit":"image","source":"observed"}]}`)
	require.Error(t, err, "duplicate facts are a corrupt input, not two usage entries")

	_, err = ParseMediaFacts(`{"version":2,"facts":[{"name":"count","dimension":"image","quantity":"1","unit":"image","source":"observed"}]}`)
	require.Error(t, err, "unknown versions must be rejected instead of guessed")

	parsed, err := ParseMediaFacts("")
	require.NoError(t, err)
	assert.Nil(t, parsed)
}

func TestUpdateImageCountRecordsObservedFact(t *testing.T) {
	info := &RelayInfo{MediaLedgerEnabled: true}
	info.UpdateImageCount(2)

	parsed, err := ParseMediaFacts(info.MediaFacts.CanonicalJSON())
	require.NoError(t, err)
	require.NotNil(t, parsed)
	require.Len(t, parsed.Facts, 1)
	assert.Equal(t, MediaFactNameImageCount, parsed.Facts[0].Name)
	require.NotNil(t, parsed.Facts[0].Quantity)
	assert.Equal(t, "2", *parsed.Facts[0].Quantity)
	assert.Equal(t, MediaFactSourceObserved, parsed.Facts[0].Source)

	disabled := &RelayInfo{}
	disabled.UpdateImageCount(2)
	assert.True(t, disabled.MediaFacts.IsEmpty(), "the switch-off path must not record facts")
}

func TestMediaFactsCanonicalJSONEmpty(t *testing.T) {
	var facts MediaFacts
	assert.Empty(t, facts.CanonicalJSON(), "an empty fact list must not reach the wire or the diff")
}
