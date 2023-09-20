package app

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/helmfile/vals"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	ffs "github.com/helmfile/helmfile/pkg/filesystem"
	"github.com/helmfile/helmfile/pkg/testhelper"
	"github.com/helmfile/helmfile/pkg/testutil"
)

func testDAG(t *testing.T, cfg configImpl) {
	type testcase struct {
		environment string
		ns          string
		error       string
		selectors   []string
		expected    string
	}

	check := func(t *testing.T, tc testcase, cfg configImpl) {
		t.Helper()

		bs := runWithLogCapture(t, "debug", func(t *testing.T, logger *zap.SugaredLogger) {
			t.Helper()

			valsRuntime, err := vals.New(vals.Options{CacheSize: 32})
			if err != nil {
				t.Errorf("unexpected error creating vals runtime: %v", err)
			}

			files := map[string]string{
				"/path/to/helmfile.yaml": `
environments:
  development: {}
  shared: {}
---
releases:
- name: logging
  chart: incubator/raw
  namespace: kube-system

- name: kubernetes-external-secrets
  chart: incubator/raw
  namespace: kube-system
  needs:
  - kube-system/logging

- name: external-secrets
  chart: incubator/raw
  namespace: default
  labels:
    app: test
  needs:
  - kube-system/kubernetes-external-secrets

- name: my-release
  chart: incubator/raw
  namespace: default
  labels:
    app: test
  needs:
  - default/external-secrets


# Disabled releases are treated as missing
- name: disabled
  chart: incubator/raw
  namespace: kube-system
  installed: false

- name: test2
  chart: incubator/raw
  needs:
  - kube-system/disabled

- name: test3
  chart: incubator/raw
  needs:
  - test2
`,
			}

			app := appWithFs(&App{
				OverrideHelmBinary:  DefaultHelmBinary,
				fs:                  ffs.DefaultFileSystem(),
				OverrideKubeContext: "default",
				Env:                 tc.environment,
				Logger:              logger,
				valsRuntime:         valsRuntime,
			}, files)

			expectNoCallsToHelm(app)

			if tc.ns != "" {
				app.Namespace = tc.ns
			}

			if tc.selectors != nil {
				app.Selectors = tc.selectors
			}

			var dagErr error
			out, err := testutil.CaptureStdout(func() {
				dagErr = app.PrintState(cfg)
			})
			assert.NoError(t, err)

			var gotErr string
			if dagErr != nil {
				gotErr = dagErr.Error()
			}

			if d := cmp.Diff(tc.error, gotErr); d != "" {
				t.Fatalf("unexpected error: want (-), got (+): %s", d)
			}

			assert.Equal(t, tc.expected, out)
		})

		testhelper.RequireLog(t, "dag_test", bs)
	}

	t.Run("DAG lists dependencies in order", func(t *testing.T) {
		check(t, testcase{
			environment: "default",
			expected: `GROUP RELEASES
1     default/kube-system/logging, default/kube-system/disabled
2     default/kube-system/kubernetes-external-secrets, default//test2
3     default/default/external-secrets, default//test3
4     default/default/my-release
`,
		}, cfg)
	})
}

func TestDAG(t *testing.T) {
	t.Run("DAG", func(t *testing.T) {
		testDAG(t, configImpl{dag: true})
	})
}
