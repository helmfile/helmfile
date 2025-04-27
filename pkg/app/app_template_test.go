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
	"github.com/helmfile/helmfile/pkg/runtime"
)

func TestTemplate(t *testing.T) {
	type fields struct {
		skipNeeds              bool
		includeNeeds           bool
		includeTransitiveNeeds bool
		noHooks                bool
		showOnly               []string
	}

	type testcase struct {
		fields    fields
		ns        string
		error     string
		selectors []string
		templated []exectest.Release
	}

	check := func(t *testing.T, tc testcase) {
		t.Helper()

		wantTemplates := tc.templated

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
				fs:                  &ffs.FileSystem{Glob: filepath.Glob},
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

			tmplErr := app.Template(applyConfig{
				// if we check log output, concurrency must be 1. otherwise the test becomes non-deterministic.
				concurrency:            1,
				logger:                 logger,
				skipNeeds:              tc.fields.skipNeeds,
				includeNeeds:           tc.fields.includeNeeds,
				includeTransitiveNeeds: tc.fields.includeTransitiveNeeds,
				showOnly:               tc.fields.showOnly,
				noHooks:                tc.fields.noHooks,
			})

			var gotErr string
			if tmplErr != nil {
				gotErr = tmplErr.Error()
			}

			if d := cmp.Diff(tc.error, gotErr); d != "" {
				t.Fatalf("unexpected error: want (-), got (+): %s", d)
			}

			require.Equal(t, wantTemplates, helm.Templated)
		})

		testNameComponents := strings.Split(t.Name(), "/")
		testBaseName := strings.ToLower(
			strings.ReplaceAll(
				testNameComponents[len(testNameComponents)-1],
				" ",
				"_",
			),
		)
		wantLogFileDir := filepath.Join("testdata", "app_template_test")
		wantLogFile := filepath.Join(wantLogFileDir, testBaseName)
		wantLogData, err := os.ReadFile(wantLogFile)
		updateLogFile := err != nil
		wantLog := string(wantLogData)
		gotLog := bs.String()
		if updateLogFile {
			if err := os.MkdirAll(wantLogFileDir, 0755); err != nil {
				t.Fatalf("unable to create directory %q: %v", wantLogFileDir, err)
			}
			if err := os.WriteFile(wantLogFile, bs.Bytes(), 0644); err != nil {
				t.Fatalf("unable to update lint log snapshot: %v", err)
			}
		}

		assert.Equal(t, wantLog, gotLog)
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
			templated: []exectest.Release{
				{Name: "external-secrets", Flags: []string{"--namespace", "default"}},
				{Name: "my-release", Flags: []string{"--namespace", "default"}},
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
			templated: []exectest.Release{
				// TODO: Turned out we can't differentiate needs vs transitive needs in this case :thinking:
				{Name: "logging", Flags: []string{"--namespace", "kube-system"}},
				{Name: "kubernetes-external-secrets", Flags: []string{"--namespace", "kube-system"}},
				{Name: "external-secrets", Flags: []string{"--namespace", "default"}},
				{Name: "my-release", Flags: []string{"--namespace", "default"}},
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
			templated: []exectest.Release{
				{Name: "logging", Flags: []string{"--namespace", "kube-system"}},
				{Name: "kubernetes-external-secrets", Flags: []string{"--namespace", "kube-system"}},
				{Name: "external-secrets", Flags: []string{"--namespace", "default"}},
				{Name: "my-release", Flags: []string{"--namespace", "default"}},
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
			templated: []exectest.Release{
				{Name: "test2"},
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
			templated: []exectest.Release{
				{Name: "test2"},
				{Name: "test3"},
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
			templated: []exectest.Release{
				{Name: "test2"},
				{Name: "test3"},
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
			templated: []exectest.Release{
				{Name: "test2"},
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
			templated: []exectest.Release{
				{Name: "test2"},
				{Name: "test3"},
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
			templated: []exectest.Release{
				{Name: "external-secrets", Flags: []string{"--namespace", "default", "--no-hooks"}},
				{Name: "my-release", Flags: []string{"--namespace", "default", "--no-hooks"}},
			},
		})
	})

	t.Run("bad selector", func(t *testing.T) {
		check(t, testcase{
			selectors: []string{"app=test_non_existent"},
			templated: nil,
			error:     "err: no releases found that matches specified selector(app=test_non_existent) and environment(default), in any helmfile",
		})
	})

	t.Run("show-only", func(t *testing.T) {
		check(t, testcase{
			fields: fields{
				showOnly: []string{"templates/resources.yaml"},
			},
			selectors: []string{"name=logging"},
			templated: []exectest.Release{
				{Name: "logging", Flags: []string{"--show-only", "templates/resources.yaml", "--namespace", "kube-system"}},
			},
		})
	})
}

func TestTemplate_StrictParsing(t *testing.T) {
	type testcase struct {
		goccyGoYaml bool
		ns          string
		error       string
	}

	check := func(t *testing.T, tc testcase) {
		t.Helper()

		v := runtime.GoccyGoYaml
		runtime.GoccyGoYaml = tc.goccyGoYaml
		t.Cleanup(func() {
			runtime.GoccyGoYaml = v
		})

		var helm = &exectest.Helm{
			FailOnUnexpectedList: true,
			FailOnUnexpectedDiff: true,
			DiffMutex:            &sync.Mutex{},
			ChartsMutex:          &sync.Mutex{},
			ReleasesMutex:        &sync.Mutex{},
			Helm3:                true,
		}

		_ = runWithLogCapture(t, "debug", func(t *testing.T, logger *zap.SugaredLogger) {
			t.Helper()

			valsRuntime, err := vals.New(vals.Options{CacheSize: 32})
			if err != nil {
				t.Errorf("unexpected error creating vals runtime: %v", err)
			}

			files := map[string]string{
				"/path/to/helmfile.yaml": `
releases:
- name: app1
  foobar: FOOBAR
  chart: incubator/raw
`,
			}

			app := appWithFs(&App{
				OverrideHelmBinary:  DefaultHelmBinary,
				fs:                  &ffs.FileSystem{Glob: filepath.Glob},
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

			tmplErr := app.Template(applyConfig{
				// if we check log output, concurrency must be 1. otherwise the test becomes non-deterministic.
				concurrency: 1,
				logger:      logger,
			})

			var gotErr string
			if tmplErr != nil {
				gotErr = tmplErr.Error()
			}

			if d := cmp.Diff(tc.error, gotErr); d != "" {
				t.Fatalf("unexpected error: want (-), got (+): %s", d)
			}
		})
	}

	t.Run("fail due to unknown field with goccy/go-yaml", func(t *testing.T) {
		check(t, testcase{
			goccyGoYaml: true,
			error: `in ./helmfile.yaml: failed to read helmfile.yaml: reading document at index 1. Started seeing this since Helmfile v1? Add the .gotmpl file extension: [4:3] unknown field "foobar"
   2 | releases:
   3 | - name: app1
>  4 |   foobar: FOOBAR
         ^
   5 |   chart: incubator/raw`,
		})
	})

	t.Run("fail due to unknown field with gopkg.in/yaml.v2", func(t *testing.T) {
		check(t, testcase{
			goccyGoYaml: false,
			error: `in ./helmfile.yaml: failed to read helmfile.yaml: reading document at index 1. Started seeing this since Helmfile v1? Add the .gotmpl file extension: yaml: unmarshal errors:
  line 4: field foobar not found in type state.ReleaseSpec`,
		})
	})
}

func TestTemplate_CyclicInheritance(t *testing.T) {
	type testcase struct {
		ns    string
		error string
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

		_ = runWithLogCapture(t, "debug", func(t *testing.T, logger *zap.SugaredLogger) {
			t.Helper()

			valsRuntime, err := vals.New(vals.Options{CacheSize: 32})
			if err != nil {
				t.Errorf("unexpected error creating vals runtime: %v", err)
			}

			files := map[string]string{
				"/path/to/helmfile.yaml": `
templates:
  a:
    inherit:
    - template: b
    values:
    - a.yaml
  b:
    inherit:
    - template: c
    values:
    - b.yaml
  c:
    inherit:
    - template: a
    values:
    - c.yaml
releases:
- name: app1
  inherit:
  - template: a
  chart: incubator/raw
`,
			}

			app := appWithFs(&App{
				OverrideHelmBinary:  DefaultHelmBinary,
				fs:                  &ffs.FileSystem{Glob: filepath.Glob},
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

			tmplErr := app.Template(applyConfig{
				// if we check log output, concurrency must be 1. otherwise the test becomes non-deterministic.
				concurrency: 1,
				logger:      logger,
			})

			var gotErr string
			if tmplErr != nil {
				gotErr = tmplErr.Error()
			}

			if d := cmp.Diff(tc.error, gotErr); d != "" {
				t.Fatalf("unexpected error: want (-), got (+): %s", d)
			}
		})
	}

	t.Run("fail due to cyclic inheritance", func(t *testing.T) {
		check(t, testcase{
			error: `in ./helmfile.yaml: failed executing release templates in "helmfile.yaml": unable to load release "app1" with template: cyclic inheritance detected: a->b->c->a`,
		})
	})
}
