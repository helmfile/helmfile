package state

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestValidateAndDryRunMutualExclusion verifies that when --validate is set,
// --dry-run=server is NOT added to TemplateArgs in Helm 4 compatibility.
// This is a regression test for issue #2355.
//
// Background: In Helm 4, the --validate and --dry-run flags are mutually exclusive.
// When helmfile uses kustomize/chartify, it was adding --dry-run=server for cluster-
// requiring commands (like diff, apply) to support the lookup() function. However,
// if --validate is already set, we should NOT add --dry-run=server because:
// 1. They are mutually exclusive in Helm 4
// 2. --validate already provides server-side validation
//
// These tests exercise the real HelmState.buildChartifyTemplateArgs method (the pure
// helper that processChartification delegates to), not a duplicated copy of the logic.
func TestValidateAndDryRunMutualExclusion(t *testing.T) {
	tests := []struct {
		name            string
		helmfileCommand string
		validate        bool
		expectedDryRun  bool // Should --dry-run=server be added?
	}{
		// Cluster-requiring commands without --validate should get --dry-run=server
		{"diff without validate", "diff", false, true},
		{"apply without validate", "apply", false, true},
		{"sync without validate", "sync", false, true},
		{"destroy without validate", "destroy", false, true},
		{"delete without validate", "delete", false, true},
		{"test without validate", "test", false, true},
		{"status without validate", "status", false, true},

		// Cluster-requiring commands WITH --validate should NOT get --dry-run=server
		// This is the fix for issue #2355
		{"diff with validate", "diff", true, false},
		{"apply with validate", "apply", true, false},
		{"sync with validate", "sync", true, false},
		{"destroy with validate", "destroy", true, false},
		{"delete with validate", "delete", true, false},
		{"test with validate", "test", true, false},
		{"status with validate", "status", true, false},

		// Non-cluster commands should never get --dry-run=server
		{"template without validate", "template", false, false},
		{"template with validate", "template", true, false},
		{"lint without validate", "lint", false, false},
		{"lint with validate", "lint", true, false},
		{"build without validate", "build", false, false},
		{"build with validate", "build", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := &HelmState{}
			got := st.buildChartifyTemplateArgs(tt.helmfileCommand, "", tt.validate, "", "")

			hasDryRun := strings.Contains(got, "--dry-run=server")
			assert.Equalf(t, tt.expectedDryRun, hasDryRun,
				"buildChartifyTemplateArgs(%q, validate=%v) = %q, hasDryRun=%v; want hasDryRun=%v",
				tt.helmfileCommand, tt.validate, got, hasDryRun, tt.expectedDryRun)
		})
	}
}

// TestDryRunServerWithExistingTemplateArgs verifies that --dry-run=server is
// appended correctly when there are existing template args, and is not duplicated
// when a --dry-run variant is already present.
func TestDryRunServerWithExistingTemplateArgs(t *testing.T) {
	tests := []struct {
		name             string
		helmfileCommand  string
		validate         bool
		userArgs         string
		existingArgs     string
		expectedContains string
		shouldHaveDryRun bool
	}{
		{
			name:             "append to existing args when validate is false",
			helmfileCommand:  "diff",
			validate:         false,
			existingArgs:     "--some-flag",
			expectedContains: "--some-flag",
			shouldHaveDryRun: true,
		},
		{
			name:             "do not append when validate is true",
			helmfileCommand:  "diff",
			validate:         true,
			existingArgs:     "--some-flag",
			expectedContains: "--some-flag",
			shouldHaveDryRun: false,
		},
		{
			name:             "do not duplicate if dry-run already exists in existing args",
			helmfileCommand:  "diff",
			validate:         false,
			existingArgs:     "--dry-run=client",
			expectedContains: "--dry-run=client",
			shouldHaveDryRun: false, // Already has --dry-run, should not add server
		},
		{
			name:             "do not duplicate if dry-run provided via user template args",
			helmfileCommand:  "diff",
			validate:         false,
			userArgs:         "--dry-run=server",
			expectedContains: "--dry-run=server",
			shouldHaveDryRun: true, // present once, not duplicated
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := &HelmState{}
			got := st.buildChartifyTemplateArgs(tt.helmfileCommand, "", tt.validate, tt.userArgs, tt.existingArgs)

			if tt.expectedContains != "" {
				assert.Containsf(t, got, tt.expectedContains,
					"buildChartifyTemplateArgs() = %q; want to contain %q", got, tt.expectedContains)
			}

			hasDryRunServer := strings.Contains(got, "--dry-run=server")
			assert.Equalf(t, tt.shouldHaveDryRun, hasDryRunServer,
				"buildChartifyTemplateArgs() = %q; hasDryRunServer=%v, want %v", got, hasDryRunServer, tt.shouldHaveDryRun)

			// --dry-run=server should never appear more than once.
			assert.LessOrEqualf(t, strings.Count(got, "--dry-run=server"), 1,
				"buildChartifyTemplateArgs() = %q; --dry-run=server duplicated", got)
		})
	}
}
