package app

import (
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/variantdev/vals"
	"go.uber.org/zap"

	"github.com/helmfile/helmfile/pkg/exectest"
	"github.com/helmfile/helmfile/pkg/filesystem"
	"github.com/helmfile/helmfile/pkg/helmexec"
	"github.com/helmfile/helmfile/pkg/testhelper"
)

func TestApply_3(t *testing.T) {
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

		bs := runWithLogCapture(t, "debug", func(t *testing.T, logger *zap.SugaredLogger) {
			t.Helper()

			valsRuntime, err := vals.New(vals.Options{CacheSize: 32})
			if err != nil {
				t.Errorf("unexpected error creating vals runtime: %v", err)
			}

			app := appWithFs(&App{
				OverrideHelmBinary:  DefaultHelmBinary,
				fs:                  filesystem.DefaultFileSystem(),
				OverrideKubeContext: "",
				Env:                 "default",
				Logger:              logger,
				helms: map[helmKey]helmexec.Interface{
					createHelmKey("helm", ""): helm,
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

			diff, exists := testhelper.Diff(tc.log, actual, 3)
			if exists {
				t.Errorf("unexpected log:\nDIFF\n%s\nEOD", diff)
			}
		} else {
			assertLogEqualsToSnapshot(t, bs.String())
		}
	}

	t.Run("skip-needs=true", func(t *testing.T) {
		check(t, testcase{
			fields: fields{
				skipNeeds: true,
			},
			files: map[string]string{
				"/path/to/helmfile.yaml": `
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
			upgraded: []exectest.Release{
				{Name: "external-secrets", Flags: []string{"--namespace", "default"}},
				{Name: "my-release", Flags: []string{"--namespace", "default"}},
			},
			diffs: map[exectest.DiffKey]error{
				{Name: "external-secrets", Chart: "incubator/raw", Flags: "--namespacedefault--detailed-exitcode--reset-values"}: helmexec.ExitError{Code: 2},
				{Name: "my-release", Chart: "incubator/raw", Flags: "--namespacedefault--detailed-exitcode--reset-values"}:       helmexec.ExitError{Code: 2},
			},
			lists: map[exectest.ListKey]string{
				{Filter: "^external-secrets$", Flags: helmV2ListFlagsWithoutKubeContext}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
external-secrets 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	raw-3.1.0	3.1.0      	default
`,
				{Filter: "^my-release$", Flags: helmV2ListFlagsWithoutKubeContext}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
my-release 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	raw-3.1.0	3.1.0      	default
`,
			},
			// as we check for log output, set concurrency to 1 to avoid non-deterministic test result
			concurrency: 1,
		})
	})

	t.Run("skip-needs=true with no diff on a release", func(t *testing.T) {
		check(t, testcase{
			fields: fields{
				skipNeeds: true,
			},
			files: map[string]string{
				"/path/to/helmfile.yaml": `
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
			upgraded: []exectest.Release{
				{Name: "external-secrets", Flags: []string{"--namespace", "default"}},
			},
			lists: map[exectest.ListKey]string{
				{Filter: "^external-secrets$", Flags: helmV2ListFlagsWithoutKubeContext}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
^external-secrets$ 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	raw-3.1.0	3.1.0      	default
`,
			},
			diffs: map[exectest.DiffKey]error{
				{Name: "external-secrets", Chart: "incubator/raw", Flags: "--namespacedefault--detailed-exitcode--reset-values"}: helmexec.ExitError{Code: 2},
				{Name: "my-release", Chart: "incubator/raw", Flags: "--namespacedefault--detailed-exitcode--reset-values"}:       nil,
			},
			// as we check for log output, set concurrency to 1 to avoid non-deterministic test result
			concurrency: 1,
		})
	})

	t.Run("skip-needs=false include-needs=true", func(t *testing.T) {
		check(t, testcase{
			fields: fields{
				skipNeeds:    false,
				includeNeeds: true,
			},
			error: ``,
			files: map[string]string{
				"/path/to/helmfile.yaml": `
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
			upgraded:  []exectest.Release{},
			diffs: map[exectest.DiffKey]error{
				{Name: "kubernetes-external-secrets", Chart: "incubator/raw", Flags: "--namespacekube-system--detailed-exitcode--reset-values"}: helmexec.ExitError{Code: 2},
				{Name: "external-secrets", Chart: "incubator/raw", Flags: "--namespacedefault--detailed-exitcode--reset-values"}:                helmexec.ExitError{Code: 2},
				{Name: "my-release", Chart: "incubator/raw", Flags: "--namespacedefault--detailed-exitcode--reset-values"}:                      helmexec.ExitError{Code: 2},
			},
			lists: map[exectest.ListKey]string{
				{Filter: "^kubernetes-external-secrets$", Flags: helmV2ListFlagsWithoutKubeContext}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
kubernetes-external-secrets 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	raw-3.1.0	3.1.0      	default
`,
				{Filter: "^external-secrets$", Flags: helmV2ListFlagsWithoutKubeContext}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
external-secrets 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	raw-3.1.0	3.1.0      	default
`,
				{Filter: "^my-release$", Flags: helmV2ListFlagsWithoutKubeContext}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
my-release 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	raw-3.1.0	3.1.0      	default
`,
			},
			// as we check for log output, set concurrency to 1 to avoid non-deterministic test result
			concurrency: 1,
		})
	})

	t.Run("skip-needs=false include-needs=true but no diff on needed release", func(t *testing.T) {
		check(t, testcase{
			fields: fields{
				skipNeeds:    false,
				includeNeeds: true,
			},
			error: ``,
			files: map[string]string{
				"/path/to/helmfile.yaml": `
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
			upgraded:  []exectest.Release{},
			diffs: map[exectest.DiffKey]error{
				{Name: "kubernetes-external-secrets", Chart: "incubator/raw", Flags: "--namespacekube-system--detailed-exitcode--reset-values"}: nil,
				{Name: "external-secrets", Chart: "incubator/raw", Flags: "--namespacedefault--detailed-exitcode--reset-values"}:                helmexec.ExitError{Code: 2},
				{Name: "my-release", Chart: "incubator/raw", Flags: "--namespacedefault--detailed-exitcode--reset-values"}:                      helmexec.ExitError{Code: 2},
			},
			lists: map[exectest.ListKey]string{
				{Filter: "^external-secrets$", Flags: helmV2ListFlagsWithoutKubeContext}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
external-secrets 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	raw-3.1.0	3.1.0      	default
`,
				{Filter: "^my-release$", Flags: helmV2ListFlagsWithoutKubeContext}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
my-release 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	raw-3.1.0	3.1.0      	default
`,
			},
			// as we check for log output, set concurrency to 1 to avoid non-deterministic test result
			concurrency: 1,
		})
	})

	t.Run("skip-needs=false include-needs=true with installed but disabled release", func(t *testing.T) {
		check(t, testcase{
			fields: fields{
				skipNeeds:    false,
				includeNeeds: true,
			},
			error: ``,
			files: map[string]string{
				"/path/to/helmfile.yaml": `
{{ $mark := "a" }}

releases:
- name: kubernetes-external-secrets
  chart: incubator/raw
  namespace: kube-system
  installed: false

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
			upgraded:  []exectest.Release{},
			lists: map[exectest.ListKey]string{
				{Filter: "^kubernetes-external-secrets$", Flags: helmV2ListFlagsWithoutKubeContext}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
				kubernetes-external-secrets 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	raw-3.1.0	3.1.0      	default
`,
				{Filter: "^external-secrets$", Flags: helmV2ListFlagsWithoutKubeContext}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
external-secrets 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	raw-3.1.0	3.1.0      	default
`,
				{Filter: "^my-release$", Flags: helmV2ListFlagsWithoutKubeContext}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
my-release 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	raw-3.1.0	3.1.0      	default
`,
			},
			diffs: map[exectest.DiffKey]error{
				{Name: "external-secrets", Chart: "incubator/raw", Flags: "--namespacedefault--detailed-exitcode--reset-values"}: helmexec.ExitError{Code: 2},
				{Name: "my-release", Chart: "incubator/raw", Flags: "--namespacedefault--detailed-exitcode--reset-values"}:       helmexec.ExitError{Code: 2},
			},
			// as we check for log output, set concurrency to 1 to avoid non-deterministic test result
			concurrency: 1,
		})
	})

	t.Run("skip-needs=false include-needs=true with not installed and disabled release", func(t *testing.T) {
		check(t, testcase{
			fields: fields{
				skipNeeds:    false,
				includeNeeds: true,
			},
			error: ``,
			files: map[string]string{
				"/path/to/helmfile.yaml": `
{{ $mark := "a" }}

releases:
- name: kubernetes-external-secrets
  chart: incubator/raw
  namespace: kube-system
  installed: false

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
			upgraded:  []exectest.Release{},
			lists: map[exectest.ListKey]string{
				{Filter: "^kubernetes-external-secrets$", Flags: helmV2ListFlagsWithoutKubeContext}: ``,
				{Filter: "^external-secrets$", Flags: helmV2ListFlagsWithoutKubeContext}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
external-secrets 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	raw-3.1.0	3.1.0      	default
`,
				{Filter: "^my-release$", Flags: helmV2ListFlagsWithoutKubeContext}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
my-release 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	raw-3.1.0	3.1.0      	default
`,
			},
			diffs: map[exectest.DiffKey]error{
				{Name: "external-secrets", Chart: "incubator/raw", Flags: "--namespacedefault--detailed-exitcode--reset-values"}: helmexec.ExitError{Code: 2},
				{Name: "my-release", Chart: "incubator/raw", Flags: "--namespacedefault--detailed-exitcode--reset-values"}:       helmexec.ExitError{Code: 2},
			},
			// as we check for log output, set concurrency to 1 to avoid non-deterministic test result
			concurrency: 1,
		})
	})

	t.Run("bad --selector", func(t *testing.T) {
		check(t, testcase{
			files: map[string]string{
				"/path/to/helmfile.yaml": `
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
			upgraded:  []exectest.Release{},
			error:     "err: no releases found that matches specified selector(app=test_non_existent) and environment(default), in any helmfile",
			// as we check for log output, set concurrency to 1 to avoid non-deterministic test result
			concurrency: 1,
			log: `processing file "helmfile.yaml" in directory "."
changing working directory to "/path/to"
first-pass rendering starting for "helmfile.yaml.part.0": inherited=&{default map[] map[]}, overrode=<nil>
first-pass uses: &{default map[] map[]}
first-pass rendering output of "helmfile.yaml.part.0":
 0: 
 1: 
 2: 
 3: releases:
 4: - name: kubernetes-external-secrets
 5:   chart: incubator/raw
 6:   namespace: kube-system
 7: 
 8: - name: external-secrets
 9:   chart: incubator/raw
10:   namespace: default
11:   labels:
12:     app: test
13:   needs:
14:   - kube-system/kubernetes-external-secrets
15: 
16: - name: my-release
17:   chart: incubator/raw
18:   namespace: default
19:   labels:
20:     app: test
21:   needs:
22:   - default/external-secrets
23: 

first-pass produced: &{default map[] map[]}
first-pass rendering result of "helmfile.yaml.part.0": {default map[] map[]}
vals:
map[]
defaultVals:[]
second-pass rendering result of "helmfile.yaml.part.0":
 0: 
 1: 
 2: 
 3: releases:
 4: - name: kubernetes-external-secrets
 5:   chart: incubator/raw
 6:   namespace: kube-system
 7: 
 8: - name: external-secrets
 9:   chart: incubator/raw
10:   namespace: default
11:   labels:
12:     app: test
13:   needs:
14:   - kube-system/kubernetes-external-secrets
15: 
16: - name: my-release
17:   chart: incubator/raw
18:   namespace: default
19:   labels:
20:     app: test
21:   needs:
22:   - default/external-secrets
23: 

merged environment: &{default map[] map[]}
0 release(s) matching app=test_non_existent found in helmfile.yaml

changing working directory back to "/path/to"
`,
		})
	})
}
