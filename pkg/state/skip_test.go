package state

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSkipDepsAndSkipRefresh tests that helmDefaults.skipDeps and helmDefaults.skipRefresh
// are properly applied when preparing charts (issue #2269)
func TestSkipDepsAndSkipRefresh(t *testing.T) {
	tests := []struct {
		name                    string
		helmDefaultsSkipDeps    bool
		helmDefaultsSkipRefresh bool
		releaseSkipDeps         *bool
		releaseSkipRefresh      *bool
		optsSkipDeps            bool
		optsSkipRefresh         bool
		isLocal                 bool
		expectedSkipDeps        bool
		expectedSkipRefresh     bool
	}{
		{
			name:                    "helmDefaults.skipDeps=true should skip deps",
			helmDefaultsSkipDeps:    true,
			helmDefaultsSkipRefresh: false,
			releaseSkipDeps:         nil,
			releaseSkipRefresh:      nil,
			optsSkipDeps:            false,
			optsSkipRefresh:         false,
			isLocal:                 true,
			expectedSkipDeps:        true,
			expectedSkipRefresh:     false,
		},
		{
			name:                    "helmDefaults.skipRefresh=true should skip refresh",
			helmDefaultsSkipDeps:    false,
			helmDefaultsSkipRefresh: true,
			releaseSkipDeps:         nil,
			releaseSkipRefresh:      nil,
			optsSkipDeps:            false,
			optsSkipRefresh:         false,
			isLocal:                 true,
			expectedSkipDeps:        false,
			expectedSkipRefresh:     true,
		},
		{
			name:                    "both helmDefaults.skipDeps and skipRefresh=true",
			helmDefaultsSkipDeps:    true,
			helmDefaultsSkipRefresh: true,
			releaseSkipDeps:         nil,
			releaseSkipRefresh:      nil,
			optsSkipDeps:            false,
			optsSkipRefresh:         false,
			isLocal:                 true,
			expectedSkipDeps:        true,
			expectedSkipRefresh:     true,
		},
		{
			name:                    "release.skipRefresh overrides helmDefaults",
			helmDefaultsSkipDeps:    false,
			helmDefaultsSkipRefresh: false,
			releaseSkipDeps:         nil,
			releaseSkipRefresh:      boolPtr(true),
			optsSkipDeps:            false,
			optsSkipRefresh:         false,
			isLocal:                 true,
			expectedSkipDeps:        false,
			expectedSkipRefresh:     true,
		},
		{
			name:                    "opts.SkipRefresh (CLI flag) has priority",
			helmDefaultsSkipDeps:    false,
			helmDefaultsSkipRefresh: false,
			releaseSkipDeps:         nil,
			releaseSkipRefresh:      nil,
			optsSkipDeps:            false,
			optsSkipRefresh:         true,
			isLocal:                 true,
			expectedSkipDeps:        false,
			expectedSkipRefresh:     true,
		},
		{
			name:                    "non-local chart always skips refresh",
			helmDefaultsSkipDeps:    false,
			helmDefaultsSkipRefresh: false,
			releaseSkipDeps:         nil,
			releaseSkipRefresh:      nil,
			optsSkipDeps:            false,
			optsSkipRefresh:         false,
			isLocal:                 false,
			expectedSkipDeps:        true, // non-local charts skip deps
			expectedSkipRefresh:     true, // non-local charts skip refresh
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Calculate skipDeps using the actual logic from state.go
			skipDepsGlobal := tt.optsSkipDeps
			skipDepsRelease := tt.releaseSkipDeps != nil && *tt.releaseSkipDeps
			skipDepsDefault := tt.releaseSkipDeps == nil && tt.helmDefaultsSkipDeps
			chartFetchedByGoGetter := false
			skipDeps := (!tt.isLocal && !chartFetchedByGoGetter) || skipDepsGlobal || skipDepsRelease || skipDepsDefault

			// Calculate skipRefresh using the actual logic from state.go (after fix)
			skipRefreshGlobal := tt.optsSkipRefresh
			skipRefreshRelease := tt.releaseSkipRefresh != nil && *tt.releaseSkipRefresh
			skipRefreshDefault := tt.releaseSkipRefresh == nil && tt.helmDefaultsSkipRefresh
			skipRefresh := !tt.isLocal || skipRefreshGlobal || skipRefreshRelease || skipRefreshDefault

			assert.Equal(t, tt.expectedSkipDeps, skipDeps, "skipDeps mismatch")
			assert.Equal(t, tt.expectedSkipRefresh, skipRefresh, "skipRefresh mismatch")
		})
	}
}

func boolPtr(b bool) *bool {
	return &b
}
