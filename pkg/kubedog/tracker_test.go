package kubedog

import (
	"math"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

func TestNewTracker_NaNQPS(t *testing.T) {
	nanQPS := float32(math.NaN())
	burst := 100

	cfg := &TrackerConfig{
		Logger:       nil,
		Namespace:    "test-ns",
		KubeContext:  "test-ctx",
		Kubeconfig:   "/nonexistent/kubeconfig",
		TrackOptions: NewTrackOptions(),
		KubedogQPS:   &nanQPS,
		KubedogBurst: &burst,
	}

	tr, err := NewTracker(cfg)

	assert.Error(t, err)
	assert.Nil(t, tr)
	assert.Contains(t, err.Error(), "invalid kubedog QPS")
	assert.Contains(t, err.Error(), "must be > 0 and finite")
}

func TestNewTracker_InfQPS(t *testing.T) {
	infQPS := float32(math.Inf(1))
	burst := 100

	cfg := &TrackerConfig{
		Logger:       nil,
		Namespace:    "test-ns",
		KubeContext:  "test-ctx",
		Kubeconfig:   "/nonexistent/kubeconfig",
		TrackOptions: NewTrackOptions(),
		KubedogQPS:   &infQPS,
		KubedogBurst: &burst,
	}

	tr, err := NewTracker(cfg)

	assert.Error(t, err)
	assert.Nil(t, tr)
	assert.Contains(t, err.Error(), "invalid kubedog QPS")
	assert.Contains(t, err.Error(), "must be > 0 and finite")
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

	// Create a minimal valid kubeconfig in a temp file
	tmpFile, err := os.CreateTemp("", "kubeconfig-*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	kubeconfigContent := `
apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://test-server:6443
  name: test-cluster
contexts:
- context:
    cluster: test-cluster
    user: test-user
  name: test-context
current-context: test-context
users:
- name: test-user
  user:
    token: test-token
`
	_, err = tmpFile.WriteString(kubeconfigContent)
	require.NoError(t, err)
	require.NoError(t, tmpFile.Close())

	cfg := &TrackerConfig{
		Logger:       nil,
		Namespace:    "test-ns",
		KubeContext:  "test-context",
		Kubeconfig:   tmpFile.Name(),
		TrackOptions: NewTrackOptions(),
		KubedogQPS:   &qps,
		KubedogBurst: &burst,
	}

	// This should succeed - validation passes and client is created
	tr, err := NewTracker(cfg)

	// The test should pass validation. It may fail later due to invalid cluster,
	// but that's okay - we're testing that QPS/Burst validation works.
	if err != nil {
		// If there's an error, it should NOT be about invalid QPS/Burst
		assert.NotContains(t, err.Error(), "invalid kubedog QPS")
		assert.NotContains(t, err.Error(), "invalid kubedog burst")
	} else {
		// If no error, tracker should be created successfully
		assert.NotNil(t, tr)
	}
}
