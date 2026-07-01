package state

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/helmfile/helmfile/pkg/environment"
)

func TestBuildInheritedConfig_OnlyRequestedFields(t *testing.T) {
	st := &HelmState{ReleaseSetSpec: ReleaseSetSpec{
		Repositories: []RepositorySpec{{Name: "a"}, {Name: "b"}},
		HelmDefaults: HelmSpec{Timeout: 300, Atomic: true},
		CommonLabels: map[string]string{"team": "platform"},
		ApiVersions:  []string{"v1"},
		KubeVersion:  "1.30.0",
		Templates:    map[string]TemplateSpec{"t": {}},
		Env:          environment.Environment{Name: "prod", Values: map[string]any{"k": "v"}},
	}}

	t.Run("repositories only", func(t *testing.T) {
		in, err := st.BuildInheritedConfig([]string{"repositories"})
		require.NoError(t, err)
		assert.Equal(t, []RepositorySpec{{Name: "a"}, {Name: "b"}}, in.Repositories)
		assert.Nil(t, in.HelmDefaults)
		assert.Nil(t, in.CommonLabels)
		assert.Nil(t, in.Env)
	})

	t.Run("helmDefaults becomes pointer", func(t *testing.T) {
		in, err := st.BuildInheritedConfig([]string{"helmDefaults"})
		require.NoError(t, err)
		require.NotNil(t, in.HelmDefaults)
		assert.Equal(t, 300, in.HelmDefaults.Timeout)
		assert.True(t, in.HelmDefaults.Atomic)
		assert.Nil(t, in.Repositories)
	})

	t.Run("environments deep-copies env", func(t *testing.T) {
		in, err := st.BuildInheritedConfig([]string{"environments"})
		require.NoError(t, err)
		require.NotNil(t, in.Env)
		assert.Equal(t, "prod", in.Env.Name)
		// mutating the copy must not affect the parent
		in.Env.Values["k"] = "mutated"
		assert.Equal(t, "v", st.Env.Values["k"])
	})

	t.Run("nothing requested yields empty config", func(t *testing.T) {
		in, err := st.BuildInheritedConfig(nil)
		require.NoError(t, err)
		require.NotNil(t, in)
		assert.Nil(t, in.Repositories)
		assert.Nil(t, in.HelmDefaults)
		assert.Nil(t, in.Env)
	})
}

// TestBuildInheritedConfig_PureFieldsAreDeepCopied verifies the returned config
// does not alias the parent's slices/maps — mutating the copy must not affect
// the parent state. This guards against the cross-state coupling noted in review.
func TestBuildInheritedConfig_PureFieldsAreDeepCopied(t *testing.T) {
	st := &HelmState{ReleaseSetSpec: ReleaseSetSpec{
		Repositories: []RepositorySpec{{Name: "a"}, {Name: "b"}},
		HelmDefaults: HelmSpec{Timeout: 300, Args: []string{"--parent-arg"}},
		CommonLabels: map[string]string{"team": "platform"},
		ApiVersions:  []string{"v1"},
		Templates:    map[string]TemplateSpec{"base": {ReleaseSpec: ReleaseSpec{Namespace: "parent-ns"}}},
	}}
	in, err := st.BuildInheritedConfig([]string{
		"repositories", "helmDefaults", "commonLabels", "apiVersions", "templates",
	})
	require.NoError(t, err)

	// Mutate every reference field on the copy.
	in.Repositories = append(in.Repositories, RepositorySpec{Name: "c"})
	in.Repositories[0].Name = "mutated"
	in.HelmDefaults.Args[0] = "--mutated"
	in.CommonLabels["team"] = "mutated"
	in.ApiVersions[0] = "mutated"
	tb := in.Templates["base"]
	tb.Namespace = "mutated"
	in.Templates["base"] = tb

	// The parent must be unaffected.
	assert.Equal(t, []RepositorySpec{{Name: "a"}, {Name: "b"}}, st.Repositories)
	assert.Equal(t, []string{"--parent-arg"}, st.HelmDefaults.Args)
	assert.Equal(t, "platform", st.CommonLabels["team"])
	assert.Equal(t, []string{"v1"}, st.ApiVersions)
	assert.Equal(t, "parent-ns", st.Templates["base"].Namespace)
}

func TestMergeInherited_NilIsNoop(t *testing.T) {
	st := &HelmState{ReleaseSetSpec: ReleaseSetSpec{Repositories: []RepositorySpec{{Name: "a"}}}}
	require.NoError(t, st.MergeInherited(nil))
	assert.Equal(t, []RepositorySpec{{Name: "a"}}, st.Repositories)
}

