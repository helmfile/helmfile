package app

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Test_ValidateConfig_NoColor_Color tests that ValidateConfig returns an error when both
func TestValidateConfig(t *testing.T) {
	tests := []struct {
		wantErr bool
		noColor bool
		color   bool
	}{
		{
			wantErr: true,
			noColor: true,
			color:   true,
		},
		{
			wantErr: false,
			noColor: false,
			color:   true,
		},
		{
			wantErr: false,
			noColor: true,
			color:   false,
		},
		{
			wantErr: false,
			noColor: false,
			color:   false,
		},
	}

	for _, tt := range tests {
		conf := applyConfig{
			noColor: tt.noColor,
			color:   tt.color,
		}

		err := ValidateConfig(conf)

		if tt.wantErr {
			require.Errorf(t, err, "ValidateConfig should return an error when color set to %v and noColor set to %v", tt.color, tt.noColor)
		} else {
			require.NoErrorf(t, err, "ValidateConfig should not return an error when color set to %v and noColor set to %v", tt.color, tt.noColor)
		}
	}
}
