package app

import (
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/helmfile/vals"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

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
		noHooks                bool
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
			Helm3:                true,
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
				noHooks:                tc.fields.noHooks,
			})

			var gotErr string
			if diffErr != nil {
				gotErr = diffErr.Error()
			}

			if d := cmp.Diff(tc.error, gotErr); d != "" {
				t.Fatalf("unexpected error: want (-), got (+): %s", d)
			}

			require.Equal(t, wantDiffs, helm.Diffed)
		})

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
				{Name: "external-secrets", Flags: []string{"--kube-context", "default", "--namespace", "default", "--reset-values"}},
				{Name: "my-release", Flags: []string{"--kube-context", "default", "--namespace", "default", "--reset-values"}},
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
				{Name: "logging", Flags: []string{"--kube-context", "default", "--namespace", "kube-system", "--reset-values"}},
				{Name: "kubernetes-external-secrets", Flags: []string{"--kube-context", "default", "--namespace", "kube-system", "--reset-values"}},
				{Name: "external-secrets", Flags: []string{"--kube-context", "default", "--namespace", "default", "--reset-values"}},
				{Name: "my-release", Flags: []string{"--kube-context", "default", "--namespace", "default", "--reset-values"}},
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
				{Name: "logging", Flags: []string{"--kube-context", "default", "--namespace", "kube-system", "--reset-values"}},
				{Name: "kubernetes-external-secrets", Flags: []string{"--kube-context", "default", "--namespace", "kube-system", "--reset-values"}},
				{Name: "external-secrets", Flags: []string{"--kube-context", "default", "--namespace", "default", "--reset-values"}},
				{Name: "my-release", Flags: []string{"--kube-context", "default", "--namespace", "default", "--reset-values"}},
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
				{Name: "test2", Flags: []string{"--kube-context", "default", "--reset-values"}},
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
				{Name: "test2", Flags: []string{"--kube-context", "default", "--reset-values"}},
				{Name: "test3", Flags: []string{"--kube-context", "default", "--reset-values"}},
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
				{Name: "test2", Flags: []string{"--kube-context", "default", "--reset-values"}},
				{Name: "test3", Flags: []string{"--kube-context", "default", "--reset-values"}},
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
				{Name: "test2", Flags: []string{"--kube-context", "default", "--reset-values"}},
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
				{Name: "test2", Flags: []string{"--kube-context", "default", "--reset-values"}},
				{Name: "test3", Flags: []string{"--kube-context", "default", "--reset-values"}},
			},
		})
	})

	t.Run("no-hooks", func(t *testing.T) {
		check(t, testcase{
			fields: fields{
				skipNeeds: true,
				noHooks:   true,
			},
			selectors: []string{"app=test"},
			diffed: []exectest.Release{
				{Name: "external-secrets", Flags: []string{"--kube-context", "default", "--namespace", "default", "--reset-values", "--no-hooks"}},
				{Name: "my-release", Flags: []string{"--kube-context", "default", "--namespace", "default", "--reset-values", "--no-hooks"}},
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

func TestDiffWithInstalled(t *testing.T) {
	type testcase struct {
		helmfile  string
		ns        string
		error     string
		selectors []string
		lists     map[exectest.ListKey]string
		diffed    []exectest.Release
	}

	check := func(t *testing.T, tc testcase) {
		t.Helper()

		wantDiffs := tc.diffed

		var helm = &exectest.Helm{
			FailOnUnexpectedList: true,
			FailOnUnexpectedDiff: true,
			Lists:                tc.lists,
			DiffMutex:            &sync.Mutex{},
			ChartsMutex:          &sync.Mutex{},
			ReleasesMutex:        &sync.Mutex{},
			Helm3:                true,
		}

		bs := runWithLogCapture(t, "debug", func(t *testing.T, logger *zap.SugaredLogger) {
			t.Helper()

			valsRuntime, err := vals.New(vals.Options{CacheSize: 32})
			if err != nil {
				t.Errorf("unexpected error creating vals runtime: %v", err)
			}

			files := map[string]string{
				"/path/to/helmfile.yaml": tc.helmfile,
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
				concurrency: 1,
				logger:      logger,
				skipNeeds:   true,
			})

			var gotErr string
			if diffErr != nil {
				gotErr = diffErr.Error()
			}

			if d := cmp.Diff(tc.error, gotErr); d != "" {
				t.Fatalf("unexpected error: want (-), got (+): %s", d)
			}

			require.Equal(t, wantDiffs, helm.Diffed)
		})

		testhelper.RequireLog(t, "app_diff_test", bs)
	}

	t.Run("show diff on changed selected release", func(t *testing.T) {
		check(t, testcase{
			helmfile: `
releases:
- name: a
  chart: incubator/raw
  namespace: default
- name: b
  chart: incubator/raw
  namespace: default
`,
			selectors: []string{"name=a"},
			lists: map[exectest.ListKey]string{
				{Filter: "^a$", Flags: listFlags("default", "default")}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
foo 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	raw-3.1.0	3.1.0      	default
`,
			},
			diffed: []exectest.Release{
				{Name: "a", Flags: []string{"--kube-context", "default", "--namespace", "default", "--reset-values"}},
			},
		})
	})

	t.Run("shows no diff on already uninstalled selected release", func(t *testing.T) {
		check(t, testcase{
			helmfile: `
releases:
- name: a
  chart: incubator/raw
  installed: false
  namespace: default
- name: b
  chart: incubator/raw
  namespace: default
`,
			selectors: []string{"name=a"},
			lists: map[exectest.ListKey]string{
				{Filter: "^a$", Flags: listFlags("default", "default")}: ``,
			},
		})
	})
}
