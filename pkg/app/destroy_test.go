package app

import (
	"strings"
	"sync"
	"testing"

	"github.com/helmfile/vals"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"github.com/helmfile/helmfile/pkg/exectest"
	ffs "github.com/helmfile/helmfile/pkg/filesystem"
	"github.com/helmfile/helmfile/pkg/helmexec"
)

const (
	helmV3ListFlags                   = "--kube-context default --uninstalling --deployed --failed --pending"
	helmV3ListFlagsWithoutKubeContext = "--uninstalling --deployed --failed --pending"
)

func listFlags(namespace, kubeContext string) string {
	var flags []string
	if kubeContext != "" {
		flags = append(flags, "--kube-context", kubeContext)
	}
	if namespace != "" {
		flags = append(flags, "--namespace", namespace)
	}
	flags = append(flags, "--uninstalling --deployed --failed --pending")

	return strings.Join(flags, " ")
}

type destroyConfig struct {
	args                   string
	cascade                string
	concurrency            int
	interactive            bool
	skipDeps               bool
	skipRefresh            bool
	logger                 *zap.SugaredLogger
	includeTransitiveNeeds bool
	skipCharts             bool
	deleteWait             bool
	deleteTimeout          int
}

func (d destroyConfig) Args() string {
	return d.args
}

func (d destroyConfig) Cascade() string {
	return d.cascade
}

func (d destroyConfig) SkipCharts() bool {
	return d.skipCharts
}

func (d destroyConfig) Interactive() bool {
	return d.interactive
}

func (d destroyConfig) Logger() *zap.SugaredLogger {
	return d.logger
}

func (d destroyConfig) Concurrency() int {
	return d.concurrency
}

func (d destroyConfig) SkipDeps() bool {
	return d.skipDeps
}

func (d destroyConfig) SkipRefresh() bool {
	return d.skipRefresh
}

func (d destroyConfig) IncludeTransitiveNeeds() bool {
	return d.includeTransitiveNeeds
}

func (d destroyConfig) DeleteWait() bool {
	return d.deleteWait
}

func (d destroyConfig) DeleteTimeout() int {
	return d.deleteTimeout
}

