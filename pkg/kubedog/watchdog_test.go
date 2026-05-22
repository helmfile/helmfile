package kubedog

import (
	"bytes"
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// newWatchdogFakeClient registers the list kinds the watchdog actually
// needs (Pods, Jobs, Deployments, ReplicaSets). The fake dynamic client
// panics on List for unregistered list kinds, so this helper is shared by
// the integration-style tests below.
func newWatchdogFakeClient(t *testing.T, objs ...runtime.Object) *fake.FakeDynamicClient {
	t.Helper()
	scheme := runtime.NewScheme()
	for _, kind := range []schema.GroupVersionKind{
		{Group: "", Version: "v1", Kind: "PodList"},
		{Group: "batch", Version: "v1", Kind: "JobList"},
		{Group: "apps", Version: "v1", Kind: "DeploymentList"},
		{Group: "apps", Version: "v1", Kind: "ReplicaSetList"},
	} {
		scheme.AddKnownTypeWithName(kind, &unstructured.UnstructuredList{})
	}
	return fake.NewSimpleDynamicClient(scheme, objs...)
}

func TestScanForMissedFailures_WarnsForUntrackedFailingPod(t *testing.T) {
	// Cluster contains a Job and one failing pod owned by it (the one
	// dyntracker missed linking). Task store has the Job but no Pod
	// children — simulating the linkage race the watchdog is built to
	// mitigate. Using a Job keeps the ownership chain direct (Pod → Job).
	jobGVK := schema.GroupVersionKind{Group: "batch", Version: "v1", Kind: "Job"}
	jobGVR := schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "jobs"}
	const jobUID = "11111111-1111-1111-1111-111111111111"

	jobObj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "batch/v1",
		"kind":       "Job",
		"metadata":   map[string]any{"name": "malware", "namespace": "ns", "uid": jobUID},
		"spec": map[string]any{
			"selector": map[string]any{"matchLabels": map[string]any{"app": "malware"}},
		},
	}}
	failingPod := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]any{
			"name":      "malware-pod-untracked",
			"namespace": "ns",
			"labels":    map[string]any{"app": "malware"},
			"ownerReferences": []any{
				map[string]any{"apiVersion": "batch/v1", "kind": "Job", "name": "malware", "uid": jobUID, "controller": true},
			},
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

	logger, buf := newCaptureLogger(t)
	tr := &Tracker{
		logger:        logger,
		dynamicClient: newWatchdogFakeClient(t, jobObj, failingPod),
		mapper: &staticRESTMapper{mappings: map[schema.GroupVersionKind]schema.GroupVersionResource{
			jobGVK:         jobGVR,
			watchdogPodGVK: watchdogPodGVR,
		}},
		trackOptions: &TrackOptions{},
	}

	taskStore := kdutil.NewConcurrent(statestore.NewTaskStore())
	warned := map[string]struct{}{}
	tr.scanForMissedFailures(context.Background(), taskStore, []watchdogWorkload{
		{kind: "job", gvk: jobGVK, name: "malware", namespace: "ns"},
	}, warned)

	out := buf.String()
	require.Contains(t, out, "malware-pod-untracked", "watchdog must name the missing failing pod")
	require.Contains(t, out, "CrashLoopBackOff", "watchdog must name the failure reason")
	require.Contains(t, out, "kubectl", "watchdog must give the operator an actionable inspection command")
}

