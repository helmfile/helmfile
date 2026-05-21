package kubedog

import (
	"bytes"
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/werf/kubedog/pkg/trackers/dyntracker/logstore"
	"github.com/werf/kubedog/pkg/trackers/dyntracker/statestore"
	kdutil "github.com/werf/kubedog/pkg/trackers/dyntracker/util"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"

	"github.com/helmfile/helmfile/pkg/resource"
)

func TestDetectPodFailureReason(t *testing.T) {
	tests := []struct {
		name string
		obj  map[string]any
		want string
	}{
		{
			name: "phase Failed",
			obj:  map[string]any{"status": map[string]any{"phase": "Failed"}},
			want: "Failed",
		},
		{
			name: "container waiting CrashLoopBackOff",
			obj: map[string]any{"status": map[string]any{
				"phase": "Running",
				"containerStatuses": []any{
					map[string]any{"state": map[string]any{
						"waiting": map[string]any{"reason": "CrashLoopBackOff"},
					}},
				},
			}},
			want: "CrashLoopBackOff",
		},
		{
			name: "container waiting ImagePullBackOff",
			obj: map[string]any{"status": map[string]any{
				"containerStatuses": []any{
					map[string]any{"state": map[string]any{
						"waiting": map[string]any{"reason": "ImagePullBackOff"},
					}},
				},
			}},
			want: "ImagePullBackOff",
		},
		{
			name: "container terminated OOMKilled",
			obj: map[string]any{"status": map[string]any{
				"containerStatuses": []any{
					map[string]any{"state": map[string]any{
						"terminated": map[string]any{"reason": "OOMKilled"},
					}},
				},
			}},
			want: "OOMKilled",
		},
		{
			name: "init container failing surfaces with init: prefix",
			obj: map[string]any{"status": map[string]any{
				"initContainerStatuses": []any{
					map[string]any{"state": map[string]any{
						"waiting": map[string]any{"reason": "CrashLoopBackOff"},
					}},
				},
			}},
			want: "init: CrashLoopBackOff",
		},
		{
			name: "running pod with no waiting/terminated reason — healthy",
			obj: map[string]any{"status": map[string]any{
				"phase": "Running",
				"containerStatuses": []any{
					map[string]any{"state": map[string]any{
						"running": map[string]any{},
					}},
				},
			}},
			want: "",
		},
		{
			name: "empty status block",
			obj:  map[string]any{},
			want: "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := detectPodFailureReason(&unstructured.Unstructured{Object: tc.obj})
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestExtractPodSelector(t *testing.T) {
	t.Run("matchLabels round-trips through SelectorFromSet", func(t *testing.T) {
		obj := &unstructured.Unstructured{Object: map[string]any{
			"spec": map[string]any{
				"selector": map[string]any{
					"matchLabels": map[string]any{
						"app": "malware", "component": "worker",
					},
				},
			},
		}}
		got := extractPodSelector(obj)
		// SelectorFromSet sorts keys, so the output is deterministic.
		assert.Equal(t, "app=malware,component=worker", got)
	})
	t.Run("missing selector returns empty string", func(t *testing.T) {
		assert.Equal(t, "", extractPodSelector(&unstructured.Unstructured{Object: map[string]any{}}))
	})
	t.Run("empty matchLabels returns empty string", func(t *testing.T) {
		obj := &unstructured.Unstructured{Object: map[string]any{
			"spec": map[string]any{
				"selector": map[string]any{
					"matchLabels": map[string]any{},
				},
			},
		}}
		assert.Equal(t, "", extractPodSelector(obj))
	})
}

func TestPodIsInTaskStore(t *testing.T) {
	// Build a task store with one Deployment that has one tracked Pod child.
	deployGVK := schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}
	taskStore := kdutil.NewConcurrent(statestore.NewTaskStore())
	ts := statestore.NewReadinessTaskState("app", "ns", deployGVK, statestore.ReadinessTaskStateOptions{})
	ts.AddResourceState("tracked-pod", "ns", watchdogPodGVK)
	ts.AddDependency(ts.Name(), ts.Namespace(), ts.GroupVersionKind(), "tracked-pod", "ns", watchdogPodGVK)
	taskStore.RWTransaction(func(s *statestore.TaskStore) {
		s.AddReadinessTaskState(kdutil.NewConcurrent(ts))
	})

	assert.True(t, podIsInTaskStore(taskStore, "tracked-pod", "ns"),
		"tracked pod must be found in the task store")
	assert.False(t, podIsInTaskStore(taskStore, "untracked-pod", "ns"),
		"pod the watchdog will need to surface must NOT be found in the task store")
	// Different namespace must not match a same-named pod.
	assert.False(t, podIsInTaskStore(taskStore, "tracked-pod", "other-ns"),
		"pod lookup must be namespace-scoped")
}

// newCaptureLogger returns a SugaredLogger whose Warn-level writes are
// captured into a buffer — sufficient to verify the watchdog actually emits
// the expected warning text.
func newCaptureLogger(t *testing.T) (*zap.SugaredLogger, *bytes.Buffer) {
	t.Helper()
	buf := &bytes.Buffer{}
	var cfg zapcore.EncoderConfig
	cfg.MessageKey = "message"
	core := zapcore.NewCore(zapcore.NewConsoleEncoder(cfg), zapcore.AddSync(&captureWriter{buf: buf, mu: &sync.Mutex{}}), zapcore.WarnLevel)
	return zap.New(core).Sugar(), buf
}

type captureWriter struct {
	buf *bytes.Buffer
	mu  *sync.Mutex
}

func (w *captureWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.Write(p)
}

func TestScanForMissedFailures_WarnsForUntrackedFailingPod(t *testing.T) {
	// Cluster contains:
	//   - Deployment "malware" with selector app=malware
	//   - Pod "malware-pod-untracked" (the one dyntracker missed linking)
	//     in CrashLoopBackOff
	// Task store has the Deployment but no Pod children — simulating the
	// linkage race the watchdog is built to mitigate.
	deployGVK := schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}
	deployGVR := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}

	deployObj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata":   map[string]any{"name": "malware", "namespace": "ns"},
		"spec": map[string]any{
			"selector": map[string]any{
				"matchLabels": map[string]any{"app": "malware"},
			},
			"replicas": int64(1),
		},
	}}
	failingPod := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]any{
			"name":      "malware-pod-untracked",
			"namespace": "ns",
			"labels":    map[string]any{"app": "malware"},
		},
		"status": map[string]any{
			"phase": "Running",
			"containerStatuses": []any{
				map[string]any{"state": map[string]any{
					"waiting": map[string]any{"reason": "CrashLoopBackOff"},
				}},
			},
		},
	}}

	scheme := runtime.NewScheme()
	scheme.AddKnownTypeWithName(watchdogPodGVK.GroupVersion().WithKind("PodList"), &unstructured.UnstructuredList{})
	scheme.AddKnownTypeWithName(deployGVK.GroupVersion().WithKind("DeploymentList"), &unstructured.UnstructuredList{})
	fakeClient := fake.NewSimpleDynamicClient(scheme, deployObj, failingPod)

	logger, buf := newCaptureLogger(t)
	tr := &Tracker{
		logger:        logger,
		dynamicClient: fakeClient,
		mapper: &staticRESTMapper{mappings: map[schema.GroupVersionKind]schema.GroupVersionResource{
			deployGVK:      deployGVR,
			watchdogPodGVK: watchdogPodGVR,
		}},
	}

	// Empty task store — dyntracker has no record of this pod.
	taskStore := kdutil.NewConcurrent(statestore.NewTaskStore())
	_ = logstore.NewLogStore()

	warned := map[string]struct{}{}
	tr.scanForMissedFailures(context.Background(), taskStore, []watchdogWorkload{
		{kind: "deploy", gvk: deployGVK, name: "malware", namespace: "ns"},
	}, warned)

	out := buf.String()
	require.Contains(t, out, "malware-pod-untracked",
		"watchdog must name the missing failing pod")
	require.Contains(t, out, "CrashLoopBackOff",
		"watchdog must name the failure reason")
	require.Contains(t, out, "kubectl",
		"watchdog must give the operator an actionable inspection command")
}

