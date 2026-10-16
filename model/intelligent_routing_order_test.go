package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfiguredChannelOrderIndexRespectsAdminOrderAndDropsMissingCandidates(t *testing.T) {
	got := configuredChannelOrderIndex([]int{8, 3, 5, 1}, []int{1, 5, 8})
	require.Equal(t, []int{8, 5, 1}, got)
}

func TestConfiguredChannelOrderIndexDoesNotInventFallbackCandidates(t *testing.T) {
	got := configuredChannelOrderIndex([]int{8, 3}, nil)
	require.Empty(t, got)
}
