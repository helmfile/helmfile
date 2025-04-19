package app

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	goruntime "runtime"
	"strings"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/helmfile/vals"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"helm.sh/helm/v3/pkg/chart"

	"github.com/helmfile/helmfile/pkg/envvar"
	"github.com/helmfile/helmfile/pkg/exectest"
	ffs "github.com/helmfile/helmfile/pkg/filesystem"
	"github.com/helmfile/helmfile/pkg/helmexec"
	"github.com/helmfile/helmfile/pkg/remote"
	"github.com/helmfile/helmfile/pkg/runtime"
	"github.com/helmfile/helmfile/pkg/state"
	"github.com/helmfile/helmfile/pkg/testhelper"
	"github.com/helmfile/helmfile/pkg/testutil"
)

func appWithFs(app *App, files map[string]string) *App {
	fs := testhelper.NewTestFs(files)
	return injectFs(app, fs)
}

func injectFs(app *App, fs *testhelper.TestFs) *App {
	if app.Set == nil {
		// Consistent behavior with NewGlobalImpl.
		// Doesn't really belong here, but simplest place for it until some refactoring happens
		app.Set = make(map[string]any)
	}

	app.fs = fs.ToFileSystem()
	return app
}

func expectNoCallsToHelm(app *App) {
	expectNoCallsToHelmVersion(app)
}

func expectNoCallsToHelmVersion(app *App) {
	if app.helms != nil {
		panic("invalid call to expectNoCallsToHelm")
	}

	app.helms = map[helmKey]helmexec.Interface{
		createHelmKey(app.OverrideHelmBinary, app.OverrideKubeContext): testutil.NewV3HelmExec(true),
	}
}

func newAppTestLogger() *zap.SugaredLogger {
	return helmexec.NewLogger(io.Discard, "debug")
}

func TestVisitDesiredStatesWithReleasesFiltered_ReleaseOrder(t *testing.T) {
	files := map[string]string{
		"/path/to/helmfile.yaml": `
helmfiles:
- helmfile.d/a*.yaml
- helmfile.d/b*.yaml
`,
		"/path/to/helmfile.d/a1.yaml": `
releases:
- name: zipkin
  chart: stable/zipkin
`,
		"/path/to/helmfile.d/a2.yaml": `
releases:
- name: prometheus
  chart: stable/prometheus
`,
		"/path/to/helmfile.d/b.yaml": `
releases:
- name: grafana
  chart: stable/grafana
`,
	}
	fs := testhelper.NewTestFs(files)
	fs.GlobFixtures["/path/to/helmfile.d/a*.yaml"] = []string{"/path/to/helmfile.d/a2.yaml", "/path/to/helmfile.d/a1.yaml"}
	app := &App{
		OverrideHelmBinary:  DefaultHelmBinary,
		OverrideKubeContext: "default",
		Logger:              newAppTestLogger(),
		Namespace:           "",
		Env:                 "default",
		FileOrDir:           "helmfile.yaml",
	}

	expectNoCallsToHelm(app)

	app = injectFs(app, fs)
	actualOrder := []string{}
	noop := func(run *Run) (bool, []error) {
		actualOrder = append(actualOrder, run.state.FilePath)
		return false, []error{}
	}

	err := app.ForEachState(
		noop,
		false,
		SetFilter(true),
	)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	expectedOrder := []string{"a1.yaml", "a2.yaml", "b.yaml", "helmfile.yaml"}
	if !reflect.DeepEqual(actualOrder, expectedOrder) {
		t.Errorf("unexpected order of processed state files: expected=%v, actual=%v", expectedOrder, actualOrder)
	}
}

func Noop(_ *Run) (bool, []error) {
	return false, []error{}
}

func TestVisitDesiredStatesWithReleasesFiltered_EnvValuesFileOrder(t *testing.T) {
	files := map[string]string{
		"/path/to/helmfile.yaml": `
environments:
  default:
    values:
    - env.*.yaml
---
releases:
- name: zipkin
  chart: stable/zipkin
`,
		"/path/to/env.1.yaml": `FOO: 1
BAR: 2
`,
		"/path/to/env.2.yaml": `BAR: 3
BAZ: 4
`,
	}
	fs := testhelper.NewTestFs(files)
	fs.GlobFixtures["/path/to/env.*.yaml"] = []string{"/path/to/env.2.yaml", "/path/to/env.1.yaml"}
	app := &App{
		OverrideHelmBinary:  DefaultHelmBinary,
		OverrideKubeContext: "default",
		Logger:              newAppTestLogger(),
		Namespace:           "",
		Env:                 "default",
		FileOrDir:           "helmfile.yaml",
	}

	expectNoCallsToHelm(app)

	app = injectFs(app, fs)

	err := app.ForEachState(
		Noop,
		false,
		SetFilter(true),
	)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	expectedOrder := []string{"helmfile.yaml", "/path/to/env.1.yaml", "/path/to/env.2.yaml"}
	actualOrder := fs.SuccessfulReads()
	if !reflect.DeepEqual(actualOrder, expectedOrder) {
		t.Errorf("unexpected order of processed state files: expected=%v, actual=%v", expectedOrder, actualOrder)
	}
}

func TestVisitDesiredStatesWithReleasesFiltered_MissingEnvValuesFile(t *testing.T) {
	files := map[string]string{
		"/path/to/helmfile.yaml": `
environments:
  default:
    values:
    - env.*.yaml
---
releases:
- name: zipkin
  chart: stable/zipkin
`,
	}
	fs := testhelper.NewTestFs(files)
	app := &App{
		OverrideHelmBinary:  DefaultHelmBinary,
		OverrideKubeContext: "default",
		Logger:              newAppTestLogger(),
		Namespace:           "",
		Env:                 "default",
		FileOrDir:           "helmfile.yaml",
	}

	expectNoCallsToHelm(app)

	app = injectFs(app, fs)

	err := app.ForEachState(
		Noop,
		false,
		SetFilter(true),
	)
	if err == nil {
		t.Fatal("expected error did not occur")
	}

	expected := "in ./helmfile.yaml: failed to read helmfile.yaml: environment values file matching \"env.*.yaml\" does not exist in \".\""
	if err.Error() != expected {
		t.Errorf("unexpected error: expected=%s, got=%v", expected, err)
	}
}

