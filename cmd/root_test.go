package cmd

import (
	"fmt"
	"testing"

	"github.com/helmfile/helmfile/pkg/config"
	"github.com/helmfile/helmfile/pkg/errors"
)

func TestToCLIError(t *testing.T) {
	globalCfg := &config.GlobalImpl{}

	tests := []struct {
		name     string
		err      error
		wantCode int
		wantErr  bool
	}{
		{
			name:    "nil error",
			err:     nil,
			wantErr: false,
		},
		{
			name:     "regular error (like from fmt.Errorf) - this was causing the panic",
			err:      fmt.Errorf("invalid semantic version"),
			wantCode: 1,
			wantErr:  true,
		},
		{
			name:     "another regular error type",
			err:      fmt.Errorf("some other error"),
			wantCode: 1,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This should not panic for any error type
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("toCLIError() panicked: %v", r)
				}
			}()

			err := toCLIError(globalCfg, tt.err)

			if (err != nil) != tt.wantErr {
				t.Errorf("toCLIError() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil {
				if exitErr, ok := err.(*errors.ExitError); ok {
					if exitErr.ExitCode() != tt.wantCode {
						t.Errorf("toCLIError() exit code = %v, want %v", exitErr.ExitCode(), tt.wantCode)
					}
				} else {
					t.Errorf("toCLIError() returned non-ExitError: %T", err)
				}
			}
		})
	}
}