package app

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"

	"github.com/helmfile/vals"

	ffs "github.com/helmfile/helmfile/pkg/filesystem"
	"github.com/helmfile/helmfile/pkg/helmexec"
	"github.com/helmfile/helmfile/pkg/testhelper"
	"github.com/helmfile/helmfile/pkg/testutil"
)

// TestSequentialHelmfilesNoChdirCalled verifies that sequential processing
// does NOT call os.Chdir(), which was the root cause of issue #2409.
func TestSequentialHelmfilesNoChdirCalled(t *testing.T) {
	files := map[string]string{
		"/path/to/helmfile.d/01-first.yaml": `
releases:
- name: first-release
  chart: stable/chart-a
  namespace: default
`,
		"/path/to/helmfile.d/02-second.yaml": `
releases:
- name: second-release
  chart: stable/chart-b
  namespace: default
`,
	}

	testFs := testhelper.NewTestFs(files)

	valsRuntime, err := vals.New(vals.Options{CacheSize: 32})
	if err != nil {
		t.Fatalf("unexpected error creating vals runtime: %v", err)
	}

	app := &App{
		OverrideHelmBinary:              DefaultHelmBinary,
		OverrideKubeContext:             "default",
		DisableKubeVersionAutoDetection: true,
		Env:                             "default",
		Logger:                          newAppTestLogger(),
		valsRuntime:                     valsRuntime,
		FileOrDir:                       "/path/to/helmfile.d",
		SequentialHelmfiles:             true,
	}

	app = injectFs(app, testFs)
	expectNoCallsToHelm(app)

	err = app.ForEachState(
		Noop,
		false,
		SetFilter(true),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if testFs.ChdirCalls != 0 {
		t.Errorf("expected 0 Chdir calls in sequential mode, got %d", testFs.ChdirCalls)
	}
}

// TestSequentialHelmfilesProcessesAllFiles verifies all files in helmfile.d
// are processed when using sequential mode.
func TestSequentialHelmfilesProcessesAllFiles(t *testing.T) {
	files := map[string]string{
		"/path/to/helmfile.d/001-app.yaml": `
releases:
- name: app1
  chart: stable/app1
  namespace: default
`,
		"/path/to/helmfile.d/002-db.yaml": `
releases:
- name: db1
  chart: stable/postgresql
  namespace: default
`,
		"/path/to/helmfile.d/003-cache.yaml": `
releases:
- name: cache1
  chart: stable/redis
  namespace: default
`,
	}

	var buffer bytes.Buffer
	syncWriter := testhelper.NewSyncWriter(&buffer)
	logger := helmexec.NewLogger(syncWriter, "debug")

	valsRuntime, err := vals.New(vals.Options{CacheSize: 32})
	if err != nil {
		t.Fatalf("unexpected error creating vals runtime: %v", err)
	}

	app := appWithFs(&App{
		OverrideHelmBinary:              DefaultHelmBinary,
		fs:                              ffs.DefaultFileSystem(),
		OverrideKubeContext:             "default",
		DisableKubeVersionAutoDetection: true,
		Env:                             "default",
		Logger:                          logger,
		valsRuntime:                     valsRuntime,
		FileOrDir:                       "/path/to/helmfile.d",
		SequentialHelmfiles:             true,
	}, files)

	expectNoCallsToHelm(app)

	out, err := testutil.CaptureStdout(func() {
		err := app.ListReleases(configImpl{
			skipCharts: false,
			output:     "json",
		})
		if err != nil {
			t.Logf("ListReleases error: %v", err)
		}
	})

	if err != nil {
		t.Fatalf("unexpected error capturing output: %v", err)
	}

	if !bytes.Contains([]byte(out), []byte("app1")) {
		t.Errorf("app1 release not found in output:\n%s", out)
	}
	if !bytes.Contains([]byte(out), []byte("db1")) {
		t.Errorf("db1 release not found in output:\n%s", out)
	}
	if !bytes.Contains([]byte(out), []byte("cache1")) {
		t.Errorf("cache1 release not found in output:\n%s", out)
	}
}

// TestSequentialHelmfilesAlphabeticalOrder verifies sequential mode processes
// files in alphabetical order.
func TestSequentialHelmfilesAlphabeticalOrder(t *testing.T) {
	files := map[string]string{
		"/path/to/helmfile.d/z-last.yaml": `
releases:
- name: zulu-release
  chart: stable/chart-z
  namespace: ns-z
`,
		"/path/to/helmfile.d/a-first.yaml": `
releases:
- name: alpha-release
  chart: stable/chart-a
  namespace: ns-a
`,
		"/path/to/helmfile.d/m-middle.yaml": `
releases:
- name: mike-release
  chart: stable/chart-m
  namespace: ns-m
`,
	}

	valsRuntime, err := vals.New(vals.Options{CacheSize: 32})
	if err != nil {
		t.Fatalf("unexpected error creating vals runtime: %v", err)
	}

	app := appWithFs(&App{
		OverrideHelmBinary:              DefaultHelmBinary,
		fs:                              ffs.DefaultFileSystem(),
		OverrideKubeContext:             "default",
		DisableKubeVersionAutoDetection: true,
		Env:                             "default",
		Logger:                          newAppTestLogger(),
		valsRuntime:                     valsRuntime,
		FileOrDir:                       "/path/to/helmfile.d",
		SequentialHelmfiles:             true,
	}, files)

	expectNoCallsToHelm(app)

	var actualOrder []string
	noop := func(run *Run) (bool, []error) {
		actualOrder = append(actualOrder, run.state.FilePath)
		return false, []error{}
	}

	err = app.ForEachState(
		noop,
		false,
		SetFilter(true),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedOrder := []string{"/path/to/helmfile.d/a-first.yaml", "/path/to/helmfile.d/m-middle.yaml", "/path/to/helmfile.d/z-last.yaml"}
	if !reflect.DeepEqual(actualOrder, expectedOrder) {
		t.Errorf("unexpected order of processed state files: expected=%v, actual=%v", expectedOrder, actualOrder)
	}
}

// TestSequentialHelmfilesMatchesParallelResults verifies that sequential and
// parallel modes produce the same set of releases.
func TestSequentialHelmfilesMatchesParallelResults(t *testing.T) {
	files := map[string]string{
		"/path/to/helmfile.d/01-app.yaml": `
releases:
- name: app-release
  chart: stable/app
  namespace: default
`,
		"/path/to/helmfile.d/02-db.yaml": `
releases:
- name: db-release
  chart: stable/postgresql
  namespace: default
`,
	}

	valsRuntime, err := vals.New(vals.Options{CacheSize: 32})
	if err != nil {
		t.Fatalf("unexpected error creating vals runtime: %v", err)
	}

	// Run in parallel mode (default)
	parallelOut, err := testutil.CaptureStdout(func() {
		var buffer bytes.Buffer
		syncWriter := testhelper.NewSyncWriter(&buffer)
		logger := helmexec.NewLogger(syncWriter, "debug")

		app := appWithFs(&App{
			OverrideHelmBinary:              DefaultHelmBinary,
			fs:                              ffs.DefaultFileSystem(),
			OverrideKubeContext:             "default",
			DisableKubeVersionAutoDetection: true,
			Env:                             "default",
			Logger:                          logger,
			valsRuntime:                     valsRuntime,
			FileOrDir:                       "/path/to/helmfile.d",
			SequentialHelmfiles:             false,
		}, files)
		expectNoCallsToHelm(app)

		if err := app.ListReleases(configImpl{skipCharts: false, output: "json"}); err != nil {
			t.Logf("parallel ListReleases error: %v", err)
		}
	})
	if err != nil {
		t.Fatalf("unexpected error capturing parallel output: %v", err)
	}

	// Run in sequential mode
	sequentialOut, err := testutil.CaptureStdout(func() {
		var buffer bytes.Buffer
		syncWriter := testhelper.NewSyncWriter(&buffer)
		logger := helmexec.NewLogger(syncWriter, "debug")

		app := appWithFs(&App{
			OverrideHelmBinary:              DefaultHelmBinary,
			fs:                              ffs.DefaultFileSystem(),
			OverrideKubeContext:             "default",
			DisableKubeVersionAutoDetection: true,
			Env:                             "default",
			Logger:                          logger,
			valsRuntime:                     valsRuntime,
			FileOrDir:                       "/path/to/helmfile.d",
			SequentialHelmfiles:             true,
		}, files)
		expectNoCallsToHelm(app)

		if err := app.ListReleases(configImpl{skipCharts: false, output: "json"}); err != nil {
			t.Logf("sequential ListReleases error: %v", err)
		}
	})
	if err != nil {
		t.Fatalf("unexpected error capturing sequential output: %v", err)
	}

	// Both modes should contain the same releases
	for _, name := range []string{"app-release", "db-release"} {
		if !bytes.Contains([]byte(parallelOut), []byte(name)) {
			t.Errorf("parallel output missing release %q:\n%s", name, parallelOut)
		}
		if !bytes.Contains([]byte(sequentialOut), []byte(name)) {
			t.Errorf("sequential output missing release %q:\n%s", name, sequentialOut)
		}
	}
}

// TestSequentialHelmfilesWithUndefinedEnv verifies that files with undefined
// environments are skipped gracefully in sequential mode.
func TestSequentialHelmfilesWithUndefinedEnv(t *testing.T) {
	files := map[string]string{
		"/path/to/helmfile.d/01-has-prod.yaml": `
environments:
  prod: {}
---
releases:
- name: prod-release
  chart: stable/prod
  namespace: default
`,
		"/path/to/helmfile.d/02-no-prod.yaml": `
environments:
  staging: {}
---
releases:
- name: staging-release
  chart: stable/staging
  namespace: default
`,
	}

	valsRuntime, err := vals.New(vals.Options{CacheSize: 32})
	if err != nil {
		t.Fatalf("unexpected error creating vals runtime: %v", err)
	}

	app := appWithFs(&App{
		OverrideHelmBinary:              DefaultHelmBinary,
		fs:                              ffs.DefaultFileSystem(),
		OverrideKubeContext:             "default",
		DisableKubeVersionAutoDetection: true,
		Env:                             "prod",
		Logger:                          newAppTestLogger(),
		valsRuntime:                     valsRuntime,
		FileOrDir:                       "/path/to/helmfile.d",
		SequentialHelmfiles:             true,
	}, files)

	expectNoCallsToHelm(app)

	out, err := testutil.CaptureStdout(func() {
		err := app.ListReleases(configImpl{
			skipCharts: false,
			output:     "json",
		})
		if err != nil {
			t.Logf("ListReleases error: %v", err)
		}
	})

	if err != nil {
		t.Fatalf("unexpected error capturing output: %v", err)
	}

	// The prod-release should be present
	if !bytes.Contains([]byte(out), []byte("prod-release")) {
		t.Errorf("prod-release not found in output:\n%s", out)
	}

	// The staging-release should NOT be present (env "prod" not defined in that file)
	if bytes.Contains([]byte(out), []byte("staging-release")) {
		t.Errorf("staging-release should have been skipped but was found in output:\n%s", out)
	}
}

// TestSequentialHelmfilesConvergeErrorPropagated verifies that errors returned
// from the converge function are properly propagated in sequential mode.
func TestSequentialHelmfilesConvergeErrorPropagated(t *testing.T) {
	files := map[string]string{
		"/path/to/helmfile.d/01-first.yaml": `
releases:
- name: first-release
  chart: stable/chart-a
  namespace: default
`,
		"/path/to/helmfile.d/02-second.yaml": `
releases:
- name: second-release
  chart: stable/chart-b
  namespace: default
`,
	}

	valsRuntime, err := vals.New(vals.Options{CacheSize: 32})
	if err != nil {
		t.Fatalf("unexpected error creating vals runtime: %v", err)
	}

	app := appWithFs(&App{
		OverrideHelmBinary:              DefaultHelmBinary,
		fs:                              ffs.DefaultFileSystem(),
		OverrideKubeContext:             "default",
		DisableKubeVersionAutoDetection: true,
		Env:                             "default",
		Logger:                          newAppTestLogger(),
		valsRuntime:                     valsRuntime,
		FileOrDir:                       "/path/to/helmfile.d",
		SequentialHelmfiles:             true,
	}, files)

	expectNoCallsToHelm(app)

	convergeErr := fmt.Errorf("simulated converge failure")
	failingConverge := func(_ *Run) (bool, []error) {
		return false, []error{convergeErr}
	}

	err = app.ForEachState(
		failingConverge,
		false,
		SetFilter(true),
	)

	if err == nil {
		t.Fatal("expected error from ForEachState, got nil")
	}

	if !bytes.Contains([]byte(err.Error()), []byte("simulated converge failure")) {
		t.Errorf("expected error to contain 'simulated converge failure', got: %v", err)
	}
}
