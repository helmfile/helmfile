package yaml

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func testYamlMarshal(t *testing.T) {
	t.Helper()

	yamlLibraryName := "goccy/go-yaml"

	tests := []struct {
		Name string `yaml:"name"`
		Info []struct {
			Age        int    `yaml:"age"`
			Address    string `yaml:"address"`
			Annotation string `yaml:"annotation"`
		} `yaml:"info"`

		expected map[string]string
	}{
		{
			Name: "John",
			Info: []struct {
				Age        int    `yaml:"age"`
				Address    string `yaml:"address"`
				Annotation string `yaml:"annotation"`
			}{{
				Age:     20,
				Address: "New York",
				// See:
				// - https://github.com/helmfile/helmfile/discussions/656
				// - https://github.com/helmfile/helmfile/pull/675
				Annotation: "on",
			}},
			expected: map[string]string{
				"goccy/go-yaml": "name: John\ninfo:\n- age: 20\n  address: New York\n  annotation: 'on'\n",
			},
		},
	}

	for _, tt := range tests {
		actual, err := Marshal(tt)
		require.NoError(t, err)
		require.Equal(t, tt.expected[yamlLibraryName], string(actual))
	}
}

func TestYamlMarshal(t *testing.T) {
	t.Run("with goccy/go-yaml", func(t *testing.T) {
		testYamlMarshal(t)
	})
}
