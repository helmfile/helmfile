package app

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/helmfile/vals"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/helmfile/helmfile/pkg/exectest"
	ffs "github.com/helmfile/helmfile/pkg/filesystem"
	"github.com/helmfile/helmfile/pkg/helmexec"
)

func TestUnittest(t *testing.T) {
	type fields struct {
		skipNeeds              bool
		includeNeeds           bool
		includeTransitiveNeeds bool
		failFast               bool
		color                  bool
		debugPlugin            bool
	}

	type testcase struct {
		fields     fields
		ns         string
		error      string
		selectors  []string
		unittested []exectest.Release
	}

	check := func(t *testing.T, tc testcase) {
		t.Helper()

		wantUnittests := tc.unittested

		var helm = &exectest.Helm{
			FailOnUnexpectedList: true,
			FailOnUnexpectedDiff: true,
			Helm4:                exectest.IsHelm4Enabled(),
			Helm3:                !exectest.IsHelm4Enabled(),
			DiffMutex:            &sync.Mutex{},
			ChartsMutex:          &sync.Mutex{},
			ReleasesMutex:        &sync.Mutex{},
		}

		bs := runWithLogCapture(t, "debug", func(t *testing.T, logger *zap.SugaredLogger) {
			t.Helper()

			valsRuntime, err := vals.New(vals.Options{CacheSize: 32})
			if err != nil {
				t.Errorf("unexpected error creating vals runtime: %v", err)
			}

			files := map[string]string{
				"/path/to/helmfile.yaml": `
releases:
- name: logging
  chart: incubator/raw
  namespace: kube-system
  unitTests:
  - tests/logging

- name: kubernetes-external-secrets
  chart: incubator/raw
  namespace: kube-system
  needs:
  - kube-system/logging
  unitTests:
  - tests/secrets

- name: external-secrets
  chart: incubator/raw
  namespace: default
  labels:
    app: test
  needs:
  - kube-system/kubernetes-external-secrets
  unitTests:
  - tests/external

- name: my-release
  chart: incubator/raw
  namespace: default
  labels:
    app: test
  needs:
  - default/external-secrets
  unitTests:
  - tests/myrelease

- name: no-tests
  chart: incubator/raw
  namespace: default
`,
			}

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
			}, files)

			if tc.ns != "" {
				app.Namespace = tc.ns
			}

			if tc.selectors != nil {
				app.Selectors = tc.selectors
			}

			unittestErr := app.Unittest(applyConfig{
				concurrency:            1,
				logger:                 logger,
				skipNeeds:              tc.fields.skipNeeds,
				includeNeeds:           tc.fields.includeNeeds,
				includeTransitiveNeeds: tc.fields.includeTransitiveNeeds,
				failFast:               tc.fields.failFast,
				color:                  tc.fields.color,
				debugPlugin:            tc.fields.debugPlugin,
			})

			var gotErr string
			if unittestErr != nil {
				gotErr = unittestErr.Error()
			}

			if d := cmp.Diff(tc.error, gotErr); d != "" {
				t.Fatalf("unexpected error: want (-), got (+): %s", d)
			}

			require.Equal(t, wantUnittests, helm.Unittested)
		})

		testNameComponents := strings.Split(t.Name(), "/")
		testBaseName := strings.ToLower(
			strings.ReplaceAll(
				testNameComponents[len(testNameComponents)-1],
				" ",
				"_",
			),
		)
		wantLogFileDir := filepath.Join("testdata", "app_unittest_test")
		snapshotName := testBaseName
		if exectest.IsHelm4Enabled() {
			if _, err := os.Stat(filepath.Join(wantLogFileDir, testBaseName+"_helm4")); err == nil {
				snapshotName = testBaseName + "_helm4"
			}
		}
		wantLogFile := filepath.Join(wantLogFileDir, snapshotName)
		wantLogData, err := os.ReadFile(wantLogFile)
		updateLogFile := err != nil
		wantLog := string(wantLogData)
		gotLog := bs.String()
		if updateLogFile {
			if err := os.MkdirAll(wantLogFileDir, 0755); err != nil {
				t.Fatalf("unable to create directory %q: %v", wantLogFileDir, err)
			}
			if err := os.WriteFile(wantLogFile, bs.Bytes(), 0644); err != nil {
				t.Fatalf("unable to update unittest log snapshot: %v", err)
			}
		}

		assert.Equal(t, wantLog, gotLog)
	}

	t.Run("unittest all releases with unitTests", func(t *testing.T) {
		check(t, testcase{
			unittested: []exectest.Release{
				{Name: "logging", Flags: []string{"--namespace", "kube-system", "--file", "tests/logging/*_test.yaml"}},
				{Name: "kubernetes-external-secrets", Flags: []string{"--namespace", "kube-system", "--file", "tests/secrets/*_test.yaml"}},
				{Name: "external-secrets", Flags: []string{"--namespace", "default", "--file", "tests/external/*_test.yaml"}},
				{Name: "my-release", Flags: []string{"--namespace", "default", "--file", "tests/myrelease/*_test.yaml"}},
			},
		})
	})

	t.Run("with dedicated flags", func(t *testing.T) {
		// --color is skipped on Helm 4 due to flag parsing issues
		expectedFlags := []string{"--namespace", "kube-system", "--failfast"}
		if !exectest.IsHelm4Enabled() {
			expectedFlags = append(expectedFlags, "--color")
		}
		expectedFlags = append(expectedFlags, "--debugPlugin", "--file", "tests/logging/*_test.yaml")

		check(t, testcase{
			fields: fields{
				failFast:    true,
				color:       true,
				debugPlugin: true,
			},
			selectors: []string{"name=logging"},
			unittested: []exectest.Release{
				{Name: "logging", Flags: expectedFlags},
			},
		})
	})

	t.Run("skip-needs", func(t *testing.T) {
		check(t, testcase{
			fields: fields{
				skipNeeds: true,
			},
			selectors: []string{"app=test"},
			unittested: []exectest.Release{
				{Name: "external-secrets", Flags: []string{"--namespace", "default", "--file", "tests/external/*_test.yaml"}},
				{Name: "my-release", Flags: []string{"--namespace", "default", "--file", "tests/myrelease/*_test.yaml"}},
			},
		})
	})

	t.Run("include-needs", func(t *testing.T) {
		check(t, testcase{
			fields: fields{
				skipNeeds:    false,
				includeNeeds: true,
			},
			selectors: []string{"app=test"},
			unittested: []exectest.Release{
				{Name: "logging", Flags: []string{"--namespace", "kube-system", "--file", "tests/logging/*_test.yaml"}},
				{Name: "kubernetes-external-secrets", Flags: []string{"--namespace", "kube-system", "--file", "tests/secrets/*_test.yaml"}},
				{Name: "external-secrets", Flags: []string{"--namespace", "default", "--file", "tests/external/*_test.yaml"}},
				{Name: "my-release", Flags: []string{"--namespace", "default", "--file", "tests/myrelease/*_test.yaml"}},
			},
		})
	})

	t.Run("release without unitTests is skipped", func(t *testing.T) {
		check(t, testcase{
			selectors:  []string{"name=no-tests"},
			unittested: nil,
		})
	})

	t.Run("bad selector", func(t *testing.T) {
		check(t, testcase{
			selectors:  []string{"app=test_non_existent"},
			unittested: nil,
			error:      "err: no releases found that matches specified selector(app=test_non_existent) and environment(default), in any helmfile",
		})
	})
}
