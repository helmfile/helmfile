package state

import (
	"strings"
	"testing"
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
			templateArgs := buildChartifyTemplateArgs(tt.helmfileCommand, tt.kubeconfig, tt.kubeContext, false, "")

			for _, flag := range tt.expectedFlags {
				if !strings.Contains(templateArgs, flag) {
					t.Errorf("buildChartifyTemplateArgs() = %q; want to contain %q", templateArgs, flag)
				}
			}

			for _, flag := range tt.unexpectedFlags {
				if strings.Contains(templateArgs, flag) {
					t.Errorf("buildChartifyTemplateArgs() = %q; want NOT to contain %q", templateArgs, flag)
				}
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
			templateArgs := buildChartifyTemplateArgs(tt.helmfileCommand, tt.kubeconfig, "", false, tt.existingArgs)

			if !strings.Contains(templateArgs, tt.expectedContains) {
				t.Errorf("buildChartifyTemplateArgs() = %q; want to contain %q", templateArgs, tt.expectedContains)
			}

			count := strings.Count(templateArgs, "--kubeconfig")
			if count != tt.expectedCount {
				t.Errorf("buildChartifyTemplateArgs() has --kubeconfig %d times; want %d", count, tt.expectedCount)
			}
		})
	}
}

// buildChartifyTemplateArgs simulates the logic from processChartification
// for building template args passed to chartify.
// This helper encapsulates the flag-building logic for testing.
//
// NOTE: This function intentionally duplicates the logic from processChartification()
// in state.go (lines 1549-1602). See issue_2355_test.go for rationale on this duplication.
//
// SYNC WARNING: If the flag-building logic in processChartification() changes
// (state.go lines 1549-1602), this function must be updated to match.
func buildChartifyTemplateArgs(helmfileCommand, kubeconfig, kubeContext string, validate bool, existingTemplateArgs string) string {
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

	if requiresCluster {
		var additionalArgs []string

		if kubeconfig != "" && !strings.Contains(templateArgs, "--kubeconfig") {
			additionalArgs = append(additionalArgs, "--kubeconfig", kubeconfig)
		}

		if kubeContext != "" && !strings.Contains(templateArgs, "--kube-context") {
			additionalArgs = append(additionalArgs, "--kube-context", kubeContext)
		}

		if !validate && !strings.Contains(templateArgs, "--dry-run") {
			additionalArgs = append(additionalArgs, "--dry-run=server")
		}

		if len(additionalArgs) > 0 {
			if templateArgs == "" {
				templateArgs = strings.Join(additionalArgs, " ")
			} else {
				templateArgs += " " + strings.Join(additionalArgs, " ")
			}
		}
	}

	return templateArgs
}
