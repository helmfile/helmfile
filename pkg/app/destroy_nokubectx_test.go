package app

import (
	"sync"
	"testing"

	"github.com/helmfile/vals"
	"go.uber.org/zap"

	"github.com/helmfile/helmfile/pkg/exectest"
	ffs "github.com/helmfile/helmfile/pkg/filesystem"
	"github.com/helmfile/helmfile/pkg/helmexec"
	"github.com/helmfile/helmfile/pkg/testhelper"
)

func TestDestroy_2(t *testing.T) {
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
		wait          bool
		deleteWait    bool
		deleteTimeout int
	}

	check := func(t *testing.T, tc testcase) {
		t.Helper()

		wantUpgrades := tc.upgraded
		wantDeletes := tc.deleted

		var helm = &exectest.Helm{
			Helm3:                true,
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

			destroyErr := app.Destroy(destroyConfig{
				// if we check log output, concurrency must be 1. otherwise the test becomes non-deterministic.
				concurrency:            tc.concurrency,
				logger:                 logger,
				includeTransitiveNeeds: false,
				deleteWait:             tc.deleteWait,
				deleteTimeout:          tc.deleteTimeout,
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

			diff, exists := testhelper.Diff(tc.log, actual, 3)
			if exists {
				t.Errorf("unexpected log:\nDIFF\n%s\nEOD", diff)
			}
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
				{Filter: "^frontend-v1$", Flags: helmV3ListFlagsWithoutKubeContext}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
`,
				{Filter: "^frontend-v2$", Flags: helmV3ListFlagsWithoutKubeContext}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
frontend-v2 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	frontend-3.1.0	3.1.0      	default
`,
				{Filter: "^frontend-v3$", Flags: helmV3ListFlagsWithoutKubeContext}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
frontend-v3 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	frontend-3.1.0	3.1.0      	default
`,
				{Filter: "^backend-v1$", Flags: helmV3ListFlagsWithoutKubeContext}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
`,
				{Filter: "^backend-v2$", Flags: helmV3ListFlagsWithoutKubeContext}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
backend-v2 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	backend-3.1.0	3.1.0      	default
`,
				{Filter: "^logging$", Flags: helmV3ListFlagsWithoutKubeContext}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
logging	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	fluent-bit-3.1.0	3.1.0      	default
`,
				{Filter: "^front-proxy$", Flags: helmV3ListFlagsWithoutKubeContext}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
front-proxy 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	envoy-3.1.0	3.1.0      	default
`,
				{Filter: "^servicemesh$", Flags: helmV3ListFlagsWithoutKubeContext}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
servicemesh 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	istio-3.1.0	3.1.0      	default
`,
				{Filter: "^database$", Flags: helmV3ListFlagsWithoutKubeContext}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
database 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	mysql-3.1.0	3.1.0      	default
`,
				{Filter: "^anotherbackend$", Flags: helmV3ListFlagsWithoutKubeContext}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
anotherbackend 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	anotherbackend-3.1.0	3.1.0      	default
`,
			},
			// Disable concurrency to avoid in-deterministic result
			concurrency: 1,
			upgraded:    []exectest.Release{},
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
			log: `processing file "helmfile.yaml" in directory "."
changing working directory to "/path/to"
first-pass rendering starting for "helmfile.yaml.part.0": inherited=&{default  map[] map[]}, overrode=<nil>
first-pass uses: &{default  map[] map[]}
first-pass rendering output of "helmfile.yaml.part.0":
 0: 
 1: releases:
 2: - name: database
 3:   chart: charts/mysql
 4:   needs:
 5:   - logging
 6: - name: frontend-v1
 7:   chart: charts/frontend
 8:   installed: false
 9:   needs:
10:   - servicemesh
11:   - logging
12:   - backend-v1
13: - name: frontend-v2
14:   chart: charts/frontend
15:   needs:
16:   - servicemesh
17:   - logging
18:   - backend-v2
19: - name: frontend-v3
20:   chart: charts/frontend
21:   needs:
22:   - servicemesh
23:   - logging
24:   - backend-v2
25: - name: backend-v1
26:   chart: charts/backend
27:   installed: false
28:   needs:
29:   - servicemesh
30:   - logging
31:   - database
32:   - anotherbackend
33: - name: backend-v2
34:   chart: charts/backend
35:   needs:
36:   - servicemesh
37:   - logging
38:   - database
39:   - anotherbackend
40: - name: anotherbackend
41:   chart: charts/anotherbackend
42:   needs:
43:   - servicemesh
44:   - logging
45:   - database
46: - name: servicemesh
47:   chart: charts/istio
48:   needs:
49:   - logging
50: - name: logging
51:   chart: charts/fluent-bit
52: - name: front-proxy
53:   chart: stable/envoy
54: 

first-pass produced: &{default  map[] map[]}
first-pass rendering result of "helmfile.yaml.part.0": {default  map[] map[]}
vals:
map[]
defaultVals:[]
second-pass rendering result of "helmfile.yaml.part.0":
 0: 
 1: releases:
 2: - name: database
 3:   chart: charts/mysql
 4:   needs:
 5:   - logging
 6: - name: frontend-v1
 7:   chart: charts/frontend
 8:   installed: false
 9:   needs:
10:   - servicemesh
11:   - logging
12:   - backend-v1
13: - name: frontend-v2
14:   chart: charts/frontend
15:   needs:
16:   - servicemesh
17:   - logging
18:   - backend-v2
19: - name: frontend-v3
20:   chart: charts/frontend
21:   needs:
22:   - servicemesh
23:   - logging
24:   - backend-v2
25: - name: backend-v1
26:   chart: charts/backend
27:   installed: false
28:   needs:
29:   - servicemesh
30:   - logging
31:   - database
32:   - anotherbackend
33: - name: backend-v2
34:   chart: charts/backend
35:   needs:
36:   - servicemesh
37:   - logging
38:   - database
39:   - anotherbackend
40: - name: anotherbackend
41:   chart: charts/anotherbackend
42:   needs:
43:   - servicemesh
44:   - logging
45:   - database
46: - name: servicemesh
47:   chart: charts/istio
48:   needs:
49:   - logging
50: - name: logging
51:   chart: charts/fluent-bit
52: - name: front-proxy
53:   chart: stable/envoy
54: 

merged environment: &{default  map[] map[]}
10 release(s) found in helmfile.yaml

processing 5 groups of releases in this order:
GROUP RELEASES
1     frontend-v3, frontend-v2, frontend-v1
2     backend-v2, backend-v1
3     anotherbackend
4     servicemesh, database
5     front-proxy, logging

processing releases in group 1/5: frontend-v3, frontend-v2, frontend-v1
release "frontend-v3" processed
release "frontend-v2" processed
release "frontend-v1" processed
processing releases in group 2/5: backend-v2, backend-v1
release "backend-v2" processed
release "backend-v1" processed
processing releases in group 3/5: anotherbackend
release "anotherbackend" processed
processing releases in group 4/5: servicemesh, database
release "servicemesh" processed
release "database" processed
processing releases in group 5/5: front-proxy, logging
release "front-proxy" processed
release "logging" processed

DELETED RELEASES:
NAME             DURATION
frontend-v3            0s
frontend-v2            0s
frontend-v1            0s
backend-v2             0s
backend-v1             0s
anotherbackend         0s
servicemesh            0s
database               0s
front-proxy            0s
logging                0s

changing working directory back to "/path/to"
`,
		})
	})

	t.Run("destroy only one release with selector", func(t *testing.T) {
		check(t, testcase{
			files:     files,
			selectors: []string{"name=logging"},
			diffs:     map[exectest.DiffKey]error{},
			lists: map[exectest.ListKey]string{
				{Filter: "^frontend-v1$", Flags: helmV3ListFlagsWithoutKubeContext}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
`,
				{Filter: "^frontend-v2$", Flags: helmV3ListFlagsWithoutKubeContext}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
frontend-v2 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	frontend-3.1.0	3.1.0      	default
`,
				{Filter: "^frontend-v3$", Flags: helmV3ListFlagsWithoutKubeContext}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
frontend-v3 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	frontend-3.1.0	3.1.0      	default
`,
				{Filter: "^backend-v1$", Flags: helmV3ListFlagsWithoutKubeContext}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
`,
				{Filter: "^backend-v2$", Flags: helmV3ListFlagsWithoutKubeContext}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
backend-v2 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	backend-3.1.0	3.1.0      	default
`,
				{Filter: "^logging$", Flags: helmV3ListFlagsWithoutKubeContext}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
logging	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	fluent-bit-3.1.0	3.1.0      	default
`,
				{Filter: "^front-proxy$", Flags: helmV3ListFlagsWithoutKubeContext}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
front-proxy 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	envoy-3.1.0	3.1.0      	default
`,
				{Filter: "^servicemesh$", Flags: helmV3ListFlagsWithoutKubeContext}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
servicemesh 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	istio-3.1.0	3.1.0      	default
`,
				{Filter: "^database$", Flags: helmV3ListFlagsWithoutKubeContext}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
database 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	mysql-3.1.0	3.1.0      	default
`,
				{Filter: "^anotherbackend$", Flags: helmV3ListFlagsWithoutKubeContext}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
anotherbackend 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	anotherbackend-3.1.0	3.1.0      	default
`,
			},
			// Enable wait and set timeout for destroy
			deleteWait: true,
			deleteTimeout: 300,
			// Disable concurrency to avoid in-deterministic result
			concurrency: 1,
			upgraded:    []exectest.Release{},
			deleted: []exectest.Release{
				{Name: "logging", Flags: []string{}},
			},
			log: `processing file "helmfile.yaml" in directory "."
changing working directory to "/path/to"
first-pass rendering starting for "helmfile.yaml.part.0": inherited=&{default  map[] map[]}, overrode=<nil>
first-pass uses: &{default  map[] map[]}
first-pass rendering output of "helmfile.yaml.part.0":
 0: 
 1: releases:
 2: - name: database
 3:   chart: charts/mysql
 4:   needs:
 5:   - logging
 6: - name: frontend-v1
 7:   chart: charts/frontend
 8:   installed: false
 9:   needs:
10:   - servicemesh
11:   - logging
12:   - backend-v1
13: - name: frontend-v2
14:   chart: charts/frontend
15:   needs:
16:   - servicemesh
17:   - logging
18:   - backend-v2
19: - name: frontend-v3
20:   chart: charts/frontend
21:   needs:
22:   - servicemesh
23:   - logging
24:   - backend-v2
25: - name: backend-v1
26:   chart: charts/backend
27:   installed: false
28:   needs:
29:   - servicemesh
30:   - logging
31:   - database
32:   - anotherbackend
33: - name: backend-v2
34:   chart: charts/backend
35:   needs:
36:   - servicemesh
37:   - logging
38:   - database
39:   - anotherbackend
40: - name: anotherbackend
41:   chart: charts/anotherbackend
42:   needs:
43:   - servicemesh
44:   - logging
45:   - database
46: - name: servicemesh
47:   chart: charts/istio
48:   needs:
49:   - logging
50: - name: logging
51:   chart: charts/fluent-bit
52: - name: front-proxy
53:   chart: stable/envoy
54: 

first-pass produced: &{default  map[] map[]}
first-pass rendering result of "helmfile.yaml.part.0": {default  map[] map[]}
vals:
map[]
defaultVals:[]
second-pass rendering result of "helmfile.yaml.part.0":
 0: 
 1: releases:
 2: - name: database
 3:   chart: charts/mysql
 4:   needs:
 5:   - logging
 6: - name: frontend-v1
 7:   chart: charts/frontend
 8:   installed: false
 9:   needs:
10:   - servicemesh
11:   - logging
12:   - backend-v1
13: - name: frontend-v2
14:   chart: charts/frontend
15:   needs:
16:   - servicemesh
17:   - logging
18:   - backend-v2
19: - name: frontend-v3
20:   chart: charts/frontend
21:   needs:
22:   - servicemesh
23:   - logging
24:   - backend-v2
25: - name: backend-v1
26:   chart: charts/backend
27:   installed: false
28:   needs:
29:   - servicemesh
30:   - logging
31:   - database
32:   - anotherbackend
33: - name: backend-v2
34:   chart: charts/backend
35:   needs:
36:   - servicemesh
37:   - logging
38:   - database
39:   - anotherbackend
40: - name: anotherbackend
41:   chart: charts/anotherbackend
42:   needs:
43:   - servicemesh
44:   - logging
45:   - database
46: - name: servicemesh
47:   chart: charts/istio
48:   needs:
49:   - logging
50: - name: logging
51:   chart: charts/fluent-bit
52: - name: front-proxy
53:   chart: stable/envoy
54: 

merged environment: &{default  map[] map[]}
1 release(s) matching name=logging found in helmfile.yaml

processing 1 groups of releases in this order:
GROUP RELEASES
1     logging

processing releases in group 1/1: logging
release "logging" processed

DELETED RELEASES:
NAME      DURATION
logging         0s

changing working directory back to "/path/to"
`,
		})
	})

	t.Run("destroy installed but disabled release", func(t *testing.T) {
		check(t, testcase{
			files: filesForTwoReleases,
			diffs: map[exectest.DiffKey]error{},
			lists: map[exectest.ListKey]string{
				{Filter: "^frontend-v1$", Flags: helmV3ListFlagsWithoutKubeContext}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
`,
				{Filter: "^backend-v1$", Flags: helmV3ListFlagsWithoutKubeContext}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
`,
			},
			// Disable concurrency to avoid in-deterministic result
			concurrency: 1,
			upgraded:    []exectest.Release{},
			deleted: []exectest.Release{
				{Name: "frontend-v1", Flags: []string{}},
			},
			log: `processing file "helmfile.yaml" in directory "."
changing working directory to "/path/to"
first-pass rendering starting for "helmfile.yaml.part.0": inherited=&{default  map[] map[]}, overrode=<nil>
first-pass uses: &{default  map[] map[]}
first-pass rendering output of "helmfile.yaml.part.0":
 0: 
 1: releases:
 2: - name: backend-v1
 3:   chart: charts/backend
 4:   installed: false
 5: - name: frontend-v1
 6:   chart: charts/frontend
 7:   needs:
 8:   - backend-v1
 9: 

first-pass produced: &{default  map[] map[]}
first-pass rendering result of "helmfile.yaml.part.0": {default  map[] map[]}
vals:
map[]
defaultVals:[]
second-pass rendering result of "helmfile.yaml.part.0":
 0: 
 1: releases:
 2: - name: backend-v1
 3:   chart: charts/backend
 4:   installed: false
 5: - name: frontend-v1
 6:   chart: charts/frontend
 7:   needs:
 8:   - backend-v1
 9: 

merged environment: &{default  map[] map[]}
2 release(s) found in helmfile.yaml

processing 2 groups of releases in this order:
GROUP RELEASES
1     frontend-v1
2     backend-v1

processing releases in group 1/2: frontend-v1
release "frontend-v1" processed
processing releases in group 2/2: backend-v1
release "backend-v1" processed

DELETED RELEASES:
NAME          DURATION
frontend-v1         0s
backend-v1          0s

changing working directory back to "/path/to"
`,
		})
	})

	t.Run("helm3", func(t *testing.T) {
		check(t, testcase{
			files: filesForTwoReleases,
			diffs: map[exectest.DiffKey]error{},
			lists: map[exectest.ListKey]string{
				{Filter: "^frontend-v1$", Flags: helmV3ListFlagsWithoutKubeContext}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
`,
				{Filter: "^backend-v1$", Flags: helmV3ListFlagsWithoutKubeContext}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
`,
			},
			// Disable concurrency to avoid in-deterministic result
			concurrency: 1,
			upgraded:    []exectest.Release{},
			deleted: []exectest.Release{
				{Name: "frontend-v1", Flags: []string{}},
			},
			log: `processing file "helmfile.yaml" in directory "."
changing working directory to "/path/to"
first-pass rendering starting for "helmfile.yaml.part.0": inherited=&{default  map[] map[]}, overrode=<nil>
first-pass uses: &{default  map[] map[]}
first-pass rendering output of "helmfile.yaml.part.0":
 0: 
 1: releases:
 2: - name: backend-v1
 3:   chart: charts/backend
 4:   installed: false
 5: - name: frontend-v1
 6:   chart: charts/frontend
 7:   needs:
 8:   - backend-v1
 9: 

first-pass produced: &{default  map[] map[]}
first-pass rendering result of "helmfile.yaml.part.0": {default  map[] map[]}
vals:
map[]
defaultVals:[]
second-pass rendering result of "helmfile.yaml.part.0":
 0: 
 1: releases:
 2: - name: backend-v1
 3:   chart: charts/backend
 4:   installed: false
 5: - name: frontend-v1
 6:   chart: charts/frontend
 7:   needs:
 8:   - backend-v1
 9: 

merged environment: &{default  map[] map[]}
2 release(s) found in helmfile.yaml

processing 2 groups of releases in this order:
GROUP RELEASES
1     frontend-v1
2     backend-v1

processing releases in group 1/2: frontend-v1
release "frontend-v1" processed
processing releases in group 2/2: backend-v1
release "backend-v1" processed

DELETED RELEASES:
NAME          DURATION
frontend-v1         0s
backend-v1          0s

changing working directory back to "/path/to"
`,
		})
	})
}
