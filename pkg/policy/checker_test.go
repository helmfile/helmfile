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
		content     []byte
		expectedErr bool
		isStrict    bool
	}{
		{
			name:        "no error when only releases",
			filePath:    "helmfile.yaml",
			content:     []byte("releases:\n"),
			v1mode:      false,
			expectedErr: false,
			isStrict:    false,
		},
		{
			name:        "no error when only environments",
			filePath:    "helmfile.yaml",
			content:     []byte("environments:\n"),
			v1mode:      false,
			expectedErr: false,
			isStrict:    false,
		},
		{
			name:        "no error when has --- between releases and environments",
			filePath:    "helmfile.yaml",
			content:     []byte("environments:\n---\nreleases:\n"),
			v1mode:      false,
			expectedErr: false,
			isStrict:    false,
		},
		{
			name:        "no error when has --- between releases and environments, and --- on top of helmfile.yaml.gotmpl",
			filePath:    "helmfile.yaml",
			content:     []byte("---\nenvironments:\n---\nreleases:\n"),
			v1mode:      false,
			expectedErr: false,
			isStrict:    false,
		},
		{
			name:        "error when both releases and environments",
			filePath:    "helmfile.yaml",
			content:     []byte("environments:\nreleases:\n"),
			v1mode:      false,
			expectedErr: true,
			isStrict:    false,
		},
		{
			name:        "error when both releases and environments for plain yaml on v1",
			filePath:    "helmfile.yaml",
			content:     []byte("environments:\nreleases:\n"),
			v1mode:      true,
			expectedErr: true,
			isStrict:    true,
		},
	}

	v1mode := runtime.V1Mode
	t.Cleanup(func() {
		runtime.V1Mode = v1mode
	})

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			runtime.V1Mode = tc.v1mode
			isStrict, err := forbidEnvironmentsWithReleases(tc.filePath, tc.content)
			require.Equal(t, tc.isStrict, isStrict, "expected isStrict=%v, got=%v", tc.isStrict, isStrict)
			if tc.expectedErr {
				require.ErrorIsf(t, err, EnvironmentsAndReleasesWithinSameYamlPartErr, "expected error=%v, got=%v", EnvironmentsAndReleasesWithinSameYamlPartErr, err)
			} else {
				require.NoError(t, err, "expected no error but got error: %v", err)
			}
		})
	}
}

func TestTopKeys(t *testing.T) {
	tests := []struct {
		name            string
		helmfileContent []byte
		hasSeparator    bool
		want            []string
	}{
		{
			name:            "get top keys",
			helmfileContent: []byte("bases:\nenvironments:\nreleases:\n"),
			want:            []string{"bases", "environments", "releases"},
		},
		{
			name:            "get top keys with ---",
			helmfileContent: []byte("bases:\n---\nreleases:\n"),
			hasSeparator:    true,
			want:            []string{"bases", "---", "releases"},
		},
		{
			name:            "get top keys with empty array",
			helmfileContent: []byte("bases: []\n---\nreleases: []\n"),
			hasSeparator:    true,
			want:            []string{"bases", "---", "releases"},
		},
		{
			name:            "get empty keys",
			helmfileContent: []byte(""),
			want:            nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TopKeys(tt.helmfileContent, tt.hasSeparator)
			require.Equal(t, tt.want, got, "expected %v, got=%v", tt.want, got)
		})
	}
}
