package yaml

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/helmfile/helmfile/pkg/runtime"
)

func testYamlMarshal(t *testing.T, goccyGoYaml bool) {
	t.Helper()

	v := runtime.GoccyGoYaml
	runtime.GoccyGoYaml = goccyGoYaml
	t.Cleanup(func() {
		runtime.GoccyGoYaml = v
	})

	tests := []struct {
		Name string `yaml:"name"`
		Info []struct {
			Age        int    `yaml:"age"`
			Address    string `yaml:"address"`
			Annotation string `yaml:"annotation"`
		} `yaml:"info"`

		expected string
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
			expected: "name: John\ninfo:\n- age: 20\n  address: New York\n  annotation: 'on'\n",
		},
	}

	for _, tt := range tests {
		actual, err := Marshal(tt)
		require.NoError(t, err)
		require.Equal(t, tt.expected, string(actual))
	}
}

func TestYamlMarshal(t *testing.T) {
	t.Run("with goccy/go-yaml", func(t *testing.T) {
		testYamlMarshal(t, true)
	})

	t.Run("with gopkg.in/yaml.v2", func(t *testing.T) {
		testYamlMarshal(t, false)
	})
}
