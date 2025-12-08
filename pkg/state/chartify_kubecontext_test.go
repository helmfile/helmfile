package state

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/helmfile/helmfile/pkg/environment"
	"github.com/helmfile/helmfile/pkg/filesystem"
)

// TestProcessChartificationKubeContext tests that --kube-context flag is properly
// added to chartifyOpts.TemplateArgs when using jsonPatches with cluster-requiring commands.
// This is a regression test for the issue where helm template does not receive
// --kube-context arg when using kustomize (jsonPatches).
func TestProcessChartificationKubeContext(t *testing.T) {
	tests := []struct {
		name            string
		helmfileCommand string
		kubeContext     string
		envKubeContext  string
		helmDefaults    string
		expectContext   bool
		expectDryRun    bool
	}{
		{
			name:            "diff command with helmDefaults kubeContext",
			helmfileCommand: "diff",
			helmDefaults:    "minikube",
			expectContext:   true,
			expectDryRun:    true,
		},
		{
			name:            "apply command with release kubeContext",
			helmfileCommand: "apply",
			kubeContext:     "prod-cluster",
			expectContext:   true,
			expectDryRun:    true,
		},
		{
			name:            "sync command with env kubeContext",
			helmfileCommand: "sync",
			envKubeContext:  "staging-cluster",
			expectContext:   true,
			expectDryRun:    true,
		},
		{
			name:            "template command should not add cluster flags",
			helmfileCommand: "template",
			helmDefaults:    "minikube",
			expectContext:   false,
			expectDryRun:    false,
		},
		{
			name:            "build command should not add cluster flags",
			helmfileCommand: "build",
			helmDefaults:    "minikube",
			expectContext:   false,
			expectDryRun:    false,
		},
		{
			name:            "diff command without kubeContext",
			helmfileCommand: "diff",
			expectContext:   false,
			expectDryRun:    true,
		},
		{
			name:            "destroy command with kubeContext",
			helmfileCommand: "destroy",
			helmDefaults:    "test-cluster",
			expectContext:   true,
			expectDryRun:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a release with jsonPatches to trigger chartification
			release := &ReleaseSpec{
				Name:      "test-release",
				Namespace: "default",
				Chart:     "./test-chart",
				JSONPatches: []interface{}{
					map[string]interface{}{
						"target": map[string]interface{}{
							"group":   "apps",
							"version": "v1",
							"kind":    "Deployment",
							"name":    "test",
						},
						"patch": []interface{}{
							map[string]interface{}{
								"op":    "add",
								"path":  "/spec/template/spec/containers/0/args/-",
								"value": "test",
							},
						},
					},
				},
			}

			// Set kubeContext on release if provided
			if tt.kubeContext != "" {
				release.KubeContext = tt.kubeContext
			}

			// Create HelmState
			state := &HelmState{
				basePath: "/tmp/test",
				fs: filesystem.FromFileSystem(filesystem.FileSystem{
					DirectoryExistsAt: func(path string) bool {
						return strings.Contains(path, "test-chart")
					},
					FileExistsAt: func(path string) bool {
						return false
					},
					DeleteFile: func(path string) error {
						return nil
					},
					Glob: func(pattern string) ([]string, error) {
						return nil, nil
					},
				}),
				logger:         logger,
				valsRuntime:    valsRuntime,
				RenderedValues: map[string]any{},
				ReleaseSetSpec: ReleaseSetSpec{
					Env: environment.Environment{
						Name: "default",
					},
					Environments: map[string]EnvironmentSpec{
						"default": {
							KubeContext: tt.envKubeContext,
						},
					},
				},
			}

			// Set helmDefaults kubeContext if provided
			if tt.helmDefaults != "" {
				state.ReleaseSetSpec.HelmDefaults.KubeContext = tt.helmDefaults
			}

			// Prepare chartify (this generates the Chartify object with jsonPatches)
			chartification, clean, err := state.PrepareChartify(nil, release, "./test-chart", 0)
			require.NoError(t, err)
			defer clean()

			// Ensure chartification is needed (jsonPatches should trigger it)
			require.NotNil(t, chartification, "Chartification should be needed when jsonPatches are present")

			// Process chartification with the test command
			opts := ChartPrepareOptions{}
			_, _, err = state.processChartification(chartification, release, "./test-chart", opts, false, tt.helmfileCommand)

			// We expect an error because we don't have an actual chart, but we can still
			// check that TemplateArgs was set correctly
			// The error will come from chartify.Chartify, not from our logic
			// So let's check the chartification.Opts.TemplateArgs directly

			if tt.expectContext {
				// Determine which kubeContext should be used
				expectedContext := tt.kubeContext
				if expectedContext == "" {
					expectedContext = tt.envKubeContext
				}
				if expectedContext == "" {
					expectedContext = tt.helmDefaults
				}

				assert.Contains(t, chartification.Opts.TemplateArgs, "--kube-context",
					"TemplateArgs should contain --kube-context flag")
				assert.Contains(t, chartification.Opts.TemplateArgs, expectedContext,
					"TemplateArgs should contain the expected kube context: %s", expectedContext)
			} else if tt.helmDefaults != "" || tt.kubeContext != "" || tt.envKubeContext != "" {
				// If a context is configured but not expected (offline commands)
				assert.NotContains(t, chartification.Opts.TemplateArgs, "--kube-context",
					"TemplateArgs should not contain --kube-context flag for offline commands")
			}

			if tt.expectDryRun {
				assert.Contains(t, chartification.Opts.TemplateArgs, "--dry-run=server",
					"TemplateArgs should contain --dry-run=server flag")
			} else {
				assert.NotContains(t, chartification.Opts.TemplateArgs, "--dry-run",
					"TemplateArgs should not contain --dry-run flag for offline commands")
			}
		})
	}
}

