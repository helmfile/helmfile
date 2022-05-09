package helmexec

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewExecutionID(t *testing.T) {
	idx := newExecutionID()
	idy := newExecutionID()
	require.NotEqual(t, idx, idy, "Execution IDs should be unique")
}
