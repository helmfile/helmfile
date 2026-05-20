package kubedog

import (
	"math"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

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

func TestClassifyResource(t *testing.T) {
	tests := []struct {
		rawKind  string
		wantKind string
		wantGVK  schema.GroupVersionKind
		wantOK   bool
	}{
		{"Deployment", "deploy", schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}, true},
		{"deployment", "deploy", schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}, true},
		{"deploy", "deploy", schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}, true},
		{"StatefulSet", "sts", schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "StatefulSet"}, true},
		{"sts", "sts", schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "StatefulSet"}, true},
		{"DaemonSet", "ds", schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "DaemonSet"}, true},
		{"Job", "job", schema.GroupVersionKind{Group: "batch", Version: "v1", Kind: "Job"}, true},
		{"Canary", "canary", schema.GroupVersionKind{Group: "flagger.app", Version: "v1beta1", Kind: "Canary"}, true},
		{"PersistentVolumeClaim", "pvc", schema.GroupVersionKind{Group: "", Version: "v1", Kind: "PersistentVolumeClaim"}, true},
		{"pvc", "pvc", schema.GroupVersionKind{Group: "", Version: "v1", Kind: "PersistentVolumeClaim"}, true},
		{"ConfigMap", "", schema.GroupVersionKind{}, false},
		{"Service", "", schema.GroupVersionKind{}, false},
		{"", "", schema.GroupVersionKind{}, false},
	}
	for _, tc := range tests {
		t.Run(tc.rawKind, func(t *testing.T) {
			kind, gvk, ok := classifyResource(tc.rawKind)
			assert.Equal(t, tc.wantOK, ok)
			assert.Equal(t, tc.wantKind, kind)
			assert.Equal(t, tc.wantGVK, gvk)
		})
	}
}

func TestBaselineKey(t *testing.T) {
	assert.Equal(t, "deploy/ns/name", BaselineKey("deploy", "ns", "name"))
	assert.Equal(t, "job//standalone", BaselineKey("job", "", "standalone"))
}

func TestResourceBaseline_DefaultsAreZeroValue(t *testing.T) {
	var b ResourceBaseline
	assert.Equal(t, types.UID(""), b.UID)
	assert.Equal(t, int64(0), b.Generation)
	assert.False(t, b.Exists)
}

func TestTrackOptions_WithColor(t *testing.T) {
	opts := NewTrackOptions()
	assert.False(t, opts.Color)

	opts = opts.WithColor(true)
	assert.True(t, opts.Color)
}

func TestTrackOptions_WithFailedLogsOnly(t *testing.T) {
	opts := NewTrackOptions()
	assert.False(t, opts.FailedLogsOnly)

	opts = opts.WithFailedLogsOnly(true)
	assert.True(t, opts.FailedLogsOnly)
}

func TestTrackOptions_WithBaselines(t *testing.T) {
	opts := NewTrackOptions()
	assert.Nil(t, opts.Baselines)

	baselines := map[string]ResourceBaseline{
		BaselineKey("deploy", "ns", "foo"): {UID: "uid-a", Generation: 3, Exists: true},
	}
	opts = opts.WithBaselines(baselines)
	assert.Equal(t, baselines, opts.Baselines)

	// Mutating the original map after the call must be visible through the
	// stored field — confirms the setter retains the map by reference rather
	// than copying.
	baselines[BaselineKey("deploy", "ns", "bar")] = ResourceBaseline{Generation: 5}
	assert.Contains(t, opts.Baselines, BaselineKey("deploy", "ns", "bar"))
}

func TestTrackOptions_Chaining_AllSetters(t *testing.T) {
	filter := &resource.FilterConfig{TrackKinds: []string{"Deployment"}}
	baselines := map[string]ResourceBaseline{
		BaselineKey("deploy", "ns", "foo"): {UID: "uid-a", Generation: 1, Exists: true},
	}
	opts := NewTrackOptions().
		WithTimeout(2*time.Minute).
		WithLogs(true).
		WithFailedLogsOnly(true).
		WithFilterConfig(filter).
		WithQPS(50).
		WithBurst(80).
		WithColor(true).
		WithBaselines(baselines)

	assert.Equal(t, 2*time.Minute, opts.Timeout)
	assert.True(t, opts.Logs)
	assert.True(t, opts.FailedLogsOnly)
	assert.Equal(t, filter, opts.Filter)
	assert.Equal(t, float32(50), opts.QPS)
	assert.Equal(t, 80, opts.Burst)
	assert.True(t, opts.Color)
	assert.Equal(t, baselines, opts.Baselines)
}
