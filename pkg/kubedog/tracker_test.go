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

func TestTrackOptions_WithQPS(t *testing.T) {
	opts := NewTrackOptions()
	opts = opts.WithQPS(50.0)

	assert.Equal(t, float32(50.0), opts.QPS)
}

func TestTrackOptions_WithBurst(t *testing.T) {
	opts := NewTrackOptions()
	opts = opts.WithBurst(100)

	assert.Equal(t, 100, opts.Burst)
}

func TestTrackOptions_DefaultQPSBurst(t *testing.T) {
	opts := NewTrackOptions()

	assert.Equal(t, float32(100), opts.QPS)
	assert.Equal(t, 200, opts.Burst)
}

func TestTrackerConfig_WithQPSBurst(t *testing.T) {
	qps := float32(50.0)
	burst := 100
	config := &TrackerConfig{
		Logger:       nil,
		Namespace:    "test-ns",
		KubeContext:  "test-ctx",
		Kubeconfig:   "/test/kubeconfig",
		TrackOptions: NewTrackOptions(),
		KubedogQPS:   &qps,
		KubedogBurst: &burst,
	}

	assert.NotNil(t, config)
	assert.Equal(t, "test-ns", config.Namespace)
	assert.Equal(t, &qps, config.KubedogQPS)
	assert.Equal(t, &burst, config.KubedogBurst)
	assert.Equal(t, float32(50.0), *config.KubedogQPS)
	assert.Equal(t, 100, *config.KubedogBurst)
}

func TestNewTracker_InvalidQPS(t *testing.T) {
	invalidQPS := float32(-1.0)
	burst := 100

	cfg := &TrackerConfig{
		Logger:       nil,
		Namespace:    "test-ns",
		KubeContext:  "test-ctx",
		Kubeconfig:   "/nonexistent/kubeconfig",
		TrackOptions: NewTrackOptions(),
		KubedogQPS:   &invalidQPS,
		KubedogBurst: &burst,
	}

	tr, err := NewTracker(cfg)

	assert.Error(t, err)
	assert.Nil(t, tr)
	assert.Contains(t, err.Error(), "invalid kubedog QPS")
	assert.Contains(t, err.Error(), "must be > 0")
}

func TestNewTracker_InvalidBurst(t *testing.T) {
	qps := float32(50.0)
	invalidBurst := 0

	cfg := &TrackerConfig{
		Logger:       nil,
		Namespace:    "test-ns",
		KubeContext:  "test-ctx",
		Kubeconfig:   "/nonexistent/kubeconfig",
		TrackOptions: NewTrackOptions(),
		KubedogQPS:   &qps,
		KubedogBurst: &invalidBurst,
	}

	tr, err := NewTracker(cfg)

	assert.Error(t, err)
	assert.Nil(t, tr)
	assert.Contains(t, err.Error(), "invalid kubedog burst")
	assert.Contains(t, err.Error(), "must be >= 1")
}

func TestNewTracker_ValidQPSBurst(t *testing.T) {
	qps := float32(50.0)
	burst := 100

	cfg := &TrackerConfig{
		Logger:       nil,
		Namespace:    "test-ns",
		KubeContext:  "",
		Kubeconfig:   "", // Will use default kubeconfig
		TrackOptions: NewTrackOptions(),
		KubedogQPS:   &qps,
		KubedogBurst: &burst,
	}

	// This test may fail if no kubeconfig is available, which is expected
	// in CI environments. The important part is that validation passes.
	tr, err := NewTracker(cfg)

	// If kubeconfig doesn't exist, we expect an error about loading kubeconfig,
	// NOT an error about invalid QPS/Burst
	if err != nil {
		assert.NotContains(t, err.Error(), "invalid kubedog QPS")
		assert.NotContains(t, err.Error(), "invalid kubedog burst")
		assert.Nil(t, tr)
	} else {
		assert.NotNil(t, tr)
	}
}
