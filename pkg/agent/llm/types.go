package llm

import "time"

// Config is the configuration for an LLM endpoint that speaks the OpenAI
// compatible Chat Completions protocol (e.g. One-API, LiteLLM, Azure OpenAI
// gateway, Cloudflare AI Gateway, or any direct OpenAI-compatible provider).
//
// Configuration precedence when used from `helmfile doctor`:
//
//	env < helmfile.yaml (llm: block) < --llm-* flag
//
// BaseURL is OPTIONAL: when empty, the client falls back to OpenAI's official
// endpoint (https://api.openai.com/v1). Set BaseURL only when targeting a
// gateway or non-OpenAI provider. APIKey + Model are always required.
//
// If APIKey or Model is empty the LLM is considered unconfigured and
// `helmfile doctor` degrades to plain `helmfile diff`.
type Config struct {
	// BaseURL is the OpenAI-compatible endpoint base URL, e.g.
	// "https://one-api.internal/v1" or "https://api.deepseek.com/v1".
	// When empty, defaults to "https://api.openai.com/v1".
	BaseURL string `yaml:"baseURL,omitempty"`

	// APIKey authenticates against BaseURL. May be templated in helmfile.yaml
	// via {{ env "KEY" }}.
	APIKey string `yaml:"apiKey,omitempty"`

	// Model is the chat completion model identifier, e.g. "gpt-4o",
	// "claude-3-5-sonnet" (via gateway), "deepseek-chat".
	Model string `yaml:"model,omitempty"`

	// Timeout is the per-request timeout. Defaults to 60s when zero.
	Timeout time.Duration `yaml:"timeout,omitempty"`

	// MaxTokens caps the completion length. Defaults to 4096 when zero.
	MaxTokens int `yaml:"maxTokens,omitempty"`

	// Temperature controls generation randomness. Defaults to 0.2 when zero
	// (deterministic-ish for risk analysis).
	Temperature float32 `yaml:"temperature,omitempty"`
}

// IsConfigured reports whether enough information is present to call the LLM.
// Helmfile uses this to decide whether `doctor` should run AI analysis or
// degrade to plain `diff`.
//
// BaseURL is intentionally NOT required: a user with an OpenAI account only
// needs APIKey + Model. The openai client falls back to OpenAI's official
// endpoint when BaseURL is empty.
func (c Config) IsConfigured() bool {
	return c.APIKey != "" && c.Model != ""
}

// WithDefaults returns a copy with zero values replaced by sane defaults.
func (c Config) WithDefaults() Config {
	out := c
	if out.Timeout == 0 {
		out.Timeout = 60 * time.Second
	}
	if out.MaxTokens == 0 {
		out.MaxTokens = 4096
	}
	if out.Temperature == 0 {
		out.Temperature = 0.2
	}
	return out
}

// Merge returns a copy of `base` overwritten by the non-zero fields of `override`.
// Used to implement the `env < yaml < flag` precedence: each layer merges on top
// of the previous one, and only explicit non-zero values override.
//
// ZERO-VALUE SEMANTICS: because Merge treats "field == zero value" as
// "field not set by this layer", it is IMPOSSIBLE to use a flag to force a
// field back to its zero value. Concretely:
//
//   - `--llm-max-tokens 0` does NOT reset MaxTokens to 0; it is treated as
//     "flag not set" and the env/yaml value wins. Use the YAML block to
//     explicitly request "0" if you really want it (although the openai
//     client treats 0 as "use the library default", so this is rarely useful).
//   - `--llm-timeout 0s` is similarly a no-op.
//
// This is a deliberate trade-off: it matches helmfile's existing flag-override
// convention (see e.g. --concurrency, --context) and keeps the merge logic
// branch-free. If a future use case requires forcing zeros, switch to
// *string / *int pointers throughout.
func Merge(base, override Config) Config {
	out := base
	if override.BaseURL != "" {
		out.BaseURL = override.BaseURL
	}
	if override.APIKey != "" {
		out.APIKey = override.APIKey
	}
	if override.Model != "" {
		out.Model = override.Model
	}
	if override.Timeout != 0 {
		out.Timeout = override.Timeout
	}
	if override.MaxTokens != 0 {
		out.MaxTokens = override.MaxTokens
	}
	if override.Temperature != 0 {
		out.Temperature = override.Temperature
	}
	return out
}

// RiskLevel is the severity assigned by the LLM to a detected risk.
type RiskLevel string

const (
	RiskLevelLow    RiskLevel = "low"
	RiskLevelMedium RiskLevel = "medium"
	RiskLevelHigh   RiskLevel = "high"
)

// Risk is a single concern the LLM flagged while reviewing the diff.
type Risk struct {
	// Level is low|medium|high.
	Level RiskLevel `json:"level" yaml:"level"`
	// Category is a short stable tag like "data-loss", "security",
	// "breaking-change", "performance", "best-practice".
	Category string `json:"category" yaml:"category"`
	// Description explains the risk in 1-3 sentences.
	Description string `json:"description" yaml:"description"`
	// Suggestion is an actionable mitigation step (may be empty).
	Suggestion string `json:"suggestion,omitempty" yaml:"suggestion,omitempty"`
}

// Analysis is the structured output produced by the LLM.
type Analysis struct {
	// Summary is a single-paragraph summary of the whole diff.
	Summary string `json:"summary" yaml:"summary"`
	// Risks is the list of risks ordered by severity (high first).
	Risks []Risk `json:"risks" yaml:"risks"`
	// AffectedResources lists Kubernetes objects the LLM identified as
	// impacted (e.g. "Deployment/foo", "PersistentVolume/bar").
	AffectedResources []string `json:"affected_resources,omitempty" yaml:"affected_resources,omitempty"`
}

// HasHighRisk reports whether the analysis contains any high-severity risk.
// Used by `helmfile doctor` to decide whether to block (exit code 2) in CI.
func (a Analysis) HasHighRisk() bool {
	for _, r := range a.Risks {
		if r.Level == RiskLevelHigh {
			return true
		}
	}
	return false
}
