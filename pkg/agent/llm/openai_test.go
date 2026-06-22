package llm

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sashabaranov/go-openai"
)

// TestOpenAIClient_ResponseFormatFallback verifies the fallback when the
// backend rejects response_format (HTTP 400 mentioning the field name).
// Many self-hosted OpenAI-compatible gateways (early One-API, some LiteLLM
// configs, Ollama's OpenAI shim) don't support JSON object mode. Without
// the fallback, doctor would silently degrade to plain diff forever and
// the user would never know why.
//
// The test server:
//   - First request (with response_format): returns 400
//   - Second request (without response_format): returns valid JSON
//
// The client must transparently retry and return the Analysis.
func TestOpenAIClient_ResponseFormatFallback(t *testing.T) {
	var requestCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := requestCount.Add(1)
		body, _ := io.ReadAll(r.Body)

		if n == 1 && strings.Contains(string(body), "response_format") {
			// Simulate a backend that doesn't support response_format.
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{
					"message": "response_format is not supported by this backend",
					"type":    "invalid_request_error",
					"code":    "unsupported_parameter",
				},
			})
			return
		}

		// Second request (no response_format): succeed.
		resp := chatRespShape{
			Choices: []chatChoiceShape{{Message: chatMessageShape{
				Content: `{"summary":"ok","risks":[]}`,
			}}},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := Config{BaseURL: server.URL + "/v1", APIKey: "k", Model: "m"}
	client := newOpenAIClient(cfg.WithDefaults())

	got, err := client.Analyze(context.Background(), "diff", AnalyzeInput{})
	if err != nil {
		t.Fatalf("expected fallback to succeed, got: %v", err)
	}
	if got.Summary != "ok" {
		t.Errorf("Summary = %q, want \"ok\"", got.Summary)
	}
	if c := requestCount.Load(); c != 2 {
		t.Errorf("expected 2 requests (initial + fallback), got %d", c)
	}
}

// TestOpenAIClient_ResponseFormatNoRetryOnUnrelated400 ensures the fallback
// ONLY triggers on response_format-related errors, not on every 400. A bad
// model name should fail immediately without a pointless retry.
func TestOpenAIClient_ResponseFormatNoRetryOnUnrelated400(t *testing.T) {
	var requestCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"message": "The model 'gpt-nonexistent' does not exist",
				"type":    "invalid_request_error",
			},
		})
	}))
	defer server.Close()

	cfg := Config{BaseURL: server.URL + "/v1", APIKey: "k", Model: "gpt-nonexistent"}
	client := newOpenAIClient(cfg.WithDefaults())

	_, err := client.Analyze(context.Background(), "diff", AnalyzeInput{})
	if err == nil {
		t.Fatal("expected error on unrelated 400")
	}
	if c := requestCount.Load(); c != 1 {
		t.Errorf("expected exactly 1 request (no retry on unrelated 400), got %d", c)
	}
}

