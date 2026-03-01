package kubedog

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/helmfile/helmfile/pkg/resource"
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
		WithLogs(true)

	assert.Equal(t, 20*time.Second, opts.Timeout)
	assert.True(t, opts.Logs)
}

func TestResource(t *testing.T) {
	res := &resource.Resource{
		Name:      "test-resource",
		Namespace: "test-ns",
		Kind:      "deployment",
	}

	assert.Equal(t, "test-resource", res.Name)
	assert.Equal(t, "test-ns", res.Namespace)
	assert.Equal(t, "deployment", res.Kind)
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

func TestTrackOptions_WithFilterConfig(t *testing.T) {
	opts := NewTrackOptions()
	filter := &resource.FilterConfig{
		TrackKinds: []string{"Deployment", "StatefulSet"},
		SkipKinds:  []string{"ConfigMap"},
	}
	opts = opts.WithFilterConfig(filter)

	assert.NotNil(t, opts.Filter)
	assert.Equal(t, []string{"Deployment", "StatefulSet"}, opts.Filter.TrackKinds)
	assert.Equal(t, []string{"ConfigMap"}, opts.Filter.SkipKinds)
}
