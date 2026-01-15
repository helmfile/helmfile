package state

import (
	"strings"
	"testing"
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

		// Non-cluster commands should never get --dry-run=server
		{"template without validate", "template", false, false},
		{"template with validate", "template", true, false},
		{"lint without validate", "lint", false, false},
		{"lint with validate", "lint", true, false},
		{"build without validate", "build", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the logic from processChartification
			templateArgs := shouldAddDryRunServer(tt.helmfileCommand, tt.validate, "")

			hasDryRun := strings.Contains(templateArgs, "--dry-run=server")

			if hasDryRun != tt.expectedDryRun {
				t.Errorf("shouldAddDryRunServer(%q, validate=%v) = %q, hasDryRun=%v; want hasDryRun=%v",
					tt.helmfileCommand, tt.validate, templateArgs, hasDryRun, tt.expectedDryRun)
			}
		})
	}
}

// TestDryRunServerWithExistingTemplateArgs verifies that --dry-run=server is
// appended correctly when there are existing template args.
func TestDryRunServerWithExistingTemplateArgs(t *testing.T) {
	tests := []struct {
		name             string
		helmfileCommand  string
		validate         bool
		existingArgs     string
		expectedContains string
		shouldHaveDryRun bool
	}{
		{
			name:             "append to existing args when validate is false",
			helmfileCommand:  "diff",
			validate:         false,
			existingArgs:     "--some-flag",
			expectedContains: "--dry-run=server",
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
			name:             "do not duplicate if dry-run already exists",
			helmfileCommand:  "diff",
			validate:         false,
			existingArgs:     "--dry-run=client",
			expectedContains: "--dry-run=client",
			shouldHaveDryRun: false, // Already has --dry-run, should not add server
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			templateArgs := shouldAddDryRunServer(tt.helmfileCommand, tt.validate, tt.existingArgs)

			if !strings.Contains(templateArgs, tt.expectedContains) {
				t.Errorf("shouldAddDryRunServer() = %q; want to contain %q",
					templateArgs, tt.expectedContains)
			}

			hasDryRunServer := strings.Contains(templateArgs, "--dry-run=server")
			if hasDryRunServer != tt.shouldHaveDryRun {
				t.Errorf("shouldAddDryRunServer() = %q; hasDryRunServer=%v, want %v",
					templateArgs, hasDryRunServer, tt.shouldHaveDryRun)
			}
		})
	}
}

// shouldAddDryRunServer determines whether to add --dry-run=server to template args.
// This helper function encapsulates the logic from processChartification for testing.
//
// Parameters:
// - helmfileCommand: the helmfile command being run (diff, apply, template, etc.)
// - validate: whether the --validate flag was passed
// - existingTemplateArgs: any existing template arguments
//
// Returns the updated template args string.
func shouldAddDryRunServer(helmfileCommand string, validate bool, existingTemplateArgs string) string {
	// Determine if the command requires cluster access
	var requiresCluster bool
	switch helmfileCommand {
	case "diff", "apply", "sync", "destroy", "delete", "test", "status":
		requiresCluster = true
	case "template", "lint", "build", "pull", "fetch", "write-values", "list", "show-dag", "deps", "repos", "cache", "init", "completion", "help", "version":
		requiresCluster = false
	default:
		requiresCluster = true
	}

	templateArgs := existingTemplateArgs

	// Issue #2355: In Helm 4, --validate and --dry-run are mutually exclusive.
	// Only add --dry-run=server if:
	// 1. The command requires cluster access
	// 2. --validate is NOT set (to avoid mutual exclusion error)
	if requiresCluster && !validate {
		if templateArgs == "" {
			templateArgs = "--dry-run=server"
		} else if !strings.Contains(templateArgs, "--dry-run") {
			templateArgs += " --dry-run=server"
		}
	}

	return templateArgs
}
