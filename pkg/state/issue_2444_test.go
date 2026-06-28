package state

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestKubeconfigPassedToChartify verifies that when --kubeconfig is set,
// it is passed to chartify's internal helm template call.
// This is a regression test for issue #2444.
//
// Background: When using jsonPatches or kustomize patches with helmfile,
// chartify runs "helm template" internally to render the chart before applying patches.
// The lookup() helm function requires cluster access (--dry-run=server).
// Without --kubeconfig being passed to the internal helm template call,
// it fails to connect to the cluster when the user's kubeconfig is not in the default location.
//
// These tests exercise the real HelmState.buildChartifyTemplateArgs method (the pure
// helper that processChartification delegates to), not a duplicated copy of the logic.
func TestKubeconfigPassedToChartify(t *testing.T) {
	tests := []struct {
		name            string
		helmfileCommand string
		kubeconfig      string
		kubeContext     string
		expectedFlags   []string
		unexpectedFlags []string
	}{
		{
			name:            "sync with kubeconfig should pass both kubeconfig and dry-run=server",
			helmfileCommand: "sync",
			kubeconfig:      "/path/to/kubeconfig",
			kubeContext:     "",
			expectedFlags:   []string{"--kubeconfig", "/path/to/kubeconfig", "--dry-run=server"},
			unexpectedFlags: []string{},
		},
		{
			name:            "sync with kubeconfig and kube-context should pass both",
			helmfileCommand: "sync",
			kubeconfig:      "/path/to/kubeconfig",
			kubeContext:     "my-context",
			expectedFlags:   []string{"--kubeconfig", "/path/to/kubeconfig", "--kube-context", "my-context", "--dry-run=server"},
			unexpectedFlags: []string{},
		},
		{
			name:            "apply with kubeconfig should pass kubeconfig",
			helmfileCommand: "apply",
			kubeconfig:      "/custom/kubeconfig",
			kubeContext:     "",
			expectedFlags:   []string{"--kubeconfig", "/custom/kubeconfig", "--dry-run=server"},
			unexpectedFlags: []string{},
		},
		{
			name:            "diff with kubeconfig should pass kubeconfig",
			helmfileCommand: "diff",
			kubeconfig:      "/etc/kubeconfig",
			kubeContext:     "prod",
			expectedFlags:   []string{"--kubeconfig", "/etc/kubeconfig", "--kube-context", "prod", "--dry-run=server"},
			unexpectedFlags: []string{},
		},
		{
			name:            "template command should not pass kubeconfig (offline command)",
			helmfileCommand: "template",
			kubeconfig:      "/path/to/kubeconfig",
			kubeContext:     "",
			expectedFlags:   []string{},
			unexpectedFlags: []string{"--kubeconfig", "--dry-run=server"},
		},
		{
			name:            "build command should not pass kubeconfig (offline command)",
			helmfileCommand: "build",
			kubeconfig:      "/path/to/kubeconfig",
			kubeContext:     "",
			expectedFlags:   []string{},
			unexpectedFlags: []string{"--kubeconfig", "--dry-run=server"},
		},
		{
			name:            "no kubeconfig should not add kubeconfig flag",
			helmfileCommand: "sync",
			kubeconfig:      "",
			kubeContext:     "my-context",
			expectedFlags:   []string{"--kube-context", "my-context", "--dry-run=server"},
			unexpectedFlags: []string{"--kubeconfig"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := &HelmState{kubeconfig: tt.kubeconfig}
			got := st.buildChartifyTemplateArgs(tt.helmfileCommand, tt.kubeContext, false, "", "")

			for _, flag := range tt.expectedFlags {
				assert.Truef(t, strings.Contains(got, flag),
					"buildChartifyTemplateArgs() = %q; want to contain %q", got, flag)
			}

			for _, flag := range tt.unexpectedFlags {
				assert.Falsef(t, strings.Contains(got, flag),
					"buildChartifyTemplateArgs() = %q; want NOT to contain %q", got, flag)
			}
		})
	}
}

