package state

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTrimPartInfo(t *testing.T) {
	tests := []struct {
		name     string
		sfName   string
		expected string
	}{
		{

			name:     "no need to trim",
			sfName:   "test1.yaml",
			expected: "test1.yaml",
		},
		{
			name:     "trim part info",
			sfName:   "test2.yaml.part.1",
			expected: "test2.yaml",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sf := NewStateFileInfo(test.sfName, "")
			require.Equalf(t, test.expected, sf.Path.Base, "expected %s, got %s", test.expected, sf.Path.Base)
		})
	}
}
