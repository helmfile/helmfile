package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	ffs "github.com/helmfile/helmfile/pkg/filesystem"
	"github.com/helmfile/helmfile/pkg/testutil"
)

// TestListReflectsLockedVersions tests that helmfile list shows versions from
// the lock file when available, both with and without --skip-charts
func TestListReflectsLockedVersions(t *testing.T) {
	testCases := []struct {
		name       string
		skipCharts bool
	}{
		{
			name:       "with skipCharts=false",
			skipCharts: false,
		},
		{
			name:       "with skipCharts=true", 
			skipCharts: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Mock files: helmfile.yaml with a chart dependency and helmfile.lock with resolved version
			files := map[string]string{
				"/path/to/helmfile.yaml": `
repositories:
- name: bitnami
  url: https://charts.bitnami.com/bitnami

releases:
- name: redis
  chart: bitnami/redis
  version: "*"
`,
				"/path/to/helmfile.lock": `
version: v0.170.1
dependencies:
- name: redis
  repository: https://charts.bitnami.com/bitnami
  version: 18.1.5
digest: sha256:abcd1234
generated: "2023-01-01T00:00:00Z"
`,
			}

			logger := zap.NewNop().Sugar()

			app := appWithFs(&App{
				OverrideHelmBinary:  DefaultHelmBinary,
				fs:                  ffs.DefaultFileSystem(),
				OverrideKubeContext: "default",
				Env:                 "default",
				Logger:              logger,
				FileOrDir:           "/path/to/helmfile.yaml",
			}, files)

			expectNoCallsToHelm(app)

			cfg := configImpl{
				skipCharts: tc.skipCharts,
				output:     "json",
			}

			out, err := testutil.CaptureStdout(func() {
				listErr := app.ListReleases(cfg)
				assert.NoError(t, listErr)
			})
			assert.NoError(t, err)

			// The output should contain the locked version (18.1.5) not the constraint (*)
			assert.Contains(t, out, "18.1.5", "Expected to see locked version 18.1.5 in output, but got: %s", out)
			assert.NotContains(t, out, `"version":"*"`, "Should not see version constraint in output, but got: %s", out)
		})
	}
}