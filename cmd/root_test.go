package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"

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

func TestSelectorFlagCompletion_NonDirKey(t *testing.T) {
	// For any selector value that is not a dir= prefix, completion should
	// suppress file completion and return no suggestions.
	cases := []string{"", "name=", "name=foo", "tier=backend,name", "dir"}
	for _, in := range cases {
		t.Run(in, func(t *testing.T) {
			suggestions, dir := selectorFlagCompletion(nil, nil, in)
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
		assert.NoError(t, os.MkdirAll(filepath.Join(root, sub), 0o755))
	}
	assert.NoError(t, os.WriteFile(filepath.Join(root, "helmfile.yaml"), []byte("releases: []"), 0o644))

	prev, err := os.Getwd()
	assert.NoError(t, err)
	assert.NoError(t, os.Chdir(root))
	t.Cleanup(func() { _ = os.Chdir(prev) })

	suggestions, _ := selectorFlagCompletion(nil, nil, "dir=")
	assert.ElementsMatch(t, []string{"dir=apps", "dir=infra"}, suggestions)
}

func TestSelectorFlagCompletion_DirPartialPath(t *testing.T) {
	root := t.TempDir()
	for _, sub := range []string{"apps/opencloud", "apps/openproject", "apps/xwiki"} {
		assert.NoError(t, os.MkdirAll(filepath.Join(root, sub), 0o755))
	}
	prev, err := os.Getwd()
	assert.NoError(t, err)
	assert.NoError(t, os.Chdir(root))
	t.Cleanup(func() { _ = os.Chdir(prev) })

	suggestions, _ := selectorFlagCompletion(nil, nil, "dir=apps/op")
	assert.ElementsMatch(t, []string{"dir=apps/opencloud", "dir=apps/openproject"}, suggestions)
}

func TestSelectorFlagCompletion_DirCarriesOverPriorGroups(t *testing.T) {
	root := t.TempDir()
	assert.NoError(t, os.MkdirAll(filepath.Join(root, "apps"), 0o755))
	prev, err := os.Getwd()
	assert.NoError(t, err)
	assert.NoError(t, os.Chdir(root))
	t.Cleanup(func() { _ = os.Chdir(prev) })

	suggestions, _ := selectorFlagCompletion(nil, nil, "name=foo,dir=")
	assert.ElementsMatch(t, []string{"name=foo,dir=apps"}, suggestions)
}

func TestSelectorFlagCompletion_NonexistentDirReturnsNoSuggestions(t *testing.T) {
	suggestions, dir := selectorFlagCompletion(nil, nil, "dir=/does/not/exist/")
	assert.Nil(t, suggestions)
	assert.Equal(t, cobra.ShellCompDirectiveNoFileComp, dir)
}
