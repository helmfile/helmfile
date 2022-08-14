package argparser

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/helmfile/helmfile/pkg/state"
)

// TestGetArgs tests the GetArgs function
func TestGetArgs(t *testing.T) {
	tests := []struct {
		args     string
		expected string
	}{
		{
			args:     "--timeout=3600 --set app1.bootstrap=true --set app2.bootstrap=false --tiller-namespace ns",
			expected: "--timeout=3600 --set app1.bootstrap=true --set app2.bootstrap=false --tiller-namespace ns --recreate-pods --force",
		},
		{
			args:     "--timeout=3600 --set app1.bootstrap=true --set app2.bootstrap=false,app3.bootstrap=true --tiller-namespace ns",
			expected: "--timeout=3600 --set app1.bootstrap=true --set app2.bootstrap=false,app3.bootstrap=true --tiller-namespace ns --recreate-pods --force",
		},
	}
	for _, test := range tests {
		defaultArgs := []string{"--recreate-pods", "--force"}
		Helmdefaults := state.HelmSpec{KubeContext: "test", TillerNamespace: "test-namespace", Args: defaultArgs}
		testState := &state.HelmState{
			ReleaseSetSpec: state.ReleaseSetSpec{
				HelmDefaults: Helmdefaults,
			},
		}
		receivedArgs := GetArgs(test.args, testState)

		require.Equalf(t, test.expected, strings.Join(receivedArgs, " "), "expected args %s, received args %s", test.expected, strings.Join(receivedArgs, " "))
	}
}

// TestSetArg tests the SetArg function
func TestSetArg(t *testing.T) {
	ap := newArgMap()

	tests := []struct {
		// check if changes have been made to the map
		change  bool
		flag    string
		arg     string
		isSpace bool
	}{
		{
			flag:    "--set",
			arg:     "app1.bootstrap=true",
			isSpace: false,
			change:  true,
		},
		{
			flag:    "--timeout",
			arg:     "3600",
			isSpace: false,
			change:  true,
		},
		{
			flag:    "--force",
			arg:     "",
			isSpace: false,
			change:  true,
		},
		{
			flag:    "",
			arg:     "",
			isSpace: false,
			change:  false,
		},
	}

	for _, test := range tests {
		ap.SetArg(test.flag, test.arg, test.isSpace)
		if test.change {
			require.Containsf(t, ap.flags, test.flag, "expected flag %s to be set", test.flag)
			require.Containsf(t, ap.m, test.flag, "expected m %s to be set", test.flag)
			kv := &keyVal{key: test.flag, val: test.arg, spaceFlag: test.isSpace}
			require.Containsf(t, ap.m[test.flag], kv, "expected %v in m[%s]", kv, test.flag)
		} else {
			require.NotContainsf(t, ap.flags, test.flag, "expected flag %s to be not set", test.flag)
			require.NotContainsf(t, ap.m, test.flag, "expected m %s to be not set", test.flag)
		}
	}
}