func TestScanForMissedFailures_StaysQuietWhenDyntrackerAlreadyHasPod(t *testing.T) {
	// Same setup as the previous test, except the task store DOES contain
	// the failing pod — so dyntracker is already surfacing it via the
	// normal progress/log pipeline, and the watchdog should stay silent
	// to avoid duplicate output.
	deployGVK := schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}
	deployGVR := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}

	deployObj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata":   map[string]any{"name": "app", "namespace": "ns"},
		"spec": map[string]any{
			"selector": map[string]any{"matchLabels": map[string]any{"app": "app"}},
			"replicas": int64(1),
		},
	}}
	failingPod := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata":   map[string]any{"name": "app-pod", "namespace": "ns", "labels": map[string]any{"app": "app"}},
		"status": map[string]any{
			"containerStatuses": []any{
				map[string]any{"state": map[string]any{
					"waiting": map[string]any{"reason": "CrashLoopBackOff"},
				}},
			},
		},
	}}

	scheme := runtime.NewScheme()
	fakeClient := fake.NewSimpleDynamicClient(scheme, deployObj, failingPod)

	logger, buf := newCaptureLogger(t)
	tr := &Tracker{
		logger:        logger,
		dynamicClient: fakeClient,
		mapper: &staticRESTMapper{mappings: map[schema.GroupVersionKind]schema.GroupVersionResource{
			deployGVK:      deployGVR,
			watchdogPodGVK: watchdogPodGVR,
		}},
	}

	taskStore := kdutil.NewConcurrent(statestore.NewTaskStore())
	ts := statestore.NewReadinessTaskState("app", "ns", deployGVK, statestore.ReadinessTaskStateOptions{})
	ts.AddResourceState("app-pod", "ns", watchdogPodGVK)
	ts.AddDependency(ts.Name(), ts.Namespace(), ts.GroupVersionKind(), "app-pod", "ns", watchdogPodGVK)
	taskStore.RWTransaction(func(s *statestore.TaskStore) {
		s.AddReadinessTaskState(kdutil.NewConcurrent(ts))
	})

	warned := map[string]struct{}{}
	tr.scanForMissedFailures(context.Background(), taskStore, []watchdogWorkload{
		{kind: "deploy", gvk: deployGVK, name: "app", namespace: "ns"},
	}, warned)

	out := buf.String()
	assert.NotContains(t, out, "watchdog",
		"watchdog must not warn when dyntracker is already tracking the failing pod")
	assert.NotContains(t, out, "app-pod", "no per-pod warning expected")
}