// TestProcessChartificationKubeContextPriority tests the priority order
// for kube context selection: release.KubeContext > env.KubeContext > helmDefaults.KubeContext
func TestProcessChartificationKubeContextPriority(t *testing.T) {
	tests := []struct {
		name             string
		releaseContext   string
		envContext       string
		helmDefaultsContext string
		expectedContext  string
	}{
		{
			name:             "release context takes priority",
			releaseContext:   "release-ctx",
			envContext:       "env-ctx",
			helmDefaultsContext: "defaults-ctx",
			expectedContext:  "release-ctx",
		},
		{
			name:             "env context when release not set",
			envContext:       "env-ctx",
			helmDefaultsContext: "defaults-ctx",
			expectedContext:  "env-ctx",
		},
		{
			name:             "helmDefaults context when others not set",
			helmDefaultsContext: "defaults-ctx",
			expectedContext:  "defaults-ctx",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			release := &ReleaseSpec{
				Name:      "test-release",
				Namespace: "default",
				Chart:     "./test-chart",
				JSONPatches: []interface{}{
					map[string]interface{}{
						"target": map[string]interface{}{
							"kind": "Deployment",
						},
					},
				},
			}

			if tt.releaseContext != "" {
				release.KubeContext = tt.releaseContext
			}

			state := &HelmState{
				basePath: "/tmp/test",
				fs: filesystem.FromFileSystem(filesystem.FileSystem{
					DirectoryExistsAt: func(path string) bool {
						return strings.Contains(path, "test-chart")
					},
					FileExistsAt: func(path string) bool {
						return false
					},
					DeleteFile: func(path string) error {
						return nil
					},
					Glob: func(pattern string) ([]string, error) {
						return nil, nil
					},
				}),
				logger:         logger,
				valsRuntime:    valsRuntime,
				RenderedValues: map[string]any{},
				ReleaseSetSpec: ReleaseSetSpec{
					Env: environment.Environment{
						Name: "default",
					},
					Environments: map[string]EnvironmentSpec{
						"default": {
							KubeContext: tt.envContext,
						},
					},
					HelmDefaults: HelmSpec{
						KubeContext: tt.helmDefaultsContext,
					},
				},
			}

			chartification, clean, err := state.PrepareChartify(nil, release, "./test-chart", 0)
			require.NoError(t, err)
			defer clean()
			require.NotNil(t, chartification)

			opts := ChartPrepareOptions{}
			_, _, _ = state.processChartification(chartification, release, "./test-chart", opts, false, "diff")

			assert.Contains(t, chartification.Opts.TemplateArgs, tt.expectedContext,
				"TemplateArgs should contain the expected context: %s", tt.expectedContext)
		})
	}
}
