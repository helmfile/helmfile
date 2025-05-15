package yaml

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/helmfile/helmfile/pkg/runtime"
)

func testYamlMarshal(t *testing.T, GoYamlV3 bool) {
	t.Helper()

	var yamlLibraryName string
	if GoYamlV3 {
		yamlLibraryName = "gopkg.in/yaml.v3"
	} else {
		yamlLibraryName = "gopkg.in/yaml.v2"
	}

	v := runtime.GoYamlV3
	runtime.GoYamlV3 = GoYamlV3
	t.Cleanup(func() {
		runtime.GoYamlV3 = v
	})

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
				"gopkg.in/yaml.v2": "name: John\ninfo:\n- age: 20\n  address: New York\n  annotation: 'on'\n",
				"gopkg.in/yaml.v3": "name: John\ninfo:\n  - age: 20\n    address: New York\n    annotation: \"on\"\n",
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
	t.Run("with gopkg.in/yaml.v2", func(t *testing.T) {
		testYamlMarshal(t, true)
	})

	t.Run("with gopkg.in/yaml.v3", func(t *testing.T) {
		testYamlMarshal(t, false)
	})
}
