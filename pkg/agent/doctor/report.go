package doctor

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/helmfile/helmfile/pkg/agent/llm"
)

// Format is the report rendering format selected via `helmfile doctor --output`.
type Format string

const (
	// FormatText renders a human-readable markdown report (the default).
	FormatText Format = "text"
	// FormatJSON renders the full Result as JSON, including the diff and model
	// metadata. The diff is always post-redaction. Suitable for CI pipelines
	// that want to post-process.
	FormatJSON Format = "json"
)

// ParseFormat parses the --output value. Accepts "text"/"markdown" and "json".
// Unknown values default to FormatText so a typo never breaks CI.
func ParseFormat(s string) Format {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "json":
		return FormatJSON
	case "markdown", "md":
		return FormatText
	default:
		return FormatText
	}
}

// ReportJSON renders Result as JSON. Always succeeds; if r.Analysis is nil
// the JSON still carries Diff and metadata.
//
// Field shape (stable, ordered for human readability):
//
//	{
//	  "summary": "...",            # only when Analysis is present
//	  "risks": [...],              # only when Analysis is present; EMPTY array
//	                               # when Analysis.Risks is nil/empty — NOT
//	                               # omitted, because CI consumers want to
//	                               # distinguish "LLM said no risks" from
//	                               # "field missing entirely".
//	  "affected_resources": [...], # only when non-empty (omitempty is fine
//	                               # here: an empty list and a missing list
//	                               # are semantically equivalent).
//	  "diff": "...",               # the diff, ALWAYS post-redaction
//	  "secrets_redacted": N,       # always present so users can tell
//	  "model": "gpt-4o",
//	  "duration": "8.2s",
//	  "timestamp": "2026-...",
//	  "llm_error": "..."           # only when LLMCallFailed
//	}
//
// The "diff" field is the diff AFTER secret redaction. We never expose the
// raw, pre-redaction diff through this API — doctor's safety contract is
// that no secret can leave the process via stdout/JSON.
//
// Implementation note: `Risks` is a *[]llm.Risk pointer specifically because
// Go's `omitempty` on a slice treats `[]Risk{}` (non-nil empty) and `nil`
// identically — both are dropped. Using a pointer lets us distinguish:
//
//   - Analysis present, no risks    → non-nil pointer to empty slice → `"risks":[]`
//   - Analysis absent (unconfigured) → nil pointer                     → field omitted
//
// This was a real bug: users got JSON without a `risks` field when the LLM
// happily reported "no risks", which looked identical to "analysis never
// happened". See TestReportJSON_EmptyRisks RendersAsEmptyArray.
func ReportJSON(r Result) string {
	out := reportJSON{
		Diff:            r.RawDiff,
		SecretsRedacted: r.SecretsRedacted,
		Model:           r.Model,
		Duration:        r.Duration.String(),
		Timestamp:       time.Now().UTC().Format(time.RFC3339),
	}
	if r.Analysis != nil {
		out.Summary = r.Analysis.Summary
		risks := r.Analysis.Risks
		if risks == nil {
			risks = []llm.Risk{}
		}
		out.Risks = &risks
		out.AffectedResources = r.Analysis.AffectedResources
	}
	if r.LLMCallFailed && r.LLMError != nil {
		out.LLMError = r.LLMError.Error()
	}

	b, err := json.MarshalIndent(out, "", "  ")
	// json.MarshalIndent on a struct of string/int/*[]Risk cannot fail in
	// practice — the only way to error would be an unsupported type, and
	// every field here is a primitive or a known slice. We panic on the
	// impossible case rather than emit malformed JSON: a silent fallback
	// would hide a programming error from the user.
	if err != nil {
		panic(fmt.Sprintf("doctor: reportJSON marshal failed (impossible for typed struct): %v", err))
	}
	return string(b)
}

