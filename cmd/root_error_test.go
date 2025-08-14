package cmd

import (
	"testing"
	"fmt"

	"github.com/helmfile/helmfile/pkg/config"
)

// TestToCLIErrorHandlesStandardErrors tests that toCLIError properly handles 
// standard Go errors instead of panicking. This test prevents regression of
// the issue fixed in https://github.com/helmfile/helmfile/issues/2119
func TestToCLIErrorHandlesStandardErrors(t *testing.T) {
	globalImpl := &config.GlobalImpl{}

	tests := []struct {
		name string
		err  error
		expectPanic bool
	}{
		{
			name: "nil error should not panic",
			err:  nil,
			expectPanic: false,
		},
		{
			name: "standard fmt.Errorf should not panic",
			err:  fmt.Errorf("error find helm srmver version '%s': unable to find semver info", "invalid"),
			expectPanic: false,
		},
		{
			name: "standard error string should not panic", 
			err:  fmt.Errorf("empty helm version"),
			expectPanic: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					if !tt.expectPanic {
						t.Errorf("toCLIError() panicked unexpectedly: %v", r)
					}
				} else {
					if tt.expectPanic {
						t.Errorf("toCLIError() expected to panic but did not")
					}
				}
			}()
			
			_ = toCLIError(globalImpl, tt.err)
		})
	}
}