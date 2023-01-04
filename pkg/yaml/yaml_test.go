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
			Age     int    `yaml:"age"`
			Address string `yaml:"address"`
		} `yaml:"info"`

		expected string
	}{
		{
			Name: "John",
			Info: []struct {
				Age     int    `yaml:"age"`
				Address string `yaml:"address"`
			}{{Age: 20, Address: "New York"}},
			expected: "name: John\ninfo:\n- age: 20\n  address: New York\n",
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