// TestShouldRetryWithoutResponseFormat verifies the error classifier.
func TestShouldRetryWithoutResponseFormat(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "400 with response_format in message",
			err:  &openai.APIError{HTTPStatusCode: 400, Message: "response_format is not supported"},
			want: true,
		},
		{
			name: "400 with json_object in message",
			err:  &openai.APIError{HTTPStatusCode: 400, Message: "unknown parameter: json_object"},
			want: true,
		},
		{
			name: "400 with json mode in message",
			err:  &openai.APIError{HTTPStatusCode: 400, Message: "json mode is not available"},
			want: true,
		},
		{
			name: "400 with unrelated message (bad model)",
			err:  &openai.APIError{HTTPStatusCode: 400, Message: "The model 'gpt-x' does not exist"},
			want: false,
		},
		{
			name: "401 unauthorized",
			err:  &openai.APIError{HTTPStatusCode: 401, Message: "Invalid API key"},
			want: false,
		},
		{
			name: "500 server error",
			err:  &openai.APIError{HTTPStatusCode: 500, Message: "Internal error"},
			want: false,
		},
		{
			name: "plain error (not APIError)",
			err:  errors.New("network timeout"),
			want: false,
		},
		{
			name: "nil",
			err:  nil,
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldRetryWithoutResponseFormat(tt.err); got != tt.want {
				t.Errorf("shouldRetryWithoutResponseFormat() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestAnalysisRaw_ToAnalysis_MultiRiskSort verifies that risks returned by
// the model are re-sorted by severity (high first) via severityRank +
// slices.SortStableFunc. This is the ONLY test that exercises severityRank
// (all other tests return 0 or 1 risk, so the sort callback never fires).
func TestAnalysisRaw_ToAnalysis_MultiRiskSort(t *testing.T) {
	raw := analysisRaw{
		Summary: "mixed risks",
		Risks: []Risk{
			{Level: RiskLevelLow, Category: "best-practice"},
			{Level: RiskLevelHigh, Category: "data-loss"},
			{Level: RiskLevelMedium, Category: "security"},
			{Level: RiskLevelHigh, Category: "breaking-change"},
			{Level: RiskLevelLow, Category: "performance"},
		},
	}
	a := raw.ToAnalysis()

	// Expected order: high, high, medium, low, low (stable within equal severity).
	wantLevels := []RiskLevel{RiskLevelHigh, RiskLevelHigh, RiskLevelMedium, RiskLevelLow, RiskLevelLow}
	for i, r := range a.Risks {
		if r.Level != wantLevels[i] {
			t.Errorf("Risks[%d].Level = %v, want %v", i, r.Level, wantLevels[i])
		}
	}
	// Stability check: the two high risks should preserve their model-given order.
	if a.Risks[0].Category != "data-loss" || a.Risks[1].Category != "breaking-change" {
		t.Errorf("stability broken within high-severity risks: got %s, %s",
			a.Risks[0].Category, a.Risks[1].Category)
	}
}

// TestAnalysisRaw_ToAnalysis_NilRisksEnsuresEmptyArray verifies that a nil
// Risks slice is normalized to []Risk{} (not nil), so ReportJSON can
// distinguish "0 risks" from "analysis absent".
func TestAnalysisRaw_ToAnalysis_NilRisksEnsuresEmptyArray(t *testing.T) {
	raw := analysisRaw{Summary: "no risks", Risks: nil}
	a := raw.ToAnalysis()
	if a.Risks == nil {
		t.Error("Risks should be []Risk{}, not nil")
	}
	if len(a.Risks) != 0 {
		t.Errorf("Risks len = %d, want 0", len(a.Risks))
	}
}

// TestAnalysisRaw_ToAnalysis_UnknownLevelSinksToBottom verifies that risks
// with unrecognized levels are sorted after all known levels via
// severityRank's default branch (return 3). This is the ONLY test that
// covers severityRank's default path.
func TestAnalysisRaw_ToAnalysis_UnknownLevelSinksToBottom(t *testing.T) {
	raw := analysisRaw{
		Risks: []Risk{
			{Level: RiskLevel("critical"), Category: "unknown"},
			{Level: RiskLevelHigh, Category: "known-high"},
		},
	}
	a := raw.ToAnalysis()
	if a.Risks[0].Level != RiskLevelHigh {
		t.Errorf("known-high should sort first; got %v", a.Risks[0].Level)
	}
	if a.Risks[1].Level != RiskLevel("critical") {
		t.Errorf("unknown should sink to bottom; got %v", a.Risks[1].Level)
	}
}

// TestOpenAIClient_SuccessPath exercises the real openaiClient against an
// httptest server that returns a well-formed ChatCompletion response. This
// is the only test that actually drives the sashabaranov/go-openai HTTP
// machinery end-to-end; if the library changes its request shape, this test
// catches it.
func TestOpenAIClient_SuccessPath(t *testing.T) {
	var seenRequest atomic.Value // chatReq
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.Error(w, "bad path", http.StatusBadRequest)
			return
		}
		var req chatReqShape
		_ = json.NewDecoder(r.Body).Decode(&req)
		seenRequest.Store(req)

		// Echo back a fixed analysis the parser must accept.
		body := `{
		  "summary": "one risk found",
		  "risks": [{"level":"high","category":"data-loss","description":"pv deleted","suggestion":"backup first"}],
		  "affected_resources": ["PersistentVolume/data"]
		}`
		resp := chatRespShape{
			Choices: []chatChoiceShape{{Message: chatMessageShape{Content: body}}},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := Config{
		BaseURL: server.URL + "/v1",
		APIKey:  "test-key",
		Model:   "gpt-4o-test",
	}
	client := newOpenAIClient(cfg.WithDefaults())

	got, err := client.Analyze(context.Background(), "fake helm diff", AnalyzeInput{
		Environment: "prod",
		Releases:    []string{"my-release"},
	})
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}
	if got.Summary != "one risk found" {
		t.Errorf("Summary = %q", got.Summary)
	}
	if !got.HasHighRisk() {
		t.Errorf("expected high risk; got %+v", got.Risks)
	}
	if len(got.AffectedResources) != 1 || got.AffectedResources[0] != "PersistentVolume/data" {
		t.Errorf("AffectedResources = %v", got.AffectedResources)
	}

	// Verify the request shape: model + 2 messages + JSON response_format.
	req, ok := seenRequest.Load().(chatReqShape)
	if !ok {
		t.Fatal("no request captured")
	}
	if req.Model != "gpt-4o-test" {
		t.Errorf("request model = %q", req.Model)
	}
	if len(req.Messages) != 2 {
		t.Errorf("expected 2 messages (system+user), got %d", len(req.Messages))
	}
	if req.ResponseFormat == nil || req.ResponseFormat.Type != "json_object" {
		t.Errorf("expected response_format=json_object, got %v", req.ResponseFormat)
	}
}

// TestOpenAIClient_HTTP500ReturnsError verifies that a 5xx from the gateway
// is surfaced as an error rather than silently swallowed. The doctor layer
// uses this to decide whether to degrade to plain diff.
func TestOpenAIClient_HTTP500ReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "upstream overloaded", http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := Config{
		BaseURL: server.URL + "/v1",
		APIKey:  "k",
		Model:   "m",
	}
	client := newOpenAIClient(cfg.WithDefaults())

	_, err := client.Analyze(context.Background(), "diff", AnalyzeInput{})
	if err == nil {
		t.Fatal("expected error from 500 response, got nil")
	}
	// Doctor surfaces this verbatim to the user.
	if !strings.Contains(err.Error(), "chat completion failed") {
		t.Errorf("error message should mention chat completion; got %v", err)
	}
}

// TestOpenAIClient_ContextCanceledPropagates verifies the client honors
// context cancellation (e.g. user Ctrl+C). We cancel BEFORE calling Analyze
// so the http request fails fast with the context error.
func TestOpenAIClient_ContextCanceledPropagates(t *testing.T) {
	// Server that sleeps to simulate slow LLM. Should never be reached
	// because context is pre-canceled.
	hangServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Second)
		http.Error(w, "should not reach", http.StatusOK)
	}))
	defer hangServer.Close()

	cfg := Config{
		BaseURL: hangServer.URL + "/v1",
		APIKey:  "k",
		Model:   "m",
	}
	client := newOpenAIClient(cfg.WithDefaults())

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := client.Analyze(ctx, "diff", AnalyzeInput{})
	if err == nil {
		t.Fatal("expected context-canceled error, got nil")
	}
}