func TestMergeInherited_RepositoriesAppendsAndDedupsChildWins(t *testing.T) {
	parent := &HelmState{ReleaseSetSpec: ReleaseSetSpec{
		Repositories: []RepositorySpec{{Name: "shared", URL: "parent-url"}, {Name: "only-parent"}},
	}}
	in, err := parent.BuildInheritedConfig([]string{"repositories"})
	require.NoError(t, err)

	child := &HelmState{ReleaseSetSpec: ReleaseSetSpec{
		Repositories: []RepositorySpec{{Name: "shared", URL: "child-url"}, {Name: "only-child"}},
	}}
	require.NoError(t, child.MergeInherited(in))

	names := repoNames(child.Repositories)
	assert.ElementsMatch(t, []string{"shared", "only-parent", "only-child"}, names)
	// child's "shared" wins over parent's
	for _, r := range child.Repositories {
		if r.Name == "shared" {
			assert.Equal(t, "child-url", r.URL)
		}
	}
}

func TestMergeInherited_HelmDefaultsParentFillsChildGaps(t *testing.T) {
	parent := &HelmState{ReleaseSetSpec: ReleaseSetSpec{
		HelmDefaults: HelmSpec{Timeout: 300, Atomic: true},
	}}
	in, err := parent.BuildInheritedConfig([]string{"helmDefaults"})
	require.NoError(t, err)

	t.Run("child omits helmDefaults entirely", func(t *testing.T) {
		child := &HelmState{}
		require.NoError(t, child.MergeInherited(in))
		assert.Equal(t, 300, child.HelmDefaults.Timeout)
		assert.True(t, child.HelmDefaults.Atomic)
	})

	t.Run("child sets a non-zero field, parent fills the rest", func(t *testing.T) {
		child := &HelmState{ReleaseSetSpec: ReleaseSetSpec{HelmDefaults: HelmSpec{Wait: true}}}
		require.NoError(t, child.MergeInherited(in))
		assert.Equal(t, 300, child.HelmDefaults.Timeout, "parent fills child gap")
		assert.True(t, child.HelmDefaults.Atomic, "parent fills child gap")
		assert.True(t, child.HelmDefaults.Wait, "child non-zero field wins")
	})
}

func TestMergeInherited_CommonLabelsUnionChildWins(t *testing.T) {
	parent := &HelmState{ReleaseSetSpec: ReleaseSetSpec{
		CommonLabels: map[string]string{"team": "platform", "shared": "parent"},
	}}
	in, err := parent.BuildInheritedConfig([]string{"commonLabels"})
	require.NoError(t, err)

	child := &HelmState{ReleaseSetSpec: ReleaseSetSpec{
		CommonLabels: map[string]string{"shared": "child", "local": "c"},
	}}
	require.NoError(t, child.MergeInherited(in))

	assert.Equal(t, "platform", child.CommonLabels["team"], "parent-only key added")
	assert.Equal(t, "child", child.CommonLabels["shared"], "child wins on conflict")
	assert.Equal(t, "c", child.CommonLabels["local"], "child-only key kept")
}

func TestMergeInherited_TemplatesUnionChildWins(t *testing.T) {
	parent := &HelmState{ReleaseSetSpec: ReleaseSetSpec{
		Templates: map[string]TemplateSpec{"base": {ReleaseSpec: ReleaseSpec{Namespace: "a"}}, "shared": {ReleaseSpec: ReleaseSpec{Namespace: "p"}}},
	}}
	in, err := parent.BuildInheritedConfig([]string{"templates"})
	require.NoError(t, err)

	child := &HelmState{ReleaseSetSpec: ReleaseSetSpec{
		Templates: map[string]TemplateSpec{"shared": {ReleaseSpec: ReleaseSpec{Namespace: "c"}}, "local": {ReleaseSpec: ReleaseSpec{Namespace: "x"}}},
	}}
	require.NoError(t, child.MergeInherited(in))

	assert.Contains(t, child.Templates, "base", "parent-only template added")
	assert.Contains(t, child.Templates, "local", "child-only template kept")
	assert.Equal(t, "c", child.Templates["shared"].Namespace, "child wins on conflict")
}

func TestMergeInherited_ApiVersionsAppendsAndDedups(t *testing.T) {
	parent := &HelmState{ReleaseSetSpec: ReleaseSetSpec{ApiVersions: []string{"v1", "v2"}}}
	in, err := parent.BuildInheritedConfig([]string{"apiVersions"})
	require.NoError(t, err)

	child := &HelmState{ReleaseSetSpec: ReleaseSetSpec{ApiVersions: []string{"v2", "v3"}}}
	require.NoError(t, child.MergeInherited(in))

	assert.Equal(t, []string{"v1", "v2", "v3"}, child.ApiVersions)
}

