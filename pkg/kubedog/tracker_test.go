package kubedog

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTrackMode(t *testing.T) {
	assert.Equal(t, "helm", string(TrackModeHelm))
	assert.Equal(t, "kubedog", string(TrackModeKubedog))
}

func TestNewTrackOptions(t *testing.T) {
	opts := NewTrackOptions()
	assert.NotNil(t, opts)
	assert.Equal(t, 5*time.Minute, opts.Timeout)
	assert.Equal(t, false, opts.Logs)
	assert.Equal(t, 10*time.Minute, opts.LogsSince)
}

func TestTrackOptions_WithTimeout(t *testing.T) {
	opts := NewTrackOptions()
	opts = opts.WithTimeout(10 * time.Second)

	assert.Equal(t, 10*time.Second, opts.Timeout)
}

func TestTrackOptions_WithLogs(t *testing.T) {
	opts := NewTrackOptions()
	opts = opts.WithLogs(true)

	assert.True(t, opts.Logs)
}

func TestTrackOptions_Chaining(t *testing.T) {
	opts := NewTrackOptions()
	opts = opts.
		WithTimeout(20 * time.Second).
		WithLogs(true).
		WithLogsSince(5 * time.Minute).
		WithNamespace("test-ns").
		WithKubeContext("test-context").
		WithKubeconfig("/test/kubeconfig")

	assert.Equal(t, 20*time.Second, opts.Timeout)
	assert.True(t, opts.Logs)
	assert.Equal(t, 5*time.Minute, opts.LogsSince)
	assert.Equal(t, "test-ns", opts.Namespace)
	assert.Equal(t, "test-context", opts.KubeContext)
	assert.Equal(t, "/test/kubeconfig", opts.Kubeconfig)
}

func TestResourceSpec(t *testing.T) {
	spec := &ResourceSpec{
		Name:      "test-resource",
		Namespace: "test-ns",
		Kind:      "deployment",
	}

	assert.Equal(t, "test-resource", spec.Name)
	assert.Equal(t, "test-ns", spec.Namespace)
	assert.Equal(t, "deployment", spec.Kind)
}

func TestTrackerConfig(t *testing.T) {
	config := &TrackerConfig{
		Logger:       nil,
		Namespace:    "test-ns",
		KubeContext:  "test-ctx",
		Kubeconfig:   "/test/kubeconfig",
		TrackOptions: NewTrackOptions(),
	}

	assert.NotNil(t, config)
	assert.Equal(t, "test-ns", config.Namespace)
	assert.Equal(t, "test-ctx", config.KubeContext)
	assert.Equal(t, "/test/kubeconfig", config.Kubeconfig)
	assert.NotNil(t, config.TrackOptions)
}
