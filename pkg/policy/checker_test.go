package policy

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/helmfile/helmfile/pkg/runtime"
)

func TestForbidEnvironmentsWithReleases(t *testing.T) {
	testCases := []struct {
		name        string
		filePath    string
		v1mode      bool
		helmState   map[string]interface{}
		expectedErr bool
		isStrict    bool
	}{
		{
			name:     "no error when only releases",
			filePath: "helmfile.yaml",
			v1mode:   false,
			helmState: map[string]interface{}{
				"releases": interface{}(nil),
			},
			expectedErr: false,
			isStrict:    false,
		},
		{
			name:     "no error when only environments",
			filePath: "helmfile.yaml",
			v1mode:   false,
			helmState: map[string]interface{}{
				"environments": map[string]interface{}{},
			},
			expectedErr: false,
			isStrict:    false,
		},
		{
			name:     "error when both releases and environments",
			filePath: "helmfile.yaml",
			v1mode:   false,
			helmState: map[string]interface{}{
				"environments": interface{}(nil),
				"releases":     interface{}(nil),
			},
			expectedErr: true,
			isStrict:    false,
		},
		{
			name:     "no error when both releases and environments for plain yaml on v1",
			filePath: "helmfile.yaml",
			v1mode:   true,
			helmState: map[string]interface{}{
				"environments": interface{}(nil),
				"releases":     interface{}(nil),
			},
			expectedErr: false,
			isStrict:    false,
		},
	}

	v1mode := runtime.V1Mode
	t.Cleanup(func() {
		runtime.V1Mode = v1mode
	})

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			runtime.V1Mode = tc.v1mode
			isStrict, err := forbidEnvironmentsWithReleases(tc.filePath, tc.helmState)
			require.Equal(t, tc.isStrict, isStrict, "expected isStrict=%v, got=%v", tc.isStrict, isStrict)
			if tc.expectedErr {
				require.ErrorIsf(t, err, EnvironmentsAndReleasesWithinSameYamlPartErr, "expected error=%v, got=%v", EnvironmentsAndReleasesWithinSameYamlPartErr, err)
			} else {
				require.NoError(t, err, "expected no error but got error: %v", err)
			}
		})
	}
}