// TestKubeconfigNotDuplicated verifies that kubeconfig is not duplicated
// when it already exists in the template args.
func TestKubeconfigNotDuplicated(t *testing.T) {
	tests := []struct {
		name             string
		helmfileCommand  string
		kubeconfig       string
		existingArgs     string
		expectedCount    int
		expectedContains string
	}{
		{
			name:             "do not duplicate kubeconfig",
			helmfileCommand:  "sync",
			kubeconfig:       "/path/to/kubeconfig",
			existingArgs:     "--kubeconfig /existing/kubeconfig",
			expectedCount:    1,
			expectedContains: "--kubeconfig /existing/kubeconfig",
		},
		{
			name:             "add kubeconfig when not present",
			helmfileCommand:  "sync",
			kubeconfig:       "/path/to/kubeconfig",
			existingArgs:     "--some-flag",
			expectedCount:    1,
			expectedContains: "--kubeconfig /path/to/kubeconfig",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := &HelmState{kubeconfig: tt.kubeconfig}
			got := st.buildChartifyTemplateArgs(tt.helmfileCommand, "", false, "", tt.existingArgs)

			assert.Truef(t, strings.Contains(got, tt.expectedContains),
				"buildChartifyTemplateArgs() = %q; want to contain %q", got, tt.expectedContains)

			assert.Equalf(t, tt.expectedCount, strings.Count(got, "--kubeconfig"),
				"buildChartifyTemplateArgs() has --kubeconfig %d times; want %d", strings.Count(got, "--kubeconfig"), tt.expectedCount)
		})
	}
}

// TestTemplateArgsDryRunTriggersKubeInjection verifies that when the user passes
// --template-args="--dry-run=server" to an offline command (e.g. `helmfile template`),
// the kubeconfig and kube-context ARE injected into chartify's internal helm template
// so lookup() can actually reach the cluster. Regression test for issue #1833.
func TestTemplateArgsDryRunTriggersKubeInjection(t *testing.T) {
	st := &HelmState{kubeconfig: "/path/to/kubeconfig"}

	got := st.buildChartifyTemplateArgs("template", "my-context", false, "--dry-run=server", "")

	// User-provided arg is preserved
	assert.Contains(t, got, "--dry-run=server")
	// Cluster connection flags are injected even though "template" is offline
	assert.Contains(t, got, "--kubeconfig /path/to/kubeconfig")
	assert.Contains(t, got, "--kube-context my-context")
	// --dry-run=server is NOT duplicated (only the user's copy is present)
	assert.Equal(t, 1, strings.Count(got, "--dry-run"))
}

// TestTemplateArgsMergedBeforeInjection verifies that user-provided template args
// are merged into chartify's existing template args before the cluster-connectivity
// injection, so that duplicate --dry-run / --kubeconfig flags are deduplicated.
func TestTemplateArgsMergedBeforeInjection(t *testing.T) {
	st := &HelmState{kubeconfig: "/path/to/kubeconfig"}

	got := st.buildChartifyTemplateArgs(
		"sync", "ctx", false,
		"--dry-run=server", // user arg
		"--enable-dns",     // existing chartify arg
	)

	assert.Contains(t, got, "--enable-dns")
	assert.Contains(t, got, "--dry-run=server")
	// kubeconfig injected once, dry-run appears once (not duplicated)
	assert.Equal(t, 1, strings.Count(got, "--dry-run"))
	assert.Equal(t, 1, strings.Count(got, "--kubeconfig"))
}

// TestEffectiveTemplateArgs verifies CLI args take precedence over helmDefaults.templateArgs,
// mirroring the DiffArgs precedence (switch: CLI wins, else helmDefaults, else empty).
func TestEffectiveTemplateArgs(t *testing.T) {
	tests := []struct {
		name         string
		cliArgs      string
		helmDefaults []string
		want         string
	}{
		{
			name:         "CLI args win over helmDefaults",
			cliArgs:      "--dry-run=server",
			helmDefaults: []string{"--enable-dns"},
			want:         "--dry-run=server",
		},
		{
			name:         "helmDefaults used when CLI empty",
			cliArgs:      "",
			helmDefaults: []string{"--dry-run=server", "--enable-dns"},
			want:         "--dry-run=server --enable-dns",
		},
		{
			name:    "empty when neither set",
			cliArgs: "",
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := &HelmState{
				ReleaseSetSpec: ReleaseSetSpec{
					HelmDefaults: HelmSpec{TemplateArgs: tt.helmDefaults},
				},
			}
			assert.Equal(t, tt.want, st.effectiveTemplateArgs(tt.cliArgs))
		})
	}
}
