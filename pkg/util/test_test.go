package util

import (
	"fmt"
	"testing"
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
		result := CaptureStdout(func() {
			fmt.Print(test.output)
		})
		if result != test.expected {
			t.Errorf("CaptureStdout() = %v, want %v", result, test.expected)
		}
	}
}
