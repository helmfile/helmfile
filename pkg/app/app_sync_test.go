package app

import (
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/helmfile/vals"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"github.com/helmfile/helmfile/pkg/exectest"
	ffs "github.com/helmfile/helmfile/pkg/filesystem"
	"github.com/helmfile/helmfile/pkg/helmexec"
)

func TestSync(t *testing.T) {
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

			syncErr := app.Sync(applyConfig{
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

			assert.Equal(t, tc.log, actual)
		}
	}

	t.Run("skip-needs=true", func(t *testing.T) {
		check(t, testcase{
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
		})
	})

	t.Run("check-duration", func(t *testing.T) {
		check(t, testcase{
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
  hooks:
    - name: my-release
      events:
        - postsync
      showlogs: true
      command: sleep
      args: [5s]
`,
			},
			selectors: []string{"app=test"},
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
			upgraded:  []exectest.Release{},
			lists: map[exectest.ListKey]string{
				{Filter: "^kubernetes-external-secrets$", Flags: listFlags("kube-system", "default")}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
kubernetes-external-secrets 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	raw-3.1.0	3.1.0      	default
				`,
				{Filter: "^external-secrets$", Flags: listFlags("default", "default")}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
external-secrets 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	raw-3.1.0	3.1.0      	default
				`,
				{Filter: "^my-release$", Flags: listFlags("default", "default")}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
my-release 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	raw-3.1.0	3.1.0      	default
				`,
			},
			// as we check for log output, set concurrency to 1 to avoid non-deterministic test result
			concurrency: 1,
		})
	})

	t.Run("include-transitive-needs=true", func(t *testing.T) {
		check(t, testcase{
			fields: fields{
				skipNeeds:              false,
				includeTransitiveNeeds: true,
			},
			error: ``,
			files: map[string]string{
				"/path/to/helmfile.yaml.gotmpl": `
{{ $mark := "a" }}

releases:
- name: serviceA
  chart: my/chart
  needs:
  - serviceB

- name: serviceB
  chart: my/chart
  needs:
  - serviceC

- name: serviceC
  chart: my/chart

- name: serviceD
  chart: my/chart
`,
			},
			selectors: []string{"name=serviceA"},
			upgraded:  []exectest.Release{},
			lists: map[exectest.ListKey]string{
				{Filter: "^serviceC$", Flags: listFlags("", "default")}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
serviceC 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	chart-3.1.0	3.1.0      	default
				`,
				{Filter: "^serviceB$", Flags: listFlags("", "default")}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
serviceB 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	chart-3.1.0	3.1.0      	default
				`,
				{Filter: "^serviceA$", Flags: listFlags("", "default")}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
serviceA 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	chart-3.1.0	3.1.0      	default
				`,
			},
			// as we check for log output, set concurrency to 1 to avoid non-deterministic test result
			concurrency: 1,
		})
	})

	t.Run("skip-needs=false include-needs=true with installed but disabled release", func(t *testing.T) {
		check(t, testcase{
			fields: fields{
				skipNeeds:              false,
				includeNeeds:           true,
				includeTransitiveNeeds: false,
			},
			error: ``,
			files: map[string]string{
				"/path/to/helmfile.yaml.gotmpl": `
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
				{Filter: "^kubernetes-external-secrets$", Flags: listFlags("kube-system", "default")}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
kubernetes-external-secrets 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	raw-3.1.0	3.1.0      	default
				`,
				{Filter: "^external-secrets$", Flags: listFlags("default", "default")}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
external-secrets 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	raw-3.1.0	3.1.0      	default
				`,
				{Filter: "^my-release$", Flags: listFlags("default", "default")}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
my-release 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	raw-3.1.0	3.1.0      	default
				`,
			},
			// as we check for log output, set concurrency to 1 to avoid non-deterministic test result
			concurrency: 1,
		})
	})

	t.Run("skip-needs=false include-needs=true with not installed and disabled release", func(t *testing.T) {
		check(t, testcase{
			fields: fields{
				skipNeeds:              false,
				includeTransitiveNeeds: false,
				includeNeeds:           true,
			},
			error: ``,
			files: map[string]string{
				"/path/to/helmfile.yaml.gotmpl": `
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
				// delete frontend-v1 and backend-v1
				{Filter: "^kubernetes-external-secrets$", Flags: listFlags("kube-system", "default")}: ``,
				{Filter: "^external-secrets$", Flags: listFlags("default", "default")}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
external-secrets 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	raw-3.1.0	3.1.0      	default
				`,
				{Filter: "^my-release$", Flags: listFlags("default", "default")}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
my-release 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	raw-3.1.0	3.1.0      	default
				`,
			},
			// as we check for log output, set concurrency to 1 to avoid non-deterministic test result
			concurrency: 1,
		})
	})

	t.Run("bad --selector", func(t *testing.T) {
		check(t, testcase{
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
			upgraded:  []exectest.Release{},
			error:     "err: no releases found that matches specified selector(app=test_non_existent) and environment(default), in any helmfile",
			// as we check for log output, set concurrency to 1 to avoid non-deterministic test result
			concurrency: 1,
		})
	})
}
