package llm

import (
	goContext "context"
	"sync"
)

// MockClient is a test-only Client returning a canned Analysis or error.
//
// Concurrent-safe: Analyze calls are mutex-serialized so LastCall() recordings
// stay consistent under `go test -parallel`. For per-call isolation, create a
// fresh MockClient per case rather than relying on the mutex.
type MockClient struct {
	mu       sync.Mutex
	Analysis Analysis
	Err      error

	lastDiff  string
	lastInput AnalyzeInput
}

// NewMockClient returns a MockClient that responds with the given Analysis.
func NewMockClient(a Analysis) *MockClient {
	return &MockClient{Analysis: a}
}

// Analyze implements Client. The returned Analysis is a deep copy so test
// mutations on the returned value do not corrupt canned state for the next
// caller.
func (m *MockClient) Analyze(_ goContext.Context, diff string, in AnalyzeInput) (Analysis, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.lastDiff = diff
	m.lastInput = in

	if m.Err != nil {
		return Analysis{}, m.Err
	}
	return cloneAnalysis(m.Analysis), nil
}

// LastCall returns the most recent (diff, input) handed to Analyze. Safe to
// call from a different goroutine than Analyze.
func (m *MockClient) LastCall() (string, AnalyzeInput) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastDiff, m.lastInput
}

// cloneAnalysis returns a deep-enough copy of a for MockClient use: a test
// that does `out, _ := c.Analyze(...); out.Risks[0].Level = "high"` must not
// change what the NEXT Analyze() returns.
func cloneAnalysis(a Analysis) Analysis {
	out := a
	if a.Risks != nil {
		out.Risks = append([]Risk(nil), a.Risks...)
	}
	if a.AffectedResources != nil {
		out.AffectedResources = append([]string(nil), a.AffectedResources...)
	}
	return out
}