// reportJSON is the typed shape used by ReportJSON. Field order here is the
// field order in the JSON output (Go's encoding/json preserves struct field
// order, unlike map[string]any which is alphabetized).
type reportJSON struct {
	Summary           string      `json:"summary,omitempty"`
	Risks             *[]llm.Risk `json:"risks,omitempty"`
	AffectedResources []string    `json:"affected_resources,omitempty"`
	Diff              string      `json:"diff"`
	SecretsRedacted   int         `json:"secrets_redacted"`
	Model             string      `json:"model,omitempty"`
	Duration          string      `json:"duration"`
	Timestamp         string      `json:"timestamp"`
	LLMError          string      `json:"llm_error,omitempty"`
}

// ReportText renders Result as markdown. Designed to be readable in a CI log
// and in an interactive terminal.
//
// Layout:
//
//	# Helmfile Doctor Report
//	## Summary
//	<one paragraph>
//	## Risks
//	### <emoji> [LEVEL] Category
//	Description...
//	Suggestion: ...
//	## Affected Resources
//	- ...
//	---
//	Model: gpt-4o | Duration: 8.2s | Secrets redacted: 3
//
// When the LLM call failed, falls back to printing the (redacted) diff with
// a warning banner so the caller never loses the diff content.
// When no LLM is configured, echoes the (redacted) diff verbatim.
//
// In ALL code paths the diff is post-redaction — doctor never prints
// pre-redaction diff content to stdout.
func ReportText(r Result) string {
	if r.RawDiff == "" && r.Analysis == nil {
		return ""
	}

	var b strings.Builder

	// LLM failure: print the redacted diff under a warning banner and bail out.
	if r.LLMCallFailed {
		b.WriteString("# Helmfile Doctor (degraded)\n\n")
		fmt.Fprintf(&b, "> ⚠️  LLM analysis failed: %v\n", r.LLMError)
		b.WriteString("> Falling back to redacted diff output.\n")
		fmt.Fprintf(&b, "> 🔒 %d secret(s) redacted.\n", r.SecretsRedacted)
		b.WriteString("\n```\n")
		b.WriteString(r.RawDiff)
		b.WriteString("\n```\n")
		return b.String()
	}

	// Unconfigured path: there is no Analysis, just echo the redacted diff.
	if r.Analysis == nil {
		return r.RawDiff
	}

	b.WriteString("# Helmfile Doctor Report\n\n")

	b.WriteString("## Summary\n\n")
	if r.Analysis.Summary == "" {
		b.WriteString("_No summary produced._\n\n")
	} else {
		b.WriteString(r.Analysis.Summary)
		b.WriteString("\n\n")
	}

	risks := r.Analysis.Risks
	if len(risks) == 0 {
		b.WriteString("## Risks\n\n_No risks identified._\n\n")
	} else {
		b.WriteString("## Risks\n\n")
		for _, risk := range risks {
			writeRiskSection(&b, risk)
		}
	}

	if len(r.Analysis.AffectedResources) > 0 {
		b.WriteString("## Affected Resources\n\n")
		for _, res := range r.Analysis.AffectedResources {
			fmt.Fprintf(&b, "- %s\n", res)
		}
		b.WriteString("\n")
	}

	b.WriteString("---\n")
	footer := []string{}
	if r.Model != "" {
		footer = append(footer, "Model: "+r.Model)
	}
	footer = append(footer, "Duration: "+roundDuration(r.Duration))
	// Always show redaction count (even 0) so users can confirm the redactor
	// ran. Matches ReportJSON which always includes secrets_redacted.
	footer = append(footer, fmt.Sprintf("Secrets redacted: %d", r.SecretsRedacted))
	b.WriteString(strings.Join(footer, " | "))
	b.WriteString("\n")
	return b.String()
}

func writeRiskSection(b *strings.Builder, risk llm.Risk) {
	fmt.Fprintf(b, "### %s [%s] %s\n\n", levelEmoji(string(risk.Level)), strings.ToUpper(string(risk.Level)), risk.Category)
	if risk.Description != "" {
		fmt.Fprintf(b, "%s\n\n", risk.Description)
	}
	if risk.Suggestion != "" {
		fmt.Fprintf(b, "**Suggestion:** %s\n\n", risk.Suggestion)
	}
}

func levelEmoji(level string) string {
	switch strings.ToLower(level) {
	case "high":
		return "🔴"
	case "medium":
		return "🟡"
	case "low":
		return "🟢"
	}
	return "⚪"
}

func roundDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}
