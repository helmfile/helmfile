package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/helmfile/helmfile/pkg/config"
	"github.com/helmfile/helmfile/pkg/errors"
	"github.com/helmfile/helmfile/pkg/helmexec"
)

func TestToCLIError(t *testing.T) {
	g := config.NewGlobalImpl(&config.GlobalOptions{})

	tests := []struct {
		name            string
		err             error
		wantNil         bool
		wantExitCode    int
		wantMsgContains string
	}{
		{
			name:    "nil error returns nil",
			err:     nil,
			wantNil: true,
		},
		{
			name: "helmexec.ExitError returns correct exit code",
			err: helmexec.ExitError{
				Message: "helm command failed",
				Code:    7,
			},
			wantExitCode:    7,
			wantMsgContains: "helm command failed",
		},
		{
			name:            "wrapped helmexec.ExitError preserves exit code",
			err:             fmt.Errorf("helm version failed: %w", helmexec.ExitError{Message: "exit status 7", Code: 7}),
			wantExitCode:    7,
			wantMsgContains: "exit status 7",
		},
		{
			name:            "unknown error type returns exit code 1 without panic",
			err:             fmt.Errorf("some unexpected error"),
			wantExitCode:    1,
			wantMsgContains: "unexpected error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should never panic
			var result error
			assert.NotPanics(t, func() {
				result = toCLIError(g, tt.err)
			})

			if tt.wantNil {
				assert.NoError(t, result)
				return
			}

			assert.Error(t, result)
			exitErr, ok := result.(*errors.ExitError)
			assert.True(t, ok, "expected *errors.ExitError, got %T", result)
			assert.Equal(t, tt.wantExitCode, exitErr.ExitCode())
			assert.Contains(t, exitErr.Error(), tt.wantMsgContains)
		})
	}
}

func TestRootCmdRegistersOtelTracingFlag(t *testing.T) {
	rootCmd, err := NewRootCmd(&config.GlobalOptions{})
	require.NoError(t, err)

	flag := rootCmd.PersistentFlags().Lookup("otel-tracing")
	require.NotNil(t, flag, "--otel-tracing flag should be registered")
	assert.Equal(t, "bool", flag.Value.Type())
	assert.Equal(t, "false", flag.DefValue)
	assert.Contains(t, flag.Usage, "HELMFILE_OTEL_TRACING")
}

func TestTrackLogsUntilReadyFlag(t *testing.T) {
	for _, command := range []string{"sync", "apply"} {
		t.Run(command, func(t *testing.T) {
			rootCmd, err := NewRootCmd(&config.GlobalOptions{})
			require.NoError(t, err)
			cmd, _, err := rootCmd.Find([]string{command})
			require.NoError(t, err)

			flag := cmd.Flags().Lookup("track-logs-until-ready")
			require.NotNil(t, flag)
			assert.Equal(t, "false", flag.DefValue)
			require.NoError(t, cmd.ParseFlags([]string{"--track-logs", "--track-logs-until-ready"}))
			enabled, err := cmd.Flags().GetBool("track-logs-until-ready")
			require.NoError(t, err)
			assert.True(t, enabled)
		})
	}
}

func TestSelectorFlagCompletion_NonDirKey(t *testing.T) {
	// For any selector value that is not a dir= prefix, completion should
	// suppress file completion and return no suggestions.
	cases := []string{"", "name=", "name=foo", "tier=backend,name", "dir"}
	for _, in := range cases {
		t.Run(in, func(t *testing.T) {
			suggestions, dir := selectorFlagCompletion(&config.GlobalOptions{})(nil, nil, in)
			assert.Nil(t, suggestions)
			assert.Equal(t, cobra.ShellCompDirectiveNoFileComp, dir)
		})
	}
}

func TestSelectorFlagCompletion_DirEnumeratesDirectories(t *testing.T) {
	// Set up a temp tree with two subdirs and one file. Completion against
	// `dir=` should suggest only the subdirs, prefixed with `dir=`.
	root := t.TempDir()
	for _, sub := range []string{"apps", "infra"} {
		require.NoError(t, os.MkdirAll(filepath.Join(root, sub), 0o755))
	}
	require.NoError(t, os.WriteFile(filepath.Join(root, "helmfile.yaml"), []byte("releases: []"), 0o644))

	t.Chdir(root)

	suggestions, _ := selectorFlagCompletion(&config.GlobalOptions{})(nil, nil, "dir=")
	assert.ElementsMatch(t, []string{"dir=apps", "dir=infra"}, suggestions)
}

