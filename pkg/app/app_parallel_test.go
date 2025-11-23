package app

import (
	"bytes"
	"testing"

	"github.com/helmfile/vals"

	ffs "github.com/helmfile/helmfile/pkg/filesystem"
	"github.com/helmfile/helmfile/pkg/helmexec"
	"github.com/helmfile/helmfile/pkg/testhelper"
	"github.com/helmfile/helmfile/pkg/testutil"
)

// TestParallelProcessingDeterministicOutput verifies that ListReleases produces
// consistent sorted output even with parallel processing of multiple helmfile.d files
func TestParallelProcessingDeterministicOutput(t *testing.T) {
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

	// Run ListReleases multiple times to verify consistent ordering
	var outputs []string
	for i := 0; i < 5; i++ {
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
		}, files)

		expectNoCallsToHelm(app)

		err = app.ListReleases(configImpl{
			skipCharts: false,
			output:     "table",
		})

		if err != nil {
			t.Fatalf("unexpected error on iteration %d: %v", i, err)
		}

		outputs = append(outputs, buffer.String())
	}

	// Verify all outputs are identical (deterministic)
	firstOutput := outputs[0]
	for i, output := range outputs[1:] {
		if output != firstOutput {
			t.Errorf("output %d differs from first output (non-deterministic ordering)", i+1)
			t.Logf("First output:\n%s", firstOutput)
			t.Logf("Output %d:\n%s", i+1, output)
		}
	}
}

// TestMultipleHelmfileDFiles verifies that all files in helmfile.d are processed
func TestMultipleHelmfileDFiles(t *testing.T) {
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
	}, files)

	expectNoCallsToHelm(app)

	// Capture stdout since ListReleases outputs to stdout
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

	// Verify all three releases are present in output (JSON format)
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
