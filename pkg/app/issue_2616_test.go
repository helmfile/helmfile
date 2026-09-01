package app

import (
	"path/filepath"
	"sync"
	"testing"

	"github.com/helmfile/vals"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/helmfile/helmfile/pkg/exectest"
	ffs "github.com/helmfile/helmfile/pkg/filesystem"
	"github.com/helmfile/helmfile/pkg/helmexec"
)

// issue2616HelmfileContent defines two releases: one that is templated fine
// (logging) and one that always fails chart preparation (error), because OCI
// charts do not support the "latest" version tag.
const issue2616HelmfileContent = `
releases:
- name: logging
  chart: incubator/raw
  namespace: kube-system

- name: error
  chart: oci://example.com/chart/error
  namespace: failed
  version: latest
`

// TestTemplateAllowFailedReleases verifies the behavior of the
// --allow-failed-releases flag end to end for `helmfile template`:
// when chart preparation fails for one release, the remaining releases are
// still templated, the failed release is skipped (i.e. never executed against
// its un-prepared chart reference) and all failures are reported at the end.
func TestTemplateAllowFailedReleases(t *testing.T) {
	testcases := []struct {
		name               string
		allowFailedRelease bool

		// wantTemplated contains the releases that must have been passed to
		// `helm template`; the release failing chart preparation must never
		// appear here.
		wantTemplated []exectest.Release
	}{
		{
			name:               "default: abort on chart preparation failure",
			allowFailedRelease: false,
			wantTemplated:      nil,
		},
		{
			name:               "allow-failed-releases: skip failed release, template the rest",
			allowFailedRelease: true,
			wantTemplated:      []exectest.Release{{Name: "logging", Flags: []string{"--kube-context", "default", "--namespace", "kube-system"}}},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			var helm = &exectest.Helm{
				FailOnUnexpectedList: true,
				FailOnUnexpectedDiff: true,
				DiffMutex:            &sync.Mutex{},
				ChartsMutex:          &sync.Mutex{},
				ReleasesMutex:        &sync.Mutex{},
			}

			_ = runWithLogCapture(t, "debug", func(t *testing.T, logger *zap.SugaredLogger) {
				t.Helper()

				valsRuntime, err := vals.New(vals.Options{CacheSize: 32})
				require.NoError(t, err)

				files := map[string]string{
					"/path/to/helmfile.yaml": issue2616HelmfileContent,
				}

				app := appWithFs(&App{
					OverrideHelmBinary:              DefaultHelmBinary,
					fs:                              &ffs.FileSystem{Glob: filepath.Glob},
					OverrideKubeContext:             "default",
					DisableKubeVersionAutoDetection: true,
					Env:                             "default",
					Logger:                          logger,
					helms: map[helmKey]helmexec.Interface{
						createHelmKey("helm", "default"): helm,
					},
					valsRuntime: valsRuntime,
				}, files)

				tmplErr := app.Template(applyConfig{
					// if we check log output, concurrency must be 1. otherwise the test becomes non-deterministic.
					concurrency:         1,
					logger:              logger,
					allowFailedReleases: tc.allowFailedRelease,
				})

				// The chart preparation failure must be reported in both modes.
				require.Error(t, tmplErr)
				assert.Contains(t, tmplErr.Error(), "the version for OCI charts should be semver compliant")

				// The release that failed chart preparation must never be executed.
				require.Equal(t, tc.wantTemplated, helm.Templated)
			})
		})
	}
}
