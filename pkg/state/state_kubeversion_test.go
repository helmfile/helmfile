package state

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestAppendApiVersionsFlags_KubeVersion tests that kubeVersion is properly
// passed to helm diff. This is a regression test for issue #2275.
// Priority: 1) state.KubeVersion (helmfile.yaml), 2) paramKubeVersion (auto-detected)
func TestAppendApiVersionsFlags_KubeVersion(t *testing.T) {
	tests := []struct {
		name             string
		stateKubeVersion string // kubeVersion from HelmState (helmfile.yaml)
		paramKubeVersion string // kubeVersion parameter passed to appendApiVersionsFlags
		expectedVersion  string // which version should be in the flags
	}{
		{
			name:             "state kubeVersion should be used when param is empty",
			stateKubeVersion: "1.34.0",
			paramKubeVersion: "",
			expectedVersion:  "1.34.0",
		},
		{
			name:             "param kubeVersion takes precedence over state",
			stateKubeVersion: "1.34.0",
			paramKubeVersion: "1.30.0",
			expectedVersion:  "1.30.0",
		},
		{
			name:             "no version when both are empty",
			stateKubeVersion: "",
			paramKubeVersion: "",
			expectedVersion:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &HelmState{
				ReleaseSetSpec: ReleaseSetSpec{
					KubeVersion: tt.stateKubeVersion,
				},
			}

			release := &ReleaseSpec{
				Name:  "test-release",
				Chart: "test/chart",
			}

			result := state.appendApiVersionsFlags([]string{}, release, tt.paramKubeVersion)

			if tt.expectedVersion != "" {
				// Should have --kube-version flag
				foundKubeVersion := false
				for i := 0; i < len(result)-1; i++ {
					if result[i] == "--kube-version" {
						require.Equal(t, tt.expectedVersion, result[i+1],
							"kube-version value should match expected")
						foundKubeVersion = true
						break
					}
				}
				require.True(t, foundKubeVersion, "Should have --kube-version flag in result")
			} else {
				// Should NOT have --kube-version flag
				for i := 0; i < len(result); i++ {
					require.NotEqual(t, "--kube-version", result[i],
						"Should not have --kube-version flag when nothing is set")
				}
			}
		})
	}
}
