package doctor

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/helmfile/helmfile/pkg/agent/llm"
)

// errMockClient is a Client that always returns errBoom to exercise the
// degraded-output path.
type errMockClient struct{ err error }

func (m *errMockClient) Analyze(_ context.Context, _ string, _ llm.AnalyzeInput) (llm.Analysis, error) {
	return llm.Analysis{}, m.err
}

func TestAnalyze_EmptyDiffReturnsEmptyResult(t *testing.T) {
	r := Analyze(context.Background(), "", Options{Client: nil})
	// Empty Result means RawDiff empty AND Analysis nil — doctor.go uses
	// these field checks directly (rather than a Result.IsEmpty helper) to
	// decide whether to print anything.
	if r.RawDiff != "" || r.Analysis != nil {
		t.Fatalf("expected empty result, got %+v", r)
	}
}

func TestAnalyze_NilClientReturnsRawDiff(t *testing.T) {
	r := Analyze(context.Background(), "diff body", Options{Client: nil})
	if r.Analysis != nil {
		t.Errorf("Analysis should be nil, got %+v", r.Analysis)
	}
	if r.RawDiff != "diff body" {
		t.Errorf("RawDiff = %q, want %q", r.RawDiff, "diff body")
	}
	if r.LLMCallFailed {
		t.Error("LLMCallFailed should be false when client is nil")
	}
}

func TestAnalyze_Success(t *testing.T) {
	want := llm.Analysis{Summary: "ok", Risks: []llm.Risk{{Level: llm.RiskLevelLow}}}
	c := llm.NewMockClient(want)

	r := Analyze(context.Background(), "diff", Options{
		Client: c,
		Model:  "gpt-4o",
	})
	if r.Analysis == nil {
		t.Fatal("Analysis is nil")
	}
	if r.Analysis.Summary != "ok" {
		t.Errorf("Summary = %q, want ok", r.Analysis.Summary)
	}
	if r.LLMCallFailed {
		t.Error("LLMCallFailed should be false on success")
	}
	if r.Model != "gpt-4o" {
		t.Errorf("Model = %q, want gpt-4o", r.Model)
	}
}

func TestAnalyze_LLMFailureSetsFlag(t *testing.T) {
	boom := errors.New("upstream 500")
	c := &errMockClient{err: boom}

	r := Analyze(context.Background(), "diff", Options{Client: c, Model: "m"})
	if !r.LLMCallFailed {
		t.Fatal("LLMCallFailed should be true")
	}
	if r.LLMError != boom {
		t.Errorf("LLMError = %v, want %v", r.LLMError, boom)
	}
	if r.RawDiff != "diff" {
		t.Errorf("RawDiff should still be populated, got %q", r.RawDiff)
	}
}

func TestResult_HasHighRisk(t *testing.T) {
	tests := []struct {
		name string
		r    Result
		want bool
	}{
		{name: "nil analysis", r: Result{}, want: false},
		{name: "no risks", r: Result{Analysis: &llm.Analysis{}}, want: false},
		{name: "only low", r: Result{Analysis: &llm.Analysis{Risks: []llm.Risk{{Level: llm.RiskLevelLow}}}}, want: false},
		{name: "has high", r: Result{Analysis: &llm.Analysis{Risks: []llm.Risk{{Level: llm.RiskLevelHigh}}}}, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.r.HasHighRisk(); got != tt.want {
				t.Errorf("HasHighRisk() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestRoundDuration guards against roundDuration misbehaving on common
// durations.
func TestRoundDuration(t *testing.T) {
	if got := roundDuration(500 * time.Millisecond); got != "500ms" {
		t.Errorf("got %q want 500ms", got)
	}
	if got := roundDuration(2200 * time.Millisecond); got != "2.2s" {
		t.Errorf("got %q want 2.2s", got)
	}
}
