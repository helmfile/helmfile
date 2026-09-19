package state

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/helmfile/helmfile/pkg/yaml"
)

func TestReleaseSpec_ContinueOnErrorYAML(t *testing.T) {
	var st HelmState
	require.NoError(t, yaml.Unmarshal([]byte(`
releases:
- name: absent
  chart: foo
- name: explicit-false
  chart: foo
  continueOnError: false
- name: explicit-true
  chart: foo
  continueOnError: true
`), &st))
	require.Len(t, st.Releases, 3)
	require.Nil(t, st.Releases[0].ContinueOnError)
	require.NotNil(t, st.Releases[1].ContinueOnError)
	require.False(t, *st.Releases[1].ContinueOnError)
	require.NotNil(t, st.Releases[2].ContinueOnError)
	require.True(t, *st.Releases[2].ContinueOnError)
}
