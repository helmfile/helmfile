package cluster

import (
	"fmt"

	"k8s.io/client-go/discovery"
	"k8s.io/client-go/tools/clientcmd"
)

// DetectServerVersion detects the Kubernetes server version by connecting to the cluster.
// It returns the version in the format "major.minor.patch" (e.g., "1.34.1").
// Returns an error if detection fails.
func DetectServerVersion(kubeconfig, context string) (string, error) {
	// Build the kubeconfig
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if kubeconfig != "" {
		loadingRules.ExplicitPath = kubeconfig
	}

	configOverrides := &clientcmd.ConfigOverrides{}
	if context != "" {
		configOverrides.CurrentContext = context
	}

	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules,
		configOverrides,
	).ClientConfig()
	if err != nil {
		return "", fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	// Create discovery client
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return "", fmt.Errorf("failed to create discovery client: %w", err)
	}

	// Get server version
	serverVersion, err := discoveryClient.ServerVersion()
	if err != nil {
		return "", fmt.Errorf("failed to get server version: %w", err)
	}

	// ServerVersion.GitVersion includes "v" prefix (e.g., "v1.34.1")
	// Strip the "v" prefix to match Helm's --kube-version format (e.g., "1.34.1")
	version := serverVersion.GitVersion
	if len(version) > 0 && version[0] == 'v' {
		version = version[1:]
	}

	return version, nil
}
