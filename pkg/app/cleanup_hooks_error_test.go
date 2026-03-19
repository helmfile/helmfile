package app

import (
	"sync"
	"testing"

	"github.com/helmfile/vals"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"github.com/helmfile/helmfile/pkg/exectest"
	ffs "github.com/helmfile/helmfile/pkg/filesystem"
	"github.com/helmfile/helmfile/pkg/helmexec"
)

func TestCleanupHooksErrorPropagation(t *testing.T) {
	type testcase struct {
		files          map[string]string
		releaseName    string
		expectedError  bool
		expectedInLogs string
	}

	check := func(t *testing.T, tc testcase) {
		t.Helper()

		var helm = &exectest.Helm{
			FailOnUnexpectedList: true,
			FailOnUnexpectedDiff: true,
			DiffMutex:            &sync.Mutex{},
			ChartsMutex:          &sync.Mutex{},
			ReleasesMutex:        &sync.Mutex{},
		}

		valsRuntime, err := vals.New(vals.Options{CacheSize: 32})
		if err != nil {
			t.Fatalf("unexpected error creating vals runtime: %v", err)
		}

		bs := runWithLogCapture(t, "info", func(t *testing.T, logger *zap.SugaredLogger) {
			t.Helper()

			app := appWithFs(&App{
				OverrideHelmBinary:              DefaultHelmBinary,
				fs:                              ffs.DefaultFileSystem(),
				OverrideKubeContext:             "default",
				DisableKubeVersionAutoDetection: true,
				Env:                             "default",
				Logger:                          logger,
				helms: map[helmKey]helmexec.Interface{
					createHelmKey("helm", "default"): helm,
				},
				valsRuntime: valsRuntime,
			}, tc.files)

			syncErr := app.Sync(applyConfig{
				concurrency: 1,
				logger:      logger,
			})

			if tc.expectedError {
				assert.Error(t, syncErr, "expected error for release %s", tc.releaseName)
			} else {
				assert.NoError(t, syncErr, "unexpected error for release %s", tc.releaseName)
			}
		})

		logOutput := bs.String()
		assert.Contains(t, logOutput, tc.expectedInLogs, "unexpected log output")
	}

	t.Run("cleanup hook receives error when sync fails", func(t *testing.T) {
		check(t, testcase{
			releaseName: "error-release",
			files: map[string]string{
				"/path/to/helmfile.yaml": `
hooks:
  - name: global-cleanup
    events:
      - cleanup
    showlogs: true
    command: echo
    args:
      - "error is '{{ .Event.Error }}'"

releases:
  - name: error-release
    chart: incubator/raw
    namespace: default
`,
			},
			expectedError:  true,
			expectedInLogs: "error is 'failed processing release error-release: error'",
		})
	})

	t.Run("cleanup hook receives nil when sync succeeds", func(t *testing.T) {
		check(t, testcase{
			releaseName: "success-release",
			files: map[string]string{
				"/path/to/helmfile.yaml": `
hooks:
  - name: global-cleanup
    events:
      - cleanup
    showlogs: true
    command: echo
    args:
      - "error is '{{ .Event.Error }}'"

releases:
  - name: success-release
    chart: incubator/raw
    namespace: default
`,
			},
			expectedError:  false,
			expectedInLogs: "error is '<nil>'",
		})
	})
}
