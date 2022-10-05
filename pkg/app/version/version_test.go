package version

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVersion(t *testing.T) {
	v := Version()
	require.Equalf(t, "0.0.0-dev", v, "expected version to be %q, got %q", "0.0.0-dev", v)
}