func TestScanForMissedFailures_IgnoresStalePodFromPreviousInstall(t *testing.T) {
	// Reproduces the "stale failing pod" false positive: the current Job
	// has its own UID, but a leftover failing pod from the previous install
	// (with a different owner UID) still matches the label selector. The
	// watchdog must skip it — that's not "our" pod and helm will clean it
	// up shortly anyway.
	jobGVK := schema.GroupVersionKind{Group: "batch", Version: "v1", Kind: "Job"}
	jobGVR := schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "jobs"}
	const currentJobUID = "22222222-2222-2222-2222-222222222222"
	const previousJobUID = "33333333-3333-3333-3333-333333333333"

	currentJob := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "batch/v1",
		"kind":       "Job",
		"metadata":   map[string]any{"name": "feeds-db-insert-init", "namespace": "ns", "uid": currentJobUID},
		"spec": map[string]any{
			"selector": map[string]any{"matchLabels": map[string]any{"app": "feeds-db-insert-init"}},
		},
	}}
	stalePod := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]any{
			"name":      "feeds-db-insert-init-cbxvc",
			"namespace": "ns",
			"labels":    map[string]any{"app": "feeds-db-insert-init"},
			"ownerReferences": []any{
				map[string]any{"apiVersion": "batch/v1", "kind": "Job", "name": "feeds-db-insert-init", "uid": previousJobUID, "controller": true},
			},
		},
		"status": map[string]any{"phase": "Failed"},
	}}

	logger, buf := newCaptureLogger(t)
	tr := &Tracker{
		logger:        logger,
		dynamicClient: newWatchdogFakeClient(t, currentJob, stalePod),
		mapper: &staticRESTMapper{mappings: map[schema.GroupVersionKind]schema.GroupVersionResource{
			jobGVK:         jobGVR,
			watchdogPodGVK: watchdogPodGVR,
		}},
		trackOptions: &TrackOptions{},
	}

	taskStore := kdutil.NewConcurrent(statestore.NewTaskStore())
	warned := map[string]struct{}{}
	tr.scanForMissedFailures(context.Background(), taskStore, []watchdogWorkload{
		{kind: "job", gvk: jobGVK, name: "feeds-db-insert-init", namespace: "ns"},
	}, warned)

	assert.NotContains(t, buf.String(), "feeds-db-insert-init-cbxvc",
		"watchdog must NOT warn about a failing pod left over from a previous install (different owner UID)")
}

func TestScanForMissedFailures_StaysQuietWhenDyntrackerAlreadyHasPod(t *testing.T) {
	// Task store DOES contain the failing pod, so dyntracker is already
	// surfacing it via the normal pipeline. Watchdog stays silent.
	jobGVK := schema.GroupVersionKind{Group: "batch", Version: "v1", Kind: "Job"}
	jobGVR := schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "jobs"}
	const jobUID = "44444444-4444-4444-4444-444444444444"

	jobObj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "batch/v1",
		"kind":       "Job",
		"metadata":   map[string]any{"name": "app", "namespace": "ns", "uid": jobUID},
		"spec":       map[string]any{"selector": map[string]any{"matchLabels": map[string]any{"app": "app"}}},
	}}
	failingPod := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]any{
			"name": "app-pod", "namespace": "ns",
			"labels": map[string]any{"app": "app"},
			"ownerReferences": []any{
				map[string]any{"apiVersion": "batch/v1", "kind": "Job", "name": "app", "uid": jobUID, "controller": true},
			},
		},
		"status": map[string]any{
			"containerStatuses": []any{
				map[string]any{"state": map[string]any{
					"waiting": map[string]any{"reason": "CrashLoopBackOff"},
				}},
			},
		},
	}}

	logger, buf := newCaptureLogger(t)
	tr := &Tracker{
		logger:        logger,
		dynamicClient: newWatchdogFakeClient(t, jobObj, failingPod),
		mapper: &staticRESTMapper{mappings: map[schema.GroupVersionKind]schema.GroupVersionResource{
			jobGVK:         jobGVR,
			watchdogPodGVK: watchdogPodGVR,
		}},
		trackOptions: &TrackOptions{},
	}

	taskStore := kdutil.NewConcurrent(statestore.NewTaskStore())
	ts := statestore.NewReadinessTaskState("app", "ns", jobGVK, statestore.ReadinessTaskStateOptions{})
	ts.AddResourceState("app-pod", "ns", watchdogPodGVK)
	ts.AddDependency(ts.Name(), ts.Namespace(), ts.GroupVersionKind(), "app-pod", "ns", watchdogPodGVK)
	taskStore.RWTransaction(func(s *statestore.TaskStore) {
		s.AddReadinessTaskState(kdutil.NewConcurrent(ts))
	})

	warned := map[string]struct{}{}
	tr.scanForMissedFailures(context.Background(), taskStore, []watchdogWorkload{
		{kind: "job", gvk: jobGVK, name: "app", namespace: "ns"},
	}, warned)

	assert.NotContains(t, buf.String(), "watchdog", "watchdog must not warn when dyntracker is already tracking the failing pod")
}

