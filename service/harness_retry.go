package service

// HarnessMaxRetryLimit bounds how many extra candidates a Harness call may try.
// The relay loop's Retry times one attempt, so a pool with more than eleven
// candidates still stops after eleven retries; that keeps one stuck request from
// occupying a worker far longer than the configured policy intended.
const HarnessMaxRetryLimit = 10

// HarnessRetryBudget returns the retry index bound for one Harness relay loop.
//
// A pool can contain several enabled channels for the same model, and each of
// them is a candidate. The global RetryTimes setting was written for ordinary
// multi-group retries and can be smaller than the candidate count, which used to
// cut the candidate list short: the relay stopped while untried channels were
// still available. For Harness calls the budget therefore becomes
// max(configuredRetries, candidateCount-1), capped at HarnessMaxRetryLimit.
// Candidate selection itself is unchanged; only the loop bound grows.
//
// candidateCount is the number of channels the pool reports for the model; zero
// means the pool could not be inspected, and the configured policy is kept.
func HarnessRetryBudget(configuredRetries int, candidateCount int) int {
	limit := configuredRetries
	if candidateCount > 1 {
		if candidateBudget := candidateCount - 1; candidateBudget > limit {
			limit = candidateBudget
		}
	}
	if limit < 0 {
		limit = 0
	}
	if limit > HarnessMaxRetryLimit {
		limit = HarnessMaxRetryLimit
	}
	return limit
}
