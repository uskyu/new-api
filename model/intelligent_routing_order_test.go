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

func TestConfiguredChannelOrderRetryExhaustionDoesNotRepeatLastCandidate(t *testing.T) {
	require.Nil(t, configuredChannelForRetry([]int{8}, 1))
	require.Equal(t, 8, *configuredChannelForRetry([]int{8, 3}, 0))
	require.Equal(t, 3, *configuredChannelForRetry([]int{8, 3}, 1))
}
