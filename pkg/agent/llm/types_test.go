package llm

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestConfig_IsConfigured(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		want bool
	}{
		{
			name: "all empty",
			cfg:  Config{},
			want: false,
		},
		{
			name: "only baseURL",
			cfg:  Config{BaseURL: "https://x"},
			want: false,
		},
		{
			name: "baseURL + apiKey but no model",
			cfg:  Config{BaseURL: "https://x", APIKey: "k"},
			want: false,
		},
		{
			name: "apiKey + model, no baseURL (OpenAI official user)",
			cfg:  Config{APIKey: "sk-...", Model: "gpt-4o"},
			want: true,
		},
		{
			name: "all set",
			cfg:  Config{BaseURL: "https://x", APIKey: "k", Model: "gpt-4o"},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.IsConfigured(); got != tt.want {
				t.Errorf("IsConfigured() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConfig_WithDefaults(t *testing.T) {
	cfg := Config{BaseURL: "https://x", APIKey: "k", Model: "m"}
	out := cfg.WithDefaults()
	if out.Timeout != 60*time.Second {
		t.Errorf("Timeout = %v, want 60s", out.Timeout)
	}
	if out.MaxTokens != 4096 {
		t.Errorf("MaxTokens = %v, want 4096", out.MaxTokens)
	}
	if out.Temperature != 0.2 {
		t.Errorf("Temperature = %v, want 0.2", out.Temperature)
	}
	// Originals preserved
	if out.BaseURL != "https://x" || out.APIKey != "k" || out.Model != "m" {
		t.Errorf("WithDefaults overwrote explicit values: %+v", out)
	}
}

func TestMerge_FlagOverridesYamlOverridesEnv(t *testing.T) {
	env := Config{BaseURL: "https://env.example", APIKey: "envkey", Model: "env-model"}
	yaml := Config{BaseURL: "https://yaml.example", MaxTokens: 1024}
	flag := Config{Model: "flag-model"}

	got := Merge(Merge(env, yaml), flag)

	if got.BaseURL != "https://yaml.example" {
		t.Errorf("BaseURL = %s, want yaml to override env", got.BaseURL)
	}
	if got.APIKey != "envkey" {
		t.Errorf("APIKey = %s, want env (no override)", got.APIKey)
	}
	if got.Model != "flag-model" {
		t.Errorf("Model = %s, want flag to override env", got.Model)
	}
	if got.MaxTokens != 1024 {
		t.Errorf("MaxTokens = %d, want yaml value", got.MaxTokens)
	}
}

func TestAnalysis_HasHighRisk(t *testing.T) {
	tests := []struct {
		name string
		a    Analysis
		want bool
	}{
		{name: "empty", a: Analysis{}, want: false},
		{name: "low only", a: Analysis{Risks: []Risk{{Level: RiskLevelLow}}}, want: false},
		{name: "medium only", a: Analysis{Risks: []Risk{{Level: RiskLevelMedium}}}, want: false},
		{name: "high present", a: Analysis{Risks: []Risk{{Level: RiskLevelLow}, {Level: RiskLevelHigh}}}, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.a.HasHighRisk(); got != tt.want {
				t.Errorf("HasHighRisk() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMockClient_Analyze_CapturesInput(t *testing.T) {
	canned := Analysis{Summary: "ok"}
	c := NewMockClient(canned)
	got, err := c.Analyze(context.TODO(), "some diff", AnalyzeInput{Environment: "prod", Releases: []string{"a"}})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got.Summary != "ok" {
		t.Errorf("got = %+v, want %+v", got, canned)
	}
	lastDiff, lastInput := c.LastCall()
	if lastDiff != "some diff" {
		t.Errorf("lastDiff = %q, want %q", lastDiff, "some diff")
	}
	if lastInput.Environment != "prod" {
		t.Errorf("lastInput.Environment = %q, want prod", lastInput.Environment)
	}
}

func TestMockClient_Analyze_PropagatesErr(t *testing.T) {
	c := &MockClient{Err: errBoom}
	if _, err := c.Analyze(context.TODO(), "diff", AnalyzeInput{}); err == nil {
		t.Fatal("expected error, got nil")
	}
}

// TestMockClient_ConcurrentSafe is a regression guard against a data race the
// original implementation had: LastDiff/LastInput were unsynchronized fields
// written on every Analyze call. Under `go test -race -parallel` the test
// runner would flag the race. The fix wraps the fields behind a mutex and
// exposes them via LastCall().
func TestMockClient_ConcurrentSafe(t *testing.T) {
	c := NewMockClient(Analysis{Summary: "ok"})
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, _ = c.Analyze(context.TODO(), "diff-a", AnalyzeInput{Releases: []string{"a"}})
		}()
		go func() {
			defer wg.Done()
			_, _ = c.LastCall()
		}()
	}
	wg.Wait()
}

var errBoom = newBoom("boom")

type boomError struct{ msg string }

func (b *boomError) Error() string { return b.msg }

func newBoom(s string) error { return &boomError{msg: s} }
