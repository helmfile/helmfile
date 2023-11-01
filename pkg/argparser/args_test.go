package argparser

import (
	"testing"

	"github.com/stretchr/testify/require"
)

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