func TestVisitDesiredStatesWithReleasesFiltered_Issue1008_MissingNonDefaultEnvInBase(t *testing.T) {
	files := map[string]string{
		"/path/to/base.yaml": `
helmDefaults:
  wait: true
  waitForJobs: true
`,
		"/path/to/helmfile.yaml": `
bases:
- base.yaml
environments:
  test:
---
releases:
- name: zipkin
  chart: stable/zipkin
`,
	}
	fs := testhelper.NewTestFs(files)
	app := &App{
		OverrideHelmBinary:  DefaultHelmBinary,
		OverrideKubeContext: "default",
		Logger:              newAppTestLogger(),
		Namespace:           "",
		Env:                 "test",
	}

	expectNoCallsToHelm(app)

	app = injectFs(app, fs)

	err := app.ForEachState(
		Noop,
		false,
		SetFilter(true),
	)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestVisitDesiredStatesWithReleasesFiltered_MissingEnvValuesFileHandler(t *testing.T) {
	testcases := []struct {
		name        string
		handler     string
		filePattern string
		expectErr   bool
	}{
		{name: "error handler with no files matching glob", handler: "Error", filePattern: "env.*.yaml", expectErr: true},
		{name: "warn handler with no files matching glob", handler: "Warn", filePattern: "env.*.yaml", expectErr: false},
		{name: "info handler with no files matching glob", handler: "Info", filePattern: "env.*.yaml", expectErr: false},
		{name: "debug handler with no files matching glob", handler: "Debug", filePattern: "env.*.yaml", expectErr: false},
	}

	for i := range testcases {
		testcase := testcases[i]
		t.Run(testcase.name, func(t *testing.T) {
			files := map[string]string{
				"/path/to/helmfile.yaml": fmt.Sprintf(`
environments:
  default:
    missingFileHandler: %s
    values:
    - %s
---
releases:
- name: zipkin
  chart: stable/zipkin
`, testcase.handler, testcase.filePattern),
			}
			fs := testhelper.NewTestFs(files)
			app := &App{
				OverrideHelmBinary:  DefaultHelmBinary,
				OverrideKubeContext: "default",
				Logger:              newAppTestLogger(),
				Namespace:           "",
				Env:                 "default",
				FileOrDir:           "helmfile.yaml",
			}

			expectNoCallsToHelm(app)

			app = injectFs(app, fs)

			err := app.ForEachState(
				Noop,
				false,
				SetFilter(true),
			)
			if testcase.expectErr && err == nil {
				t.Fatal("expected error did not occur")
			}

			if !testcase.expectErr && err != nil {
				t.Errorf("not error expected, but got: %v", err)
			}
		})
	}
}

// See https://github.com/roboll/helmfile/issues/193
func TestVisitDesiredStatesWithReleasesFiltered(t *testing.T) {
	files := map[string]string{
		"/path/to/helmfile.yaml": `
helmfiles:
- helmfile.d/a*.yaml
- helmfile.d/b*.yaml
`,
		"/path/to/helmfile.d/a1.yaml": `
releases:
- name: zipkin
  chart: stable/zipkin
`,
		"/path/to/helmfile.d/a2.yaml": `
releases:
- name: prometheus
  chart: stable/prometheus
`,
		"/path/to/helmfile.d/b.yaml": `
releases:
- name: grafana
  chart: stable/grafana
`,
	}
	testcases := []struct {
		name      string
		expectErr bool
	}{
		{name: "prometheus", expectErr: false},
		{name: "zipkin", expectErr: false},
		{name: "grafana", expectErr: false},
		{name: "elasticsearch", expectErr: true},
	}

	for _, testcase := range testcases {
		fs := testhelper.NewTestFs(files)
		fs.GlobFixtures["/path/to/helmfile.d/a*.yaml"] = []string{"/path/to/helmfile.d/a2.yaml", "/path/to/helmfile.d/a1.yaml"}
		app := &App{
			OverrideHelmBinary:  DefaultHelmBinary,
			OverrideKubeContext: "default",
			Logger:              newAppTestLogger(),
			Selectors:           []string{fmt.Sprintf("name=%s", testcase.name)},
			Namespace:           "",
			Env:                 "default",
			FileOrDir:           "helmfile.yaml",
		}

		expectNoCallsToHelm(app)

		app = injectFs(app, fs)

		err := app.ForEachState(
			Noop,
			false,
			SetFilter(true),
		)
		if testcase.expectErr && err == nil {
			t.Errorf("error expected but not happened for name=%s", testcase.name)
		} else if !testcase.expectErr && err != nil {
			t.Errorf("unexpected error for name=%s: %v", testcase.name, err)
		}
	}
}

// See https://github.com/roboll/helmfile/issues/320
func TestVisitDesiredStatesWithReleasesFiltered_UndefinedEnv(t *testing.T) {
	files := map[string]string{
		"/path/to/helmfile.yaml": `
environments:
  prod:

helmfiles:
- helmfile.d/a*.yaml
`,
		"/path/to/helmfile.d/a1.yaml": `
environments:
  prod:
---
releases:
- name: zipkin
  chart: stable/zipkin
`,
	}

	testcases := []struct {
		name      string
		expectErr bool
	}{
		{name: "undefined_env", expectErr: true},
		{name: "default", expectErr: false},
		{name: "prod", expectErr: false},
	}

	for _, testcase := range testcases {
		app := appWithFs(&App{
			OverrideHelmBinary:  DefaultHelmBinary,
			OverrideKubeContext: "default",
			Logger:              newAppTestLogger(),
			Namespace:           "",
			Selectors:           []string{},
			Env:                 testcase.name,
			FileOrDir:           "helmfile.yaml",
		}, files)

		expectNoCallsToHelm(app)

		err := app.ForEachState(
			Noop,
			false,
			SetFilter(true),
		)
		if testcase.expectErr && err == nil {
			t.Errorf("error expected but not happened for environment=%s", testcase.name)
		} else if !testcase.expectErr && err != nil {
			t.Errorf("unexpected error for environment=%s: %v", testcase.name, err)
		}
	}
}

type ctxLogger struct {
	label string
}

func (cl *ctxLogger) Write(b []byte) (int, error) {
	return os.Stderr.Write(append([]byte(cl.label+":"), b...))
}

// See https://github.com/roboll/helmfile/issues/322
func TestVisitDesiredStatesWithReleasesFiltered_Selectors(t *testing.T) {
	files := map[string]string{
		"/path/to/helmfile.yaml": `
helmfiles:
- helmfile.d/a*.yaml
- helmfile.d/b*.yaml
`,
		"/path/to/helmfile.d/a1.yaml": `
releases:
- name: zipkin
  chart: stable/zipkin
`,
		"/path/to/helmfile.d/a2.yaml": `
releases:
- name: prometheus
  chart: stable/prometheus
`,
		"/path/to/helmfile.d/b.yaml": `
releases:
- name: grafana
  chart: stable/grafana
- name: foo
  chart: charts/foo
  labels:
    duplicatedNs: yes
- name: foo
  chart: charts/foo
  labels:
    duplicatedNs: yes
- name: grafana
  chart: stable/grafana
- name: foo
  chart: charts/foo
  kubeContext: baz
  labels:
    duplicatedCtx: yes
- name: foo
  chart: charts/foo
  kubeContext: baz
  labels:
    duplicatedCtx: yes
- name: bar
  chart: charts/foo
  namespace:  bar1
  labels:
    duplicatedOK: yes
- name: bar
  chart: charts/foo
  namespace: bar2
  labels:
    duplicatedOK: yes
`,
	}

	testcases := []struct {
		label         string
		expectedCount int
		expectErr     bool
		errMsg        string
	}{
		{label: "name=prometheus", expectedCount: 1, expectErr: false},
		{label: "name=", expectedCount: 0, expectErr: true, errMsg: "in ./helmfile.yaml: in .helmfiles[0]: in /path/to/helmfile.d/a1.yaml: malformed label: name=. Expected label in form k=v or k!=v"},
		{label: "name!=", expectedCount: 0, expectErr: true, errMsg: "in ./helmfile.yaml: in .helmfiles[0]: in /path/to/helmfile.d/a1.yaml: malformed label: name!=. Expected label in form k=v or k!=v"},
		{label: "name", expectedCount: 0, expectErr: true, errMsg: "in ./helmfile.yaml: in .helmfiles[0]: in /path/to/helmfile.d/a1.yaml: malformed label: name. Expected label in form k=v or k!=v"},
		// See https://github.com/roboll/helmfile/issues/193
		{label: "duplicatedNs=yes", expectedCount: 0, expectErr: true, errMsg: "in ./helmfile.yaml: in .helmfiles[2]: in /path/to/helmfile.d/b.yaml: duplicate release \"foo\" found in kubecontext \"default\": there were 2 releases named \"foo\" matching specified selector"},
		{label: "duplicatedCtx=yes", expectedCount: 0, expectErr: true, errMsg: "in ./helmfile.yaml: in .helmfiles[2]: in /path/to/helmfile.d/b.yaml: duplicate release \"foo\" found in kubecontext \"default\": there were 2 releases named \"foo\" matching specified selector"},
		{label: "duplicatedOK=yes", expectedCount: 2, expectErr: false},
	}
	for _, testcase := range testcases {
		t.Run(testcase.label, func(t *testing.T) {
			actual := []string{}

			collectReleases := func(run *Run) (bool, []error) {
				for _, r := range run.state.Releases {
					actual = append(actual, r.Name)
				}
				return false, []error{}
			}

			app := appWithFs(&App{
				OverrideHelmBinary:  DefaultHelmBinary,
				OverrideKubeContext: "default",
				Logger:              helmexec.NewLogger(&ctxLogger{label: testcase.label}, "debug"),
				Namespace:           "",
				Selectors:           []string{testcase.label},
				Env:                 "default",
				FileOrDir:           "helmfile.yaml",
			}, files)

			expectNoCallsToHelm(app)

			err := app.ForEachState(
				collectReleases,
				false,
				SetFilter(true),
			)
			if testcase.expectErr {
				if err == nil {
					t.Errorf("error expected but not happened for selector %s", testcase.label)
				} else if err.Error() != testcase.errMsg {
					t.Errorf("unexpected error message: expected=\"%s\", actual=\"%s\"", testcase.errMsg, err.Error())
				}
			} else if !testcase.expectErr && err != nil {
				t.Errorf("unexpected error for selector %s: %v", testcase.label, err)
			}
			if len(actual) != testcase.expectedCount {
				t.Errorf("unexpected release count for selector %s: expected=%d, actual=%d", testcase.label, testcase.expectedCount, len(actual))
			}
		})
	}
}

func TestVisitDesiredStatesWithReleasesFiltered_EmbeddedSelectors(t *testing.T) {
	files := map[string]string{
		"/path/to/helmfile.yaml": `
helmfiles:
- path: helmfile.d/a*.yaml
  selectors:
  - name=prometheus
  - name=zipkin
- helmfile.d/b*.yaml
- path: helmfile.d/c*.yaml
  selectors: []
`,
		"/path/to/helmfile.d/a1.yaml": `
releases:
- name: zipkin
  chart: stable/zipkin
`,
		"/path/to/helmfile.d/a2.yaml": `
releases:
- name: prometheus
  chart: stable/prometheus
`,
		"/path/to/helmfile.d/a3.yaml": `
releases:
- name: mongodb
  chart: stable/mongodb
`,
		"/path/to/helmfile.d/b.yaml": `
releases:
- name: grafana
  chart: stable/grafana
- name: bar
  chart: charts/foo
  namespace:  bar1
  labels:
    duplicatedOK: yes
- name: bar
  chart: charts/foo
  namespace: bar2
  labels:
    duplicatedOK: yes
`,
		"/path/to/helmfile.d/c.yaml": `
releases:
- name: grafana
  chart: stable/grafana
- name: postgresql
  chart: charts/postgresql
  labels:
    whatever: yes
`,
	}
	// Check with legacy behavior, that is when no explicit selector then sub-helmfiles inherits from command line selector
	legacyTestcases := []struct {
		label            string
		expectedReleases []string
		expectErr        bool
		errMsg           string
	}{
		{label: "duplicatedOK=yes", expectedReleases: []string{"zipkin", "prometheus", "bar", "bar", "grafana", "postgresql"}, expectErr: false},
		{label: "name=zipkin", expectedReleases: []string{"zipkin", "prometheus", "grafana", "postgresql"}, expectErr: false},
		{label: "name=grafana", expectedReleases: []string{"zipkin", "prometheus", "grafana", "grafana", "postgresql"}, expectErr: false},
		{label: "name=doesnotexists", expectedReleases: []string{"zipkin", "prometheus", "grafana", "postgresql"}, expectErr: false},
	}
	runFilterSubHelmFilesTests(legacyTestcases, files, t, "1st EmbeddedSelectors")

	// Check with experimental behavior, that is when no explicit selector then sub-helmfiles do no inherit from any selector
	desiredTestcases := []struct {
		label            string
		expectedReleases []string
		expectErr        bool
		errMsg           string
	}{
		{label: "duplicatedOK=yes", expectedReleases: []string{"zipkin", "prometheus", "grafana", "bar", "bar", "grafana", "postgresql"}, expectErr: false},
		{label: "name=doesnotexists", expectedReleases: []string{"zipkin", "prometheus", "grafana", "bar", "bar", "grafana", "postgresql"}, expectErr: false},
	}

	t.Setenv(envvar.Experimental, ExperimentalSelectorExplicit)

	runFilterSubHelmFilesTests(desiredTestcases, files, t, "2nd EmbeddedSelectors")
}

func TestVisitDesiredStatesWithReleasesFiltered_InheritedSelectors_3leveldeep(t *testing.T) {
	files := map[string]string{
		"/path/to/helmfile.yaml": `
helmfiles:
- helmfile.d/a*.yaml
releases:
- name: mongodb
  chart: stable/mongodb
`,
		"/path/to/helmfile.d/a.yaml": `
helmfiles:
- b*.yaml
releases:
- name: zipkin
  chart: stable/zipkin
`,
		"/path/to/helmfile.d/b.yaml": `
releases:
- name: grafana
  chart: stable/grafana
`,
	}
	// Check with legacy behavior, that is when no explicit selector then sub-helmfiles inherits from command line selector
	legacyTestcases := []struct {
		label            string
		expectedReleases []string
		expectErr        bool
		errMsg           string
	}{
		{label: "name!=grafana", expectedReleases: []string{"zipkin", "mongodb"}, expectErr: false},
	}
	runFilterSubHelmFilesTests(legacyTestcases, files, t, "1st 3leveldeep")

	// Check with experimental behavior, that is when no explicit selector then sub-helmfiles do no inherit from any selector
	desiredTestcases := []struct {
		label            string
		expectedReleases []string
		expectErr        bool
		errMsg           string
	}{
		{label: "name!=grafana", expectedReleases: []string{"grafana", "zipkin", "mongodb"}, expectErr: false},
	}

	t.Setenv(envvar.Experimental, ExperimentalSelectorExplicit)

	runFilterSubHelmFilesTests(desiredTestcases, files, t, "2nd 3leveldeep")
}

func TestVisitDesiredStatesWithReleasesFiltered_InheritedSelectors_inherits(t *testing.T) {
	files := map[string]string{
		"/path/to/helmfile.yaml": `
helmfiles:
- helmfile.d/a*.yaml
- path: helmfile.d/a*.yaml
  selectors:
  - select=foo
releases:
- name: mongodb
  chart: stable/mongodb
`,
		"/path/to/helmfile.d/a.yaml": `
helmfiles:
- path: b*.yaml
  selectorsInherited: true
releases:
- name: zipkin
  chart: stable/zipkin
  labels:
    select: foo
`,
		"/path/to/helmfile.d/b.yaml": `
releases:
- name: grafana
  chart: stable/grafana
- name: prometheus
  chart: stable/prometheus
  labels:
    select: foo
`,
	}
	// Check with legacy behavior, that is when no explicit selector then sub-helmfiles inherits from command line selector
	legacyTestcases := []struct {
		label            string
		expectedReleases []string
		expectErr        bool
		errMsg           string
	}{
		{label: "name=grafana", expectedReleases: []string{"grafana", "prometheus", "zipkin"}, expectErr: false},
		{label: "select!=foo", expectedReleases: []string{"grafana", "prometheus", "zipkin", "mongodb"}, expectErr: false},
	}
	runFilterSubHelmFilesTests(legacyTestcases, files, t, "1st inherits")

	// Check with experimental behavior, that is when no explicit selector then sub-helmfiles do no inherit from any selector
	desiredTestcases := []struct {
		label            string
		expectedReleases []string
		expectErr        bool
		errMsg           string
	}{
		{label: "name=grafana", expectedReleases: []string{"grafana", "prometheus", "zipkin", "prometheus", "zipkin"}, expectErr: false},
		{label: "select!=foo", expectedReleases: []string{"grafana", "prometheus", "zipkin", "prometheus", "zipkin", "mongodb"}, expectErr: false},
	}

	t.Setenv(envvar.Experimental, ExperimentalSelectorExplicit)

	runFilterSubHelmFilesTests(desiredTestcases, files, t, "2nd inherits")
}

func runFilterSubHelmFilesTests(testcases []struct {
	label            string
	expectedReleases []string
	expectErr        bool
	errMsg           string
}, files map[string]string, t *testing.T, testName string) {
	t.Helper()

	for _, testcase := range testcases {
		actual := []string{}

		collectReleases := func(run *Run) (bool, []error) {
			for _, r := range run.state.Releases {
				actual = append(actual, r.Name)
			}
			return false, []error{}
		}

		app := appWithFs(&App{
			OverrideHelmBinary:  DefaultHelmBinary,
			OverrideKubeContext: "default",
			Logger:              newAppTestLogger(),
			Namespace:           "",
			Selectors:           []string{testcase.label},
			Env:                 "default",
			FileOrDir:           "helmfile.yaml",
		}, files)

		expectNoCallsToHelm(app)

		err := app.ForEachState(
			collectReleases,
			false,
			SetFilter(true),
		)
		if testcase.expectErr {
			if err == nil {
				t.Errorf("[%s]error expected but not happened for selector %s", testName, testcase.label)
			} else if err.Error() != testcase.errMsg {
				t.Errorf("[%s]unexpected error message: expected=\"%s\", actual=\"%s\"", testName, testcase.errMsg, err.Error())
			}
		} else if !testcase.expectErr && err != nil {
			t.Errorf("[%s]unexpected error for selector %s: %v", testName, testcase.label, err)
		}
		if !reflect.DeepEqual(actual, testcase.expectedReleases) {
			t.Errorf("[%s]unexpected releases for selector %s: expected=%v, actual=%v", testName, testcase.label, testcase.expectedReleases, actual)
		}
	}
}

func TestVisitDesiredStatesWithReleasesFiltered_EmbeddedNestedStateAdditionalEnvValues(t *testing.T) {
	files := map[string]string{
		"/path/to/helmfile.yaml.gotmpl": `
helmfiles:
- path: helmfile.d/a*.yaml.gotmpl
  values:
  - env.values.yaml
- helmfile.d/b*.yaml.gotmpl
- path: helmfile.d/c*.yaml.gotmpl
  values:
  - env.values.yaml
`,
		"/path/to/helmfile.d/a1.yaml.gotmpl": `
environments:
  default:
    values:
    - ns: INLINE_NS
---
releases:
- name: foo
  chart: stable/zipkin
  namespace: {{ .Environment.Values.ns }}
`,
		"/path/to/helmfile.d/b.yaml.gotmpl": `
environments:
  default:
    values:
    - ns: INLINE_NS
---
releases:
- name: bar
  chart: stable/grafana
  namespace: {{ .Environment.Values.ns }}
`,
		"/path/to/helmfile.d/c.yaml.gotmpl": `
environments:
  default:
    values:
    - ns: INLINE_NS
---
releases:
- name: baz
  chart: stable/envoy
  namespace: {{ .Environment.Values.ns }}
`,
		"/path/to/env.values.yaml": `
ns: INLINE_NS
`,
	}

	app := appWithFs(&App{
		OverrideHelmBinary:  DefaultHelmBinary,
		OverrideKubeContext: "default",
		Logger:              newAppTestLogger(),
		Namespace:           "",
		Selectors:           []string{},
		Env:                 "default",
		FileOrDir:           "/path/to/helmfile.yaml.gotmpl",
	}, files)

	expectNoCallsToHelm(app)

	processed := []state.ReleaseSpec{}

	collectReleases := func(run *Run) (bool, []error) {
		processed = append(processed, run.state.Releases...)
		return false, []error{}
	}

	err := app.ForEachState(
		collectReleases,
		false,
		SetFilter(true),
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	type release struct {
		chart string
		ns    string
	}

	expectedReleases := map[string]release{
		"foo": {"stable/zipkin", "INLINE_NS"},
		"bar": {"stable/grafana", "INLINE_NS"},
		"baz": {"stable/envoy", "INLINE_NS"},
	}

	for name := range processed {
		actual := processed[name]
		t.Run(actual.Name, func(t *testing.T) {
			expected, ok := expectedReleases[actual.Name]
			if !ok {
				t.Fatalf("unexpected release processed: %v", actual)
			}

			if expected.chart != actual.Chart {
				t.Errorf("unexpected chart: expected=%s, got=%s", expected.chart, actual.Chart)
			}

			if expected.ns != actual.Namespace {
				t.Errorf("unexpected namespace: expected=%s, got=%s", expected.ns, actual.Namespace)
			}
		})
	}
}

// See https://github.com/roboll/helmfile/issues/312
func TestVisitDesiredStatesWithReleasesFiltered_ReverseOrder(t *testing.T) {
	files := map[string]string{
		"/path/to/helmfile.yaml": `
helmfiles:
- helmfile.d/a*.yaml
- helmfile.d/b*.yaml
`,
		"/path/to/helmfile.d/a1.yaml": `
releases:
- name: zipkin
  chart: stable/zipkin
`,
		"/path/to/helmfile.d/a2.yaml": `
releases:
- name: prometheus
  chart: stable/prometheus
- name: elasticsearch
  chart: stable/elasticsearch
`,
		"/path/to/helmfile.d/b.yaml": `
releases:
- name: grafana
  chart: stable/grafana
`,
	}

	expected := []string{"grafana", "elasticsearch", "prometheus", "zipkin"}

	testcases := []struct {
		reverse  bool
		expected []string
	}{
		{reverse: false, expected: []string{"zipkin", "prometheus", "elasticsearch", "grafana"}},
		{reverse: true, expected: []string{"grafana", "elasticsearch", "prometheus", "zipkin"}},
	}
	for _, testcase := range testcases {
		actual := []string{}

		collectReleases := func(run *Run) (bool, []error) {
			for _, r := range run.state.Releases {
				actual = append(actual, r.Name)
			}
			return false, []error{}
		}
		app := appWithFs(&App{
			OverrideHelmBinary:  DefaultHelmBinary,
			OverrideKubeContext: "default",
			Logger:              newAppTestLogger(),
			Namespace:           "",
			Selectors:           []string{},
			Env:                 "default",
			FileOrDir:           "helmfile.yaml",
		}, files)

		expectNoCallsToHelm(app)

		err := app.ForEachState(
			collectReleases,
			false,
			SetReverse(testcase.reverse),
			SetFilter(true),
		)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !reflect.DeepEqual(testcase.expected, actual) {
			t.Errorf("releases did not match: expected=%v actual=%v", expected, actual)
		}
	}
}

func TestVisitDesiredStatesWithReleasesFiltered_EnvironmentValueOverrides(t *testing.T) {
	files := map[string]string{
		"/path/to/helmfile.yaml.gotmpl": `
environments:
  default:
    values:
    - values.yaml
---
releases:
- name: {{ .Environment.Values.foo }}-{{ .Environment.Values.bar }}-{{ .Environment.Values.baz }}
  chart: stable/zipkin
`,
		"/path/to/values.yaml": `
foo: foo
bar: bar
baz: baz
`,
		"/path/to/overrides.yaml": `
foo: "foo1"
bar: "bar1"
`,
	}

	testcases := []struct {
		expected string
	}{
		{expected: "foo1-bar2-baz1"},
	}
	for _, testcase := range testcases {
		actual := []string{}

		collectReleases := func(run *Run) (bool, []error) {
			for _, r := range run.state.Releases {
				actual = append(actual, r.Name)
			}
			return false, []error{}
		}
		app := appWithFs(&App{
			OverrideHelmBinary:  DefaultHelmBinary,
			OverrideKubeContext: "default",
			Logger:              newAppTestLogger(),
			Namespace:           "",
			Selectors:           []string{},
			Env:                 "default",
			ValuesFiles:         []string{"overrides.yaml"},
			Set:                 map[string]any{"bar": "bar2", "baz": "baz1"},
			FileOrDir:           "helmfile.yaml.gotmpl",
		}, files)

		expectNoCallsToHelm(app)

		err := app.ForEachState(
			collectReleases,
			false,
			SetFilter(true),
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(actual) != 1 {
			t.Errorf("unexpected number of processed releases: expected=1, got=%d", len(actual))
		}
		if actual[0] != testcase.expected {
			t.Errorf("unexpected result: expected=%s, got=%s", testcase.expected, actual[0])
		}
	}
}

func TestVisitDesiredStatesWithReleasesFiltered_StateValueOverrides(t *testing.T) {
	envTmplExpr := "{{ .Values.x.foo }}-{{ .Values.x.bar }}-{{ .Values.x.baz }}-{{ .Values.x.hoge }}-{{ .Values.x.fuga }}-{{ .Values.x.a | first | pluck \"b\" | first | first | pluck \"c\" | first }}"
	relTmplExpr := "\"{{`{{ .Values.x.foo }}-{{ .Values.x.bar }}-{{ .Values.x.baz }}-{{ .Values.x.hoge }}-{{ .Values.x.fuga }}-{{ .Values.x.a | first | pluck \\\"b\\\" | first | first | pluck \\\"c\\\" | first }}`}}\""

	testcases := []struct {
		expr, env, expected string
	}{
		{
			expr:     envTmplExpr,
			env:      "default",
			expected: "foo-bar_default-baz_override-hoge_set-fuga_set-C",
		},
		{
			expr:     envTmplExpr,
			env:      "production",
			expected: "foo-bar_production-baz_override-hoge_set-fuga_set-C",
		},
		{
			expr:     relTmplExpr,
			env:      "default",
			expected: "foo-bar_default-baz_override-hoge_set-fuga_set-C",
		},
		{
			expr:     relTmplExpr,
			env:      "production",
			expected: "foo-bar_production-baz_override-hoge_set-fuga_set-C",
		},
	}
	for i := range testcases {
		testcase := testcases[i]
		t.Run(fmt.Sprintf("case %d", i), func(t *testing.T) {
			files := map[string]string{
				"/path/to/helmfile.yaml.gotmpl": fmt.Sprintf(`
# The top-level "values" are "base" values has inherited to state values with the lowest priority.
# The lowest priority results in environment-specific values to override values defined in the base.
values:
- values.yaml

environments:
  default:
    values:
    - default.yaml
  production:
    values:
    - production.yaml
---
releases:
- name: %s
  chart: %s
  namespace: %s
`, testcase.expr, testcase.expr, testcase.expr),
				"/path/to/values.yaml": `
x:
  foo: foo
  bar: bar
  baz: baz
  hoge: hoge
  fuga: fuga
  a: []
`,
				"/path/to/default.yaml": `
x:
  bar: "bar_default"
  baz: "baz_default"
  a:
  - b: []
`,
				"/path/to/production.yaml": `
x:
  bar: "bar_production"
  baz: "baz_production"
  a:
  - b: []
`,
				"/path/to/overrides.yaml": `
x:
  baz: baz_override
  hoge: hoge_override
  a:
  - b:
    - c: C
`,
			}

			actual := []state.ReleaseSpec{}

			collectReleases := func(run *Run) (bool, []error) {
				actual = append(actual, run.state.Releases...)
				return false, []error{}
			}
			app := appWithFs(&App{
				OverrideHelmBinary:  DefaultHelmBinary,
				OverrideKubeContext: "default",
				Logger:              newAppTestLogger(),
				Namespace:           "",
				Selectors:           []string{},
				Env:                 testcase.env,
				ValuesFiles:         []string{"overrides.yaml"},
				Set:                 map[string]any{"x": map[string]any{"hoge": "hoge_set", "fuga": "fuga_set"}},
				FileOrDir:           "helmfile.yaml.gotmpl",
			}, files)

			expectNoCallsToHelm(app)

			err := app.ForEachState(
				collectReleases,
				false,
				SetFilter(true),
			)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(actual) != 1 {
				t.Errorf("unexpected number of processed releases: expected=1, got=%d", len(actual))
			}
			if actual[0].Name != testcase.expected {
				t.Errorf("unexpected name: expected=%s, got=%s", testcase.expected, actual[0].Name)
			}
			if actual[0].Chart != testcase.expected {
				t.Errorf("unexpected chart: expected=%s, got=%s", testcase.expected, actual[0].Chart)
			}
			if actual[0].Namespace != testcase.expected {
				t.Errorf("unexpected namespace: expected=%s, got=%s", testcase.expected, actual[0].Namespace)
			}
		})
	}
}

func TestVisitDesiredStatesWithReleasesFiltered_ChartAtAbsPath(t *testing.T) {
	files := map[string]string{
		"/path/to/helmfile.yaml": `
releases:
- name: myapp
  chart: /path/to/mychart
`,
	}

	actual := []state.ReleaseSpec{}

	collectReleases := func(run *Run) (bool, []error) {
		actual = append(actual, run.state.Releases...)
		return false, []error{}
	}
	app := appWithFs(&App{
		OverrideHelmBinary:  DefaultHelmBinary,
		OverrideKubeContext: "default",
		Logger:              newAppTestLogger(),
		Namespace:           "",
		Env:                 "default",
		Selectors:           []string{},
		FileOrDir:           "helmfile.yaml",
	}, files)

	expectNoCallsToHelm(app)

	err := app.ForEachState(
		collectReleases,
		false,
		SetFilter(true),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actual) != 1 {
		t.Errorf("unexpected number of processed releases: expected=1, got=%d", len(actual))
	}
	if actual[0].Name != "myapp" {
		t.Errorf("unexpected name: expected=%s, got=%s", "myapp", actual[0].Name)
	}
	if actual[0].Chart != "/path/to/mychart" {
		t.Errorf("unexpected chart: expected=%s, got=%s", "/path/to/mychart", actual[0].Chart)
	}
}

func TestVisitDesiredStatesWithReleasesFiltered_RemoteTgzAsChart(t *testing.T) {
	testcases := []struct {
		expr, env, expected string
	}{
		{
			expected: "https://github.com/arangodb/kube-arangodb/releases/download/0.3.11/kube-arangodb-crd.tgz",
		},
	}
	for i := range testcases {
		testcase := testcases[i]
		t.Run(fmt.Sprintf("case %d", i), func(t *testing.T) {
			files := map[string]string{
				"/path/to/helmfile.yaml": `
releases:
  - name: arangodb-crd
    chart: https://github.com/arangodb/kube-arangodb/releases/download/0.3.11/kube-arangodb-crd.tgz
`,
			}

			actual := []state.ReleaseSpec{}

			collectReleases := func(run *Run) (bool, []error) {
				actual = append(actual, run.state.Releases...)
				return false, []error{}
			}
			app := appWithFs(&App{
				OverrideHelmBinary:  DefaultHelmBinary,
				OverrideKubeContext: "default",
				Logger:              newAppTestLogger(),
				Namespace:           "",
				Selectors:           []string{},
				Env:                 "default",
				FileOrDir:           "helmfile.yaml",
			}, files)

			expectNoCallsToHelm(app)

			err := app.ForEachState(
				collectReleases,
				false,
				SetFilter(true),
			)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(actual) != 1 {
				t.Errorf("unexpected number of processed releases: expected=1, got=%d", len(actual))
			}
			if actual[0].Chart != testcase.expected {
				t.Errorf("unexpected chart: expected=%s, got=%s", testcase.expected, actual[0].Chart)
			}
		})
	}
}

// See https://github.com/roboll/helmfile/issues/1213
func TestVisitDesiredStatesWithReleases_NoDuplicateReleasesHelm3(t *testing.T) {
	files := map[string]string{
		"/path/to/helmfile.yaml": `
releases:
- name: foo
  namespace: foo
  chart: charts/foo
- name: foo
  namespace: bar
  chart: charts/foo
`,
	}

	actual := []state.ReleaseSpec{}

	collectReleases := func(run *Run) (bool, []error) {
		actual = append(actual, run.state.Releases...)
		return false, []error{}
	}
	app := appWithFs(&App{
		OverrideHelmBinary:  DefaultHelmBinary,
		OverrideKubeContext: "default",
		Logger:              newAppTestLogger(),
		Namespace:           "",
		Env:                 "default",
		FileOrDir:           "helmfile.yaml",
	}, files)

	expectNoCallsToHelmVersion(app)

	err := app.ForEachState(
		collectReleases,
		false,
		SetFilter(true),
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// See https://github.com/roboll/helmfile/issues/1213
func TestVisitDesiredStatesWithReleases_DuplicateReleasesHelm3(t *testing.T) {
	files := map[string]string{
		"/path/to/helmfile.yaml": `
releases:
- name: foo
  namespace: foo
  chart: charts/foo
- name: foo
  namespace: foo
  chart: charts/foo
`,
	}

	actual := []state.ReleaseSpec{}

	collectReleases := func(run *Run) (bool, []error) {
		actual = append(actual, run.state.Releases...)
		return false, []error{}
	}
	app := appWithFs(&App{
		OverrideHelmBinary:  DefaultHelmBinary,
		OverrideKubeContext: "default",
		Logger:              newAppTestLogger(),
		Namespace:           "",
		Env:                 "default",
		FileOrDir:           "helmfile.yaml",
	}, files)

	expectNoCallsToHelmVersion(app)

	err := app.ForEachState(
		collectReleases,
		false,
		SetFilter(true),
	)

	expected := "in ./helmfile.yaml: duplicate release \"foo\" found in namespace \"foo\" in kubecontext \"default\": there were 2 releases named \"foo\" matching specified selector"
	if err == nil {
		t.Errorf("error expected but not happened")
	} else if err.Error() != expected {
		t.Errorf("unexpected error message: expected=\"%s\", actual=\"%s\"", expected, err.Error())
	}
}

func TestVisitDesiredStatesWithReleases_DuplicateReleasesInNsKubeContextHelm3(t *testing.T) {
	files := map[string]string{
		"/path/to/helmfile.yaml": `
releases:
- name: foo
  namespace: foo
  chart: charts/foo
  kubeContext: foo
- name: foo
  namespace: foo
  chart: charts/foo
  kubeContext: foo
`,
	}

	actual := []state.ReleaseSpec{}

	collectReleases := func(run *Run) (bool, []error) {
		actual = append(actual, run.state.Releases...)
		return false, []error{}
	}
	app := appWithFs(&App{
		OverrideHelmBinary:  DefaultHelmBinary,
		OverrideKubeContext: "default",
		Logger:              newAppTestLogger(),
		Namespace:           "",
		Env:                 "default",
		FileOrDir:           "helmfile.yaml",
	}, files)

	expectNoCallsToHelmVersion(app)

	err := app.ForEachState(
		collectReleases,
		false,
		SetFilter(true),
	)

	expected := "in ./helmfile.yaml: duplicate release \"foo\" found in namespace \"foo\" in kubecontext \"default\": there were 2 releases named \"foo\" matching specified selector"
	if err == nil {
		t.Errorf("error expected but not happened")
	} else if err.Error() != expected {
		t.Errorf("unexpected error message: expected=\"%s\", actual=\"%s\"", expected, err.Error())
	}
}

func TestLoadDesiredStateFromYaml_DuplicateReleaseName(t *testing.T) {
	yamlFile := "example/path/to/yaml/file"
	yamlContent := []byte(`releases:
- name: myrelease1
  chart: mychart1
  labels:
    stage: pre
    foo: bar
- name: myrelease1
  chart: mychart2
  labels:
    stage: post
`)
	readFile := func(filename string) ([]byte, error) {
		if filepath.ToSlash(filename) != yamlFile {
			return nil, fmt.Errorf("unexpected filename: %s", filename)
		}
		return yamlContent, nil
	}
	fs := ffs.FromFileSystem(ffs.FileSystem{ReadFile: readFile})
	app := &App{
		OverrideHelmBinary:  DefaultHelmBinary,
		OverrideKubeContext: "default",
		fs:                  fs,
		Env:                 "default",
		Logger:              newAppTestLogger(),
	}

	expectNoCallsToHelm(app)

	_, err := app.loadDesiredStateFromYaml(yamlFile)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLoadDesiredStateFromYaml_Bases(t *testing.T) {
	yamlFile := "/path/to/yaml/file.gotmpl"
	yamlContent := `bases:
- ../base.yaml
- ../base.gotmpl

{{ readFile "templates.yaml" }}

releases:
- name: myrelease1
  chart: mychart1
  labels:
    stage: pre
    foo: bar
- name: myrelease1
  chart: mychart2
  labels:
    stage: post
  <<: *default
`
	testFs := testhelper.NewTestFs(map[string]string{
		yamlFile: yamlContent,
		"/path/to/base.yaml": `environments:
  default:
    values:
    - environments/default/1.yaml
`,
		"/path/to/yaml/environments/default/1.yaml": `foo: FOO`,
		"/path/to/base.gotmpl": `environments:
  default:
    values:
    - environments/default/2.yaml

helmDefaults:
`,
		"/path/to/yaml/environments/default/2.yaml": `tillerNs: TILLER_NS`,
		"/path/to/yaml/templates.yaml": `templates:
  default: &default
    missingFileHandler: Warn
    values: ["` + "{{`" + `{{.Release.Name}}` + "`}}" + `/values.yaml"]
`,
	})
	app := &App{
		OverrideHelmBinary:  DefaultHelmBinary,
		OverrideKubeContext: "default",
		fs:                  testFs.ToFileSystem(),
		Env:                 "default",
		Logger:              newAppTestLogger(),
	}
	app.remote = remote.NewRemote(app.Logger, "", app.fs)

	expectNoCallsToHelm(app)

	st, err := app.loadDesiredStateFromYaml(yamlFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if *st.Releases[1].MissingFileHandler != "Warn" {
		t.Errorf("unexpected releases[0].missingFileHandler: expected=Warn, got=%s", *st.Releases[1].MissingFileHandler)
	}

	if st.Releases[1].Values[0] != "{{`{{.Release.Name}}`}}/values.yaml" {
		t.Errorf("unexpected releases[0].missingFileHandler: expected={{`{{.Release.Name}}`}}/values.yaml, got=%s", st.Releases[1].Values[0])
	}

	if st.FilePath != yamlFile {
		t.Errorf("unexpected filePath: expected=%s, got=%s", yamlFile, st.FilePath)
	}
}

func TestLoadDesiredStateFromYaml_MultiPartTemplate(t *testing.T) {
	yamlFile := "/path/to/yaml/file.gotmpl"
	yamlContent := `bases:
- ../base.yaml
---
bases:
- ../base.gotmpl
---
helmDefaults:
  kubeContext: {{ .Environment.Values.foo }}
---
releases:
- name: myrelease0
  chart: mychart0
---

{{ readFile "templates.yaml" }}

releases:
- name: myrelease1
  chart: mychart1
  labels:
    stage: pre
    foo: bar
- name: myrelease1
  chart: mychart2
  labels:
    stage: post
  <<: *default
`
	testFs := testhelper.NewTestFs(map[string]string{
		yamlFile: yamlContent,
		"/path/to/base.yaml": `environments:
  default:
    values:
    - environments/default/1.yaml
`,
		"/path/to/yaml/environments/default/1.yaml": `foo: FOO`,
		"/path/to/base.gotmpl": `environments:
  default:
    values:
    - environments/default/2.yaml

helmDefaults:
`,
		"/path/to/yaml/environments/default/2.yaml": `tillerNs: TILLER_NS`,
		"/path/to/yaml/templates.yaml": `templates:
  default: &default
    missingFileHandler: Warn
    values: ["` + "{{`" + `{{.Release.Name}}` + "`}}" + `/values.yaml"]
`,
	})
	app := &App{
		OverrideHelmBinary: DefaultHelmBinary,
		fs:                 testFs.ToFileSystem(),
		Env:                "default",
		Logger:             newAppTestLogger(),
	}
	app.remote = remote.NewRemote(app.Logger, testFs.Cwd, app.fs)

	expectNoCallsToHelm(app)

	st, err := app.loadDesiredStateFromYaml(yamlFile)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	firstRelease := st.Releases[0]
	if firstRelease.Name != "myrelease1" {
		t.Errorf("unexpected releases[1].name: expected=myrelease1, got=%s", firstRelease.Name)
	}
	secondRelease := st.Releases[1]
	if secondRelease.Name != "myrelease1" {
		t.Errorf("unexpected releases[2].name: expected=myrelease1, got=%s", secondRelease.Name)
	}
	if secondRelease.Values[0] != "{{`{{.Release.Name}}`}}/values.yaml" {
		t.Errorf("unexpected releases[2].missingFileHandler: expected={{`{{.Release.Name}}`}}/values.yaml, got=%s", firstRelease.Values[0])
	}
	if *secondRelease.MissingFileHandler != "Warn" {
		t.Errorf("unexpected releases[2].missingFileHandler: expected=Warn, got=%s", *firstRelease.MissingFileHandler)
	}

	if secondRelease.Values[0] != "{{`{{.Release.Name}}`}}/values.yaml" {
		t.Errorf("unexpected releases[2].missingFileHandler: expected={{`{{.Release.Name}}`}}/values.yaml, got=%s", firstRelease.Values[0])
	}

	if st.HelmDefaults.KubeContext != "FOO" {
		t.Errorf("unexpected helmDefaults.kubeContext: expected=FOO, got=%s", st.HelmDefaults.KubeContext)
	}
}

func TestLoadDesiredStateFromYaml_EnvvalsInheritanceToBaseTemplate(t *testing.T) {
	yamlFile := "/path/to/yaml/file"
	yamlContent := `bases:
- ../base.yaml
---
bases:
# "envvals inheritance"
# base.gotmpl should be able to reference environment values defined in the base.yaml and default/1.yaml
- ../base.gotmpl
---
releases:
- name: myrelease0
  chart: mychart0
`
	testFs := testhelper.NewTestFs(map[string]string{
		yamlFile: yamlContent,
		"/path/to/base.yaml": `environments:
  default:
    values:
    - environments/default/1.yaml
`,
		"/path/to/base.gotmpl": `helmDefaults:
  kubeContext: {{ .Environment.Values.foo }}
`,
		"/path/to/yaml/environments/default/1.yaml": `tillerNs: TILLER_NS
foo: FOO
`,
		"/path/to/yaml/templates.yaml": `templates:
  default: &default
    missingFileHandler: Warn
    values: ["` + "{{`" + `{{.Release.Name}}` + "`}}" + `/values.yaml"]
`,
	})
	app := &App{
		OverrideHelmBinary: DefaultHelmBinary,
		fs:                 testFs.ToFileSystem(),
		Env:                "default",
		Logger:             newAppTestLogger(),
	}
	app.remote = remote.NewRemote(app.Logger, testFs.Cwd, app.fs)

	expectNoCallsToHelm(app)

	st, err := app.loadDesiredStateFromYaml(yamlFile)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if st.Releases[0].Name != "myrelease0" {
		t.Errorf("unexpected releases[0].name: expected=myrelease0, got=%s", st.Releases[0].Name)
	}

	if st.HelmDefaults.KubeContext != "FOO" {
		t.Errorf("unexpected helmDefaults.kubeContext: expected=FOO, got=%s", st.HelmDefaults.KubeContext)
	}
}

func TestLoadDesiredStateFromYaml_InlineEnvVals(t *testing.T) {
	yamlFile := "/path/to/yaml/file"
	yamlContent := `bases:
- ../base.yaml
---
bases:
# "envvals inheritance"
# base.gotmpl should be able to reference environment values defined in the base.yaml and default/1.yaml
- ../base.gotmpl
---
releases:
- name: myrelease0
  chart: mychart0
`
	testFs := testhelper.NewTestFs(map[string]string{
		yamlFile: yamlContent,
		"/path/to/base.yaml": `environments:
  default:
    values:
    - environments/default/1.yaml
    - tillerNs: INLINE_TILLER_NS
`,
		"/path/to/base.gotmpl": `helmDefaults:
  kubeContext: {{ .Environment.Values.foo }}
`,
		"/path/to/yaml/environments/default/1.yaml": `tillerNs: TILLER_NS
foo: FOO
`,
		"/path/to/yaml/templates.yaml": `templates:
  default: &default
    missingFileHandler: Warn
    values: ["` + "{{`" + `{{.Release.Name}}` + "`}}" + `/values.yaml"]
`,
	})
	app := &App{
		OverrideHelmBinary: DefaultHelmBinary,
		fs:                 testFs.ToFileSystem(),
		Env:                "default",
		Logger:             newAppTestLogger(),
	}
	app.remote = remote.NewRemote(app.Logger, testFs.Cwd, app.fs)

	expectNoCallsToHelm(app)

	st, err := app.loadDesiredStateFromYaml(yamlFile)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if st.Releases[0].Name != "myrelease0" {
		t.Errorf("unexpected releases[0].name: expected=myrelease0, got=%s", st.Releases[0].Name)
	}

	if st.HelmDefaults.KubeContext != "FOO" {
		t.Errorf("unexpected helmDefaults.kubeContext: expected=FOO, got=%s", st.HelmDefaults.KubeContext)
	}
}

func TestLoadDesiredStateFromYaml_MultiPartTemplate_WithNonDefaultEnv(t *testing.T) {
	yamlFile := "/path/to/yaml/file.gotmpl"
	yamlContent := `bases:
- ../base.yaml
---
bases:
- ../base.gotmpl
---
helmDefaults:
  kubeContext: {{ .Environment.Values.foo }}
---
releases:
- name: myrelease0
  chart: mychart0
---

{{ readFile "templates.yaml" }}

releases:
- name: myrelease1
  chart: mychart1
  labels:
    stage: pre
    foo: bar
- name: myrelease1
  chart: mychart2
  labels:
    stage: post
  <<: *default
`
	testFs := testhelper.NewTestFs(map[string]string{
		yamlFile: yamlContent,
		"/path/to/base.yaml": `environments:
  test:
    values:
    - environments/default/1.yaml
`,
		"/path/to/yaml/environments/default/1.yaml": `foo: FOO`,
		"/path/to/base.gotmpl": `environments:
  test:
    values:
    - environments/default/2.yaml

helmDefaults:
`,
		"/path/to/yaml/environments/default/2.yaml": `tillerNs: TILLER_NS`,
		"/path/to/yaml/templates.yaml": `templates:
  default: &default
    missingFileHandler: Warn
    values: ["` + "{{`" + `{{.Release.Name}}` + "`}}" + `/values.yaml"]
`,
	})
	app := &App{
		OverrideHelmBinary: DefaultHelmBinary,
		fs:                 testFs.ToFileSystem(),
		Env:                "test",
		Logger:             newAppTestLogger(),
	}
	app.remote = remote.NewRemote(app.Logger, testFs.Cwd, app.fs)

	expectNoCallsToHelm(app)

	st, err := app.loadDesiredStateFromYaml(yamlFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	firstRelease := st.Releases[0]
	if firstRelease.Name != "myrelease1" {
		t.Errorf("unexpected releases[1].name: expected=myrelease1, got=%s", firstRelease.Name)
	}
	secondRelease := st.Releases[1]
	if secondRelease.Name != "myrelease1" {
		t.Errorf("unexpected releases[2].name: expected=myrelease1, got=%s", secondRelease.Name)
	}
	if secondRelease.Values[0] != "{{`{{.Release.Name}}`}}/values.yaml" {
		t.Errorf("unexpected releases[2].missingFileHandler: expected={{`{{.Release.Name}}`}}/values.yaml, got=%s", firstRelease.Values[0])
	}
	if *secondRelease.MissingFileHandler != "Warn" {
		t.Errorf("unexpected releases[2].missingFileHandler: expected=Warn, got=%s", *firstRelease.MissingFileHandler)
	}

	if secondRelease.Values[0] != "{{`{{.Release.Name}}`}}/values.yaml" {
		t.Errorf("unexpected releases[2].missingFileHandler: expected={{`{{.Release.Name}}`}}/values.yaml, got=%s", firstRelease.Values[0])
	}

	if st.HelmDefaults.KubeContext != "FOO" {
		t.Errorf("unexpected helmDefaults.kubeContext: expected=FOO, got=%s", st.HelmDefaults.KubeContext)
	}
}

func TestLoadDesiredStateFromYaml_MultiPartTemplate_WithReverse(t *testing.T) {
	yamlFile := "/path/to/yaml/file.gotmpl"
	yamlContent := `
{{ readFile "templates.yaml" }}

releases:
- name: myrelease0
  chart: mychart0
- name: myrelease1
  chart: mychart1
  <<: *default
---

{{ readFile "templates.yaml" }}

releases:
- name: myrelease2
  chart: mychart2
- name: myrelease3
  chart: mychart3
  <<: *default
`
	testFs := testhelper.NewTestFs(map[string]string{
		yamlFile: yamlContent,
		"/path/to/yaml/templates.yaml": `templates:
  default: &default
    missingFileHandler: Warn
    values: ["` + "{{`" + `{{.Release.Name}}` + "`}}" + `/values.yaml"]
`,
	})
	app := &App{
		OverrideHelmBinary: DefaultHelmBinary,
		fs:                 testFs.ToFileSystem(),
		Env:                "default",
		Logger:             newAppTestLogger(),
	}
	app.remote = remote.NewRemote(app.Logger, testFs.Cwd, app.fs)

	expectNoCallsToHelm(app)

	st, err := app.loadDesiredStateFromYaml(yamlFile, LoadOpts{Reverse: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if st.Releases[0].Name != "myrelease3" {
		t.Errorf("unexpected releases[0].name: expected=myrelease3, got=%s", st.Releases[0].Name)
	}
	if st.Releases[1].Name != "myrelease2" {
		t.Errorf("unexpected releases[0].name: expected=myrelease2, got=%s", st.Releases[1].Name)
	}

	if len(st.Releases) != 2 {
		t.Errorf("unexpected number of releases: expected=2, got=%d", len(st.Releases))
	}
}

// See https://github.com/roboll/helmfile/issues/615
func TestLoadDesiredStateFromYaml_MultiPartTemplate_NoMergeArrayInEnvVal(t *testing.T) {
	statePath := "/path/to/helmfile.yaml.gotmpl"
	stateContent := `
environments:
  default:
    values:
    - foo: ["foo"]
---
environments:
  default:
    values:
    - foo: ["FOO"]
    - 1.yaml
---
environments:
  default:
    values:
    - 2.yaml
---
releases:
- name: {{ .Environment.Values.foo | quote }}
  chart: {{ .Environment.Values.bar | quote }}
`
	testFs := testhelper.NewTestFs(map[string]string{
		statePath:         stateContent,
		"/path/to/1.yaml": `bar: ["bar"]`,
		"/path/to/2.yaml": `bar: ["BAR"]`,
	})
	app := &App{
		OverrideHelmBinary: DefaultHelmBinary,
		fs:                 testFs.ToFileSystem(),
		Env:                "default",
		Logger:             newAppTestLogger(),
	}

	app.remote = remote.NewRemote(app.Logger, testFs.Cwd, app.fs)
	expectNoCallsToHelm(app)

	st, err := app.loadDesiredStateFromYaml(statePath, LoadOpts{Reverse: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if st.Releases[0].Name != "[FOO]" {
		t.Errorf("unexpected releases[0].name: expected=FOO, got=%s", st.Releases[0].Name)
	}
	if st.Releases[0].Chart != "[BAR]" {
		t.Errorf("unexpected releases[0].chart: expected=BAR, got=%s", st.Releases[0].Chart)
	}
}

// See https://github.com/roboll/helmfile/issues/623
func TestLoadDesiredStateFromYaml_MultiPartTemplate_MergeMapsVariousKeys(t *testing.T) {
	type testcase struct {
		overrideValues any
		expected       string
	}
	testcases := []testcase{
		{map[any]any{"foo": "FOO"}, `FOO`},
		{map[any]any{"foo": map[any]any{"foo": "FOO"}}, `map[foo:FOO]`},
		{map[any]any{"foo": map[string]any{"foo": "FOO"}}, `map[foo:FOO]`},
		{map[any]any{"foo": []any{"foo"}}, `[foo]`},
		{map[any]any{"foo": "FOO"}, `FOO`},
	}
	for i := range testcases {
		tc := testcases[i]
		statePath := "/path/to/helmfile.yaml.gotmpl"
		stateContent := `
environments:
  default:
    values:
    - 1.yaml
    - 2.yaml
---
releases:
- name: {{ .Environment.Values.foo | quote }}
  chart: {{ .Environment.Values.bar | quote }}
`
		testFs := testhelper.NewTestFs(map[string]string{
			statePath:         stateContent,
			"/path/to/1.yaml": `bar: ["bar"]`,
			"/path/to/2.yaml": `bar: ["BAR"]`,
		})
		app := &App{
			OverrideHelmBinary: DefaultHelmBinary,
			fs:                 testFs.ToFileSystem(),
			Env:                "default",
			Logger:             newAppTestLogger(),
		}
		app.remote = remote.NewRemote(app.Logger, testFs.Cwd, app.fs)

		opts := LoadOpts{
			CalleePath: statePath,
			Environment: state.SubhelmfileEnvironmentSpec{
				OverrideValues: []any{tc.overrideValues},
			},
			Reverse: true,
		}

		expectNoCallsToHelm(app)

		st, err := app.loadDesiredStateFromYaml(statePath, opts)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if st.Releases[0].Name != tc.expected {
			t.Errorf("unexpected releases[0].name: expected=%s, got=%s", tc.expected, st.Releases[0].Name)
		}
		if st.Releases[0].Chart != "[BAR]" {
			t.Errorf("unexpected releases[0].chart: expected=BAR, got=%s", st.Releases[0].Chart)
		}
	}
}

func TestLoadDesiredStateFromYaml_MultiPartTemplate_SprigDictFuncs(t *testing.T) {
	type testcase struct {
		state    string
		expr     string
		expected string
	}
	stateInline := `
environments:
  default:
    values:
    - foo: FOO
      bar: { "baz": "BAZ" }
---
releases:
- name: %s
  chart: stable/nginx
`
	stateExternal := `
environments:
  default:
    values:
    - 1.yaml
    - 2.yaml
---
releases:
- name: %s
  chart: stable/nginx
`
	testcases := []testcase{
		{stateInline, `{{ getOrNil "foo" .Environment.Values }}`, `FOO`},
		{stateInline, `{{ getOrNil "baz" (getOrNil "bar" .Environment.Values) }}`, `BAZ`},
		{stateInline, `{{ if hasKey .Environment.Values "foo" }}{{ .Environment.Values.foo }}{{ end }}`, `FOO`},
		{stateInline, `{{ if hasKey .Environment.Values "bar" }}{{ .Environment.Values.bar.baz }}{{ end }}`, `BAZ`},
		{stateInline, `{{ if (keys .Environment.Values | has "foo") }}{{ .Environment.Values.foo }}{{ end }}`, `FOO`},
		// See https://github.com/roboll/helmfile/issues/624
		// This fails when .Environment.Values.bar is not map[string]any. At the time of #624 it was map[any]any, which sprig's dict funcs don't support.
		{stateInline, `{{ if (keys .Environment.Values | has "bar") }}{{ if (keys .Environment.Values.bar | has "baz") }}{{ .Environment.Values.bar.baz }}{{ end }}{{ end }}`, `BAZ`},
		{stateExternal, `{{ getOrNil "foo" .Environment.Values }}`, `FOO`},
		{stateExternal, `{{ getOrNil "baz" (getOrNil "bar" .Environment.Values) }}`, `BAZ`},
		{stateExternal, `{{ if hasKey .Environment.Values "foo" }}{{ .Environment.Values.foo }}{{ end }}`, `FOO`},
		{stateExternal, `{{ if hasKey .Environment.Values "bar" }}{{ .Environment.Values.bar.baz }}{{ end }}`, `BAZ`},
		{stateExternal, `{{ if (keys .Environment.Values | has "foo") }}{{ .Environment.Values.foo }}{{ end }}`, `FOO`},
		// See https://github.com/roboll/helmfile/issues/624
		{stateExternal, `{{ if (keys .Environment.Values | has "bar") }}{{ if (keys .Environment.Values.bar | has "baz") }}{{ .Environment.Values.bar.baz }}{{ end }}{{ end }}`, `BAZ`},
		// See https://github.com/roboll/helmfile/issues/643
		{stateExternal, `{{ range $service := .Environment.Values.services }}{{ $service.name }}{{ if hasKey $service "something" }}{{ $service.something }}{{ end }}{{ end }}`, `xyfalse`},
		// Same test with .Values
		{stateInline, `{{ getOrNil "foo" .Values }}`, `FOO`},
		{stateInline, `{{ getOrNil "baz" (getOrNil "bar" .Values) }}`, `BAZ`},
		{stateInline, `{{ if hasKey .Values "foo" }}{{ .Values.foo }}{{ end }}`, `FOO`},
		{stateInline, `{{ if hasKey .Values "bar" }}{{ .Values.bar.baz }}{{ end }}`, `BAZ`},
		{stateInline, `{{ if (keys .Values | has "foo") }}{{ .Values.foo }}{{ end }}`, `FOO`},
		// See https://github.com/roboll/helmfile/issues/624
		// This fails when .Values.bar is not map[string]any. At the time of #624 it was map[any]any, which sprig's dict funcs don't support.
		{stateInline, `{{ if (keys .Values | has "bar") }}{{ if (keys .Values.bar | has "baz") }}{{ .Values.bar.baz }}{{ end }}{{ end }}`, `BAZ`},
		{stateExternal, `{{ getOrNil "foo" .Values }}`, `FOO`},
		{stateExternal, `{{ getOrNil "baz" (getOrNil "bar" .Values) }}`, `BAZ`},
		{stateExternal, `{{ if hasKey .Values "foo" }}{{ .Values.foo }}{{ end }}`, `FOO`},
		{stateExternal, `{{ if hasKey .Values "bar" }}{{ .Values.bar.baz }}{{ end }}`, `BAZ`},
		{stateExternal, `{{ if (keys .Values | has "foo") }}{{ .Values.foo }}{{ end }}`, `FOO`},
		// See https://github.com/roboll/helmfile/issues/624
		{stateExternal, `{{ if (keys .Values | has "bar") }}{{ if (keys .Values.bar | has "baz") }}{{ .Values.bar.baz }}{{ end }}{{ end }}`, `BAZ`},
		// See https://github.com/roboll/helmfile/issues/643
		{stateExternal, `{{ range $service := .Values.services }}{{ $service.name }}{{ if hasKey $service "something" }}{{ $service.something }}{{ end }}{{ end }}`, `xyfalse`},
	}
	for i := range testcases {
		tc := testcases[i]
		statePath := "/path/to/helmfile.yaml.gotmpl"
		stateContent := fmt.Sprintf(tc.state, tc.expr)
		testFs := testhelper.NewTestFs(map[string]string{
			statePath:         stateContent,
			"/path/to/1.yaml": `foo: FOO`,
			"/path/to/2.yaml": `bar: { "baz": "BAZ" }
services:
  - name: "x"
  - name: "y"
    something: false
`,
		})
		app := &App{
			OverrideHelmBinary: DefaultHelmBinary,
			fs:                 testFs.ToFileSystem(),
			Env:                "default",
			Logger:             newAppTestLogger(),
		}
		app.remote = remote.NewRemote(app.Logger, testFs.Cwd, app.fs)

		expectNoCallsToHelm(app)

		st, err := app.loadDesiredStateFromYaml(statePath, LoadOpts{Reverse: true})

		if err != nil {
			t.Fatalf("unexpected error at %d: %v", i, err)
		}

		if st.Releases[0].Name != tc.expected {
			t.Errorf("unexpected releases[0].name at %d: expected=%s, got=%s", i, tc.expected, st.Releases[0].Name)
		}
	}
}

type configImpl struct {
	selectors            []string
	set                  []string
	output               string
	noHooks              bool
	includeCRDs          bool
	skipCleanup          bool
	skipCRDs             bool
	skipDeps             bool
	skipTests            bool
	skipSchemaValidation bool
	skipRefresh          bool

	skipNeeds              bool
	includeNeeds           bool
	includeTransitiveNeeds bool
	skipCharts             bool
	kubeVersion            string
}

func (c configImpl) Selectors() []string {
	return c.selectors
}

func (c configImpl) Set() []string {
	return c.set
}

func (c configImpl) Values() []string {
	return []string{}
}

func (c configImpl) Args() string {
	return "some args"
}

func (c configImpl) Validate() bool {
	return true
}

func (c configImpl) SkipCleanup() bool {
	return c.skipCleanup
}

func (c configImpl) SkipCRDs() bool {
	return c.skipCRDs
}

func (c configImpl) SkipDeps() bool {
	return c.skipDeps
}

func (c configImpl) SkipRefresh() bool {
	return c.skipRefresh
}

func (c configImpl) SkipNeeds() bool {
	return c.skipNeeds
}

func (c configImpl) SkipTests() bool {
	return c.skipTests
}

func (c configImpl) IncludeNeeds() bool {
	return c.includeNeeds || c.IncludeTransitiveNeeds()
}

func (c configImpl) IncludeTransitiveNeeds() bool {
	return c.includeTransitiveNeeds
}

func (c configImpl) OutputDir() string {
	return "output/subdir"
}

func (c configImpl) OutputDirTemplate() string {
	return ""
}

func (c configImpl) IncludeCRDs() bool {
	return c.includeCRDs
}

func (c configImpl) NoHooks() bool {
	return c.noHooks
}

func (c configImpl) Concurrency() int {
	return 1
}

func (c configImpl) EmbedValues() bool {
	return false
}

func (c configImpl) Output() string {
	return c.output
}

func (c configImpl) SkipCharts() bool {
	return c.skipCharts
}

func (c configImpl) PostRenderer() string {
	return ""
}

func (c configImpl) PostRendererArgs() []string {
	return nil
}

func (c configImpl) KubeVersion() string {
	return c.kubeVersion
}

func (c configImpl) SkipSchemaValidation() bool {
	return c.skipSchemaValidation
}

func (c configImpl) ShowOnly() []string {
	return nil
}

type applyConfig struct {
	args    string
	cascade string
	values  []string

	set                     []string
	validate                bool
	skipCleanup             bool
	skipCRDs                bool
	skipDeps                bool
	skipRefresh             bool
	skipNeeds               bool
	includeNeeds            bool
	includeTransitiveNeeds  bool
	includeTests            bool
	suppress                []string
	suppressSecrets         bool
	showSecrets             bool
	noHooks                 bool
	suppressDiff            bool
	noColor                 bool
	color                   bool
	context                 int
	diffOutput              string
	concurrency             int
	detailedExitcode        bool
	stripTrailingCR         bool
	interactive             bool
	skipDiffOnInstall       bool
	syncArgs                string
	diffArgs                string
	logger                  *zap.SugaredLogger
	wait                    bool
	waitRetries             int
	waitForJobs             bool
	reuseValues             bool
	postRenderer            string
	postRendererArgs        []string
	skipSchemaValidation    bool
	kubeVersion             string
	suppressOutputLineRegex []string
	showOnly                []string
	hideNotes               bool
	takeOwnership           bool
	syncReleaseLabels       bool

	// template-only options
	includeCRDs, skipTests       bool
	outputDir, outputDirTemplate string
}

func (a applyConfig) Args() string {
	return a.args
}

func (a applyConfig) Cascade() string {
	return a.cascade
}

func (a applyConfig) Wait() bool {
	return a.wait
}

func (a applyConfig) WaitRetries() int {
	return a.waitRetries
}

func (a applyConfig) WaitForJobs() bool {
	return a.waitForJobs
}

func (a applyConfig) Values() []string {
	return a.values
}

func (a applyConfig) Set() []string {
	return a.set
}

func (a applyConfig) Validate() bool {
	return a.validate
}

func (a applyConfig) SkipCleanup() bool {
	return a.skipCleanup
}

func (a applyConfig) SkipCRDs() bool {
	return a.skipCRDs
}

func (a applyConfig) SkipDeps() bool {
	return a.skipDeps
}

func (a applyConfig) SkipRefresh() bool {
	return a.skipRefresh
}

func (a applyConfig) SkipNeeds() bool {
	return a.skipNeeds
}

func (a applyConfig) IncludeNeeds() bool {
	return a.includeNeeds || a.IncludeTransitiveNeeds()
}

func (a applyConfig) IncludeTransitiveNeeds() bool {
	return a.includeTransitiveNeeds
}

func (a applyConfig) IncludeTests() bool {
	return a.includeTests
}

func (a applyConfig) Suppress() []string {
	return a.suppress
}

func (a applyConfig) SuppressSecrets() bool {
	return a.suppressSecrets
}

func (a applyConfig) ShowSecrets() bool {
	return a.showSecrets
}

func (a applyConfig) NoHooks() bool {
	return a.noHooks
}

func (a applyConfig) SuppressDiff() bool {
	return a.suppressDiff
}

func (a applyConfig) Color() bool {
	return a.color
}

func (a applyConfig) NoColor() bool {
	return a.noColor
}

func (a applyConfig) Context() int {
	return a.context
}

func (a applyConfig) DiffOutput() string {
	return a.diffOutput
}

func (a applyConfig) Concurrency() int {
	return a.concurrency
}

func (a applyConfig) DetailedExitcode() bool {
	return a.detailedExitcode
}

func (a applyConfig) StripTrailingCR() bool {
	return a.stripTrailingCR
}

func (a applyConfig) Interactive() bool {
	return a.interactive
}

func (a applyConfig) Logger() *zap.SugaredLogger {
	return a.logger
}

func (a applyConfig) SkipDiffOnInstall() bool {
	return a.skipDiffOnInstall
}

func (a applyConfig) SyncArgs() string {
	return a.syncArgs
}

func (a applyConfig) DiffArgs() string {
	return a.diffArgs
}

// helmfile-template-only flags
func (a applyConfig) IncludeCRDs() bool {
	return a.includeCRDs
}

func (a applyConfig) SkipTests() bool {
	return a.skipTests
}

func (a applyConfig) OutputDir() string {
	return a.outputDir
}
func (a applyConfig) OutputDirTemplate() string {
	return a.outputDirTemplate
}

func (a applyConfig) ReuseValues() bool {
	return a.reuseValues
}

func (a applyConfig) ResetValues() bool {
	return !a.reuseValues
}

func (a applyConfig) PostRenderer() string {
	return a.postRenderer
}

func (a applyConfig) PostRendererArgs() []string {
	return a.postRendererArgs
}

func (a applyConfig) SuppressOutputLineRegex() []string {
	return a.suppressOutputLineRegex
}

func (a applyConfig) KubeVersion() string {
	return a.kubeVersion
}

func (a applyConfig) SkipSchemaValidation() bool {
	return a.skipSchemaValidation
}

func (a applyConfig) ShowOnly() []string {
	return a.showOnly
}

func (a applyConfig) HideNotes() bool {
	return a.hideNotes
}

func (a applyConfig) TakeOwnership() bool {
	return a.takeOwnership
}

func (a applyConfig) SyncReleaseLabels() bool {
	return a.syncReleaseLabels
}

type depsConfig struct {
	skipRepos              bool
	includeTransitiveNeeds bool
}

func (d depsConfig) SkipRepos() bool {
	return d.skipRepos
}

func (d depsConfig) IncludeTransitiveNeeds() bool {
	return d.includeTransitiveNeeds
}

func (d depsConfig) Args() string {
	return ""
}

func (d depsConfig) Concurrency() int {
	return 2
}

// Mocking the command-line runner

type mockRunner struct {
}

func (mock *mockRunner) ExecuteStdIn(cmd string, args []string, env map[string]string, stdin io.Reader) ([]byte, error) {
	return []byte{}, nil
}

func (mock *mockRunner) Execute(cmd string, args []string, env map[string]string, enableLiveOutput bool) ([]byte, error) {
	return []byte{}, nil
}

func MockExecer(logger *zap.SugaredLogger, kubeContext string) helmexec.Interface {
	execer := helmexec.New("helm", helmexec.HelmExecOptions{}, logger, "", kubeContext, &mockRunner{})
	return execer
}

// mocking helmexec.Interface

type mockHelmExec struct {
	templated []mockTemplates
	repos     []mockRepo
}

type mockTemplates struct {
	name, chart string
	flags       []string
}

type mockRepo struct {
	Name string
}

func (helm *mockHelmExec) TemplateRelease(name, chart string, flags ...string) error {
	helm.templated = append(helm.templated, mockTemplates{name: name, chart: chart, flags: flags})
	return nil
}

func (helm *mockHelmExec) ChartPull(chart string, path string, flags ...string) error {
	return nil
}

func (helm *mockHelmExec) ChartExport(chart string, path string) error {
	return nil
}

func (helm *mockHelmExec) UpdateDeps(chart string) error {
	return nil
}

func (helm *mockHelmExec) BuildDeps(name, chart string, flags ...string) error {
	return nil
}

func (helm *mockHelmExec) SetExtraArgs(args ...string) {
}

func (helm *mockHelmExec) SetHelmBinary(bin string) {
}

func (helm *mockHelmExec) SetEnableLiveOutput(enableLiveOutput bool) {
}

func (helm *mockHelmExec) SetDisableForceUpdate(forceUpdate bool) {
}

func (helm *mockHelmExec) AddRepo(name, repository, cafile, certfile, keyfile, username, password string, managed string, passCredentials, skipTLSVerify bool) error {
	helm.repos = append(helm.repos, mockRepo{Name: name})
	return nil
}
func (helm *mockHelmExec) UpdateRepo() error {
	return nil
}
func (helm *mockHelmExec) RegistryLogin(name, username, password, caFile, certFile, keyFile string, skipTLSVerify bool) error {
	return nil
}
func (helm *mockHelmExec) SyncRelease(context helmexec.HelmContext, name, chart, namespace string, flags ...string) error {
	return nil
}
func (helm *mockHelmExec) DiffRelease(context helmexec.HelmContext, name, chart, namespace string, suppressDiff bool, flags ...string) error {
	return nil
}
func (helm *mockHelmExec) ReleaseStatus(context helmexec.HelmContext, release string, flags ...string) error {
	return nil
}
func (helm *mockHelmExec) DeleteRelease(context helmexec.HelmContext, name string, flags ...string) error {
	return nil
}

func (helm *mockHelmExec) List(context helmexec.HelmContext, filter string, flags ...string) (string, error) {
	return "", nil
}

func (helm *mockHelmExec) DecryptSecret(context helmexec.HelmContext, name string, flags ...string) (string, error) {
	return "", nil
}
func (helm *mockHelmExec) TestRelease(context helmexec.HelmContext, name string, flags ...string) error {
	return nil
}
func (helm *mockHelmExec) Fetch(chart string, flags ...string) error {
	return nil
}
func (helm *mockHelmExec) Lint(name, chart string, flags ...string) error {
	return nil
}
func (helm *mockHelmExec) IsHelm3() bool {
	return true
}

func (helm *mockHelmExec) GetVersion() helmexec.Version {
	return helmexec.Version{}
}

func (helm *mockHelmExec) IsVersionAtLeast(versionStr string) bool {
	return false
}

func (helm *mockHelmExec) ShowChart(chartPath string) (chart.Metadata, error) {
	return chart.Metadata{}, errors.New("tests logs rely on this error")
}

func TestTemplate_SingleStateFile(t *testing.T) {
	files := map[string]string{
		"/path/to/helmfile.yaml": `
repositories:
- name: stable
  url: https://kubernetes-charts.storage.googleapis.com
- name: stable2
  url: https://kubernetes-charts.storage.googleapis.com

releases:
- name: myrelease1
  chart: stable/mychart1
  labels:
    group: one
- name: myrelease2
  chart: stable/mychart2
  labels:
    group: one
- name: myrelease3
  chart: stable2/mychart3
`,
	}

	var helm = &mockHelmExec{}
	var wantReleases = []mockTemplates{
		{name: "myrelease1", chart: "stable/mychart1", flags: []string{"--namespace", "testNamespace", "--set", "foo=a", "--set", "bar=b", "--output-dir", "output/subdir/helmfile-[a-z0-9]{8}-myrelease1"}},
		{name: "myrelease2", chart: "stable/mychart2", flags: []string{"--namespace", "testNamespace", "--set", "foo=a", "--set", "bar=b", "--output-dir", "output/subdir/helmfile-[a-z0-9]{8}-myrelease2"}},
	}

	var wantRepos = []mockRepo{
		{Name: "stable"},
		{Name: "stable2"},
	}

	var buffer bytes.Buffer
	logger := helmexec.NewLogger(&buffer, "debug")

	valsRuntime, err := vals.New(vals.Options{CacheSize: 32})
	if err != nil {
		t.Errorf("unexpected error creating vals runtime: %v", err)
	}

	app := appWithFs(&App{
		OverrideHelmBinary:  DefaultHelmBinary,
		fs:                  ffs.DefaultFileSystem(),
		OverrideKubeContext: "default",
		Env:                 "default",
		Logger:              logger,
		helms: map[helmKey]helmexec.Interface{
			createHelmKey("helm", "default"): helm,
		},
		Namespace:   "testNamespace",
		valsRuntime: valsRuntime,
		Selectors: []string{
			"group=one",
		},
	}, files)

	if err := app.Template(configImpl{set: []string{"foo=a", "bar=b"}, skipDeps: false}); err != nil {
		t.Fatalf("%v", err)
	}

	if diff := cmp.Diff(wantRepos, helm.repos); diff != "" {
		t.Errorf("unexpected add repo:\n%s", diff)
	}

	for i := range wantReleases {
		if wantReleases[i].name != helm.templated[i].name {
			t.Errorf("name = [%v], want %v", helm.templated[i].name, wantReleases[i].name)
		}
		if !strings.Contains(helm.templated[i].chart, wantReleases[i].chart) {
			t.Errorf("chart = [%v], want %v", helm.templated[i].chart, wantReleases[i].chart)
		}
		for j := range wantReleases[i].flags {
			if j == 7 {
				matched, _ := regexp.Match(wantReleases[i].flags[j], []byte(helm.templated[i].flags[j]))
				if !matched {
					t.Errorf("HelmState.TemplateReleases() = [%v], want %v", helm.templated[i].flags[j], wantReleases[i].flags[j])
				}
			} else if wantReleases[i].flags[j] != helm.templated[i].flags[j] {
				t.Errorf("HelmState.TemplateReleases() = [%v], want %v", helm.templated[i].flags[j], wantReleases[i].flags[j])
			}
		}
	}
}

func TestTemplate_ApiVersionsAndKubeVersion(t *testing.T) {
	files := map[string]string{
		"/path/to/helmfile.yaml": `
apiVersions:
- helmfile.test/v1
- helmfile.test/v2

kubeVersion: v1.21

releases:
- name: myrelease1
  chart: stable/mychart1
`,
	}

	var helm = &mockHelmExec{}
	var wantReleases = []mockTemplates{
		{name: "myrelease1", chart: "stable/mychart1", flags: []string{"--api-versions", "helmfile.test/v1", "--api-versions", "helmfile.test/v2", "--kube-version", "v1.21", "--namespace", "testNamespace", "--output-dir", "output/subdir/helmfile-[a-z0-9]{8}-myrelease1"}},
	}

	var buffer bytes.Buffer
	logger := helmexec.NewLogger(&buffer, "debug")

	valsRuntime, err := vals.New(vals.Options{CacheSize: 32})
	if err != nil {
		t.Errorf("unexpected error creating vals runtime: %v", err)
	}

	app := appWithFs(&App{
		OverrideHelmBinary:  DefaultHelmBinary,
		OverrideKubeContext: "default",
		Env:                 "default",
		Logger:              logger,
		helms: map[helmKey]helmexec.Interface{
			createHelmKey("helm", "default"): helm,
		},
		Namespace:   "testNamespace",
		valsRuntime: valsRuntime,
	}, files)

	fmt.Printf("CRAFTED APP WITH %p\n", app.fs.DirectoryExistsAt)

	if err := app.Template(configImpl{}); err != nil {
		t.Fatalf("%v", err)
	}

	for i := range wantReleases {
		if wantReleases[i].name != helm.templated[i].name {
			t.Errorf("name = [%v], want %v", helm.templated[i].name, wantReleases[i].name)
		}
		if !strings.Contains(helm.templated[i].chart, wantReleases[i].chart) {
			t.Errorf("chart = [%v], want %v", helm.templated[i].chart, wantReleases[i].chart)
		}
		for j := range wantReleases[i].flags {
			if j == 9 {
				matched, _ := regexp.Match(wantReleases[i].flags[j], []byte(helm.templated[i].flags[j]))
				if !matched {
					t.Errorf("HelmState.TemplateReleases() = [%v], want %v", helm.templated[i].flags[j], wantReleases[i].flags[j])
				}
			} else if wantReleases[i].flags[j] != helm.templated[i].flags[j] {
				t.Errorf("HelmState.TemplateReleases() = [%v], want %v", helm.templated[i].flags[j], wantReleases[i].flags[j])
			}
		}
	}
}

func TestApply(t *testing.T) {
	type fields struct {
		skipNeeds    bool
		includeNeeds bool
	}
	testcases := []struct {
		name              string
		loc               string
		fields            fields
		ns                string
		concurrency       int
		skipDiffOnInstall bool
		error             string
		files             map[string]string
		selectors         []string
		lists             map[exectest.ListKey]string
		diffs             map[exectest.DiffKey]error
		upgraded          []exectest.Release
		deleted           []exectest.Release
		envs              map[string]string
		log               string
	}{
		//
		// helm-status-check-to-release-existence
		//
		{
			name: "helm-status-check-to-release-existence",
			loc:  location(),
			files: map[string]string{
				"/path/to/helmfile.yaml": `
releases:
- name: bar
  chart: stable/mychart2
- name: foo_notFound
  chart: stable/mychart1
  installed: false
`,
			},
			diffs: map[exectest.DiffKey]error{
				{Name: "bar", Chart: "stable/mychart2", Flags: "--kube-context default --reset-values --detailed-exitcode"}:          nil,
				{Name: "foo_notFound", Chart: "stable/mychart1", Flags: "--kube-context default --reset-values --detailed-exitcode"}: nil,
			},
			lists:    map[exectest.ListKey]string{},
			upgraded: []exectest.Release{},
			deleted:  []exectest.Release{},
			envs:     map[string]string{"HELMFILE_USE_HELM_STATUS_TO_CHECK_RELEASE_EXISTENCE": "true"},
		},
		//
		// complex test cases for smoke testing
		//
		{
			name: "smoke",
			loc:  location(),
			files: map[string]string{
				"/path/to/helmfile.yaml": `
releases:
- name: database
  chart: charts/mysql
  needs:
  - logging
- name: frontend-v1
  chart: charts/frontend
  installed: false
  needs:
  - servicemesh
  - logging
  - backend-v1
- name: frontend-v2
  chart: charts/frontend
  needs:
  - servicemesh
  - logging
  - backend-v2
- name: frontend-v3
  chart: charts/frontend
  needs:
  - servicemesh
  - logging
  - backend-v2
- name: backend-v1
  chart: charts/backend
  installed: false
  needs:
  - servicemesh
  - logging
  - database
  - anotherbackend
- name: backend-v2
  chart: charts/backend
  needs:
  - servicemesh
  - logging
  - database
  - anotherbackend
- name: anotherbackend
  chart: charts/anotherbackend
  needs:
  - servicemesh
  - logging
  - database
- name: servicemesh
  chart: charts/istio
  needs:
  - logging
- name: logging
  chart: charts/fluent-bit
- name: front-proxy
  chart: stable/envoy
`,
			},
			diffs: map[exectest.DiffKey]error{
				// noop on frontend-v2
				{Name: "frontend-v2", Chart: "charts/frontend", Flags: "--kube-context default --reset-values --detailed-exitcode"}: nil,
				// install frontend-v3
				{Name: "frontend-v3", Chart: "charts/frontend", Flags: "--kube-context default --reset-values --detailed-exitcode"}: helmexec.ExitError{Code: 2},
				// upgrades
				{Name: "logging", Chart: "charts/fluent-bit", Flags: "--kube-context default --reset-values --detailed-exitcode"}:            helmexec.ExitError{Code: 2},
				{Name: "front-proxy", Chart: "stable/envoy", Flags: "--kube-context default --reset-values --detailed-exitcode"}:             helmexec.ExitError{Code: 2},
				{Name: "servicemesh", Chart: "charts/istio", Flags: "--kube-context default --reset-values --detailed-exitcode"}:             helmexec.ExitError{Code: 2},
				{Name: "database", Chart: "charts/mysql", Flags: "--kube-context default --reset-values --detailed-exitcode"}:                helmexec.ExitError{Code: 2},
				{Name: "backend-v2", Chart: "charts/backend", Flags: "--kube-context default --reset-values --detailed-exitcode"}:            helmexec.ExitError{Code: 2},
				{Name: "anotherbackend", Chart: "charts/anotherbackend", Flags: "--kube-context default --reset-values --detailed-exitcode"}: helmexec.ExitError{Code: 2},
			},
			lists: map[exectest.ListKey]string{
				// delete frontend-v1 and backend-v1
				{Filter: "^logging$", Flags: listFlags("", "default")}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
logging 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	fluent-bit-3.1.0	3.1.0      	default
`,
				{Filter: "^front-proxy$", Flags: listFlags("", "default")}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
front-proxy 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	envoy-3.1.0	3.1.0      	default
`,
				{Filter: "^database$", Flags: listFlags("", "default")}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
database 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	mysql-3.1.0	3.1.0      	default
`,
				{Filter: "^servicemesh$", Flags: listFlags("", "default")}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
servicemesh 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	istio-3.1.0	3.1.0      	default
`,
				{Filter: "^anotherbackend$", Flags: listFlags("", "default")}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
anotherbackend 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	anotherbackend-3.1.0	3.1.0      	default
`,
				{Filter: "^frontend-v1$", Flags: listFlags("", "default")}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
frontend-v1 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	frontend-3.1.0	3.1.0      	default
`,
				{Filter: "^frontend-v3$", Flags: listFlags("", "default")}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
frontend-v3 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	frontend-3.1.0	3.1.0      	default
`,
				{Filter: "^backend-v1$", Flags: listFlags("", "default")}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
backend-v1 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	backend-3.1.0	3.1.0      	default
`,
				{Filter: "^backend-v2$", Flags: listFlags("", "default")}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
backend-v2 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	backend-3.1.0	3.1.0      	default
`,
			},
			// Disable concurrency to avoid in-deterministic result
			concurrency: 1,
			upgraded: []exectest.Release{
				{Name: "logging", Flags: []string{}},
				{Name: "front-proxy", Flags: []string{}},
				{Name: "database", Flags: []string{}},
				{Name: "servicemesh", Flags: []string{}},
				{Name: "anotherbackend", Flags: []string{}},
				{Name: "backend-v2", Flags: []string{}},
				{Name: "frontend-v3", Flags: []string{}},
			},
			deleted: []exectest.Release{
				{Name: "frontend-v1", Flags: []string{}},
			},
		},
		//
		// noop: no changes
		//
		{
			name: "noop",
			loc:  location(),
			files: map[string]string{
				"/path/to/helmfile.yaml": `
releases:
- name: bar
  chart: stable/mychart2
- name: foo
  chart: stable/mychart1
  installed: false
  needs:
  - bar
`,
			},
			diffs: map[exectest.DiffKey]error{
				{Name: "bar", Chart: "stable/mychart2", Flags: "--kube-context default --reset-values --detailed-exitcode"}: nil,
			},
			lists: map[exectest.ListKey]string{
				{Filter: "^foo$", Flags: listFlags("", "default")}: ``,
				{Filter: "^bar$", Flags: listFlags("", "default")}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
bar 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	mychart2-3.1.0	3.1.0      	default
`,
			},
			upgraded: []exectest.Release{},
			deleted:  []exectest.Release{},
		},
		//
		// install
		//
		{
			name: "install",
			loc:  location(),
			files: map[string]string{
				"/path/to/helmfile.yaml": `
releases:
- name: baz
  chart: stable/mychart3
- name: foo
  chart: stable/mychart1
  needs:
  - bar
- name: bar
  chart: stable/mychart2
`,
			},
			diffs: map[exectest.DiffKey]error{
				{Name: "foo", Chart: "stable/mychart1", Flags: "--kube-context default --reset-values --detailed-exitcode"}: helmexec.ExitError{Code: 2},
				{Name: "bar", Chart: "stable/mychart2", Flags: "--kube-context default --reset-values --detailed-exitcode"}: helmexec.ExitError{Code: 2},
				{Name: "baz", Chart: "stable/mychart3", Flags: "--kube-context default --reset-values --detailed-exitcode"}: helmexec.ExitError{Code: 2},
			},
			lists: map[exectest.ListKey]string{},
			upgraded: []exectest.Release{
				{Name: "baz", Flags: []string{}},
				{Name: "bar", Flags: []string{}},
				{Name: "foo", Flags: []string{}},
			},
			deleted:     []exectest.Release{},
			concurrency: 1,
		},
		//
		// install with upgrade
		//
		{
			name: "install-with-upgrade-with-validation-control",
			loc:  location(),
			files: map[string]string{
				"/path/to/helmfile.yaml": `
releases:
- name: baz
  chart: stable/mychart3
  disableValidationOnInstall: true
- name: foo
  chart: stable/mychart1
  disableValidationOnInstall: true
  needs:
  - bar
- name: bar
  chart: stable/mychart2
  disableValidation: true
`,
			},
			diffs: map[exectest.DiffKey]error{
				{Name: "baz", Chart: "stable/mychart3", Flags: "--kube-context default --reset-values --detailed-exitcode"}:                      helmexec.ExitError{Code: 2},
				{Name: "foo", Chart: "stable/mychart1", Flags: "--disable-validation --kube-context default --reset-values --detailed-exitcode"}: helmexec.ExitError{Code: 2},
				{Name: "bar", Chart: "stable/mychart2", Flags: "--disable-validation --kube-context default --reset-values --detailed-exitcode"}: helmexec.ExitError{Code: 2},
			},
			lists: map[exectest.ListKey]string{
				{Filter: "^foo$", Flags: listFlags("", "default")}: ``,
				{Filter: "^bar$", Flags: listFlags("", "default")}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
bar 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	mychart2-3.1.0	3.1.0      	default
`,
				{Filter: "^baz$", Flags: listFlags("", "default")}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
baz 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	mychart3-3.1.0	3.1.0      	default
`,
			},
			upgraded: []exectest.Release{
				{Name: "baz", Flags: []string{"--kube-context", "default"}},
				{Name: "bar", Flags: []string{"--kube-context", "default"}},
				{Name: "foo", Flags: []string{"--kube-context", "default"}},
			},
			deleted:     []exectest.Release{},
			concurrency: 1,
		},
		//
		// install with upgrade and --skip-diff-on-install
		//
		{
			name:              "install-with-upgrade-with-skip-diff-on-install",
			loc:               location(),
			skipDiffOnInstall: true,
			files: map[string]string{
				"/path/to/helmfile.yaml": `
releases:
- name: baz
  chart: stable/mychart3
  disableValidationOnInstall: true
- name: foo
  chart: stable/mychart1
  disableValidationOnInstall: true
  needs:
  - bar
- name: bar
  chart: stable/mychart2
  disableValidation: true
`,
			},
			diffs: map[exectest.DiffKey]error{
				{Name: "baz", Chart: "stable/mychart3", Flags: "--kube-context default --reset-values --detailed-exitcode"}:                      helmexec.ExitError{Code: 2},
				{Name: "bar", Chart: "stable/mychart2", Flags: "--disable-validation --kube-context default --reset-values --detailed-exitcode"}: helmexec.ExitError{Code: 2},
			},
			lists: map[exectest.ListKey]string{
				{Filter: "^foo$", Flags: listFlags("", "default")}: ``,
				{Filter: "^bar$", Flags: listFlags("", "default")}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
bar 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	mychart2-3.1.0	3.1.0      	default
`,
				{Filter: "^baz$", Flags: listFlags("", "default")}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
baz 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	mychart3-3.1.0	3.1.0      	default
`,
			},
			upgraded: []exectest.Release{
				{Name: "baz", Flags: []string{"--kube-context", "default"}},
				{Name: "bar", Flags: []string{"--kube-context", "default"}},
				{Name: "foo", Flags: []string{"--kube-context", "default"}},
			},
			deleted:     []exectest.Release{},
			concurrency: 1,
		},
		//
		// upgrades
		//
		{
			name: "upgrade when foo needs bar",
			loc:  location(),
			files: map[string]string{
				"/path/to/helmfile.yaml": `
releases:
- name: bar
  chart: stable/mychart2
- name: foo
  chart: stable/mychart1
  needs:
  - bar
`,
			},
			diffs: map[exectest.DiffKey]error{
				{Name: "bar", Chart: "stable/mychart2", Flags: "--kube-context default --reset-values --detailed-exitcode"}: helmexec.ExitError{Code: 2},
				{Name: "foo", Chart: "stable/mychart1", Flags: "--kube-context default --reset-values --detailed-exitcode"}: helmexec.ExitError{Code: 2},
			},
			upgraded: []exectest.Release{
				{Name: "bar", Flags: []string{}},
				{Name: "foo", Flags: []string{}},
			},
		},
		{
			name: "upgrade when bar needs foo",
			loc:  location(),
			files: map[string]string{
				"/path/to/helmfile.yaml": `
releases:
- name: foo
  chart: stable/mychart1
- name: bar
  chart: stable/mychart2
  needs:
  - foo
`,
			},
			diffs: map[exectest.DiffKey]error{
				{Name: "bar", Chart: "stable/mychart2", Flags: "--kube-context default --reset-values --detailed-exitcode"}: helmexec.ExitError{Code: 2},
				{Name: "foo", Chart: "stable/mychart1", Flags: "--kube-context default --reset-values --detailed-exitcode"}: helmexec.ExitError{Code: 2},
			},
			upgraded: []exectest.Release{
				{Name: "foo", Flags: []string{}},
				{Name: "bar", Flags: []string{}},
			},
		},
		{
			name: "upgrade when foo needs bar, with ns override",
			loc:  location(),
			ns:   "testNamespace",
			files: map[string]string{
				"/path/to/helmfile.yaml": `
releases:
- name: bar
  chart: stable/mychart2
- name: foo
  chart: stable/mychart1
  needs:
  - bar
`,
			},
			diffs: map[exectest.DiffKey]error{
				{Name: "bar", Chart: "stable/mychart2", Flags: "--kube-context default --namespace testNamespace --reset-values --detailed-exitcode"}: helmexec.ExitError{Code: 2},
				{Name: "foo", Chart: "stable/mychart1", Flags: "--kube-context default --namespace testNamespace --reset-values --detailed-exitcode"}: helmexec.ExitError{Code: 2},
			},
			upgraded: []exectest.Release{
				{Name: "bar", Flags: []string{}},
				{Name: "foo", Flags: []string{}},
			},
		},
		{
			name: "upgrade when bar needs foo, with ns override",
			loc:  location(),
			ns:   "testNamespace",
			files: map[string]string{
				"/path/to/helmfile.yaml": `
releases:
- name: foo
  chart: stable/mychart1
- name: bar
  chart: stable/mychart2
  needs:
  - foo
`,
			},
			diffs: map[exectest.DiffKey]error{
				{Name: "bar", Chart: "stable/mychart2", Flags: "--kube-context default --namespace testNamespace --reset-values --detailed-exitcode"}: helmexec.ExitError{Code: 2},
				{Name: "foo", Chart: "stable/mychart1", Flags: "--kube-context default --namespace testNamespace --reset-values --detailed-exitcode"}: helmexec.ExitError{Code: 2},
			},
			upgraded: []exectest.Release{
				{Name: "foo", Flags: []string{}},
				{Name: "bar", Flags: []string{}},
			},
		},
		{
			name: "upgrade when ns1/foo needs ns2/bar",
			loc:  location(),
			files: map[string]string{
				"/path/to/helmfile.yaml": `
releases:
- name: foo
  chart: stable/mychart1
  namespace: ns1
  needs:
  - ns2/bar
- name: bar
  chart: stable/mychart2
  namespace: ns2
`,
			},
			diffs: map[exectest.DiffKey]error{
				{Name: "bar", Chart: "stable/mychart2", Flags: "--kube-context default --namespace ns2 --reset-values --detailed-exitcode"}: helmexec.ExitError{Code: 2},
				{Name: "foo", Chart: "stable/mychart1", Flags: "--kube-context default --namespace ns1 --reset-values --detailed-exitcode"}: helmexec.ExitError{Code: 2},
			},
			upgraded: []exectest.Release{
				{Name: "bar", Flags: []string{"--kube-context", "default", "--namespace", "ns2"}},
				{Name: "foo", Flags: []string{"--kube-context", "default", "--namespace", "ns1"}},
			},
		},
		{
			name: "upgrade when ns2/bar needs ns1/foo",
			loc:  location(),
			files: map[string]string{
				"/path/to/helmfile.yaml": `
releases:
- name: bar
  chart: stable/mychart2
  namespace: ns2
  needs:
  - ns1/foo
- name: foo
  chart: stable/mychart1
  namespace: ns1
`,
			},
			diffs: map[exectest.DiffKey]error{
				{Name: "bar", Chart: "stable/mychart2", Flags: "--kube-context default --namespace ns2 --reset-values --detailed-exitcode"}: helmexec.ExitError{Code: 2},
				{Name: "foo", Chart: "stable/mychart1", Flags: "--kube-context default --namespace ns1 --reset-values --detailed-exitcode"}: helmexec.ExitError{Code: 2},
			},
			upgraded: []exectest.Release{
				{Name: "foo", Flags: []string{"--kube-context", "default", "--namespace", "ns1"}},
				{Name: "bar", Flags: []string{"--kube-context", "default", "--namespace", "ns2"}},
			},
		},
		{
			name: "upgrade when ns1/foo needs ns1/bar and ns2/bar is disabled",
			loc:  location(),
			files: map[string]string{
				"/path/to/helmfile.yaml": `
releases:
- name: bar
  chart: stable/mychart2
  namespace: ns1
- name: bar
  chart: stable/mychart2
  namespace: ns2
  installed: false
- name: foo
  chart: stable/mychart1
  namespace: ns1
  needs:
  - ns1/bar
`,
			},
			diffs: map[exectest.DiffKey]error{
				{Name: "bar", Chart: "stable/mychart2", Flags: "--kube-context default --namespace ns1 --reset-values --detailed-exitcode"}: helmexec.ExitError{Code: 2},
				{Name: "foo", Chart: "stable/mychart1", Flags: "--kube-context default --namespace ns1 --reset-values --detailed-exitcode"}: helmexec.ExitError{Code: 2},
			},
			upgraded: []exectest.Release{
				{Name: "bar", Flags: []string{"--kube-context", "default", "--namespace", "ns1"}},
				{Name: "foo", Flags: []string{"--kube-context", "default", "--namespace", "ns1"}},
			},
		},
		//
		// deletes: deleting all releases in the correct order
		//
		{
			name: "delete foo and bar when foo needs bar",
			loc:  location(),
			files: map[string]string{
				"/path/to/helmfile.yaml": `
releases:
- name: bar
  chart: stable/mychart2
  installed: false
- name: foo
  chart: stable/mychart1
  installed: false
  needs:
  - bar
`,
			},
			diffs: map[exectest.DiffKey]error{
				{Name: "bar", Chart: "stable/mychart2", Flags: "--kube-context default --reset-values --detailed-exitcode"}: helmexec.ExitError{Code: 2},
				{Name: "foo", Chart: "stable/mychart1", Flags: "--kube-context default --reset-values --detailed-exitcode"}: helmexec.ExitError{Code: 2},
			},
			lists: map[exectest.ListKey]string{
				{Filter: "^foo$", Flags: listFlags("", "default")}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
foo 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	mychart1-3.1.0	3.1.0      	default
`,
				{Filter: "^bar$", Flags: listFlags("", "default")}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
bar 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	mychart2-3.1.0	3.1.0      	default
`,
			},
			deleted: []exectest.Release{
				{Name: "foo", Flags: []string{}},
				{Name: "bar", Flags: []string{}},
			},
		},
		{
			name: "delete foo and bar when bar needs foo",
			loc:  location(),
			files: map[string]string{
				"/path/to/helmfile.yaml": `
releases:
- name: bar
  chart: stable/mychart2
  installed: false
  needs:
  - foo
- name: foo
  chart: stable/mychart1
  installed: false
`,
			},
			diffs: map[exectest.DiffKey]error{
				{Name: "bar", Chart: "stable/mychart2", Flags: "--kube-context default --reset-values --detailed-exitcode"}: helmexec.ExitError{Code: 2},
				{Name: "foo", Chart: "stable/mychart1", Flags: "--kube-context default --reset-values --detailed-exitcode"}: helmexec.ExitError{Code: 2},
			},
			lists: map[exectest.ListKey]string{
				{Filter: "^foo$", Flags: listFlags("", "default")}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
foo 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	mychart1-3.1.0	3.1.0      	default
`,
				{Filter: "^bar$", Flags: listFlags("", "default")}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
bar 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	mychart2-3.1.0	3.1.0      	default
`,
			},
			deleted: []exectest.Release{
				{Name: "bar", Flags: []string{}},
				{Name: "foo", Flags: []string{}},
			},
		},
		//
		// upgrade and delete: upgrading one while deleting another
		//
		{
			name: "delete foo when foo needs bar",
			loc:  location(),
			files: map[string]string{
				"/path/to/helmfile.yaml": `
releases:
- name: bar
  chart: stable/mychart2
- name: foo
  chart: stable/mychart1
  installed: false
  needs:
  - bar
`,
			},
			diffs: map[exectest.DiffKey]error{
				{Name: "bar", Chart: "stable/mychart2", Flags: "--kube-context default --reset-values --detailed-exitcode"}: helmexec.ExitError{Code: 2},
				{Name: "foo", Chart: "stable/mychart1", Flags: "--kube-context default --reset-values --detailed-exitcode"}: helmexec.ExitError{Code: 2},
			},
			lists: map[exectest.ListKey]string{
				{Filter: "^foo$", Flags: listFlags("", "default")}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
foo 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	mychart1-3.1.0	3.1.0      	default
`,
				{Filter: "^bar$", Flags: listFlags("", "default")}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
bar 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	mychart2-3.1.0	3.1.0      	default
`,
			},
			upgraded: []exectest.Release{
				{Name: "bar", Flags: []string{}},
			},
			deleted: []exectest.Release{
				{Name: "foo", Flags: []string{}},
			},
		},
		{
			name: "delete bar when foo needs bar",
			loc:  location(),
			files: map[string]string{
				"/path/to/helmfile.yaml": `
releases:
- name: bar
  chart: stable/mychart2
  installed: false
- name: foo
  chart: stable/mychart1
  needs:
  - bar
`,
			},
			diffs: map[exectest.DiffKey]error{
				{Name: "bar", Chart: "stable/mychart2", Flags: "--kube-context default --reset-values --detailed-exitcode"}: helmexec.ExitError{Code: 2},
				{Name: "foo", Chart: "stable/mychart1", Flags: "--kube-context default --reset-values --detailed-exitcode"}: helmexec.ExitError{Code: 2},
			},
			lists: map[exectest.ListKey]string{
				{Filter: "^foo$", Flags: listFlags("", "default")}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
foo 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	mychart1-3.1.0	3.1.0      	default
`,
				{Filter: "^bar$", Flags: listFlags("", "default")}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
bar 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	mychart2-3.1.0	3.1.0      	default
`,
			},
			upgraded: []exectest.Release{
				{Name: "foo", Flags: []string{}},
			},
			deleted: []exectest.Release{
				{Name: "bar", Flags: []string{}},
			},
		},
		{
			name: "delete foo when bar needs foo",
			loc:  location(),
			files: map[string]string{
				"/path/to/helmfile.yaml": `
releases:
- name: foo
  chart: stable/mychart1
  installed: false
- name: bar
  chart: stable/mychart2
  needs:
  - foo
`,
			},
			diffs: map[exectest.DiffKey]error{
				{Name: "bar", Chart: "stable/mychart2", Flags: "--kube-context default --reset-values --detailed-exitcode"}: helmexec.ExitError{Code: 2},
				{Name: "foo", Chart: "stable/mychart1", Flags: "--kube-context default --reset-values --detailed-exitcode"}: helmexec.ExitError{Code: 2},
			},
			lists: map[exectest.ListKey]string{
				{Filter: "^foo$", Flags: listFlags("", "default")}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
foo 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	mychart1-3.1.0	3.1.0      	default
`,
				{Filter: "^bar$", Flags: listFlags("", "default")}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
bar 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	mychart2-3.1.0	3.1.0      	default
`,
			},
			upgraded: []exectest.Release{
				{Name: "bar", Flags: []string{}},
			},
			deleted: []exectest.Release{
				{Name: "foo", Flags: []string{}},
			},
		},
		{
			name: "delete bar when bar needs foo",
			loc:  location(),
			files: map[string]string{
				"/path/to/helmfile.yaml": `
releases:
- name: foo
  chart: stable/mychart1
- name: bar
  chart: stable/mychart2
  installed: false
  needs:
  - foo
`,
			},
			diffs: map[exectest.DiffKey]error{
				{Name: "bar", Chart: "stable/mychart2", Flags: "--kube-context default --reset-values --detailed-exitcode"}: helmexec.ExitError{Code: 2},
				{Name: "foo", Chart: "stable/mychart1", Flags: "--kube-context default --reset-values --detailed-exitcode"}: helmexec.ExitError{Code: 2},
			},
			lists: map[exectest.ListKey]string{
				{Filter: "^foo$", Flags: listFlags("", "default")}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
foo 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	mychart1-3.1.0	3.1.0      	default
`,
				{Filter: "^bar$", Flags: listFlags("", "default")}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
bar 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	mychart2-3.1.0	3.1.0      	default
`,
			},
			upgraded: []exectest.Release{
				{Name: "foo", Flags: []string{}},
			},
			deleted: []exectest.Release{
				{Name: "bar", Flags: []string{}},
			},
		},
		//
		// upgrades with selector
		//
		{
			// see https://github.com/roboll/helmfile/issues/919#issuecomment-549831747
			name: "upgrades with good selector with --skip-needs=true",
			loc:  location(),
			fields: fields{
				skipNeeds: true,
			},
			files: map[string]string{
				"/path/to/helmfile.yaml.gotmpl": `
{{ $mark := "a" }}

releases:
- name: kubernetes-external-secrets
  chart: incubator/raw
  namespace: kube-system

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
`,
			},
			selectors: []string{"app=test"},
			diffs: map[exectest.DiffKey]error{
				{Name: "external-secrets", Chart: "incubator/raw", Flags: "--kube-context default --namespace default --reset-values --detailed-exitcode"}: helmexec.ExitError{Code: 2},
				{Name: "my-release", Chart: "incubator/raw", Flags: "--kube-context default --namespace default --reset-values --detailed-exitcode"}:       helmexec.ExitError{Code: 2},
			},
			upgraded: []exectest.Release{
				{Name: "external-secrets", Flags: []string{"--kube-context", "default", "--namespace", "default"}},
				{Name: "my-release", Flags: []string{"--kube-context", "default", "--namespace", "default"}},
			},
			lists: map[exectest.ListKey]string{
				{Filter: "^external-secrets$", Flags: listFlags("default", "default")}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
external-secrets 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	raw-3.1.0	3.1.0      	default
`,
				{Filter: "^my-release$", Flags: listFlags("default", "default")}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
my-release 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	raw-3.1.0	3.1.0      	default
`,
			},
			// as we check for log output, set concurrency to 1 to avoid non-deterministic test result
			concurrency: 1,
		},
		{
			// see https://github.com/roboll/helmfile/issues/919#issuecomment-549831747
			name: "upgrades with good selector with --skip-needs=false --include-needs=true",
			loc:  location(),
			fields: fields{
				skipNeeds:    false,
				includeNeeds: true,
			},
			error: `in ./helmfile.yaml.gotmpl: release "default/default/external-secrets" depends on "default/kube-system/kubernetes-external-secrets" which does not match the selectors. Please add a selector like "--selector name=kubernetes-external-secrets", or indicate whether to skip (--skip-needs) or include (--include-needs) these dependencies`,
			files: map[string]string{
				"/path/to/helmfile.yaml.gotmpl": `
{{ $mark := "a" }}

releases:
- name: kubernetes-external-secrets
  chart: incubator/raw
  namespace: kube-system

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
`,
			},
			selectors: []string{"app=test"},
			diffs: map[exectest.DiffKey]error{
				{Name: "external-secrets", Chart: "incubator/raw", Flags: "--kube-context default --namespace default --reset-values --detailed-exitcode"}: helmexec.ExitError{Code: 2},
				{Name: "my-release", Chart: "incubator/raw", Flags: "--kube-context default --namespace default --reset-values --detailed-exitcode"}:       helmexec.ExitError{Code: 2},
			},
			upgraded: []exectest.Release{},
			// as we check for log output, set concurrency to 1 to avoid non-deterministic test result
			concurrency: 1,
		},
		{
			// see https://github.com/roboll/helmfile/issues/919#issuecomment-549831747
			name: "upgrades with bad selector",
			loc:  location(),
			files: map[string]string{
				"/path/to/helmfile.yaml.gotmpl": `
{{ $mark := "a" }}

releases:
- name: kubernetes-external-secrets
  chart: incubator/raw
  namespace: kube-system

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
`,
			},
			selectors: []string{"app=test_non_existent"},
			diffs:     map[exectest.DiffKey]error{},
			upgraded:  []exectest.Release{},
			error:     "err: no releases found that matches specified selector(app=test_non_existent) and environment(default), in any helmfile",
			// as we check for log output, set concurrency to 1 to avoid non-deterministic test result
			concurrency: 1,
		},
		//
		// error cases
		//
		{
			name:      "unselected release in needs",
			loc:       location(),
			selectors: []string{"name=foo"},
			files: map[string]string{
				"/path/to/helmfile.yaml": `
releases:
- name: bar
  namespace: ns1
  chart: mychart3
- name: foo
  chart: mychart1
  needs:
  - ns1/bar
`,
			},
			diffs: map[exectest.DiffKey]error{
				{Name: "baz", Chart: "mychart3", Flags: "--kube-context default --namespace ns1 --reset-values --detailed-exitcode"}: helmexec.ExitError{Code: 2},
				{Name: "foo", Chart: "mychart1", Flags: "--kube-context default --reset-values --detailed-exitcode"}:                 helmexec.ExitError{Code: 2},
			},
			lists:       map[exectest.ListKey]string{},
			upgraded:    []exectest.Release{},
			deleted:     []exectest.Release{},
			concurrency: 1,
			error:       `in ./helmfile.yaml: release "default//foo" depends on "default/ns1/bar" which does not match the selectors. Please add a selector like "--selector name=bar", or indicate whether to skip (--skip-needs) or include (--include-needs) these dependencies`,
		},
		{
			name: "non-existent release in needs",
			loc:  location(),
			files: map[string]string{
				"/path/to/helmfile.yaml": `
releases:
- name: baz
  namespace: ns1
  chart: mychart3
- name: foo
  chart: mychart1
  needs:
  - ns1/bar
`,
			},
			diffs: map[exectest.DiffKey]error{
				{Name: "bar", Chart: "mychart3", Flags: "--kube-context default --namespace ns1 --reset-values --detailed-exitcode"}: helmexec.ExitError{Code: 2},
				{Name: "foo", Chart: "mychart1", Flags: "--kube-context default --reset-values --detailed-exitcode"}:                 helmexec.ExitError{Code: 2},
			},
			lists:       map[exectest.ListKey]string{},
			upgraded:    []exectest.Release{},
			deleted:     []exectest.Release{},
			concurrency: 1,
			error:       "in ./helmfile.yaml: release(s) \"default//foo\" depend(s) on an undefined release \"default/ns1/bar\". Perhaps you made a typo in \"needs\" or forgot defining a release named \"bar\" with appropriate \"namespace\" and \"kubeContext\"?",
		},
		{
			name: "duplicate releases",
			loc:  location(),
			files: map[string]string{
				"/path/to/helmfile.yaml": `
releases:
- name: bar
  namespace: ns1
  chart: mychart3
- name: foo
  chart: mychart2
  needs:
  - ns1/bar
- name: foo
  chart: mychart1
  needs:
  - ns1/bar
`,
			},
			diffs: map[exectest.DiffKey]error{
				{Name: "bar", Chart: "mychart3", Flags: "--kube-context default --namespace ns1 --reset-values --detailed-exitcode"}: helmexec.ExitError{Code: 2},
				{Name: "foo", Chart: "mychart1", Flags: "--kube-context default --reset-values --detailed-exitcode"}:                 helmexec.ExitError{Code: 2},
			},
			lists:       map[exectest.ListKey]string{},
			upgraded:    []exectest.Release{},
			deleted:     []exectest.Release{},
			concurrency: 1,
			error:       "in ./helmfile.yaml: found 2 duplicate releases with ID \"default//foo\"",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.envs != nil {
				for k, v := range tc.envs {
					os.Setenv(k, v)
					defer os.Unsetenv(k)
				}
			}
			wantUpgrades := tc.upgraded
			wantDeletes := tc.deleted

			var helm = &exectest.Helm{
				FailOnUnexpectedList: true,
				FailOnUnexpectedDiff: true,
				Lists:                tc.lists,
				Diffs:                tc.diffs,
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

				app := appWithFs(&App{
					OverrideHelmBinary:  DefaultHelmBinary,
					fs:                  ffs.DefaultFileSystem(),
					OverrideKubeContext: "default",
					Env:                 "default",
					Logger:              logger,
					helms: map[helmKey]helmexec.Interface{
						createHelmKey("helm", "default"): helm,
					},
					valsRuntime: valsRuntime,
				}, tc.files)

				if tc.ns != "" {
					app.Namespace = tc.ns
				}

				if tc.selectors != nil {
					app.Selectors = tc.selectors
				}

				applyErr := app.Apply(applyConfig{
					// if we check log output, concurrency must be 1. otherwise the test becomes non-deterministic.
					concurrency:       tc.concurrency,
					logger:            logger,
					skipDiffOnInstall: tc.skipDiffOnInstall,
					skipNeeds:         tc.fields.skipNeeds,
				})
				switch {
				case tc.error == "" && applyErr != nil:
					t.Fatalf("unexpected error for data defined at %s: %v", tc.loc, applyErr)
				case tc.error != "" && applyErr == nil:
					t.Fatalf("expected error did not occur for data defined at %s", tc.loc)
				case tc.error != "" && applyErr != nil && tc.error != applyErr.Error():
					t.Fatalf("invalid error: expected %q, got %q", tc.error, applyErr.Error())
				}

				if len(wantUpgrades) > len(helm.Releases) {
					t.Fatalf("insufficient number of upgrades: got %d, want %d", len(helm.Releases), len(wantUpgrades))
				}

				for relIdx := range wantUpgrades {
					if wantUpgrades[relIdx].Name != helm.Releases[relIdx].Name {
						t.Errorf("releases[%d].name: got %q, want %q", relIdx, helm.Releases[relIdx].Name, wantUpgrades[relIdx].Name)
					}
					for flagIdx := range wantUpgrades[relIdx].Flags {
						if wantUpgrades[relIdx].Flags[flagIdx] != helm.Releases[relIdx].Flags[flagIdx] {
							t.Errorf("releaes[%d].flags[%d]: got %v, want %v", relIdx, flagIdx, helm.Releases[relIdx].Flags[flagIdx], wantUpgrades[relIdx].Flags[flagIdx])
						}
					}
				}

				if len(wantDeletes) > len(helm.Deleted) {
					t.Fatalf("insufficient number of deletes: got %d, want %d", len(helm.Deleted), len(wantDeletes))
				}

				for relIdx := range wantDeletes {
					if wantDeletes[relIdx].Name != helm.Deleted[relIdx].Name {
						t.Errorf("releases[%d].name: got %q, want %q", relIdx, helm.Deleted[relIdx].Name, wantDeletes[relIdx].Name)
					}
					for flagIdx := range wantDeletes[relIdx].Flags {
						if wantDeletes[relIdx].Flags[flagIdx] != helm.Deleted[relIdx].Flags[flagIdx] {
							t.Errorf("releaes[%d].flags[%d]: got %v, want %v", relIdx, flagIdx, helm.Deleted[relIdx].Flags[flagIdx], wantDeletes[relIdx].Flags[flagIdx])
						}
					}
				}
			})

			if tc.log != "" {
				actual := bs.String()

				assert.Equal(t, tc.log, actual)
			} else {
				assertLogEqualsToSnapshot(t, bs.String())
			}
		})
	}
}

func TestDeps(t *testing.T) {
	testcases := []struct {
		name   string
		loc    string
		error  string
		files  map[string]string
		log    string
		charts []string
	}{
		//
		// complex test cases for smoke testing
		//
		{
			name: "smoke",
			loc:  location(),
			files: map[string]string{
				"/path/to/helmfile.yaml": `
repositories:
- name: bitnami
  url: https://charts.bitnami.com/bitnami/
releases:
- name: example
  chart: /path/to/charts/example
`,
				"/path/to/charts/example/Chart.yaml": `foo: FOO`,
			},
			charts: []string{"/path/to/charts/example"},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			var helm = &exectest.Helm{
				DiffMutex:     &sync.Mutex{},
				ChartsMutex:   &sync.Mutex{},
				ReleasesMutex: &sync.Mutex{},
				Helm3:         true,
			}

			bs := runWithLogCapture(t, "debug", func(t *testing.T, logger *zap.SugaredLogger) {
				t.Helper()

				app := appWithFs(&App{
					OverrideHelmBinary:  DefaultHelmBinary,
					fs:                  ffs.DefaultFileSystem(),
					OverrideKubeContext: "default",
					Env:                 "default",
					Logger:              logger,
					helms: map[helmKey]helmexec.Interface{
						createHelmKey("helm", "default"): helm,
					},
				}, tc.files)

				depsErr := app.Deps(depsConfig{
					skipRepos:              false,
					includeTransitiveNeeds: false,
				})
				switch {
				case tc.error == "" && depsErr != nil:
					t.Fatalf("unexpected error for data defined at %s: %v", tc.loc, depsErr)
				case tc.error != "" && depsErr == nil:
					t.Fatalf("expected error did not occur for data defined at %s", tc.loc)
				case tc.error != "" && depsErr != nil && tc.error != depsErr.Error():
					t.Fatalf("invalid error: expected %q, got %q", tc.error, depsErr.Error())
				}

				if !reflect.DeepEqual(helm.Charts, tc.charts) {
					t.Fatalf("expected charts %v, got %v", helm.Charts, tc.charts)
				}
			})

			if tc.log != "" {
				actual := bs.String()
				assert.Equal(t, tc.log, actual)
			} else {
				assertLogEqualsToSnapshot(t, bs.String())
			}
		})
	}
}

func TestPrint_SingleStateFile(t *testing.T) {
	files := map[string]string{
		"/path/to/helmfile.yaml": `
releases:
- name: myrelease1
  chart: mychart1
- name: myrelease2
  chart: mychart1
`,
	}
	stdout := os.Stdout
	defer func() { os.Stdout = stdout }()

	var buffer bytes.Buffer
	logger := helmexec.NewLogger(&buffer, "debug")

	app := appWithFs(&App{
		OverrideHelmBinary:  DefaultHelmBinary,
		fs:                  ffs.DefaultFileSystem(),
		OverrideKubeContext: "default",
		Env:                 "default",
		Logger:              logger,
		Namespace:           "testNamespace",
	}, files)

	expectNoCallsToHelm(app)

	out, err := testutil.CaptureStdout(func() {
		err := app.PrintState(configImpl{})
		assert.Nil(t, err)
	})

	assert.NoError(t, err)

	assert.True(t, strings.Count(out, "---") == 1,
		"state should contain '---' yaml doc separator:\n%s\n", out)
	assert.True(t, strings.Contains(out, "helmfile.yaml"),
		"state should contain source helmfile name:\n%s\n", out)
	assert.True(t, strings.Contains(out, "name: myrelease1"),
		"state should contain releases:\n%s\n", out)
}

func TestPrint_MultiStateFile(t *testing.T) {
	files := map[string]string{
		"/path/to/helmfile.d/first.yaml": `
releases:
- name: myrelease1
  chart: mychart1
- name: myrelease2
  chart: mychart1
`,
		"/path/to/helmfile.d/second.yaml": `
releases:
- name: myrelease3
  chart: mychart1
- name: myrelease4
  chart: mychart1
`,
	}
	stdout := os.Stdout
	defer func() { os.Stdout = stdout }()

	var buffer bytes.Buffer
	logger := helmexec.NewLogger(&buffer, "debug")

	app := appWithFs(&App{
		OverrideHelmBinary:  DefaultHelmBinary,
		fs:                  ffs.DefaultFileSystem(),
		OverrideKubeContext: "default",
		Env:                 "default",
		Logger:              logger,
		Namespace:           "testNamespace",
	}, files)

	expectNoCallsToHelm(app)

	out, err := testutil.CaptureStdout(func() {
		err := app.PrintState(configImpl{})
		assert.Nil(t, err)
	})
	assert.NoError(t, err)
	assert.True(t, strings.Count(out, "---") == 2,
		"state should contain '---' yaml doc separators:\n%s\n", out)
	assert.True(t, strings.Contains(out, "second.yaml"),
		"state should contain source helmfile name:\n%s\n", out)
	assert.True(t, strings.Contains(out, "second.yaml"),
		"state should contain source helmfile name:\n%s\n", out)
}

func TestList(t *testing.T) {
	files := map[string]string{
		"/path/to/helmfile.d/first.yaml": `
commonLabels:
  common: label
environments:
  default:
    values:
     - myrelease2:
         enabled: false
---
releases:
- name: myrelease1
  chart: mychart1
  installed: false
  labels:
    id: myrelease1
- name: myrelease2
  chart: mychart1
  condition: myrelease2.enabled
`,
		"/path/to/helmfile.d/second.yaml": `
releases:
- name: myrelease3
  chart: mychart1
  installed: true
- name: myrelease4
  chart: mychart1
  labels:
    id: myrelease1
`,
	}
	stdout := os.Stdout
	defer func() { os.Stdout = stdout }()

	var buffer bytes.Buffer
	logger := helmexec.NewLogger(&buffer, "debug")

	app := appWithFs(&App{
		OverrideHelmBinary:  DefaultHelmBinary,
		fs:                  ffs.DefaultFileSystem(),
		OverrideKubeContext: "default",
		Env:                 "default",
		Logger:              logger,
		Namespace:           "testNamespace",
	}, files)

	expectNoCallsToHelm(app)

	out, err := testutil.CaptureStdout(func() {
		err := app.ListReleases(configImpl{})
		assert.Nil(t, err)
	})
	assert.NoError(t, err)

	expected := `NAME      	NAMESPACE    	ENABLED	INSTALLED	LABELS                                                                           	CHART   	VERSION
myrelease1	testNamespace	true   	false    	chart:mychart1,common:label,id:myrelease1,name:myrelease1,namespace:testNamespace	mychart1	       
myrelease2	testNamespace	false  	true     	chart:mychart1,common:label,name:myrelease2,namespace:testNamespace              	mychart1	       
myrelease3	testNamespace	true   	true     	chart:mychart1,name:myrelease3,namespace:testNamespace                           	mychart1	       
myrelease4	testNamespace	true   	true     	chart:mychart1,id:myrelease1,name:myrelease4,namespace:testNamespace             	mychart1	       
`

	assert.Equal(t, expected, out)
}
func testSetStringValuesTemplate(t *testing.T, goccyGoYaml bool) {
	t.Helper()

	v := runtime.GoccyGoYaml
	runtime.GoccyGoYaml = goccyGoYaml
	t.Cleanup(func() {
		runtime.GoccyGoYaml = v
	})

	files := map[string]string{
		"/path/to/helmfile.yaml.gotmpl": `
releases:
- name: zipkin
  chart: stable/zipkin
  values:
  - val2: "val2"
  valuesTemplate:
  - val1: '{{"{{ .Release.Name }}"}}'
  setString:
  - name: "name"
    value: "val"
`,
	}
	expectedValues := []any{
		map[string]any{"val1": "zipkin"},
		map[string]any{"val2": "val2"}}
	expectedSetValues := []state.SetValue{
		{Name: "name", Value: "val"}}

	app := appWithFs(&App{
		OverrideHelmBinary:  DefaultHelmBinary,
		OverrideKubeContext: "default",
		Logger:              newAppTestLogger(),
		Env:                 "default",
		FileOrDir:           "helmfile.yaml.gotmpl",
	}, files)

	expectNoCallsToHelm(app)

	var specs []state.ReleaseSpec
	collectReleases := func(run *Run) (bool, []error) {
		specs = append(specs, run.state.Releases...)
		return false, nil
	}

	err := app.ForEachState(
		collectReleases,
		false,
		SetFilter(true),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(specs) != 1 {
		t.Fatalf("expected 1 release; got %d releases", len(specs))
	}
	actualValues := specs[0].Values
	actualSetStringValues := specs[0].SetStringValues

	if !reflect.DeepEqual(expectedValues, actualValues) {
		t.Errorf("expected values: %v; got values: %v", expectedValues, actualValues)
	}
	if !reflect.DeepEqual(expectedSetValues, actualSetStringValues) {
		t.Errorf("expected set-string: %v; got set: %v", expectedValues, actualValues)
	}
}

func testSetValuesTemplate(t *testing.T, goccyGoYaml bool) {
	t.Helper()

	v := runtime.GoccyGoYaml
	runtime.GoccyGoYaml = goccyGoYaml
	t.Cleanup(func() {
		runtime.GoccyGoYaml = v
	})

	files := map[string]string{
		"/path/to/helmfile.yaml.gotmpl": `
releases:
- name: zipkin
  chart: stable/zipkin
  values:
  - val2: "val2"
  valuesTemplate:
  - val1: '{{"{{ .Release.Name }}"}}'
  set:
  - name: "name"
    value: "val"
  setTemplate:
  - name: name-{{"{{ .Release.Name }}"}}
    value: val-{{"{{ .Release.Name }}"}}
`,
	}
	expectedValues := []any{
		map[string]any{"val1": "zipkin"},
		map[string]any{"val2": "val2"}}
	expectedSetValues := []state.SetValue{
		{Name: "name-zipkin", Value: "val-zipkin"},
		{Name: "name", Value: "val"}}

	app := appWithFs(&App{
		OverrideHelmBinary:  DefaultHelmBinary,
		OverrideKubeContext: "default",
		Logger:              newAppTestLogger(),
		Env:                 "default",
		FileOrDir:           "helmfile.yaml.gotmpl",
	}, files)

	expectNoCallsToHelm(app)

	var specs []state.ReleaseSpec
	collectReleases := func(run *Run) (bool, []error) {
		specs = append(specs, run.state.Releases...)
		return false, nil
	}

	err := app.ForEachState(
		collectReleases,
		false,
		SetFilter(true),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(specs) != 1 {
		t.Fatalf("expected 1 release; got %d releases", len(specs))
	}
	actualValues := specs[0].Values
	actualSetValues := specs[0].SetValues

	if !reflect.DeepEqual(expectedValues, actualValues) {
		t.Errorf("expected values: %v; got values: %v", expectedValues, actualValues)
	}
	if !reflect.DeepEqual(expectedSetValues, actualSetValues) {
		t.Errorf("expected set: %v; got set: %v", expectedValues, actualValues)
	}
}

func TestSetValuesTemplate(t *testing.T) {
	t.Run("with goccy/go-yaml", func(t *testing.T) {
		testSetValuesTemplate(t, true)
	})

	t.Run("with gopkg.in/yaml.v2", func(t *testing.T) {
		testSetValuesTemplate(t, false)
	})
}

func TestSetStringValuesTemplate(t *testing.T) {
	t.Run("with goccy/go-yaml", func(t *testing.T) {
		testSetStringValuesTemplate(t, true)
	})

	t.Run("with gopkg.in/yaml.v2", func(t *testing.T) {
		testSetStringValuesTemplate(t, false)
	})
}

func location() string {
	_, fn, line, _ := goruntime.Caller(1)
	return fmt.Sprintf("%s:%d", filepath.Base(fn), line)
}

func TestGetArgs(t *testing.T) {
	tests := []struct {
		args        string
		expected    string
		defaultArgs []string
	}{
		{
			args:        "-f a.yaml -f b.yaml -i --set app1.bootstrap=true --set app2.bootstrap=false",
			defaultArgs: []string{"--recreate-pods", "--force"},
			expected:    "-f a.yaml -f b.yaml -i --set app1.bootstrap=true --set app2.bootstrap=false --recreate-pods --force",
		},
		{
			args:        "-e  a.yaml   -d b.yaml   -i --set app1.bootstrap=true --set app2.bootstrap=false",
			defaultArgs: []string{"-q www", "-w"},
			expected:    "-e a.yaml -d b.yaml -i --set app1.bootstrap=true --set app2.bootstrap=false -q www -w",
		},
		{
			args:     "--timeout=3600 --set app1.bootstrap=true --set app2.bootstrap=false",
			expected: "--timeout=3600 --set app1.bootstrap=true --set app2.bootstrap=false",
		},
		{
			args:        "--timeout=3600 --set app1.bootstrap=true --set app2.bootstrap=false,app3.bootstrap=true",
			defaultArgs: []string{"--recreate-pods", "--force"},
			expected:    "--timeout=3600 --set app1.bootstrap=true --set app2.bootstrap=false,app3.bootstrap=true --recreate-pods --force",
		},
	}
	for _, test := range tests {
		Helmdefaults := state.HelmSpec{KubeContext: "test", Args: test.defaultArgs}
		testState := &state.HelmState{
			ReleaseSetSpec: state.ReleaseSetSpec{
				HelmDefaults: Helmdefaults,
			},
		}
		receivedArgs := GetArgs(test.args, testState)

		require.Equalf(t, test.expected, strings.Join(receivedArgs, " "), "expected args %s, received args %s", test.expected, strings.Join(receivedArgs, " "))
	}
}