func TestSelectorFlagCompletion_DirPartialPath(t *testing.T) {
	root := t.TempDir()
	for _, sub := range []string{"apps/opencloud", "apps/openproject", "apps/xwiki"} {
		require.NoError(t, os.MkdirAll(filepath.Join(root, sub), 0o755))
	}
	t.Chdir(root)

	suggestions, _ := selectorFlagCompletion(&config.GlobalOptions{})(nil, nil, "dir=apps/op")
	assert.ElementsMatch(t, []string{"dir=apps/opencloud", "dir=apps/openproject"}, suggestions)
}

func TestSelectorFlagCompletion_DirCarriesOverPriorGroups(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "apps"), 0o755))
	t.Chdir(root)

	suggestions, _ := selectorFlagCompletion(&config.GlobalOptions{})(nil, nil, "name=foo,dir=")
	assert.ElementsMatch(t, []string{"name=foo,dir=apps"}, suggestions)
}

func TestSelectorFlagCompletion_UnmatchableValuesReturnNoSuggestions(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "apps"), 0o755))
	t.Chdir(filepath.Join(root, "apps"))

	for _, in := range []string{"dir=does/not/exist/", "dir=" + filepath.ToSlash(root) + "/", "dir=../", "dir=../ap"} {
		t.Run(in, func(t *testing.T) {
			suggestions, dir := selectorFlagCompletion(&config.GlobalOptions{})(nil, nil, in)
			assert.Nil(t, suggestions)
			assert.Equal(t, cobra.ShellCompDirectiveNoFileComp, dir)
		})
	}
}

func TestSelectorFlagCompletion_DirIsRelativeToRootHelmfile(t *testing.T) {
	root := t.TempDir()
	for _, sub := range []string{"deploy/apps/opencloud", "deploy/infra", "unrelated"} {
		require.NoError(t, os.MkdirAll(filepath.Join(root, sub), 0o755))
	}
	require.NoError(t, os.WriteFile(filepath.Join(root, "deploy", "helmfile.yaml"), []byte("releases: []"), 0o644))
	t.Chdir(root)

	tests := []struct {
		name       string
		file       string
		envFile    string
		toComplete string
		want       []string
	}{
		{name: "file flag", file: "deploy/helmfile.yaml", toComplete: "dir=", want: []string{"dir=apps", "dir=infra"}},
		{name: "file flag with nested partial", file: "deploy/helmfile.yaml", toComplete: "dir=apps/", want: []string{"dir=apps/opencloud"}},
		{name: "directory flag", file: "deploy", toComplete: "dir=", want: []string{"dir=apps", "dir=infra"}},
		{name: "absolute file flag", file: filepath.Join(root, "deploy", "helmfile.yaml"), toComplete: "dir=i", want: []string{"dir=infra"}},
		{name: "file from environment", envFile: "deploy/helmfile.yaml", toComplete: "dir=", want: []string{"dir=apps", "dir=infra"}},
		{name: "no file uses working directory", toComplete: "dir=", want: []string{"dir=deploy", "dir=unrelated"}},
		{name: "remote file has no local root", file: "git::https://github.com/example/repo.git@helmfile.yaml?ref=main", toComplete: "dir=", want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("HELMFILE_FILE_PATH", tt.envFile)

			suggestions, _ := selectorFlagCompletion(&config.GlobalOptions{File: tt.file})(nil, nil, tt.toComplete)
			assert.ElementsMatch(t, tt.want, suggestions)
		})
	}
}

func TestSelectorFlagCompletion_ThroughRootCmdHonorsFileFlag(t *testing.T) {
	root := t.TempDir()
	for _, sub := range []string{"deploy/apps", "unrelated"} {
		require.NoError(t, os.MkdirAll(filepath.Join(root, sub), 0o755))
	}
	t.Chdir(root)
	t.Setenv("HELMFILE_FILE_PATH", "")

	rootCmd, err := NewRootCmd(&config.GlobalOptions{})
	require.NoError(t, err)

	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetArgs([]string{cobra.ShellCompNoDescRequestCmd, "--file", "deploy/helmfile.yaml", "sync", "--selector", "dir="})
	require.NoError(t, rootCmd.Execute())

	assert.Contains(t, out.String(), "dir=apps\n")
	assert.NotContains(t, out.String(), "dir=deploy")
	assert.NotContains(t, out.String(), "dir=unrelated")
}
