package app

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/helmfile/helmfile/pkg/exectest"
	"github.com/helmfile/helmfile/pkg/helmexec"
	"github.com/helmfile/helmfile/pkg/state"
)

func boolPtr(v bool) *bool {
	return &v
}

func releaseBatch(name string, needs []string, continueOnError *bool) state.Release {
	return state.Release{ReleaseSpec: state.ReleaseSpec{Name: name, Needs: needs, ContinueOnError: continueOnError}}
}

func fakeConverge(failures map[string]bool, executed *[]string) func(*state.HelmState, helmexec.Interface) (bool, []error) {
	return func(st *state.HelmState, _ helmexec.Interface) (bool, []error) {
		var errs []error
		for i := range st.Releases {
			release := &st.Releases[i]
			*executed = append(*executed, release.Name)
			if failures[release.Name] {
				err := fmt.Errorf("failed processing release %s: boom", release.Name)
				errs = append(errs, state.NewReleaseError(release, err, state.ReleaseErrorCodeFailure))
			}
		}

		return len(st.Releases) > 0, errs
	}
}

func TestWithBatches_FailFastByDefault(t *testing.T) {
	templated := &state.HelmState{}
	var executed []string

	batches := [][]state.Release{
		{
			releaseBatch("database", nil, nil),
			releaseBatch("cache", nil, nil),
		},
		{
			releaseBatch("worker", nil, nil),
		},
	}

	any, errs := withBatches("processing", templated, batches, &exectest.Helm{}, nil, fakeConverge(map[string]bool{"database": true}, &executed))

	require.False(t, any)
	require.Len(t, errs, 1)
	require.Equal(t, []string{"database", "cache"}, executed)
	require.Equal(t, "failed processing release database: boom", errs[0].Error())
}

func TestWithBatches_ContinueOnErrorKeepsIndependentBranches(t *testing.T) {
	templated := &state.HelmState{}
	var executed []string

	batches := [][]state.Release{
		{
			releaseBatch("database", nil, boolPtr(true)),
			releaseBatch("cache", nil, nil),
		},
		{
			releaseBatch("worker", nil, nil),
		},
	}

	any, errs := withBatches("processing", templated, batches, &exectest.Helm{}, nil, fakeConverge(map[string]bool{"database": true}, &executed))

	require.True(t, any)
	require.Len(t, errs, 1)
	require.Equal(t, []string{"database", "cache", "worker"}, executed)
	require.Equal(t, "failed processing release database: boom", errs[0].Error())
}

func TestWithBatches_BlocksDependentReleasesAndAggregatesErrors(t *testing.T) {
	templated := &state.HelmState{}
	var executed []string

	batches := [][]state.Release{
		{
			releaseBatch("database", nil, boolPtr(true)),
		},
		{
			releaseBatch("backend", []string{"database"}, boolPtr(true)),
			releaseBatch("worker", nil, nil),
		},
		{
			releaseBatch("frontend", []string{"backend"}, boolPtr(true)),
		},
	}

	any, errs := withBatches("processing", templated, batches, &exectest.Helm{}, nil, fakeConverge(map[string]bool{"database": true}, &executed))

	require.True(t, any)
	require.Len(t, errs, 3)
	require.Equal(t, []string{"database", "worker"}, executed)
	require.Equal(t, "failed processing release database: boom", errs[0].Error())
	require.Contains(t, errs[1].Error(), "release \"backend\" was skipped because dependency \"database\" failed")
	require.Contains(t, errs[2].Error(), "release \"frontend\" was skipped because dependency \"backend\" failed")
}

func TestWithBatches_AggregatesMultipleReleaseErrors(t *testing.T) {
	templated := &state.HelmState{}
	var executed []string

	batches := [][]state.Release{
		{
			releaseBatch("app-a", nil, boolPtr(true)),
			releaseBatch("app-b", nil, boolPtr(true)),
		},
		{
			releaseBatch("worker", nil, nil),
		},
	}

	any, errs := withBatches("processing", templated, batches, &exectest.Helm{}, nil, fakeConverge(map[string]bool{"app-a": true, "app-b": true}, &executed))

	require.True(t, any)
	require.Len(t, errs, 2)
	require.Equal(t, []string{"app-a", "app-b", "worker"}, executed)
	for _, releaseName := range []string{"app-a", "app-b"} {
		require.Contains(t, fmt.Sprint(errs), releaseName)
	}
}
