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
		args        string
		expected    string
		defaultArgs []string
	}{
		{
			args:        "-f a.yaml -f b.yaml -i --set app1.bootstrap=true --set app2.bootstrap=false",
			defaultArgs: []string{"--recreate-pods", "--force"},
			expected:    "-f a.yaml -f b.yaml -i --set app1.bootstrap=true --set app2.bootstrap=false --recreate-pods --force",
		},
		{
			args:        "-e  a.yaml   -d b.yaml   -i --set app1.bootstrap=true --set app2.bootstrap=false",
			defaultArgs: []string{"-q www", "-w"},
			expected:    "-e a.yaml -d b.yaml -i --set app1.bootstrap=true --set app2.bootstrap=false -q www -w",
		},
		{
			args:     "--timeout=3600 --set app1.bootstrap=true --set app2.bootstrap=false --tiller-namespace ns",
			expected: "--timeout=3600 --set app1.bootstrap=true --set app2.bootstrap=false --tiller-namespace ns",
		},
		{
			args:        "--timeout=3600 --set app1.bootstrap=true --set app2.bootstrap=false,app3.bootstrap=true --tiller-namespace ns",
			defaultArgs: []string{"--recreate-pods", "--force"},
			expected:    "--timeout=3600 --set app1.bootstrap=true --set app2.bootstrap=false,app3.bootstrap=true --tiller-namespace ns --recreate-pods --force",
		},
		{
			args:     "--post-renderer=aaa --post-renderer-args=bbb",
			expected: "--post-renderer=aaa --post-renderer-args=bbb",
		},
		{
			args:     "--post-renderer aaa --post-renderer-args bbb",
			expected: "--post-renderer aaa --post-renderer-args bbb",
		},
	}
	for _, test := range tests {
		Helmdefaults := state.HelmSpec{KubeContext: "test", TillerNamespace: "test-namespace", Args: test.defaultArgs}
		testState := &state.HelmState{
			ReleaseSetSpec: state.ReleaseSetSpec{
				HelmDefaults: Helmdefaults,
			},
		}
		receivedArgs := GetArgs(test.args, testState)

		require.Equalf(t, test.expected, strings.Join(receivedArgs, " "), "expected args %s, received args %s", test.expected, strings.Join(receivedArgs, " "))
	}
}

func TestGetArgs_PostRenderer(t *testing.T) {
	tests := []struct {
		postRenderer     string
		PostRendererArgs []string
		expected         string
	}{
		{
			postRenderer:     "sed",
			PostRendererArgs: []string{"-i", "s/aaa/bb/g"},
			expected:         "--post-renderer=\"sed\" --post-renderer-args=\"-i\" --post-renderer-args=\"s/aaa/bb/g\"",
		},

		{
			postRenderer:     "sed",
			PostRendererArgs: []string{"-i", "s/aa a/b b/g"},
			expected:         "--post-renderer=\"sed\" --post-renderer-args=\"-i\" --post-renderer-args=\"s/aa a/b b/g\"",
		},
	}

	for _, test := range tests {
		Helmdefaults := state.HelmSpec{KubeContext: "test", TillerNamespace: "test-namespace", PostRenderer: test.postRenderer, PostRendererArgs: test.PostRendererArgs}
		testState := &state.HelmState{
			ReleaseSetSpec: state.ReleaseSetSpec{
				HelmDefaults: Helmdefaults,
			},
		}
		receivedArgs := GetArgs("", testState)

		require.Equalf(t, test.expected, strings.Join(receivedArgs, " "), "expected args %s, received args %s", test.expected, strings.Join(receivedArgs, " "))
	}
}

// TestIsNewFlag tests the isNewFlag function
func TestIsNewFlag(t *testing.T) {
	tests := []struct {
		arg      string
		expected bool
	}{
		{
			arg:      "-f",
			expected: true,
		},
		{
			arg:      "--file",
			expected: true,
		},
		{
			arg:      "file",
			expected: false,
		},
	}

	for _, test := range tests {
		received := isNewFlag(test.arg)
		require.Equalf(t, test.expected, received, "isNewFlag expected %t, received %t", test.expected, received)
	}
}

// TestRemoveEmptyArgs tests the removeEmptyArgs function
func TestRemoveEmptyArgs(t *testing.T) {
	args := []string{"", "--set", "", "app1.bootstrap=true", "", "--set", "", "app2.bootstrap=true", "", "--timeout", "", "3600", "", "--force", "", ""}
	expectArgs := []string{"--set", "app1.bootstrap=true", "--set", "app2.bootstrap=true", "--timeout", "3600", "--force"}

	receivedArgs := removeEmptyArgs(args)
	require.Equalf(t, expectArgs, receivedArgs, "removeEmptyArgs expected %s, received %s", expectArgs, receivedArgs)
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
			flag:    "--set",
			arg:     "app2.bootstrap=true",
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

// TestAnalyzeArgs tests the analyzeArgs function
func TestAnalyzeArgs(t *testing.T) {
	ap := newArgMap()

	tests := []struct {
		arg     string
		flag    string
		val     string
		isSpace bool
	}{
		{
			arg:     "--set app1.bootstrap=true",
			flag:    "--set",
			isSpace: true,
			val:     "app1.bootstrap=true",
		},
		{
			arg:  "--set=app2.bootstrap=true",
			flag: "--set",
			val:  "app2.bootstrap=true",
		},
		{
			arg:  "-f",
			flag: "-f",
			val:  "",
		},
	}

	for _, test := range tests {
		analyzeArgs(ap, test.arg)
		require.Containsf(t, ap.flags, test.flag, "expected flag %s to be set", test.flag)
		require.Containsf(t, ap.m, test.flag, "expected m %s to be set", test.flag)
		kv := &keyVal{key: test.flag, val: test.val, spaceFlag: test.isSpace}
		require.Containsf(t, ap.m[test.flag], kv, "expected %v in m[%s]", kv, test.flag)
	}
}