func TestMergeInherited_KubeVersionChildWinsParentFillsGap(t *testing.T) {
	t.Run("child empty inherits parent", func(t *testing.T) {
		parent := &HelmState{ReleaseSetSpec: ReleaseSetSpec{KubeVersion: "1.30.0"}}
		in, err := parent.BuildInheritedConfig([]string{"kubeVersion"})
		require.NoError(t, err)
		child := &HelmState{}
		require.NoError(t, child.MergeInherited(in))
		assert.Equal(t, "1.30.0", child.KubeVersion)
	})
	t.Run("child set keeps its own", func(t *testing.T) {
		parent := &HelmState{ReleaseSetSpec: ReleaseSetSpec{KubeVersion: "1.30.0"}}
		in, err := parent.BuildInheritedConfig([]string{"kubeVersion"})
		require.NoError(t, err)
		child := &HelmState{ReleaseSetSpec: ReleaseSetSpec{KubeVersion: "1.29.0"}}
		require.NoError(t, child.MergeInherited(in))
		assert.Equal(t, "1.29.0", child.KubeVersion)
	})
}

func newObservedLogger() (*zap.SugaredLogger, *observer.ObservedLogs) {
	core, recorded := observer.New(zapcore.WarnLevel)
	return zap.New(core).Sugar(), recorded
}

func TestWarnUninheritedRepos_WarnsWhenParentHasRepoChildLacks(t *testing.T) {
	child := &HelmState{ReleaseSetSpec: ReleaseSetSpec{
		Releases: []ReleaseSpec{{Name: "myapp", Chart: "release-charts/myapp"}},
	}}
	logger, recorded := newObservedLogger()

	child.WarnUninheritedRepos([]string{"release-charts"}, logger)

	require.Len(t, recorded.All(), 1, "expected one warning")
	assert.Contains(t, recorded.All()[0].Message, "release-charts")
	assert.Contains(t, recorded.All()[0].Message, "inherits")
}

func TestWarnUninheritedRepos_NoWarnWhenRepoInherited(t *testing.T) {
	// child has the repo (e.g. because it was inherited and merged) -> no warn
	child := &HelmState{ReleaseSetSpec: ReleaseSetSpec{
		Repositories: []RepositorySpec{{Name: "release-charts"}},
		Releases:     []ReleaseSpec{{Name: "myapp", Chart: "release-charts/myapp"}},
	}}
	logger, recorded := newObservedLogger()

	child.WarnUninheritedRepos([]string{"release-charts"}, logger)

	assert.Empty(t, recorded.All())
}

func TestWarnUninheritedRepos_NoWarnForRepoNotInParent(t *testing.T) {
	// repo absent from both -> helm will error separately, no inherit hint
	child := &HelmState{ReleaseSetSpec: ReleaseSetSpec{
		Releases: []ReleaseSpec{{Name: "myapp", Chart: "other/myapp"}},
	}}
	logger, recorded := newObservedLogger()

	child.WarnUninheritedRepos([]string{"release-charts"}, logger)

	assert.Empty(t, recorded.All())
}

func TestWarnUninheritedRepos_IgnoresLocalAndBareCharts(t *testing.T) {
	child := &HelmState{ReleaseSetSpec: ReleaseSetSpec{
		Releases: []ReleaseSpec{
			{Name: "a", Chart: "./local/chart"},
			{Name: "b", Chart: "mychart"},
			{Name: "c", Chart: "oci://registry/chart"},
			{Name: "d", Chart: "https://host/charts/x"},
			{Name: "e", Chart: "../sibling/y"},
		},
	}}
	logger, recorded := newObservedLogger()

	child.WarnUninheritedRepos([]string{"release-charts"}, logger)

	assert.Empty(t, recorded.All(), "local paths, bare names, oci://, https:// and ../ must not trigger")
}

func TestWarnUninheritedRepos_WarnsOncePerRepo(t *testing.T) {
	child := &HelmState{ReleaseSetSpec: ReleaseSetSpec{
		Releases: []ReleaseSpec{
			{Name: "a", Chart: "shared/x"},
			{Name: "b", Chart: "shared/y"},
		},
	}}
	logger, recorded := newObservedLogger()

	child.WarnUninheritedRepos([]string{"shared"}, logger)

	assert.Len(t, recorded.All(), 1, "dedup by repo name")
}

func TestWarnUninheritedRepos_NilLoggerAndEmptyInputsAreSafe(t *testing.T) {
	child := &HelmState{ReleaseSetSpec: ReleaseSetSpec{
		Releases: []ReleaseSpec{{Name: "a", Chart: "x/y"}},
	}}
	assert.NotPanics(t, func() { child.WarnUninheritedRepos(nil, nil) })
	assert.NotPanics(t, func() { child.WarnUninheritedRepos(nil, zap.NewNop().Sugar()) })
	assert.NotPanics(t, func() { child.WarnUninheritedRepos([]string{"x"}, zap.NewNop().Sugar()) })
}

func repoNames(repos []RepositorySpec) []string {
	out := make([]string, 0, len(repos))
	for _, r := range repos {
		out = append(out, r.Name)
	}
	return out
}
