package state

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/helmfile/helmfile/pkg/environment"
	"github.com/helmfile/helmfile/pkg/remote"
	"github.com/helmfile/helmfile/pkg/testhelper"
)

// loadSubHelmfileWithInheritedEnv loads childYaml the way the app package
// loads a sub-helmfile that declared `inherits: [environments]`: ctxEnv plays
// the role of the parent's already-resolved environment, which
// desiredStateLoader.Load passes down as the env values context (see
// state.BuildInheritedConfig and app.processNestedHelmfiles).
func loadSubHelmfileWithInheritedEnv(t *testing.T, childYaml string, ctxEnv *environment.Environment) *HelmState {
	t.Helper()

	logger := zap.NewNop().Sugar()
	testFs := testhelper.NewTestFs(map[string]string{})
	if testFs.Cwd == "" {
		testFs.Cwd = "/"
	}
	r := remote.NewRemote(logger, testFs.Cwd, testFs.ToFileSystem())
	st, err := NewCreator(logger, testFs.ToFileSystem(), nil, nil, "", "", r, false, "").
		ParseAndLoad([]byte(childYaml), "/", "child.yaml", ctxEnv.Name, true, true, true, ctxEnv, nil)
	require.NoError(t, err)

	return st
}

// newParentEnv returns a resolved parent environment for `inherits:
// [environments]` tests; kubeContext may be empty to model a parent that does
// not set one.
func newParentEnv(kubeContext string) *environment.Environment {
	return &environment.Environment{
		Name:         "prod",
		KubeContext:  kubeContext,
		Values:       map[string]any{},
		Defaults:     map[string]any{},
		CLIOverrides: map[string]any{},
	}
}

// TestSubHelmfileInheritsParentEnvKubeContext verifies issue #2847: a
// sub-helmfile with `inherits: [environments]` applies the parent
// environment's kubeContext to its releases, whether or not the child declares
// the environment itself, and regardless of its own helmDefaults.
func TestSubHelmfileInheritsParentEnvKubeContext(t *testing.T) {
	tests := []struct {
		name      string
		childYaml string
	}{
		{
			name: "child does not declare the environment at all",
			childYaml: `
releases:
- name: child-release
  chart: mychart
`,
		},
		{
			name: "child declares the environment without a kubeContext",
			childYaml: `
environments:
  prod: {}
releases:
- name: child-release
  chart: mychart
`,
		},
		{
			name: "inherited environment kubeContext outranks the child helmDefaults",
			childYaml: `
helmDefaults:
  kubeContext: defaults-ctx
releases:
- name: child-release
  chart: mychart
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := loadSubHelmfileWithInheritedEnv(t, tt.childYaml, newParentEnv("parent-ctx"))
			require.Len(t, st.Releases, 1)
			require.Equal(t, "parent-ctx", st.getKubeContext(&st.Releases[0]))
		})
	}
}

// TestSubHelmfileOwnEnvKubeContextWinsOverInherited verifies the precedence
// decided for issue #2847: when the child's own environment explicitly sets a
// kubeContext, it wins over the one inherited from the parent environment. The
// resolved environment must carry the child's value too, because transitive
// `inherits: [environments]` propagates it to grandchild helmfiles.
func TestSubHelmfileOwnEnvKubeContextWinsOverInherited(t *testing.T) {
	childYaml := `
environments:
  prod:
    kubeContext: child-ctx
releases:
- name: child-release
  chart: mychart
`
	st := loadSubHelmfileWithInheritedEnv(t, childYaml, newParentEnv("parent-ctx"))
	require.Len(t, st.Releases, 1)
	require.Equal(t, "child-ctx", st.getKubeContext(&st.Releases[0]))
	require.Equal(t, "child-ctx", st.Env.KubeContext)
}

// TestSubHelmfileNoEnvKubeContextFallsBackToHelmDefaults guards against a
// regression of the existing precedence when neither the child's environment
// nor the inherited parent environment sets a kubeContext: helmDefaults keeps
// filling the gap (issue #2847).
func TestSubHelmfileNoEnvKubeContextFallsBackToHelmDefaults(t *testing.T) {
	childYaml := `
helmDefaults:
  kubeContext: defaults-ctx
releases:
- name: child-release
  chart: mychart
`
	st := loadSubHelmfileWithInheritedEnv(t, childYaml, newParentEnv(""))
	require.Len(t, st.Releases, 1)
	require.Equal(t, "defaults-ctx", st.getKubeContext(&st.Releases[0]))
}
