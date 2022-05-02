package app

import (
	"bufio"
	"bytes"
	"io"
	"path/filepath"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/roboll/helmfile/pkg/exectest"
	"github.com/roboll/helmfile/pkg/helmexec"
	"github.com/roboll/helmfile/pkg/testhelper"
	"github.com/variantdev/vals"
)

func TestApply_hooks(t *testing.T) {
	type fields struct {
		skipNeeds              bool
		includeNeeds           bool
		includeTransitiveNeeds bool
	}

	type testcase struct {
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
		log               string
		logLevel          string
	}

	check := func(t *testing.T, tc testcase) {
		t.Helper()

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

			logger := helmexec.NewLogger(logWriter, tc.logLevel)

			valsRuntime, err := vals.New(vals.Options{CacheSize: 32})
			if err != nil {
				t.Errorf("unexpected error creating vals runtime: %v", err)
			}

			app := appWithFs(&App{
				OverrideHelmBinary:  DefaultHelmBinary,
				glob:                filepath.Glob,
				abs:                 filepath.Abs,
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

			syncErr := app.Apply(applyConfig{
				// if we check log output, concurrency must be 1. otherwise the test becomes non-deterministic.
				concurrency:            tc.concurrency,
				logger:                 logger,
				skipDiffOnInstall:      tc.skipDiffOnInstall,
				skipNeeds:              tc.fields.skipNeeds,
				includeNeeds:           tc.fields.includeNeeds,
				includeTransitiveNeeds: tc.fields.includeTransitiveNeeds,
			})

			var gotErr string
			if syncErr != nil {
				gotErr = syncErr.Error()
			}

			if d := cmp.Diff(tc.error, gotErr); d != "" {
				t.Fatalf("unexpected error: want (-), got (+): %s", d)
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
						t.Errorf("releases[%d].flags[%d]: got %v, want %v", relIdx, flagIdx, helm.Releases[relIdx].Flags[flagIdx], wantUpgrades[relIdx].Flags[flagIdx])
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
		}()

		if tc.log != "" {
			actual := bs.String()

			diff, exists := testhelper.Diff(tc.log, actual, 3)
			if exists {
				t.Errorf("unexpected log:\nDIFF\n%s\nEOD", diff)
			}
		} else {
			assertEqualsToSnapshot(t, "log", bs.String())
		}
	}

	t.Run("apply release with preapply hook", func(t *testing.T) {
		check(t, testcase{
			files: map[string]string{
				"/path/to/helmfile.yaml": `
releases:
- name: foo
  chart: incubator/raw
  namespace: default
  labels:
    app: test
  hooks:
  - events: ["preapply"]
    command: echo
    showlogs: true
    args: ["foo"]
`,
			},
			selectors: []string{"name=foo"},
			upgraded: []exectest.Release{
				{Name: "foo"},
			},
			diffs: map[exectest.DiffKey]error{
				{Name: "foo", Chart: "incubator/raw", Flags: "--kube-contextdefault--namespacedefault--detailed-exitcode"}: helmexec.ExitError{Code: 2},
			},
			error: "",
			// as we check for log output, set concurrency to 1 to avoid non-deterministic test result
			concurrency: 1,
			logLevel:    "info",
		})
	})

	t.Run("apply release with preapply hook", func(t *testing.T) {
		check(t, testcase{
			files: map[string]string{
				"/path/to/helmfile.yaml": `
releases:
- name: foo
  chart: incubator/raw
  namespace: default
  labels:
    app: test
  hooks:
  - events: ["prepare", "preapply", "presync"]
    command: echo
    showlogs: true
    args: ["foo"]
`,
			},
			selectors: []string{"name=foo"},
			upgraded: []exectest.Release{
				{Name: "foo"},
			},
			diffs: map[exectest.DiffKey]error{
				{Name: "foo", Chart: "incubator/raw", Flags: "--kube-contextdefault--namespacedefault--detailed-exitcode"}: helmexec.ExitError{Code: 2},
			},
			error: "",
			// as we check for log output, set concurrency to 1 to avoid non-deterministic test result
			concurrency: 1,
			logLevel:    "info",
		})
	})

	t.Run("apply release with preapply hook", func(t *testing.T) {
		check(t, testcase{
			files: map[string]string{
				"/path/to/helmfile.yaml": `
releases:
- name: foo
  chart: incubator/raw
  namespace: default
  labels:
    app: test
  hooks:
  - events: ["presync"]
    command: echo
    showlogs: true
    args: ["foo"]
`,
			},
			selectors: []string{"name=foo"},
			upgraded: []exectest.Release{
				{Name: "foo"},
			},
			diffs: map[exectest.DiffKey]error{
				{Name: "foo", Chart: "incubator/raw", Flags: "--kube-contextdefault--namespacedefault--detailed-exitcode"}: helmexec.ExitError{Code: 2},
			},
			error: "",
			// as we check for log output, set concurrency to 1 to avoid non-deterministic test result
			concurrency: 1,
			logLevel:    "info",
		})
	})

}
