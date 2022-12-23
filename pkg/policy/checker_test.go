package policy

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestForbidEnvironmentsWithReleases(t *testing.T) {
	testCases := []struct {
		name        string
		helmState   map[string]interface{}
		expectedErr bool
		isStrict    bool
	}{
		{
			name: "no error when only releases",
			helmState: map[string]interface{}{
				"releases": interface{}(nil),
			},
			expectedErr: false,
			isStrict:    false,
		},
		{
			name: "no error when only environments",
			helmState: map[string]interface{}{
				"environments": map[string]interface{}{},
			},
			expectedErr: false,
			isStrict:    false,
		},
		{
			name: "error when both releases and environments",
			helmState: map[string]interface{}{
				"environments": interface{}(nil),
				"releases":     interface{}(nil),
			},
			expectedErr: true,
			isStrict:    false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			isStrict, err := forbidEnvironmentsWithReleases(tc.helmState)
			require.Equal(t, tc.isStrict, isStrict, "expected isStrict=%v, got=%v", tc.isStrict, isStrict)
			if tc.expectedErr {
				require.ErrorIsf(t, err, EnvironmentsAndReleasesWithinSameYamlPartErr, "expected error=%v, got=%v", EnvironmentsAndReleasesWithinSameYamlPartErr, err)
			} else {
				require.NoError(t, err, "expected no error but got error: %v", err)
			}
		})
	}
}