func TestScanForMissedFailures_DoesNotRepeatWarningsAcrossScans(t *testing.T) {
	// Same untracked-failing-pod scenario as the first test, but we run two
	// consecutive scans and assert the warning appears exactly once. The
	// per-pod dedup keeps the periodic ticker from spamming the same line
	// every minute.
	deployGVK := schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}
	deployGVR := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}

	deployObj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata":   map[string]any{"name": "malware", "namespace": "ns"},
		"spec": map[string]any{
			"selector": map[string]any{"matchLabels": map[string]any{"app": "malware"}},
			"replicas": int64(1),
		},
	}}
	failingPod := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]any{
			"name":      "malware-pod-untracked",
			"namespace": "ns",
			"labels":    map[string]any{"app": "malware"},
		},
		"status": map[string]any{
			"containerStatuses": []any{
				map[string]any{"state": map[string]any{
					"waiting": map[string]any{"reason": "CrashLoopBackOff"},
				}},
			},
		},
	}}

	scheme := runtime.NewScheme()
	fakeClient := fake.NewSimpleDynamicClient(scheme, deployObj, failingPod)

	logger, buf := newCaptureLogger(t)
	tr := &Tracker{
		logger:        logger,
		dynamicClient: fakeClient,
		mapper: &staticRESTMapper{mappings: map[schema.GroupVersionKind]schema.GroupVersionResource{
			deployGVK:      deployGVR,
			watchdogPodGVK: watchdogPodGVR,
		}},
	}
	taskStore := kdutil.NewConcurrent(statestore.NewTaskStore())

	warned := map[string]struct{}{}
	workloads := []watchdogWorkload{{kind: "deploy", gvk: deployGVK, name: "malware", namespace: "ns"}}
	tr.scanForMissedFailures(context.Background(), taskStore, workloads, warned)
	tr.scanForMissedFailures(context.Background(), taskStore, workloads, warned)

	// The warning template mentions the pod name twice (once in the message,
	// once in the kubectl-logs hint), so count the "[watchdog]" prefix
	// instead — that's emitted exactly once per warning line.
	count := strings.Count(buf.String(), "[watchdog]")
	assert.Equal(t, 1, count, "watchdog must warn at most once per pod across consecutive scans")
}

func TestWatchdogWorkloads_FiltersUntrackableKinds(t *testing.T) {
	in := []*resource.Resource{
		{Kind: "Deployment", Name: "d", Namespace: "ns"},
		{Kind: "ConfigMap", Name: "cm", Namespace: "ns"},     // not tracked at all
		{Kind: "PersistentVolumeClaim", Name: "p", Namespace: "ns"}, // tracked but no pods
		{Kind: "Job", Name: "j", Namespace: "ns"},
		{Kind: "Canary", Name: "c", Namespace: "ns"}, // tracked but complex pod set; skip
	}
	got := watchdogWorkloads(in)
	require.Len(t, got, 2)
	assert.Equal(t, "deploy", got[0].kind)
	assert.Equal(t, "job", got[1].kind)
}
