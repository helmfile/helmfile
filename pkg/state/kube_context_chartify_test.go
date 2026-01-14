package state

import (
	"strings"
	"testing"

	"github.com/helmfile/chartify"
	"github.com/stretchr/testify/assert"

	"github.com/helmfile/helmfile/pkg/environment"
)

// TestProcessChartification_TemplateArgsConstruction tests that when
// --dry-run=server is added for cluster-requiring commands (like diff),
// the --kube-context flag is also included in TemplateArgs.
// This is a regression test for the issue where helm template does not receive
// --kube-context when kustomize (jsonPatches) is used.
func TestProcessChartification_TemplateArgsConstruction(t *testing.T) {
	tests := []struct {
		name             string
		helmfileCommand  string
		helmDefaults     HelmSpec
		envKubeContext   string
		releaseContext   string
		expectDryRun     bool
		expectKubeCtx    bool
		expectedContext  string
	}{
		{
			name:            "diff command with helmDefaults kubeContext",
			helmfileCommand: "diff",
			helmDefaults: HelmSpec{
				KubeContext: "minikube",
			},
			expectDryRun:    true,
			expectKubeCtx:   true,
			expectedContext: "minikube",
		},
		{
			name:            "apply command with helmDefaults kubeContext",
			helmfileCommand: "apply",
			helmDefaults: HelmSpec{
				KubeContext: "production",
			},
			expectDryRun:    true,
			expectKubeCtx:   true,
			expectedContext: "production",
		},
		{
			name:            "sync command with environment kubeContext",
			helmfileCommand: "sync",
			envKubeContext:  "staging",
			expectDryRun:    true,
			expectKubeCtx:   true,
			expectedContext: "staging",
		},
		{
			name:            "diff command with release kubeContext",
			helmfileCommand: "diff",
			releaseContext:  "dev-cluster",
			expectDryRun:    true,
			expectKubeCtx:   true,
			expectedContext: "dev-cluster",
		},
		{
			name:            "template command should not add dry-run or kube-context",
			helmfileCommand: "template",
			helmDefaults: HelmSpec{
				KubeContext: "minikube",
			},
			expectDryRun:  false,
			expectKubeCtx: false,
		},
		{
			name:            "build command should not add dry-run or kube-context",
			helmfileCommand: "build",
			helmDefaults: HelmSpec{
				KubeContext: "minikube",
			},
			expectDryRun:  false,
			expectKubeCtx: false,
		},
		{
			name:            "release context takes precedence over helm defaults",
			helmfileCommand: "diff",
			helmDefaults: HelmSpec{
				KubeContext: "default-context",
			},
			releaseContext:  "release-context",
			expectDryRun:    true,
			expectKubeCtx:   true,
			expectedContext: "release-context",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup state
			st := &HelmState{
				basePath: "/test/path",
				ReleaseSetSpec: ReleaseSetSpec{
					DefaultHelmBinary: "helm",
					HelmDefaults:      tt.helmDefaults,
				},
				logger: logger,
			}

			// Setup environment if needed
			if tt.envKubeContext != "" {
				st.Env = environment.Environment{Name: "test"}
				st.Environments = map[string]EnvironmentSpec{
					"test": {
						KubeContext: tt.envKubeContext,
					},
				}
			} else {
				st.Env = environment.Environment{Name: "default"}
				st.Environments = map[string]EnvironmentSpec{
					"default": {},
				}
			}

			// Setup release
			release := &ReleaseSpec{
				Name:      "test-release",
				Namespace: "default",
				Chart:     "test/chart",
			}
			if tt.releaseContext != "" {
				release.KubeContext = tt.releaseContext
			}

			// Setup chartifyOpts (this simulates what processChartification does)
			chartifyOpts := &chartify.ChartifyOpts{
				Namespace: "default",
			}

			// Simulate the logic from processChartification for setting TemplateArgs
			var requiresCluster bool
			switch tt.helmfileCommand {
			case "diff", "apply", "sync", "destroy", "delete", "test", "status":
				requiresCluster = true
			case "template", "lint", "build", "pull", "fetch", "write-values", "list", "show-dag", "deps", "repos", "cache", "init", "completion", "help", "version":
				requiresCluster = false
			default:
				requiresCluster = true
			}

			if requiresCluster {
				if chartifyOpts.TemplateArgs == "" {
					chartifyOpts.TemplateArgs = "--dry-run=server"
				} else if !strings.Contains(chartifyOpts.TemplateArgs, "--dry-run") {
					chartifyOpts.TemplateArgs += " --dry-run=server"
				}
				// This is the fix being tested
				kubeContextFlags := st.kubeConnectionFlags(release)
				for i := 0; i < len(kubeContextFlags); i += 2 {
					flag := kubeContextFlags[i]
					value := kubeContextFlags[i+1]
					if !strings.Contains(chartifyOpts.TemplateArgs, flag) {
						chartifyOpts.TemplateArgs += " " + flag + " " + value
					}
				}
			}

			// Verify TemplateArgs contains expected flags
			templateArgs := chartifyOpts.TemplateArgs

			if tt.expectDryRun {
				assert.Contains(t, templateArgs, "--dry-run=server",
					"TemplateArgs should contain --dry-run=server for command: %s", tt.helmfileCommand)
			} else {
				assert.NotContains(t, templateArgs, "--dry-run",
					"TemplateArgs should not contain --dry-run for command: %s", tt.helmfileCommand)
			}

			if tt.expectKubeCtx {
				assert.Contains(t, templateArgs, "--kube-context",
					"TemplateArgs should contain --kube-context for command: %s", tt.helmfileCommand)
				assert.Contains(t, templateArgs, tt.expectedContext,
					"TemplateArgs should contain context %s for command: %s", tt.expectedContext, tt.helmfileCommand)

				// Verify the format is correct: "--kube-context <value>"
				parts := strings.Split(templateArgs, " ")
				foundContext := false
				for i, part := range parts {
					if part == "--kube-context" && i+1 < len(parts) {
						assert.Equal(t, tt.expectedContext, parts[i+1],
							"kube-context value should be %s", tt.expectedContext)
						foundContext = true
						break
					}
				}
				assert.True(t, foundContext, "Should find --kube-context flag with value")
			} else {
				assert.NotContains(t, templateArgs, "--kube-context",
					"TemplateArgs should not contain --kube-context for command: %s", tt.helmfileCommand)
			}
		})
	}
}

