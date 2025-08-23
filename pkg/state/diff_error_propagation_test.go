package state

import (
	"errors"
	"sync"
	"testing"

	"go.uber.org/zap"

	"github.com/helmfile/helmfile/pkg/exectest"
	"github.com/helmfile/helmfile/pkg/helmexec"
)

func TestIsReleaseInstalled_HandlesConnectionError(t *testing.T) {
	logger := zap.NewNop().Sugar()
	
	state := &HelmState{
		logger: logger,
	}

	// Create a custom helm mock that fails on List operations 
	helm := &CustomFailingHelm{
		Helm: &exectest.Helm{
			DiffMutex:     &sync.Mutex{},
			ChartsMutex:   &sync.Mutex{},
			ReleasesMutex: &sync.Mutex{},
			Helm3:         true,
		},
	}

	release := ReleaseSpec{
		Name:      "test-release",
		Chart:     "test/chart",
		Namespace: "default",
	}

	// This should return an error due to connection failure
	_, err := state.isReleaseInstalled(helmexec.HelmContext{}, helm, release)

	// Verify that error was propagated
	if err == nil {
		t.Fatalf("expected isReleaseInstalled to return error when Kubernetes is unreachable, but got no error")
	}

	if err.Error() == "" {
		t.Fatalf("expected isReleaseInstalled to return meaningful error when Kubernetes is unreachable, but got empty error")
	}

	// Check if the error contains the expected message
	expectedMsg := "Kubernetes cluster unreachable"
	if err.Error() != expectedMsg && !contains(err.Error(), "Kubernetes cluster unreachable") {
		t.Fatalf("expected error to contain 'Kubernetes cluster unreachable', but got: %v", err.Error())
	}
}

// CustomFailingHelm wraps exectest.Helm and overrides List to simulate failures
type CustomFailingHelm struct {
	*exectest.Helm
}

func (h *CustomFailingHelm) List(context helmexec.HelmContext, filter string, flags ...string) (string, error) {
	return "", errors.New("Kubernetes cluster unreachable: Get \"http://localhost:8080/version\": dial tcp [::1]:8080: connect: connection refused")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr || (len(s) > len(substr) && contains(s[1:], substr))
}