func TestDestroy(t *testing.T) {
	type testcase struct {
		ns            string
		concurrency   int
		error         string
		files         map[string]string
		selectors     []string
		lists         map[exectest.ListKey]string
		diffs         map[exectest.DiffKey]error
		upgraded      []exectest.Release
		deleted       []exectest.Release
		log           string
		deleteWait    bool
		deleteTimeout int
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

			destroyErr := app.Destroy(destroyConfig{
				// if we check log output, concurrency must be 1. otherwise the test becomes non-deterministic.
				concurrency: tc.concurrency,
				logger:      logger,
			})

			switch {
			case tc.error == "" && destroyErr != nil:
				t.Fatalf("unexpected error: %v", destroyErr)
			case tc.error != "" && destroyErr == nil:
				t.Fatal("expected error did not occur")
			case tc.error != "" && destroyErr != nil && tc.error != destroyErr.Error():
				t.Fatalf("invalid error: expected %q, got %q", tc.error, destroyErr.Error())
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
	}

	files := map[string]string{
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
	}

	filesForTwoReleases := map[string]string{
		"/path/to/helmfile.yaml": `
releases:
- name: backend-v1
  chart: charts/backend
  installed: false
- name: frontend-v1
  chart: charts/frontend
  needs:
  - backend-v1
`,
	}

	t.Run("smoke", func(t *testing.T) {
		//
		// complex test cases for smoke testing
		//
		check(t, testcase{
			files: files,
			diffs: map[exectest.DiffKey]error{},
			lists: map[exectest.ListKey]string{
				{Filter: "^frontend-v1$", Flags: listFlags("", "default")}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
`,
				{Filter: "^frontend-v2$", Flags: listFlags("", "default")}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
frontend-v2 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	frontend-3.1.0	3.1.0      	default
`,
				{Filter: "^frontend-v3$", Flags: listFlags("", "default")}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
frontend-v3 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	frontend-3.1.0	3.1.0      	default
`,
				{Filter: "^backend-v1$", Flags: listFlags("", "default")}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
`,
				{Filter: "^backend-v2$", Flags: listFlags("", "default")}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
backend-v2 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	backend-3.1.0	3.1.0      	default
`,
				{Filter: "^logging$", Flags: listFlags("", "default")}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
logging	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	fluent-bit-3.1.0	3.1.0      	default
`,
				{Filter: "^front-proxy$", Flags: listFlags("", "default")}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
front-proxy 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	envoy-3.1.0	3.1.0      	default
`,
				{Filter: "^servicemesh$", Flags: listFlags("", "default")}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
servicemesh 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	istio-3.1.0	3.1.0      	default
`,
				{Filter: "^database$", Flags: listFlags("", "default")}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
database 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	mysql-3.1.0	3.1.0      	default
`,
				{Filter: "^anotherbackend$", Flags: listFlags("", "default")}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
anotherbackend 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	anotherbackend-3.1.0	3.1.0      	default
`,
			},
			// Disable concurrency to avoid in-deterministic result
			concurrency: 1,
			// Enable wait and set timeout for destroy
			deleteWait:    true,
			deleteTimeout: 300,
			upgraded:      []exectest.Release{},
			deleted: []exectest.Release{
				{Name: "frontend-v3", Flags: []string{}},
				{Name: "frontend-v2", Flags: []string{}},
				{Name: "frontend-v1", Flags: []string{}},
				{Name: "backend-v2", Flags: []string{}},
				{Name: "backend-v1", Flags: []string{}},
				{Name: "anotherbackend", Flags: []string{}},
				{Name: "servicemesh", Flags: []string{}},
				{Name: "database", Flags: []string{}},
				{Name: "front-proxy", Flags: []string{}},
				{Name: "logging", Flags: []string{}},
			},
		})
	})

	t.Run("destroy only one release with selector", func(t *testing.T) {
		check(t, testcase{
			files:     files,
			selectors: []string{"name=logging"},
			diffs:     map[exectest.DiffKey]error{},
			lists: map[exectest.ListKey]string{
				{Filter: "^frontend-v1$", Flags: listFlags("", "default")}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
`,
				{Filter: "^frontend-v2$", Flags: listFlags("", "default")}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
frontend-v2 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	frontend-3.1.0	3.1.0      	default
`,
				{Filter: "^frontend-v3$", Flags: listFlags("", "default")}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
frontend-v3 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	frontend-3.1.0	3.1.0      	default
`,
				{Filter: "^backend-v1$", Flags: listFlags("", "default")}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
`,
				{Filter: "^backend-v2$", Flags: listFlags("", "default")}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
backend-v2 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	backend-3.1.0	3.1.0      	default
`,
				{Filter: "^logging$", Flags: listFlags("", "default")}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
logging	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	fluent-bit-3.1.0	3.1.0      	default
`,
				{Filter: "^front-proxy$", Flags: listFlags("", "default")}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
front-proxy 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	envoy-3.1.0	3.1.0      	default
`,
				{Filter: "^servicemesh$", Flags: listFlags("", "default")}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
servicemesh 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	istio-3.1.0	3.1.0      	default
`,
				{Filter: "^database$", Flags: listFlags("", "default")}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
database 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	mysql-3.1.0	3.1.0      	default
`,
				{Filter: "^anotherbackend$", Flags: listFlags("", "default")}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
anotherbackend 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	anotherbackend-3.1.0	3.1.0      	default
`,
			},
			// Disable concurrency to avoid in-deterministic result
			concurrency: 1,
			upgraded:    []exectest.Release{},
			deleted: []exectest.Release{
				{Name: "logging", Flags: []string{}},
			},
		})
	})

	t.Run("destroy installed but disabled release", func(t *testing.T) {
		check(t, testcase{
			files: filesForTwoReleases,
			diffs: map[exectest.DiffKey]error{},
			lists: map[exectest.ListKey]string{
				{Filter: "^frontend-v1$", Flags: listFlags("", "default")}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
`,
				{Filter: "^backend-v1$", Flags: listFlags("", "default")}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
`,
			},
			// Disable concurrency to avoid in-deterministic result
			concurrency: 1,
			upgraded:    []exectest.Release{},
			deleted: []exectest.Release{
				{Name: "frontend-v1", Flags: []string{}},
			},
		})
	})

	t.Run("helm3", func(t *testing.T) {
		check(t, testcase{
			files: filesForTwoReleases,
			diffs: map[exectest.DiffKey]error{},
			lists: map[exectest.ListKey]string{
				{Filter: "^frontend-v1$", Flags: listFlags("", "default")}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
`,
				{Filter: "^backend-v1$", Flags: listFlags("", "default")}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
`,
			},
			// Disable concurrency to avoid in-deterministic result
			concurrency: 1,
			// Enable wait and set timeout for destroy
			deleteWait:    true,
			deleteTimeout: 300,
			upgraded:      []exectest.Release{},
			deleted: []exectest.Release{
				{Name: "frontend-v1", Flags: []string{}},
			},
		})
	})
}