// TestOpenAIClient_TimeoutSurfacesAsError verifies the per-request timeout
// on the LLM Config is honored. We point the client at a slow server and a
// 50ms timeout; the call must fail.
func TestOpenAIClient_TimeoutSurfacesAsError(t *testing.T) {
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer slow.Close()

	cfg := Config{
		BaseURL: slow.URL + "/v1",
		APIKey:  "k",
		Model:   "m",
		// Use an aggressive timeout for the test. WithDefaults would set 60s.
		Timeout: 50 * time.Millisecond,
	}
	client := newOpenAIClient(cfg)
	// Note: NOT calling WithDefaults because it would override our 50ms back
	// to 60s. This is intentional for the timeout test only.

	_, err := client.Analyze(context.Background(), "diff", AnalyzeInput{})
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}

// TestOpenAIClient_MalformedJSONReturnsError covers the parser branch:
// the server returns HTTP 200 but the body is not valid JSON. Doctor must
// surface this so the user understands the model produced garbage.
func TestOpenAIClient_MalformedJSONReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := chatRespShape{
			Choices: []chatChoiceShape{{Message: chatMessageShape{
				Content: "this is definitely not json",
			}}},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := Config{BaseURL: server.URL + "/v1", APIKey: "k", Model: "m"}
	client := newOpenAIClient(cfg.WithDefaults())

	_, err := client.Analyze(context.Background(), "diff", AnalyzeInput{})
	if err == nil {
		t.Fatal("expected parse error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to parse") {
		t.Errorf("error should mention parse failure; got %v", err)
	}
}

// TestOpenAIClient_ModelSignedErrorField verifies the "error" field in the
// JSON body (used by the model to signal "input too large") is surfaced as
// a Go error rather than silently producing empty Analysis.
func TestOpenAIClient_ModelSignedErrorField(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := `{"error":"diff too large, max 1000 tokens"}`
		resp := chatRespShape{
			Choices: []chatChoiceShape{{Message: chatMessageShape{Content: body}}},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := Config{BaseURL: server.URL + "/v1", APIKey: "k", Model: "m"}
	client := newOpenAIClient(cfg.WithDefaults())

	_, err := client.Analyze(context.Background(), "diff", AnalyzeInput{})
	if err == nil {
		t.Fatal("expected model-signed error, got nil")
	}
	if !strings.Contains(err.Error(), "diff too large") {
		t.Errorf("error should propagate model's message; got %v", err)
	}
}

// TestNewClient_NilWhenUnconfigured verifies the doctor degrade-to-diff
// invariant: NewClient returns nil when Config is not usable. Doctor relies
// on this nil check to decide whether to call the LLM at all.
func TestNewClient_NilWhenUnconfigured(t *testing.T) {
	cases := []Config{
		{},                       // all empty
		{APIKey: "k"},            // missing model
		{Model: "m"},             // missing key
		{APIKey: "", Model: "m"}, // explicit empty key
	}
	for i, cfg := range cases {
		if c := NewClient(cfg); c != nil {
			t.Errorf("case %d: NewClient returned non-nil for unconfigured %+v", i, cfg)
		}
	}
}

// TestNewClient_NonNilWhenConfigured is the positive counterpart. Also
// verifies that BaseURL is OPTIONAL (OpenAI official users).
func TestNewClient_NonNilWhenConfigured(t *testing.T) {
	cfg := Config{APIKey: "sk-...", Model: "gpt-4o"}
	c := NewClient(cfg)
	if c == nil {
		t.Fatal("NewClient returned nil despite APIKey+Model")
	}
	// And with BaseURL set.
	cfg2 := Config{BaseURL: "https://gw.example/v1", APIKey: "k", Model: "m"}
	if c2 := NewClient(cfg2); c2 == nil {
		t.Fatal("NewClient returned nil despite full config")
	}
}

// errMockClient is reused from analyze_test.go's errMockClient for
// stand-alone scenarios. The error it returns must propagate verbatim.
func TestClient_AnalyzeErrorPropagates(t *testing.T) {
	boom := errors.New("simulated gateway error")
	c := &MockClient{Err: boom}
	_, err := c.Analyze(context.Background(), "diff", AnalyzeInput{})
	if !errors.Is(err, boom) {
		t.Errorf("expected %v, got %v", boom, err)
	}
}

// --- wire shapes used by these tests ---------------------------------------

type chatReqShape struct {
	Model          string               `json:"model"`
	Messages       []chatMessageShape   `json:"messages"`
	ResponseFormat *responseFormatShape `json:"response_format,omitempty"`
}

type responseFormatShape struct {
	Type string `json:"type"`
}

type chatRespShape struct {
	Choices []chatChoiceShape `json:"choices"`
}

type chatChoiceShape struct {
	Message chatMessageShape `json:"message"`
}

type chatMessageShape struct {
	Content string `json:"content"`
}
