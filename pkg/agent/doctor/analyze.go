// Package doctor orchestrates the `helmfile doctor` flow: capture helm diff
// output, hand it to an LLM via the OpenAI-compatible Chat Completions
// protocol, then render a structured risk report.
//
// When no LLM is configured (APIKey or Model missing), doctor degrades to
// plain `helmfile diff` with zero behavior change.
package doctor

import (
	goContext "context"
	"time"

	"github.com/helmfile/helmfile/pkg/agent/llm"
)

// Result bundles everything `helmfile doctor` needs to render its output and
// decide on an exit code.
type Result struct {
	// Analysis is the structured LLM output. Nil when the LLM was not called
	// (diff empty) or when LLMCallFailed is set.
	Analysis *llm.Analysis
	// RawDiff is the helm diff text AFTER secret redaction. Doctor never
	// exposes unredacted secret content through Result.
	RawDiff string
	// SecretsRedacted counts secret-looking values stripped from RawDiff.
	// Surfaced in the report footer so users can spot unexpected leaks.
	SecretsRedacted int
	// LLMCallFailed indicates the LLM was configured but the call failed;
	// the caller should degrade to printing RawDiff with a warning.
	LLMCallFailed bool
	// LLMError is the underlying LLM error when LLMCallFailed is true.
	LLMError error
	// Model is the model identifier used (for the report footer).
	Model string
	// Duration is how long the LLM call took.
	Duration time.Duration
}

// HasHighRisk delegates to Analysis.HasHighRisk when an analysis exists.
func (r Result) HasHighRisk() bool {
	return r.Analysis != nil && r.Analysis.HasHighRisk()
}

// Options controls a single Analyze run.
type Options struct {
	// Client is the LLM client. When nil, Analyze returns a Result that
	// signals "no LLM" so the caller can degrade to plain diff.
	Client llm.Client
	// Environment is the --environment value (may be empty).
	Environment string
	// Releases is the list of release names that appear in the diff (may be nil).
	Releases []string
	// Model is the model identifier; surfaced in the report footer.
	Model string
	// Redactor controls secret redaction. A zero-value SecretRedactor is
	// fine: its Redact method falls back to "<REDACTED>". Redaction is
	// ALWAYS applied — there is no opt-out at this layer.
	Redactor SecretRedactor
}

// Analyze runs the full doctor pipeline against the given diff text.
//
// Behavior:
//   - Empty diff → empty Result (caller should print nothing).
//   - diff is ALWAYS redacted via opts.Redactor first. RawDiff in the
//     returned Result is the redacted text; the original is discarded.
//   - nil Client → Result with RawDiff only (the "unconfigured" path).
//     Redaction still happens — pipes downstream shouldn't see raw secrets.
//   - Non-nil Client → calls LLM; on error returns LLMCallFailed=true.
func Analyze(ctx goContext.Context, diff string, opts Options) Result {
	if diff == "" {
		return Result{}
	}

	redacted, redactionCount := opts.Redactor.Redact(diff)

	if opts.Client == nil {
		return Result{
			RawDiff:         redacted,
			SecretsRedacted: redactionCount,
		}
	}

	start := time.Now()
	a, err := opts.Client.Analyze(ctx, redacted, llm.AnalyzeInput{
		Environment: opts.Environment,
		Releases:    opts.Releases,
	})
	duration := time.Since(start)

	if err != nil {
		return Result{
			RawDiff:         redacted,
			SecretsRedacted: redactionCount,
			LLMCallFailed:   true,
			LLMError:        err,
			Model:           opts.Model,
			Duration:        duration,
		}
	}

	return Result{
		Analysis:        &a,
		RawDiff:         redacted,
		SecretsRedacted: redactionCount,
		Model:           opts.Model,
		Duration:        duration,
	}
}