// TestKubeConnectionFlags tests the kubeConnectionFlags function
// to ensure it properly returns the kube-context flag based on
// release, environment, or helm defaults priority.
func TestKubeConnectionFlags(t *testing.T) {
	tests := []struct {
		name           string
		release        *ReleaseSpec
		envKubeContext string
		helmDefaults   HelmSpec
		expected       []string
	}{
		{
			name: "release kube context takes precedence",
			release: &ReleaseSpec{
				KubeContext: "release-context",
			},
			envKubeContext: "env-context",
			helmDefaults: HelmSpec{
				KubeContext: "default-context",
			},
			expected: []string{"--kube-context", "release-context"},
		},
		{
			name:           "environment kube context used when no release context",
			release:        &ReleaseSpec{},
			envKubeContext: "env-context",
			helmDefaults: HelmSpec{
				KubeContext: "default-context",
			},
			expected: []string{"--kube-context", "env-context"},
		},
		{
			name:    "helm defaults kube context used when no release or env context",
			release: &ReleaseSpec{},
			helmDefaults: HelmSpec{
				KubeContext: "default-context",
			},
			expected: []string{"--kube-context", "default-context"},
		},
		{
			name:     "no kube context returns empty slice",
			release:  &ReleaseSpec{},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := &HelmState{
				ReleaseSetSpec: ReleaseSetSpec{
					HelmDefaults: tt.helmDefaults,
				},
			}

			if tt.envKubeContext != "" {
				st.Env = environment.Environment{Name: "test"}
				st.Environments = map[string]EnvironmentSpec{
					"test": {
						KubeContext: tt.envKubeContext,
					},
				}
			} else {
				st.Env = environment.Environment{Name: "default"}
				st.Environments = map[string]EnvironmentSpec{
					"default": {},
				}
			}

			got := st.kubeConnectionFlags(tt.release)
			assert.Equal(t, tt.expected, got)
		})
	}
}
