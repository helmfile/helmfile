package app

import (
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/variantdev/vals"
	"go.uber.org/zap"

	"github.com/helmfile/helmfile/pkg/exectest"
	ffs "github.com/helmfile/helmfile/pkg/filesystem"
	"github.com/helmfile/helmfile/pkg/helmexec"
	"github.com/helmfile/helmfile/pkg/testhelper"
)

func TestDiff_2(t *testing.T) {
	type flags struct {
		skipNeeds    bool
		includeNeeds bool
	}

	testcases := []struct {
		name             string
		loc              string
		ns               string
		concurrency      int
		detailedExitcode bool
		error            string
		flags            flags
		files            map[string]string
		selectors        []string
		lists            map[exectest.ListKey]string
		diffs            map[exectest.DiffKey]error
		upgraded         []exectest.Release
		deleted          []exectest.Release
		log              string
	}{
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
			detailedExitcode: true,
			error:            "Identified at least one change",
			diffs: map[exectest.DiffKey]error{
				// noop on frontend-v2
				{Name: "frontend-v2", Chart: "charts/frontend", Flags: "--detailed-exitcode--reset-values"}: nil,
				// install frontend-v3
				{Name: "frontend-v3", Chart: "charts/frontend", Flags: "--detailed-exitcode--reset-values"}: helmexec.ExitError{Code: 2},
				// upgrades
				{Name: "logging", Chart: "charts/fluent-bit", Flags: "--detailed-exitcode--reset-values"}:            helmexec.ExitError{Code: 2},
				{Name: "front-proxy", Chart: "stable/envoy", Flags: "--detailed-exitcode--reset-values"}:             helmexec.ExitError{Code: 2},
				{Name: "servicemesh", Chart: "charts/istio", Flags: "--detailed-exitcode--reset-values"}:             helmexec.ExitError{Code: 2},
				{Name: "database", Chart: "charts/mysql", Flags: "--detailed-exitcode--reset-values"}:                helmexec.ExitError{Code: 2},
				{Name: "backend-v2", Chart: "charts/backend", Flags: "--detailed-exitcode--reset-values"}:            helmexec.ExitError{Code: 2},
				{Name: "anotherbackend", Chart: "charts/anotherbackend", Flags: "--detailed-exitcode--reset-values"}: helmexec.ExitError{Code: 2},
			},
			lists: map[exectest.ListKey]string{
				// delete frontend-v1 and backend-v1
				{Filter: "^frontend-v1$", Flags: helmV2ListFlagsWithoutKubeContext}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
frontend-v1 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	backend-3.1.0	3.1.0      	default
`,
				{Filter: "^backend-v1$", Flags: helmV2ListFlagsWithoutKubeContext}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
backend-v1 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	backend-3.1.0	3.1.0      	default
`,
			},
			// Disable concurrency to avoid in-deterministic result
			concurrency: 1,
			upgraded:    []exectest.Release{},
			deleted:     []exectest.Release{},
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
  chart: mychart2
- name: foo
  chart: mychart1
  installed: false
  needs:
  - bar
`,
			},
			detailedExitcode: true,
			error:            "",
			diffs: map[exectest.DiffKey]error{
				{Name: "bar", Chart: "mychart2", Flags: "--detailed-exitcode--reset-values"}: nil,
			},
			lists: map[exectest.ListKey]string{
				{Filter: "^foo$", Flags: helmV2ListFlagsWithoutKubeContext}: ``,
				{Filter: "^bar$", Flags: helmV2ListFlagsWithoutKubeContext}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
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
  chart: mychart3
- name: foo
  chart: mychart1
  needs:
  - bar
- name: bar
  chart: mychart2
`,
			},
			detailedExitcode: true,
			error:            "Identified at least one change",
			diffs: map[exectest.DiffKey]error{
				{Name: "foo", Chart: "mychart1", Flags: "--detailed-exitcode--reset-values"}: helmexec.ExitError{Code: 2},
				{Name: "bar", Chart: "mychart2", Flags: "--detailed-exitcode--reset-values"}: helmexec.ExitError{Code: 2},
				{Name: "baz", Chart: "mychart3", Flags: "--detailed-exitcode--reset-values"}: helmexec.ExitError{Code: 2},
			},
			lists:       map[exectest.ListKey]string{},
			upgraded:    []exectest.Release{},
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
  chart: mychart2
- name: foo
  chart: mychart1
  needs:
  - bar
`,
			},
			detailedExitcode: true,
			error:            "Identified at least one change",
			diffs: map[exectest.DiffKey]error{
				{Name: "bar", Chart: "mychart2", Flags: "--detailed-exitcode--reset-values"}: helmexec.ExitError{Code: 2},
				{Name: "foo", Chart: "mychart1", Flags: "--detailed-exitcode--reset-values"}: helmexec.ExitError{Code: 2},
			},
			upgraded: []exectest.Release{},
		},
		{
			name: "upgrade when bar needs foo",
			loc:  location(),
			files: map[string]string{
				"/path/to/helmfile.yaml": `
releases:
- name: foo
  chart: mychart1
- name: bar
  chart: mychart2
  needs:
  - foo
`,
			},
			detailedExitcode: true,
			error:            "Identified at least one change",
			diffs: map[exectest.DiffKey]error{
				{Name: "bar", Chart: "mychart2", Flags: "--detailed-exitcode--reset-values"}: helmexec.ExitError{Code: 2},
				{Name: "foo", Chart: "mychart1", Flags: "--detailed-exitcode--reset-values"}: helmexec.ExitError{Code: 2},
			},
			upgraded: []exectest.Release{},
		},
		{
			name: "upgrade when foo needs bar, with ns override",
			loc:  location(),
			ns:   "testNamespace",
			files: map[string]string{
				"/path/to/helmfile.yaml": `
releases:
- name: bar
  chart: mychart2
- name: foo
  chart: mychart1
  needs:
  - bar
`,
			},
			detailedExitcode: true,
			error:            "Identified at least one change",
			diffs: map[exectest.DiffKey]error{
				{Name: "bar", Chart: "mychart2", Flags: "--namespacetestNamespace--detailed-exitcode--reset-values"}: helmexec.ExitError{Code: 2},
				{Name: "foo", Chart: "mychart1", Flags: "--namespacetestNamespace--detailed-exitcode--reset-values"}: helmexec.ExitError{Code: 2},
			},
			upgraded: []exectest.Release{},
		},
		{
			name: "upgrade when bar needs foo, with ns override",
			loc:  location(),
			ns:   "testNamespace",
			files: map[string]string{
				"/path/to/helmfile.yaml": `
releases:
- name: foo
  chart: mychart1
- name: bar
  chart: mychart2
  needs:
  - foo
`,
			},
			detailedExitcode: true,
			error:            "Identified at least one change",
			diffs: map[exectest.DiffKey]error{
				{Name: "bar", Chart: "mychart2", Flags: "--namespacetestNamespace--detailed-exitcode--reset-values"}: helmexec.ExitError{Code: 2},
				{Name: "foo", Chart: "mychart1", Flags: "--namespacetestNamespace--detailed-exitcode--reset-values"}: helmexec.ExitError{Code: 2},
			},
			upgraded: []exectest.Release{},
		},
		{
			name: "upgrade when ns1/foo needs ns2/bar",
			loc:  location(),
			files: map[string]string{
				"/path/to/helmfile.yaml": `
releases:
- name: foo
  chart: mychart1
  namespace: ns1
  needs:
  - ns2/bar
- name: bar
  chart: mychart2
  namespace: ns2
`,
			},
			detailedExitcode: true,
			error:            "Identified at least one change",
			diffs: map[exectest.DiffKey]error{
				{Name: "bar", Chart: "mychart2", Flags: "--namespacens2--detailed-exitcode--reset-values"}: helmexec.ExitError{Code: 2},
				{Name: "foo", Chart: "mychart1", Flags: "--namespacens1--detailed-exitcode--reset-values"}: helmexec.ExitError{Code: 2},
			},
			upgraded: []exectest.Release{},
		},
		{
			name: "upgrade when ns2/bar needs ns1/foo",
			loc:  location(),
			files: map[string]string{
				"/path/to/helmfile.yaml": `
releases:
- name: bar
  chart: mychart2
  namespace: ns2
  needs:
  - ns1/foo
- name: foo
  chart: mychart1
  namespace: ns1
`,
			},
			detailedExitcode: true,
			error:            "Identified at least one change",
			diffs: map[exectest.DiffKey]error{
				{Name: "bar", Chart: "mychart2", Flags: "--namespacens2--detailed-exitcode--reset-values"}: helmexec.ExitError{Code: 2},
				{Name: "foo", Chart: "mychart1", Flags: "--namespacens1--detailed-exitcode--reset-values"}: helmexec.ExitError{Code: 2},
			},
			upgraded: []exectest.Release{},
		},
		{
			name: "helm2 upgrade when tns1 foo needs tns2 bar",
			loc:  location(),

			files: map[string]string{
				"/path/to/helmfile.yaml": `
releases:
- name: foo
  chart: mychart1
  namespace: ns1
  tillerNamespace: tns1
  needs:
  - tns2/bar
- name: bar
  chart: mychart2
  namespace: ns2
  tillerNamespace: tns2
`,
			},
			detailedExitcode: true,
			error:            "Identified at least one change",
			diffs: map[exectest.DiffKey]error{
				{Name: "bar", Chart: "mychart2", Flags: "--tiller-namespacetns2--namespacens2--detailed-exitcode--reset-values"}: helmexec.ExitError{Code: 2},
				{Name: "foo", Chart: "mychart1", Flags: "--tiller-namespacetns1--namespacens1--detailed-exitcode--reset-values"}: helmexec.ExitError{Code: 2},
			},
			upgraded: []exectest.Release{},
		},
		{
			name: "helm2 upgrade when tns2 bar needs tns1 foo",
			loc:  location(),
			files: map[string]string{
				"/path/to/helmfile.yaml": `
releases:
- name: bar
  chart: mychart2
  namespace: ns2
  tillerNamespace: tns2
  needs:
  - tns1/foo
- name: foo
  chart: mychart1
  namespace: ns1
  tillerNamespace: tns1
`,
			},
			detailedExitcode: true,
			error:            "Identified at least one change",
			diffs: map[exectest.DiffKey]error{
				{Name: "bar", Chart: "mychart2", Flags: "--tiller-namespacetns2--namespacens2--detailed-exitcode--reset-values"}: helmexec.ExitError{Code: 2},
				{Name: "foo", Chart: "mychart1", Flags: "--tiller-namespacetns1--namespacens1--detailed-exitcode--reset-values"}: helmexec.ExitError{Code: 2},
			},
			upgraded: []exectest.Release{},
			// as we check for log output, set concurrency to 1 to avoid non-deterministic test result
			concurrency: 1,
		},
		{
			name: "helm3 upgrade when ns2 bar needs ns1 foo",
			loc:  location(),
			files: map[string]string{
				"/path/to/helmfile.yaml": `
releases:
- name: bar
  chart: mychart2
  namespace: ns2
  needs:
  - ns1/foo
- name: foo
  chart: mychart1
  namespace: ns1
`,
			},
			detailedExitcode: true,
			error:            "Identified at least one change",
			diffs: map[exectest.DiffKey]error{
				{Name: "bar", Chart: "mychart2", Flags: "--namespacens2--detailed-exitcode--reset-values"}: helmexec.ExitError{Code: 2},
				{Name: "foo", Chart: "mychart1", Flags: "--namespacens1--detailed-exitcode--reset-values"}: helmexec.ExitError{Code: 2},
			},
			upgraded: []exectest.Release{},
			// as we check for log output, set concurrency to 1 to avoid non-deterministic test result
			concurrency: 1,
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
  chart: mychart2
  installed: false
- name: foo
  chart: mychart1
  installed: false
  needs:
  - bar
`,
			},
			detailedExitcode: true,
			error:            "Identified at least one change",
			diffs: map[exectest.DiffKey]error{
				{Name: "bar", Chart: "mychart2", Flags: "--detailed-exitcode--reset-values"}: helmexec.ExitError{Code: 2},
				{Name: "foo", Chart: "mychart1", Flags: "--detailed-exitcode--reset-values"}: helmexec.ExitError{Code: 2},
			},
			lists: map[exectest.ListKey]string{
				{Filter: "^foo$", Flags: helmV2ListFlagsWithoutKubeContext}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
foo 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	mychart1-3.1.0	3.1.0      	default
`,
				{Filter: "^bar$", Flags: helmV2ListFlagsWithoutKubeContext}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
bar 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	mychart2-3.1.0	3.1.0      	default
`,
			},
			deleted: []exectest.Release{},
		},
		{
			name: "delete foo and bar when bar needs foo",
			loc:  location(),
			files: map[string]string{
				"/path/to/helmfile.yaml": `
releases:
- name: bar
  chart: mychart2
  installed: false
  needs:
  - foo
- name: foo
  chart: mychart1
  installed: false
`,
			},
			detailedExitcode: true,
			error:            "Identified at least one change",
			diffs: map[exectest.DiffKey]error{
				{Name: "bar", Chart: "mychart2", Flags: "--detailed-exitcode--reset-values"}: helmexec.ExitError{Code: 2},
				{Name: "foo", Chart: "mychart1", Flags: "--detailed-exitcode--reset-values"}: helmexec.ExitError{Code: 2},
			},
			lists: map[exectest.ListKey]string{
				{Filter: "^foo$", Flags: helmV2ListFlagsWithoutKubeContext}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
foo 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	mychart1-3.1.0	3.1.0      	default
`,
				{Filter: "^bar$", Flags: helmV2ListFlagsWithoutKubeContext}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
bar 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	mychart2-3.1.0	3.1.0      	default
`,
			},
			deleted: []exectest.Release{},
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
  chart: mychart2
- name: foo
  chart: mychart1
  installed: false
  needs:
  - bar
`,
			},
			detailedExitcode: true,
			error:            "Identified at least one change",
			diffs: map[exectest.DiffKey]error{
				{Name: "bar", Chart: "mychart2", Flags: "--detailed-exitcode--reset-values"}: helmexec.ExitError{Code: 2},
				{Name: "foo", Chart: "mychart1", Flags: "--detailed-exitcode--reset-values"}: helmexec.ExitError{Code: 2},
			},
			lists: map[exectest.ListKey]string{
				{Filter: "^foo$", Flags: helmV2ListFlagsWithoutKubeContext}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
foo 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	mychart1-3.1.0	3.1.0      	default
`,
				{Filter: "^bar$", Flags: helmV2ListFlagsWithoutKubeContext}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
bar 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	mychart2-3.1.0	3.1.0      	default
`,
			},
			upgraded: []exectest.Release{},
			deleted:  []exectest.Release{},
		},
		{
			name: "delete bar when foo needs bar",
			loc:  location(),
			files: map[string]string{
				"/path/to/helmfile.yaml": `
releases:
- name: bar
  chart: mychart2
  installed: false
- name: foo
  chart: mychart1
  needs:
  - bar
`,
			},
			detailedExitcode: true,
			error:            `in ./helmfile.yaml: release "foo" depends on "bar" which does not match the selectors. Please add a selector like "--selector name=bar", or indicate whether to skip (--skip-needs) or include (--include-needs) these dependencies`,
			diffs: map[exectest.DiffKey]error{
				{Name: "bar", Chart: "mychart2", Flags: "--detailed-exitcode--reset-values"}: helmexec.ExitError{Code: 2},
				{Name: "foo", Chart: "mychart1", Flags: "--detailed-exitcode--reset-values"}: helmexec.ExitError{Code: 2},
			},
			lists: map[exectest.ListKey]string{
				{Filter: "^foo$", Flags: helmV2ListFlagsWithoutKubeContext}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
foo 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	mychart1-3.1.0	3.1.0      	default
`,
				{Filter: "^bar$", Flags: helmV2ListFlagsWithoutKubeContext}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
bar 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	mychart2-3.1.0	3.1.0      	default
`,
			},
			upgraded: []exectest.Release{},
			deleted:  []exectest.Release{},
		},
		{
			name: "delete bar when foo needs bar with include-needs",
			loc:  location(),
			files: map[string]string{
				"/path/to/helmfile.yaml": `
releases:
- name: bar
  chart: mychart2
  installed: false
- name: foo
  chart: mychart1
  needs:
  - bar
`,
			},
			flags: flags{
				includeNeeds: true,
			},
			detailedExitcode: true,
			error:            "Identified at least one change",
			diffs: map[exectest.DiffKey]error{
				{Name: "bar", Chart: "mychart2", Flags: "--detailed-exitcode--reset-values"}: helmexec.ExitError{Code: 2},
				{Name: "foo", Chart: "mychart1", Flags: "--detailed-exitcode--reset-values"}: helmexec.ExitError{Code: 2},
			},
			lists: map[exectest.ListKey]string{
				{Filter: "^foo$", Flags: helmV2ListFlagsWithoutKubeContext}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
foo 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	mychart1-3.1.0	3.1.0      	default
`,
				{Filter: "^bar$", Flags: helmV2ListFlagsWithoutKubeContext}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
bar 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	mychart2-3.1.0	3.1.0      	default
`,
			},
			upgraded: []exectest.Release{},
			deleted:  []exectest.Release{},
		},
		{
			name: "delete bar when foo needs bar with skip-needs",
			loc:  location(),
			files: map[string]string{
				"/path/to/helmfile.yaml": `
releases:
- name: bar
  chart: mychart2
  installed: false
- name: foo
  chart: mychart1
  needs:
  - bar
`,
			},
			flags: flags{
				skipNeeds: true,
			},
			detailedExitcode: true,
			error:            "Identified at least one change",
			diffs: map[exectest.DiffKey]error{
				{Name: "bar", Chart: "mychart2", Flags: "--detailed-exitcode--reset-values"}: helmexec.ExitError{Code: 2},
				{Name: "foo", Chart: "mychart1", Flags: "--detailed-exitcode--reset-values"}: helmexec.ExitError{Code: 2},
			},
			lists: map[exectest.ListKey]string{
				{Filter: "^foo$", Flags: helmV2ListFlagsWithoutKubeContext}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
foo 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	mychart1-3.1.0	3.1.0      	default
`,
				{Filter: "^bar$", Flags: helmV2ListFlagsWithoutKubeContext}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
bar 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	mychart2-3.1.0	3.1.0      	default
`,
			},
			upgraded: []exectest.Release{},
			deleted:  []exectest.Release{},
		},
		{
			name: "delete foo when bar needs foo",
			loc:  location(),
			files: map[string]string{
				"/path/to/helmfile.yaml": `
releases:
- name: foo
  chart: mychart1
  installed: false
- name: bar
  chart: mychart2
  needs:
  - foo
`,
			},
			detailedExitcode: true,
			error:            `in ./helmfile.yaml: release "bar" depends on "foo" which does not match the selectors. Please add a selector like "--selector name=foo", or indicate whether to skip (--skip-needs) or include (--include-needs) these dependencies`,
			diffs: map[exectest.DiffKey]error{
				{Name: "bar", Chart: "mychart2", Flags: "--detailed-exitcode--reset-values"}: helmexec.ExitError{Code: 2},
				{Name: "foo", Chart: "mychart1", Flags: "--detailed-exitcode--reset-values"}: helmexec.ExitError{Code: 2},
			},
			lists: map[exectest.ListKey]string{
				{Filter: "^foo$", Flags: helmV2ListFlagsWithoutKubeContext}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
foo 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	mychart1-3.1.0	3.1.0      	default
`,
				{Filter: "^bar$", Flags: helmV2ListFlagsWithoutKubeContext}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
bar 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	mychart2-3.1.0	3.1.0      	default
`,
			},
			upgraded: []exectest.Release{},
			deleted:  []exectest.Release{},
		},
		{
			name: "delete bar when bar needs foo",
			loc:  location(),
			files: map[string]string{
				"/path/to/helmfile.yaml": `
releases:
- name: foo
  chart: mychart1
- name: bar
  chart: mychart2
  installed: false
  needs:
  - foo
`,
			},
			detailedExitcode: true,
			error:            "Identified at least one change",
			diffs: map[exectest.DiffKey]error{
				{Name: "bar", Chart: "mychart2", Flags: "--detailed-exitcode--reset-values"}: helmexec.ExitError{Code: 2},
				{Name: "foo", Chart: "mychart1", Flags: "--detailed-exitcode--reset-values"}: helmexec.ExitError{Code: 2},
			},
			lists: map[exectest.ListKey]string{
				{Filter: "^foo$", Flags: helmV2ListFlagsWithoutKubeContext}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
foo 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	mychart1-3.1.0	3.1.0      	default
`,
				{Filter: "^bar$", Flags: helmV2ListFlagsWithoutKubeContext}: `NAME	REVISION	UPDATED                 	STATUS  	CHART        	APP VERSION	NAMESPACE
bar 	4       	Fri Nov  1 08:40:07 2019	DEPLOYED	mychart2-3.1.0	3.1.0      	default
`,
			},
			upgraded: []exectest.Release{},
			deleted:  []exectest.Release{},
		},
		//
		// upgrades with selector
		//
		{
			// see https://github.com/roboll/helmfile/issues/919#issuecomment-549831747
			name:  "upgrades with good selector with --skip-needs=true",
			loc:   location(),
			flags: flags{skipNeeds: true},
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
			selectors:        []string{"app=test"},
			detailedExitcode: true,
			diffs: map[exectest.DiffKey]error{
				{Name: "external-secrets", Chart: "incubator/raw", Flags: "--namespacedefault--detailed-exitcode--reset-values"}: helmexec.ExitError{Code: 2},
				{Name: "my-release", Chart: "incubator/raw", Flags: "--namespacedefault--detailed-exitcode--reset-values"}:       helmexec.ExitError{Code: 2},
			},
			upgraded: []exectest.Release{},
			// as we check for log output, set concurrency to 1 to avoid non-deterministic test result
			concurrency: 1,
			error:       "Identified at least one change",
		},
		{
			name:  "upgrades with good selector with --skip-needs=false",
			loc:   location(),
			flags: flags{skipNeeds: false},
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
			selectors:        []string{"app=test"},
			detailedExitcode: true,
			diffs: map[exectest.DiffKey]error{
				{Name: "external-secrets", Chart: "incubator/raw", Flags: "--namespacedefault--detailed-exitcode--reset-values"}: helmexec.ExitError{Code: 2},
				{Name: "my-release", Chart: "incubator/raw", Flags: "--namespacedefault--detailed-exitcode--reset-values"}:       helmexec.ExitError{Code: 2},
			},
			upgraded: []exectest.Release{},
			// as we check for log output, set concurrency to 1 to avoid non-deterministic test result
			concurrency: 1,
			error:       `in ./helmfile.yaml: release "default/external-secrets" depends on "kube-system/kubernetes-external-secrets" which does not match the selectors. Please add a selector like "--selector name=kubernetes-external-secrets", or indicate whether to skip (--skip-needs) or include (--include-needs) these dependencies`,
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
2 release(s) matching app=test found in helmfile.yaml

err: release "default/external-secrets" depends on "kube-system/kubernetes-external-secrets" which does not match the selectors. Please add a selector like "--selector name=kubernetes-external-secrets", or indicate whether to skip (--skip-needs) or include (--include-needs) these dependencies
changing working directory back to "/path/to"
`,
		},
		{
			// see https://github.com/roboll/helmfile/issues/919#issuecomment-549831747
			name: "upgrades with bad selector",
			loc:  location(),
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
			selectors:        []string{"app=test_non_existent"},
			detailedExitcode: true,
			diffs:            map[exectest.DiffKey]error{},
			upgraded:         []exectest.Release{},
			error:            "err: no releases found that matches specified selector(app=test_non_existent) and environment(default), in any helmfile",
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
		},
		//
		// error cases
		//
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
  - bar
`,
			},
			detailedExitcode: true,
			diffs: map[exectest.DiffKey]error{
				{Name: "baz", Chart: "mychart3", Flags: "--namespacens1--detailed-exitcode--reset-values"}: helmexec.ExitError{Code: 2},
				{Name: "foo", Chart: "mychart1", Flags: "--detailed-exitcode--reset-values"}:               helmexec.ExitError{Code: 2},
			},
			lists:       map[exectest.ListKey]string{},
			upgraded:    []exectest.Release{},
			deleted:     []exectest.Release{},
			concurrency: 1,
			error:       `in ./helmfile.yaml: release(s) "foo" depend(s) on an undefined release "bar". Perhaps you made a typo in "needs" or forgot defining a release named "bar" with appropriate "namespace" and "kubeContext"?`,
			log: `processing file "helmfile.yaml" in directory "."
changing working directory to "/path/to"
first-pass rendering starting for "helmfile.yaml.part.0": inherited=&{default map[] map[]}, overrode=<nil>
first-pass uses: &{default map[] map[]}
first-pass rendering output of "helmfile.yaml.part.0":
 0: 
 1: releases:
 2: - name: baz
 3:   namespace: ns1
 4:   chart: mychart3
 5: - name: foo
 6:   chart: mychart1
 7:   needs:
 8:   - bar
 9: 

first-pass produced: &{default map[] map[]}
first-pass rendering result of "helmfile.yaml.part.0": {default map[] map[]}
vals:
map[]
defaultVals:[]
second-pass rendering result of "helmfile.yaml.part.0":
 0: 
 1: releases:
 2: - name: baz
 3:   namespace: ns1
 4:   chart: mychart3
 5: - name: foo
 6:   chart: mychart1
 7:   needs:
 8:   - bar
 9: 

merged environment: &{default map[] map[]}
2 release(s) found in helmfile.yaml

err: release(s) "foo" depend(s) on an undefined release "bar". Perhaps you made a typo in "needs" or forgot defining a release named "bar" with appropriate "namespace" and "kubeContext"?
changing working directory back to "/path/to"
`,
		},
	}

	for i := range testcases {
		tc := testcases[i]
		t.Run(tc.name, func(t *testing.T) {
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

				diffErr := app.Diff(diffConfig{
					// if we check log output, concurrency must be 1. otherwise the test becomes non-deterministic.
					concurrency:      tc.concurrency,
					logger:           logger,
					detailedExitcode: tc.detailedExitcode,
					skipNeeds:        tc.flags.skipNeeds,
					includeNeeds:     tc.flags.includeNeeds,
				})

				var diffErrStr string
				if diffErr != nil {
					diffErrStr = diffErr.Error()
				}

				if d := cmp.Diff(tc.error, diffErrStr); d != "" {
					t.Fatalf("invalid error: want (-), got (+): %s", d)
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
					t.Errorf("unexpected log for data defined %s:\nDIFF\n%s\nEOD", tc.loc, diff)
				}
			} else {
				testhelper.RequireLog(t, "app_diff_test_2", bs)
			}
		})
	}
}
