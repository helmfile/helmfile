package llm

import goContext "context"

// Client is the abstraction used by `helmfile doctor` to talk to whatever
// OpenAI-compatible backend the user configured. The default implementation
// is the OpenAI client (see openai.go). Tests inject a mock implementation.
type Client interface {
	// Analyze asks the LLM to review the given helm diff and return a
	// structured Analysis. ctx is used for cancellation/timeout.
	Analyze(ctx goContext.Context, diff string, extras AnalyzeInput) (Analysis, error)
}

// AnalyzeInput carries optional context that helps the LLM produce a more
// grounded analysis (release names, environment name, etc.). All fields are
// optional.
type AnalyzeInput struct {
	// Environment is the helmfile --environment value (e.g. "prod").
	Environment string
	// Releases is the list of release names that appear in the diff.
	Releases []string
}

// NewClient returns a Client backed by the OpenAI Chat Completions protocol.
// Returns nil when cfg.IsConfigured() is false so callers can short-circuit
// to the plain-diff fallback path.
func NewClient(cfg Config) Client {
	if !cfg.IsConfigured() {
		return nil
	}
	return newOpenAIClient(cfg.WithDefaults())
}
