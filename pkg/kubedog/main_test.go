package kubedog

import (
	"os"
	"testing"

	"github.com/gookit/color"
)

// TestMain forces color output on for the whole kubedog test binary so ANSI
// escapes are deterministic regardless of whether the test runner is attached
// to a TTY.
func TestMain(m *testing.M) {
	color.ForceColor()
	os.Exit(m.Run())
}
