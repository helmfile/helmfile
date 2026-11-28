package app

import (
	"strings"
	"sync"
	"testing"

	"github.com/helmfile/vals"
	"go.uber.org/zap"

	"github.com/helmfile/helmfile/pkg/exectest"
	ffs "github.com/helmfile/helmfile/pkg/filesystem"
	"github.com/helmfile/helmfile/pkg/helmexec"
)

func TestLint_EnforceNeedsAreInstalled(t *testing.T) {
	run := func(t *testing.T, kubeContext, helmfile string) error {
		t.Helper()
		helm := &exectest.Helm{
			DiffMutex:     &sync.Mutex{},
			ChartsMutex:   &sync.Mutex{},
			ReleasesMutex: &sync.Mutex{},
		}
		var lintErr error
		runWithLogCapture(t, "debug", func(t *testing.T, logger *zap.SugaredLogger) {
			t.Helper()
			valsRuntime, err := vals.New(vals.Options{CacheSize: 32})
			if err != nil {
				t.Fatalf("unexpected error creating vals runtime: %v", err)
			}
			app := appWithFs(&App{
				OverrideHelmBinary:              DefaultHelmBinary,
				fs:                              ffs.DefaultFileSystem(),
				OverrideKubeContext:             kubeContext,
				DisableKubeVersionAutoDetection: true,
				Env:                             "default",
				Logger:                          logger,
				helms: map[helmKey]helmexec.Interface{
					createHelmKey("helm", kubeContext): helm,
				},
				valsRuntime: valsRuntime,
			}, map[string]string{"/path/to/helmfile.yaml": helmfile})

			lintErr = app.Lint(applyConfig{
				concurrency:              1,
				logger:                   logger,
				enforceNeedsAreInstalled: true,
				skipNeeds:                true,
			})
		})
		return lintErr
	}

	const want = "has a transitive dependency disabled marked as installed=false"

	cases := []struct {
		name        string
		kubeContext string
		helmfile    string
	}{
		{
			name: "no namespace and no kube context",
			helmfile: `
releases:
- name: disabled
  chart: incubator/raw
  installed: false
- name: app
  chart: incubator/raw
  needs:
  - disabled
`,
		},
		{
			name: "releases have a namespace",
			helmfile: `
releases:
- name: disabled
  chart: incubator/raw
  namespace: kube-system
  installed: false
- name: app
  chart: incubator/raw
  namespace: kube-system
  needs:
  - disabled
`,
		},
		{
			name: "need written as namespace/name",
			helmfile: `
releases:
- name: disabled
  chart: incubator/raw
  namespace: kube-system
  installed: false
- name: app
  chart: incubator/raw
  needs:
  - kube-system/disabled
`,
		},
		{
			name:        "kube context set",
			kubeContext: "default",
			helmfile: `
releases:
- name: disabled
  chart: incubator/raw
  installed: false
- name: app
  chart: incubator/raw
  needs:
  - disabled
`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := run(t, tc.kubeContext, tc.helmfile)
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Errorf("expected error containing %q, got: %v", want, err)
			}
		})
	}
}
