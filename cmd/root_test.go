package cmd

import (
	"fmt"
	"testing"

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
