package state

import (
	"strings"
	"testing"
	"time"

	"github.com/helmfile/helmfile/pkg/agent/llm"
	yaml "github.com/helmfile/helmfile/pkg/yaml"
)

// TestReleaseSetSpec_LLMYamlRoundtrip is a regression guard for the
// ReleaseSetSpec.LLM yaml tag. Without this test, a typo in the tag (e.g.
// `yaml:"LLM"` or `yaml:"ai"`) would silently disable every helmfile.yaml
// `llm:` block in production. The tag lives at pkg/state/state.go:ReleaseSetSpec.
//
// Cases covered:
//
//   - Full llm: block unmarshals correctly into ReleaseSetSpec.LLM
//   - HelmState.UnmarshalYAML (the alias-based path used by the real loader)
//     also picks up the LLM field — guards against the alias stripping it.
//   - Missing llm: block leaves ReleaseSetSpec.LLM as zero-value Config.
//   - `llm: {}` (explicit empty) is equivalent to missing.
//   - Partial llm: block (only APIKey + Model) sets only those fields.
//
// We do NOT test template rendering ({{ env "KEY" }}) here — that is helmfile
// core two-pass renderer behavior, exercised in pkg/app tests.
func TestReleaseSetSpec_LLMYamlRoundtrip(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		want    llm.Config
		wantSet bool // whether want is fully populated (vs zero)
	}{
		{
			name: "full llm block",
			yaml: `
releases:
- name: myapp
  chart: mychart
llm:
  baseURL: https://one-api.example/v1
  apiKey: sk-test-123
  model: gpt-4o
  timeout: 90s
  maxTokens: 8192
  temperature: 0.5
`,
			want: llm.Config{
				BaseURL:     "https://one-api.example/v1",
				APIKey:      "sk-test-123",
				Model:       "gpt-4o",
				Timeout:     90 * time.Second,
				MaxTokens:   8192,
				Temperature: 0.5,
			},
			wantSet: true,
		},
		{
			name: "minimal llm block (OpenAI official user, no baseURL)",
			yaml: `
llm:
  apiKey: sk-...
  model: gpt-4o
`,
			want: llm.Config{
				APIKey: "sk-...",
				Model:  "gpt-4o",
			},
			wantSet: true,
		},
		{
			name: "missing llm block",
			yaml: `
releases:
- name: myapp
  chart: mychart
`,
			want:    llm.Config{},
			wantSet: false,
		},
		{
			name: "empty llm block",
			yaml: `
releases:
- name: myapp
  chart: mychart
llm: {}
`,
			want:    llm.Config{},
			wantSet: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use HelmState directly — this exercises the alias-based
			// UnmarshalYAML the production loader goes through.
			var st HelmState
			if err := yaml.Unmarshal([]byte(tt.yaml), &st); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}
			got := st.LLM
			if got != tt.want {
				t.Errorf("LLM mismatch:\n got  = %+v\n want = %+v", got, tt.want)
			}
			// ReleaseSpec sanity: when the yaml contained releases, they must
			// unmarshal independently of the llm: block presence (the two
			// fields must not interfere with each other).
			if strings.Contains(tt.yaml, "releases:") && len(st.Releases) == 0 {
				t.Errorf("Releases empty despite yaml containing a releases: block")
			}
			// IsConfigured must agree with wantSet.
			if got.IsConfigured() != tt.wantSet {
				t.Errorf("IsConfigured = %v, want %v", got.IsConfigured(), tt.wantSet)
			}
		})
	}
}

// TestReleaseSetSpec_LLMFieldDoesNotLeakIntoSubhelmfile verifies that the
// llm: block lives on ReleaseSetSpec (not on HelmState unexported fields),
// so the field is reachable from any code holding a *HelmState pointer via
// the embedded spec.
func TestReleaseSetSpec_LLMFieldDoesNotLeakIntoSubhelmfile(t *testing.T) {
	const src = `
llm:
  apiKey: k
  model: m
`
	var st HelmState
	if err := yaml.Unmarshal([]byte(src), &st); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	// Reach the field through every name by which downstream code might see it.
	if !st.LLM.IsConfigured() {
		t.Errorf("HelmState.LLM not configured; field not promoted from ReleaseSetSpec?")
	}
	if !st.ReleaseSetSpec.LLM.IsConfigured() {
		t.Errorf("ReleaseSetSpec.LLM not configured; yaml tag not pointing at the right field?")
	}
}