func TestScanForMissedFailures_DoesNotRepeatWarningsAcrossScans(t *testing.T) {
	jobGVK := schema.GroupVersionKind{Group: "batch", Version: "v1", Kind: "Job"}
	jobGVR := schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "jobs"}
	const jobUID = "55555555-5555-5555-5555-555555555555"

	jobObj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "batch/v1",
		"kind":       "Job",
		"metadata":   map[string]any{"name": "malware", "namespace": "ns", "uid": jobUID},
		"spec":       map[string]any{"selector": map[string]any{"matchLabels": map[string]any{"app": "malware"}}},
	}}
	failingPod := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]any{
			"name": "malware-pod-untracked", "namespace": "ns",
			"labels": map[string]any{"app": "malware"},
			"ownerReferences": []any{
				map[string]any{"apiVersion": "batch/v1", "kind": "Job", "name": "malware", "uid": jobUID, "controller": true},
			},
		},
		"status": map[string]any{
			"containerStatuses": []any{
				map[string]any{"state": map[string]any{
					"waiting": map[string]any{"reason": "CrashLoopBackOff"},
				}},
			},
		},
	}}

	logger, buf := newCaptureLogger(t)
	tr := &Tracker{
		logger:        logger,
		dynamicClient: newWatchdogFakeClient(t, jobObj, failingPod),
		mapper: &staticRESTMapper{mappings: map[schema.GroupVersionKind]schema.GroupVersionResource{
			jobGVK:         jobGVR,
			watchdogPodGVK: watchdogPodGVR,
		}},
		trackOptions: &TrackOptions{},
	}
	taskStore := kdutil.NewConcurrent(statestore.NewTaskStore())
	warned := map[string]struct{}{}
	workloads := []watchdogWorkload{{kind: "job", gvk: jobGVK, name: "malware", namespace: "ns"}}
	tr.scanForMissedFailures(context.Background(), taskStore, workloads, warned)
	tr.scanForMissedFailures(context.Background(), taskStore, workloads, warned)

	count := strings.Count(buf.String(), "[watchdog]")
	assert.Equal(t, 1, count, "watchdog must warn at most once per pod across consecutive scans")
}

func TestScanForMissedFailures_DeploymentPodMatchesViaReplicaSet(t *testing.T) {
	// Deployment pods are not directly owned by the workload; the watchdog
	// must walk Pod → ReplicaSet → Deployment to recognize a failing pod
	// as "ours".
	deployGVK := schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}
	deployGVR := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	const deployUID = "66666666-6666-6666-6666-666666666666"
	const rsUID = "77777777-7777-7777-7777-777777777777"

	deploy := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata":   map[string]any{"name": "app", "namespace": "ns", "uid": deployUID},
		"spec": map[string]any{
			"selector": map[string]any{"matchLabels": map[string]any{"app": "app"}},
			"replicas": int64(1),
		},
	}}
	rs := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apps/v1",
		"kind":       "ReplicaSet",
		"metadata": map[string]any{
			"name": "app-abc", "namespace": "ns", "uid": rsUID,
			"ownerReferences": []any{
				map[string]any{"apiVersion": "apps/v1", "kind": "Deployment", "name": "app", "uid": deployUID, "controller": true},
			},
		},
	}}
	failingPod := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]any{
			"name": "app-abc-xyz", "namespace": "ns",
			"labels": map[string]any{"app": "app"},
			"ownerReferences": []any{
				map[string]any{"apiVersion": "apps/v1", "kind": "ReplicaSet", "name": "app-abc", "uid": rsUID, "controller": true},
			},
		},
		"status": map[string]any{
			"containerStatuses": []any{
				map[string]any{"state": map[string]any{
					"waiting": map[string]any{"reason": "CrashLoopBackOff"},
				}},
			},
		},
	}}

	logger, buf := newCaptureLogger(t)
	tr := &Tracker{
		logger:        logger,
		dynamicClient: newWatchdogFakeClient(t, deploy, rs, failingPod),
		mapper: &staticRESTMapper{mappings: map[schema.GroupVersionKind]schema.GroupVersionResource{
			deployGVK:      deployGVR,
			watchdogPodGVK: watchdogPodGVR,
		}},
		trackOptions: &TrackOptions{},
	}
	taskStore := kdutil.NewConcurrent(statestore.NewTaskStore())
	warned := map[string]struct{}{}
	tr.scanForMissedFailures(context.Background(), taskStore, []watchdogWorkload{
		{kind: "deploy", gvk: deployGVK, name: "app", namespace: "ns"},
	}, warned)

	assert.Contains(t, buf.String(), "app-abc-xyz",
		"watchdog must recognize a failing Deployment pod via the Pod → RS → Deployment chain")
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
