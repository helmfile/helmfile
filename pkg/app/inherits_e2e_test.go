package app

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/helmfile/helmfile/pkg/state"
	"github.com/helmfile/helmfile/pkg/testhelper"
)

// runSubhelmfileInheritsTest loads a parent + sub-helmfile via ForEachState and
// returns each visited state keyed by its FilePath.
func runSubhelmfileInheritsTest(t *testing.T, files map[string]string, app *App) map[string]*state.HelmState {
	t.Helper()
	fs := testhelper.NewTestFs(files)
	app = injectFs(app, fs)
	expectNoCallsToHelm(app)

	states := map[string]*state.HelmState{}
	noop := func(run *Run) (bool, []error) {
		states[run.state.FilePath] = run.state
		return false, []error{}
	}
	require.NoError(t, app.ForEachState(noop, false, SetFilter(true)))
	return states
}

// TestSubhelmfileInherits_Repositories is the regression test for issue #1495:
// a repository declared in the parent helmfile is available to the sub-helmfile
// when it opts in via `inherits: [repositories]`.
func TestSubhelmfileInherits_Repositories(t *testing.T) {
	files := map[string]string{
		"/path/to/helmfile.yaml": `
repositories:
- name: release-charts
  url: registry.example.com/release/helm-charts
  oci: true
helmfiles:
- path: myapp.yaml
  inherits:
  - repositories
`,
		"/path/to/myapp.yaml": `
releases:
- name: myapp
  chart: release-charts/myapp
  namespace: myns
`,
	}
	app := &App{
		OverrideHelmBinary:              DefaultHelmBinary,
		OverrideKubeContext:             "default",
		DisableKubeVersionAutoDetection: true,
		Env:                             "default",
		FileOrDir:                       "helmfile.yaml",
		Logger:                          newAppTestLogger(),
	}
	states := runSubhelmfileInheritsTest(t, files, app)

	child, ok := states["myapp.yaml"]
	require.True(t, ok, "sub-helmfile state should be visited")
	require.Len(t, child.Repositories, 1, "child should have inherited the parent repository")
	assert.Equal(t, "release-charts", child.Repositories[0].Name)
	assert.True(t, child.Repositories[0].OCI)
}

// TestSubhelmfile_NoInherit_DoesNotGetRepositories verifies the opt-in nature:
// without `inherits:`, the sub-helmfile does NOT receive the parent's repository
// (the historical behavior that #1495 reported), preserving backward compat.
func TestSubhelmfile_NoInherit_DoesNotGetRepositories(t *testing.T) {
	files := map[string]string{
		"/path/to/helmfile.yaml": `
repositories:
- name: release-charts
  url: registry.example.com/release/helm-charts
  oci: true
helmfiles:
- path: myapp.yaml
`,
		"/path/to/myapp.yaml": `
releases:
- name: myapp
  chart: release-charts/myapp
`,
	}
	app := &App{
		OverrideHelmBinary:              DefaultHelmBinary,
		OverrideKubeContext:             "default",
		DisableKubeVersionAutoDetection: true,
		Env:                             "default",
		FileOrDir:                       "helmfile.yaml",
		Logger:                          newAppTestLogger(),
	}
	states := runSubhelmfileInheritsTest(t, files, app)

	child, ok := states["myapp.yaml"]
	require.True(t, ok)
	assert.Empty(t, child.Repositories, "without inherits:, child must NOT receive parent repos (opt-in)")
}

// TestSubhelmfile_WarnsWhenRepoNotInherited verifies the footgun warning fires
// in the real load flow when a release references a repo the parent has but the
// child lacks (and did not inherit).
func TestSubhelmfile_WarnsWhenRepoNotInherited(t *testing.T) {
	files := map[string]string{
		"/path/to/helmfile.yaml": `
repositories:
- name: release-charts
  url: registry.example.com/release/helm-charts
  oci: true
helmfiles:
- path: myapp.yaml
`,
		"/path/to/myapp.yaml": `
releases:
- name: myapp
  chart: release-charts/myapp
`,
	}
	core, recorded := observer.New(zapcore.WarnLevel)
	app := &App{
		OverrideHelmBinary:              DefaultHelmBinary,
		OverrideKubeContext:             "default",
		DisableKubeVersionAutoDetection: true,
		Env:                             "default",
		FileOrDir:                       "helmfile.yaml",
		Logger:                          zap.New(core).Sugar(),
	}
	runSubhelmfileInheritsTest(t, files, app)

	var matched int
	for _, e := range recorded.All() {
		if msg := e.Message; strings.Contains(msg, "release-charts") && strings.Contains(msg, "inherits") {
			matched++
		}
	}
	assert.GreaterOrEqual(t, matched, 1, "expected a footgun warning mentioning release-charts and inherits")
}

// TestSubhelmfile_NoWarnWhenRepoInherited verifies the warning is naturally
// suppressed when the repo is inherited (it becomes part of the child's repos).
func TestSubhelmfile_NoWarnWhenRepoInherited(t *testing.T) {
	files := map[string]string{
		"/path/to/helmfile.yaml": `
repositories:
- name: release-charts
  url: registry.example.com/release/helm-charts
  oci: true
helmfiles:
- path: myapp.yaml
  inherits:
  - repositories
`,
		"/path/to/myapp.yaml": `
releases:
- name: myapp
  chart: release-charts/myapp
`,
	}
	core, recorded := observer.New(zapcore.WarnLevel)
	app := &App{
		OverrideHelmBinary:              DefaultHelmBinary,
		OverrideKubeContext:             "default",
		DisableKubeVersionAutoDetection: true,
		Env:                             "default",
		FileOrDir:                       "helmfile.yaml",
		Logger:                          zap.New(core).Sugar(),
	}
	runSubhelmfileInheritsTest(t, files, app)

	for _, e := range recorded.All() {
		assert.False(t, strings.Contains(e.Message, "not inherited"),
			"no footgun warning expected when repositories are inherited, got: %s", e.Message)
	}
}

// TestSubhelmfileInherits_Environments verifies that a sub-helmfile opting into
// `inherits: [environments]` receives the parent's resolved environment values
// (FOO from env.yaml flows down).
func TestSubhelmfileInherits_Environments(t *testing.T) {
	files := map[string]string{
		"/path/to/helmfile.yaml": `
environments:
  default:
    values:
    - env.yaml
helmfiles:
- path: myapp.yaml
  inherits:
  - environments
`,
		"/path/to/env.yaml": `FOO: from-parent
`,
		"/path/to/myapp.yaml": `
releases:
- name: myapp
  chart: release-charts/myapp
`,
	}
	app := &App{
		OverrideHelmBinary:              DefaultHelmBinary,
		OverrideKubeContext:             "default",
		DisableKubeVersionAutoDetection: true,
		Env:                             "default",
		FileOrDir:                       "helmfile.yaml",
		Logger:                          newAppTestLogger(),
	}
	states := runSubhelmfileInheritsTest(t, files, app)

	child, ok := states["myapp.yaml"]
	require.True(t, ok, "sub-helmfile state should be visited")
	require.NotNil(t, child.RenderedValues, "child should have rendered values")
	assert.Equal(t, "from-parent", child.RenderedValues["FOO"],
		"inherited environments should make the parent's resolved FOO value visible to the child")
}
