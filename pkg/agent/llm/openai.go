package llm

import (
	goContext "context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/sashabaranov/go-openai"
)

// openaiClient speaks the OpenAI Chat Completions protocol. It works against
// any compatible gateway (One-API, LiteLLM, Azure OpenAI proxy,
// Cloudflare AI Gateway, direct provider, etc.).
type openaiClient struct {
	cfg Config
	c   *openai.Client
}

func newOpenAIClient(cfg Config) *openaiClient {
	clientConfig := openai.DefaultConfig(cfg.APIKey)
	if cfg.BaseURL != "" {
		clientConfig.BaseURL = cfg.BaseURL
	}
	return &openaiClient{
		cfg: cfg,
		c:   openai.NewClientWithConfig(clientConfig),
	}
}

// Analyze implements Client. It assembles a system+user prompt, requests a
// JSON object back, parses it into Analysis. On any protocol/parse failure
// returns the raw error so callers can degrade gracefully.
func (o *openaiClient) Analyze(ctx goContext.Context, diff string, extras AnalyzeInput) (Analysis, error) {
	if strings.TrimSpace(diff) == "" {
		return Analysis{Summary: "No changes detected by helm diff."}, nil
	}

	system := systemPrompt()
	user := userPrompt(diff, extras)

	ctx, cancel := goContext.WithTimeout(ctx, o.cfg.Timeout)
	defer cancel()

	// Build the request with JSON object response_format. Most OpenAI-
	// compatible backends support this; those that don't will return a 400
	// that we catch below and retry without response_format.
	req := openai.ChatCompletionRequest{
		Model:       o.cfg.Model,
		Temperature: o.cfg.Temperature,
		MaxTokens:   o.cfg.MaxTokens,
		ResponseFormat: &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONObject,
		},
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: system},
			{Role: openai.ChatMessageRoleUser, Content: user},
		},
	}

	resp, err := o.c.CreateChatCompletion(ctx, req)
	if err != nil && shouldRetryWithoutResponseFormat(err) {
		// Backend doesn't support response_format (common on early One-API,
		// some LiteLLM configs, Ollama OpenAI shim). Retry without it.
		// The system prompt still asks for JSON-only output, and
		// stripJSONCodeFence handles markdown fences, so this degrades
		// gracefully — just without the hard guarantee.
		req.ResponseFormat = nil
		resp, err = o.c.CreateChatCompletion(ctx, req)
	}
	if err != nil {
		return Analysis{}, fmt.Errorf("llm: chat completion failed: %w", err)
	}
	if len(resp.Choices) == 0 {
		return Analysis{}, errors.New("llm: empty completion (no choices)")
	}

	content := strings.TrimSpace(resp.Choices[0].Message.Content)
	content = stripJSONCodeFence(content)

	var raw analysisRaw
	if err := json.Unmarshal([]byte(content), &raw); err != nil {
		return Analysis{}, fmt.Errorf("llm: failed to parse model output as JSON: %w (raw=%q)", err, content)
	}
	if raw.Error != "" {
		return Analysis{}, fmt.Errorf("llm: model reported error: %s", raw.Error)
	}
	return raw.ToAnalysis(), nil
}

// shouldRetryWithoutResponseFormat reports whether err looks like a "backend
// doesn't support response_format" rejection. We match on HTTP 400 + message
// containing response_format-related keywords. This is intentionally broad:
// different backends phrase the error differently ("unknown parameter",
// "unsupported field", "must be one of", etc.) but all mention the field name.
//
// False positives (retrying on an unrelated 400) are harmless — the retry
// without response_format will still fail on the real issue (bad model,
// invalid key, etc.) and surface that error to the user.
func shouldRetryWithoutResponseFormat(err error) bool {
	var apiErr *openai.APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	if apiErr.HTTPStatusCode != 400 {
		return false
	}
	msg := strings.ToLower(apiErr.Message)
	return strings.Contains(msg, "response_format") ||
		strings.Contains(msg, "response format") ||
		strings.Contains(msg, "json_object") ||
		strings.Contains(msg, "json mode")
}

// analysisRaw is the wire schema the LLM is asked to produce. It mirrors
// Analysis but adds an optional Error field so the model can signal that
// the input was too large / unparseable instead of hallucinating.
type analysisRaw struct {
	Summary           string   `json:"summary"`
	Risks             []Risk   `json:"risks"`
	AffectedResources []string `json:"affected_resources"`
	Error             string   `json:"error,omitempty"`
}

func (r analysisRaw) ToAnalysis() Analysis {
	risks := r.Risks
	if risks == nil {
		risks = []Risk{}
	}
	// Stable sort so risks with equal severity keep their model-given
	// order. The prompt asks the model to sort by severity already, but we
	// re-sort defensively in case the gateway shuffled the JSON keys.
	slices.SortStableFunc(risks, func(a, b Risk) int {
		return severityRank(a.Level) - severityRank(b.Level)
	})
	return Analysis{
		Summary:           r.Summary,
		Risks:             risks,
		AffectedResources: r.AffectedResources,
	}
}

// stripJSONCodeFence extracts the JSON payload from an LLM completion that
// may be:
//
//  1. Pure JSON: {"summary":"..."}
//  2. Markdown-fenced: ```json\n{...}\n```
//  3. Prose + JSON: "Here is my analysis:\n```json\n{...}\n```\nLet me know."
//  4. Prose + bare JSON: "Sure! {\"summary\":\"...\"} Hope this helps."
//
// Cases 3 and 4 are common when the backend does not support
// response_format (see shouldRetryWithoutResponseFormat) and the model
// wraps the JSON in explanatory text despite the prompt asking for
// JSON-only output.
//
// Strategy: try fence stripping first (handles 1 and 2). If the result
// still doesn't look like JSON (no leading '{'), fall back to extracting
// the outermost {...} block. If THAT fails, return the original string
// and let json.Unmarshal produce the error — the caller's error message
// includes the raw content for debugging.
func stripJSONCodeFence(s string) string {
	s = strings.TrimSpace(s)

	// Case 2: fenced JSON.
	if strings.HasPrefix(s, "```") {
		rest := s[3:]
		if nl := strings.IndexByte(rest, '\n'); nl >= 0 {
			s = rest[nl+1:]
		} else {
			s = rest
		}
		s = strings.TrimSuffix(strings.TrimSpace(s), "```")
		s = strings.TrimSpace(s)
	}

	// Case 1: pure JSON — return as-is.
	if strings.HasPrefix(s, "{") {
		return s
	}

	// Cases 3 and 4: prose around JSON. Extract the outermost {...} block.
	// This is intentionally simple (no nested-brace counting) because LLM
	// output rarely has multiple top-level JSON objects, and a wrong extract
	// still produces a better error than failing on the full prose string.
	if start := strings.IndexByte(s, '{'); start >= 0 {
		if end := strings.LastIndexByte(s, '}'); end > start {
			return s[start : end+1]
		}
	}

	return s
}

// severityRank sorts high before medium before low; unknown levels sink to
// the bottom but stay ordered by their original (stable-sort) position.
func severityRank(l RiskLevel) int {
	switch l {
	case RiskLevelHigh:
		return 0
	case RiskLevelMedium:
		return 1
	case RiskLevelLow:
		return 2
	}
	return 3
}
