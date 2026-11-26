package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestHarnessRetryBudgetCoversCandidatesWithinCap pins the expanded Harness
// retry budget: the configured policy is never reduced, every candidate beyond
// the first gets an attempt, and the cap keeps a large pool bounded.
func TestHarnessRetryBudgetCoversCandidatesWithinCap(t *testing.T) {
	for _, testCase := range []struct {
		name       string
		configured int
		candidates int
		want       int
	}{
		{"no pool information keeps the configured policy", 3, 0, 3},
		{"single candidate keeps the configured policy", 3, 1, 3},
		{"two candidates need one retry", 0, 2, 1},
		{"more candidates than retries grow the budget", 2, 5, 4},
		{"the configured policy still wins when larger", 7, 3, 7},
		{"large pools are capped", 0, 20, HarnessMaxRetryLimit},
		{"a large configured policy is capped for Harness", 15, 0, HarnessMaxRetryLimit},
		{"negative input cannot underflow", -1, 0, 0},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			assert.Equal(t, testCase.want, HarnessRetryBudget(testCase.configured, testCase.candidates))
		})
	}
}
