package testutil

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestCaptureStdout tests the CaptureStdout function.
func TestCaptureStdout(t *testing.T) {
	tests := []struct {
		output   string
		expected string
	}{
		{
			output:   "123",
			expected: "123",
		},
		{
			output:   "test",
			expected: "test",
		},
		{
			output:   "",
			expected: "",
		},
		{
			output:   "...",
			expected: "...",
		},
	}

	for _, test := range tests {
		result, err := CaptureStdout(func() {
			fmt.Print(test.output)
		})
		assert.NoError(t, err)
		if result != test.expected {
			t.Errorf("CaptureStdout() = %v, want %v", result, test.expected)
		}
	}
}
