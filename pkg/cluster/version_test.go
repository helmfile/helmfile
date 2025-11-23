package cluster

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestDetectServerVersion_Integration tests the cluster version detection
// against a real Kubernetes cluster (if available).
// This test will be skipped if no cluster is accessible.
func TestDetectServerVersion_Integration(t *testing.T) {
	// Try to detect version with default kubeconfig
	version, err := DetectServerVersion("", "")

	if err != nil {
		t.Skipf("Skipping test - no accessible Kubernetes cluster: %v", err)
		return
	}

	// If we got a version, verify it's in a valid format
	require.NotEmpty(t, version, "Version should not be empty")
	require.NotContains(t, version, "v", "Version should not have 'v' prefix")

	// Version should look like "1.xx.y" format
	require.Regexp(t, `^\d+\.\d+\.\d+`, version, "Version should match semver format")
}

// TestDetectServerVersion_InvalidConfig tests error handling
func TestDetectServerVersion_InvalidConfig(t *testing.T) {
	// Try with a non-existent kubeconfig file
	_, err := DetectServerVersion("/non/existent/path/kubeconfig", "")

	require.Error(t, err, "Should return error for invalid kubeconfig")
	require.Contains(t, err.Error(), "failed to load kubeconfig", "Error should mention kubeconfig loading")
}
