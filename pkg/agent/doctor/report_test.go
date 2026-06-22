package doctor

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/helmfile/helmfile/pkg/agent/llm"
)

func TestParseFormat(t *testing.T) {
	tests := map[string]Format{
		"":         FormatText,
		"text":     FormatText,
		"markdown": FormatText,
		"md":       FormatText,
		"TEXT":     FormatText,
		"json":     FormatJSON,
		"JSON":     FormatJSON,
		"garbage":  FormatText, // unknown → safe default
	}
	for in, want := range tests {
		if got := ParseFormat(in); got != want {
			t.Errorf("ParseFormat(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestReportText_EmptyResultProducesEmptyString(t *testing.T) {
	if got := ReportText(Result{}); got != "" {
		t.Errorf("ReportText(empty) = %q, want empty", got)
	}
}

func TestReportText_UnconfiguredEchoesDiff(t *testing.T) {
	r := Result{RawDiff: "raw diff body"}
	if got := ReportText(r); got != "raw diff body" {
		t.Errorf("ReportText(unconfigured) = %q, want raw diff verbatim", got)
	}
}

func TestReportText_LLMFailureBannerAndDiff(t *testing.T) {
	r := Result{
		RawDiff:       "diff contents",
		LLMCallFailed: true,
		LLMError:      errors.New("upstream 503"),
	}
	got := ReportText(r)
	for _, want := range []string{"degraded", "503", "diff contents"} {
		if !strings.Contains(got, want) {
			t.Errorf("ReportText(failed) missing %q\nGot:\n%s", want, got)
		}
	}
}

func TestReportText_HighRiskEmojiAndOrder(t *testing.T) {
	r := Result{
		Analysis: &llm.Analysis{
			Summary: "Two risks found.",
			Risks: []llm.Risk{
				{Level: llm.RiskLevelLow, Category: "best-practice", Description: "l", Suggestion: "s"},
				{Level: llm.RiskLevelHigh, Category: "data-loss", Description: "d", Suggestion: "fix"},
			},
		},
		Model: "gpt-4o",
	}
	got := ReportText(r)
	for _, want := range []string{"HIGH", "data-loss", "fix", "gpt-4o"} {
		if !strings.Contains(got, want) {
			t.Errorf("ReportText missing %q\nGot:\n%s", want, got)
		}
	}
	if !strings.Contains(got, "🔴") {
		t.Errorf("expected red circle emoji for high risk")
	}
}

func TestReportText_NoRisksShowsFriendlyLine(t *testing.T) {
	r := Result{Analysis: &llm.Analysis{Summary: "all clear"}}
	got := ReportText(r)
	if !strings.Contains(got, "No risks identified") {
		t.Errorf("expected no-risks line; got:\n%s", got)
	}
}

func TestReportJSON_RoundTripsFields(t *testing.T) {
	r := Result{
		RawDiff: "diff",
		Analysis: &llm.Analysis{
			Summary: "summary",
			Risks:   []llm.Risk{{Level: llm.RiskLevelHigh, Category: "security", Description: "d"}},
		},
		Model:           "m",
		SecretsRedacted: 7,
	}
	out := ReportJSON(r)

	var parsed map[string]any
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("ReportJSON did not produce valid JSON: %v\nraw=%s", err, out)
	}
	// The diff field was renamed from raw_diff → diff so users don't assume
	// they're seeing the raw pre-redaction content.
	if parsed["diff"] != "diff" {
		t.Errorf("diff = %v, want \"diff\"", parsed["diff"])
	}
	if parsed["raw_diff"] != nil {
		t.Errorf("raw_diff field should be gone, got %v", parsed["raw_diff"])
	}
	if parsed["summary"] != "summary" {
		t.Errorf("summary = %v", parsed["summary"])
	}
	risks, ok := parsed["risks"].([]any)
	if !ok || len(risks) != 1 {
		t.Fatalf("risks missing or wrong shape: %v", parsed["risks"])
	}
	first := risks[0].(map[string]any)
	if first["level"] != "high" {
		t.Errorf("first risk level = %v, want high", first["level"])
	}
	// secrets_redacted is always present so users can tell redaction ran even
	// when the count is zero. (It was conditional in v1; this is the v2 shape.)
	if parsed["secrets_redacted"] != float64(7) {
		t.Errorf("secrets_redacted = %v, want 7", parsed["secrets_redacted"])
	}
}

// TestReportJSON_EmptyRisksRendersAsEmptyArray is a regression guard against
// a bug where Analysis.Risks being empty caused the "risks" field to vanish
// from the JSON output entirely. Go's `omitempty` on a slice treats
// `[]Risk{}` (non-nil empty) and `nil` identically — both are dropped — so
// users couldn't distinguish "LLM said no risks" from "analysis never ran".
//
// The fix uses `*[]llm.Risk` (pointer) so that:
//   - Analysis present + zero risks → `"risks": []`
//   - Analysis absent (unconfigured) → field omitted
//
// Both cases must be testable in CI JSON consumers without ambiguity.
// TestReportText_UnknownRiskLevelDefault covers the levelEmoji default
// branch (risk with an unrecognized level string). Previously at 0% coverage
// because all tests used high/medium/low.
func TestReportText_UnknownRiskLevelDefault(t *testing.T) {
	r := Result{
		Analysis: &llm.Analysis{
			Summary: "weird risk",
			Risks: []llm.Risk{
				{Level: llm.RiskLevel("critical"), Category: "unknown", Description: "d"},
			},
		},
		Model: "m",
	}
	got := ReportText(r)
	// Unknown level → ⚪ (default emoji).
	if !strings.Contains(got, "⚪") {
		t.Errorf("expected ⚪ emoji for unknown risk level; got:\n%s", got)
	}
	// The level is uppercased in the header: [CRITICAL].
	if !strings.Contains(got, "[CRITICAL]") {
		t.Errorf("expected uppercased level in header; got:\n%s", got)
	}
}

func TestReportJSON_EmptyRisksRendersAsEmptyArray(t *testing.T) {
	// Case A: Analysis present, zero risks → must show "risks": []
	r := Result{
		RawDiff:  "diff body",
		Analysis: &llm.Analysis{Summary: "no risks found", Risks: nil},
		Model:    "gpt-4o",
	}
	out := ReportJSON(r)

	var parsed map[string]any
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v\nraw=%s", err, out)
	}
	risks, ok := parsed["risks"]
	if !ok {
		t.Fatalf("risks field MISSING from JSON; output was: %s", out)
	}
	risksArr, ok := risks.([]any)
	if !ok {
		t.Fatalf("risks field is not an array; got %T: %v", risks, risks)
	}
	if len(risksArr) != 0 {
		t.Errorf("risks should be empty array; got %v", risksArr)
	}

	// Case B: Analysis absent (unconfigured path) → field omitted
	r2 := Result{RawDiff: "diff body"}
	out2 := ReportJSON(r2)
	var parsed2 map[string]any
	if err := json.Unmarshal([]byte(out2), &parsed2); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := parsed2["risks"]; ok {
		t.Errorf("risks should be omitted when Analysis is nil; got %v", parsed2["risks"])
	}
}

// TestReportJSON_FieldOrderIsHumanReadable locks in the typed-struct field
// order. The previous map[string]any implementation alphabetized, putting
// "duration" before "model" and "raw_diff" first. Users complained the JSON
// was hard to scan in CI logs. The struct preserves declaration order.
func TestReportJSON_FieldOrderIsHumanReadable(t *testing.T) {
	r := Result{
		RawDiff:         "diff body",
		SecretsRedacted: 2,
		Model:           "gpt-4o",
		Analysis:        &llm.Analysis{Summary: "s", Risks: []llm.Risk{{Level: llm.RiskLevelLow}}},
	}
	out := ReportJSON(r)

	// Field order in the JSON output must be:
	//   summary, risks, diff, secrets_redacted, model, duration, timestamp
	// (Declaration order of the reportJSON struct, modulo omitempty.)
	wantOrder := []string{"summary", "risks", "diff", "secrets_redacted", "model", "duration", "timestamp"}
	pos := map[string]int{}
	for i, key := range wantOrder {
		pos[key] = i
	}

	// Extract keys in the order they appear in the JSON.
	// Cheap way: scan line-by-line, the first 7 non-space chars of each line
	// are `"key":`.
	keys := []string{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, `"`) {
			continue
		}
		end := strings.Index(line, `":`)
		if end < 0 {
			continue
		}
		keys = append(keys, line[1:end])
	}

	// Verify each expected key appears, in monotonic order.
	lastIdx := -1
	for _, k := range wantOrder {
		foundAt := -1
		for i, kk := range keys {
			if kk == k {
				foundAt = i
				break
			}
		}
		if foundAt < 0 {
			t.Errorf("expected key %q in JSON output; keys seen: %v", k, keys)
			continue
		}
		if foundAt < lastIdx {
			t.Errorf("key %q appeared at position %d, but a prior expected key was at %d — field order regressed",
				k, foundAt, lastIdx)
		}
		lastIdx = foundAt
		_ = pos
	}
}

func TestReportJSON_LLMErrorIncluded(t *testing.T) {
	r := Result{
		RawDiff:       "x",
		LLMCallFailed: true,
		LLMError:      errors.New("timeout"),
	}
	out := ReportJSON(r)
	var parsed map[string]any
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if parsed["llm_error"] != "timeout" {
		t.Errorf("llm_error = %v, want timeout", parsed["llm_error"])
	}
}
