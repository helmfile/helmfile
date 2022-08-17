package app

import (
	"bufio"
	"bytes"
	"io"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"github.com/variantdev/vals"

	"github.com/helmfile/helmfile/pkg/exectest"
	"github.com/helmfile/helmfile/pkg/filesystem"
	"github.com/helmfile/helmfile/pkg/helmexec"
	"github.com/helmfile/helmfile/pkg/testhelper"
)

func TestDiffWithNeeds(t *testing.T) {
	type fields struct {
		skipNeeds              bool
		includeNeeds           bool
		includeTransitiveNeeds bool
	}

	type testcase struct {
		fields    fields
		ns        string
		error     string
		selectors []string
		diffed    []exectest.Release
	}

	check := func(t *testing.T, tc testcase) {
		t.Helper()

		wantDiffs := tc.diffed

		var helm = &exectest.Helm{
			FailOnUnexpectedList: true,
			FailOnUnexpectedDiff: true,
			DiffMutex:            &sync.Mutex{},
			ChartsMutex:          &sync.Mutex{},
			ReleasesMutex:        &sync.Mutex{},
		}

		bs := &bytes.Buffer{}

		func() {
			t.Helper()

			logReader, logWriter := io.Pipe()

			logFlushed := &sync.WaitGroup{}
			// Ensure all the log is consumed into `bs` by calling `logWriter.Close()` followed by `logFlushed.Wait()`
			logFlushed.Add(1)
			go func() {
				scanner := bufio.NewScanner(logReader)
				for scanner.Scan() {
					bs.Write(scanner.Bytes())
					bs.WriteString("\n")
				}
				logFlushed.Done()
			}()

			defer func() {
				// This is here to avoid data-trace on bytes buffer `bs` to capture logs
				if err := logWriter.Close(); err != nil {
					panic(err)
				}
				logFlushed.Wait()
			}()

			logger := helmexec.NewLogger(logWriter, "debug")

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
				fs:                  filesystem.DefaultFileSystem(),
				OverrideKubeContext: "default",
				Env:                 "default",
				Logger:              logger,
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

			diffErr := app.Diff(applyConfig{
				// if we check log output, concurrency must be 1. otherwise the test becomes non-deterministic.
				concurrency:            1,
				logger:                 logger,
				skipNeeds:              tc.fields.skipNeeds,
				includeNeeds:           tc.fields.includeNeeds,
				includeTransitiveNeeds: tc.fields.includeTransitiveNeeds,
			})

			var gotErr string
			if diffErr != nil {
				gotErr = diffErr.Error()
			}

			if d := cmp.Diff(tc.error, gotErr); d != "" {
				t.Fatalf("unexpected error: want (-), got (+): %s", d)
			}

			require.Equal(t, wantDiffs, helm.Diffed)
		}()

		testhelper.RequireLog(t, "app_diff_test", bs)
	}

	t.Run("fail on unselected need by default", func(t *testing.T) {
		check(t, testcase{
			selectors: []string{"app=test"},
			error:     `in ./helmfile.yaml: release "default/default/external-secrets" depends on "default/kube-system/kubernetes-external-secrets" which does not match the selectors. Please add a selector like "--selector name=kubernetes-external-secrets", or indicate whether to skip (--skip-needs) or include (--include-needs) these dependencies`,
		})
	})

	t.Run("skip-needs", func(t *testing.T) {
		check(t, testcase{
			fields: fields{
				skipNeeds: true,
			},
			selectors: []string{"app=test"},
			diffed: []exectest.Release{
				{Name: "external-secrets", Flags: []string{"--kube-context", "default", "--namespace", "default"}},
				{Name: "my-release", Flags: []string{"--kube-context", "default", "--namespace", "default"}},
			},
		})
	})

	t.Run("include-needs", func(t *testing.T) {
		check(t, testcase{
			fields: fields{
				skipNeeds:    false,
				includeNeeds: true,
			},
			error:     ``,
			selectors: []string{"app=test"},
			diffed: []exectest.Release{
				// TODO: Turned out we can't differentiate needs vs transitive needs in this case :thinking:
				{Name: "logging", Flags: []string{"--kube-context", "default", "--namespace", "kube-system"}},
				{Name: "kubernetes-external-secrets", Flags: []string{"--kube-context", "default", "--namespace", "kube-system"}},
				{Name: "external-secrets", Flags: []string{"--kube-context", "default", "--namespace", "default"}},
				{Name: "my-release", Flags: []string{"--kube-context", "default", "--namespace", "default"}},
			},
		})
	})

	t.Run("include-transitive-needs", func(t *testing.T) {
		check(t, testcase{
			fields: fields{
				skipNeeds:              false,
				includeTransitiveNeeds: true,
			},
			error:     ``,
			selectors: []string{"app=test"},
			diffed: []exectest.Release{
				{Name: "logging", Flags: []string{"--kube-context", "default", "--namespace", "kube-system"}},
				{Name: "kubernetes-external-secrets", Flags: []string{"--kube-context", "default", "--namespace", "kube-system"}},
				{Name: "external-secrets", Flags: []string{"--kube-context", "default", "--namespace", "default"}},
				{Name: "my-release", Flags: []string{"--kube-context", "default", "--namespace", "default"}},
			},
		})
	})

	t.Run("include-needs should not fail on disabled direct need", func(t *testing.T) {
		check(t, testcase{
			fields: fields{
				skipNeeds:    false,
				includeNeeds: true,
			},
			selectors: []string{"name=test2"},
			diffed: []exectest.Release{
				{Name: "test2", Flags: []string{"--kube-context", "default"}},
			},
		})
	})

	t.Run("include-needs should not fail on disabled transitive need", func(t *testing.T) {
		check(t, testcase{
			fields: fields{
				skipNeeds:    false,
				includeNeeds: true,
			},
			selectors: []string{"name=test3"},
			diffed: []exectest.Release{
				{Name: "test2", Flags: []string{"--kube-context", "default"}},
				{Name: "test3", Flags: []string{"--kube-context", "default"}},
			},
		})
	})

	t.Run("include-transitive-needs should not fail on disabled transitive need", func(t *testing.T) {
		check(t, testcase{
			fields: fields{
				skipNeeds:              false,
				includeNeeds:           false,
				includeTransitiveNeeds: true,
			},
			selectors: []string{"name=test3"},
			diffed: []exectest.Release{
				{Name: "test2", Flags: []string{"--kube-context", "default"}},
				{Name: "test3", Flags: []string{"--kube-context", "default"}},
			},
		})
	})

	t.Run("include-needs with include-transitive-needs should not fail on disabled direct need", func(t *testing.T) {
		check(t, testcase{
			fields: fields{
				skipNeeds:              false,
				includeNeeds:           true,
				includeTransitiveNeeds: true,
			},
			selectors: []string{"name=test2"},
			diffed: []exectest.Release{
				{Name: "test2", Flags: []string{"--kube-context", "default"}},
			},
		})
	})

	t.Run("include-needs with include-transitive-needs should not fail on disabled transitive need", func(t *testing.T) {
		check(t, testcase{
			fields: fields{
				skipNeeds:              false,
				includeNeeds:           true,
				includeTransitiveNeeds: true,
			},
			selectors: []string{"name=test3"},
			diffed: []exectest.Release{
				{Name: "test2", Flags: []string{"--kube-context", "default"}},
				{Name: "test3", Flags: []string{"--kube-context", "default"}},
			},
		})
	})

	t.Run("bad selector", func(t *testing.T) {
		check(t, testcase{
			selectors: []string{"app=test_non_existent"},
			diffed:    nil,
			error:     "err: no releases found that matches specified selector(app=test_non_existent) and environment(default), in any helmfile",
		})
	})
}
