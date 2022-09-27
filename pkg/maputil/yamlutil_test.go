package maputil

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestYamlMarshal(t *testing.T) {
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
			expected: "name: John\ninfo:\n  - age: 20\n    address: New York\n",
		},
	}

	for _, tt := range tests {
		actual, err := YamlMarshal(tt)
		require.NoError(t, err)
		require.Equal(t, tt.expected, string(actual))
	}
}
