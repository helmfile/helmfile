package kubedog

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/werf/kubedog/pkg/informer"
	kdutil "github.com/werf/kubedog/pkg/trackers/dyntracker/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic/fake"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
)

func newLogFilterPod(name, namespace, uid, readyStatus string, readyTime time.Time) *unstructured.Unstructured {
	pod := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata":   map[string]any{"name": name, "namespace": namespace, "uid": uid},
		"status":     map[string]any{"phase": "Running"},
	}}
	if readyStatus != "" {
		condition := map[string]any{"type": "Ready", "status": readyStatus}
		if !readyTime.IsZero() {
			condition["lastTransitionTime"] = readyTime.Format(time.RFC3339Nano)
		}
		pod.Object["status"].(map[string]any)["conditions"] = []any{condition}
	}
	return pod
}

func TestPodLogFilter_ReadyCondition(t *testing.T) {
	readyTime := time.Date(2026, 10, 1, 12, 0, 0, 123, time.UTC)
	for _, status := range []string{"", "False", "Unknown", "True"} {
		t.Run("Ready="+status, func(t *testing.T) {
			filter := newPodLogFilter()
			filter.update(newLogFilterPod("app", "ns", "uid", status, readyTime))
			cutoff := filter.cutoff("app", "ns", watchdogPodGVK)
			if status == "True" {
				assert.Equal(t, readyTime, cutoff)
			} else {
				assert.True(t, cutoff.IsZero(), "Running alone does not mean that a pod is ready")
			}
		})
	}
}

func TestPodLogFilter_FirstReadyTransitionIsRetained(t *testing.T) {
	filter := newPodLogFilter()
	firstReady := time.Date(2026, 10, 1, 12, 0, 0, 123, time.UTC)
	filter.update(newLogFilterPod("app", "ns", "uid", "True", firstReady))
	filter.update(newLogFilterPod("app", "ns", "uid", "False", firstReady.Add(time.Minute)))
	filter.update(newLogFilterPod("app", "ns", "uid", "True", firstReady.Add(2*time.Minute)))
	assert.Equal(t, firstReady, filter.cutoff("app", "ns", watchdogPodGVK))
}

func TestPodLogFilter_JobPodsAreExempt(t *testing.T) {
	filter := newPodLogFilter()
	pod := newLogFilterPod("job-pod", "ns", "uid", "True", time.Now())
	pod.SetOwnerReferences([]metav1.OwnerReference{{APIVersion: "batch/v1", Kind: "Job", Name: "job"}})
	filter.update(pod)
	assert.True(t, filter.cutoff("job-pod", "ns", watchdogPodGVK).IsZero())
}

func TestPodLogFilter_ReplacementPodStartsLoggingAgain(t *testing.T) {
	filter := newPodLogFilter()
	firstReady := time.Date(2026, 10, 1, 12, 0, 0, 123, time.UTC)
	filter.update(newLogFilterPod("app", "ns", "old-uid", "True", firstReady))
	filter.update(newLogFilterPod("app", "ns", "new-uid", "False", time.Time{}))
	assert.True(t, filter.cutoff("app", "ns", watchdogPodGVK).IsZero())

	nextReady := firstReady.Add(time.Minute)
	filter.update(newLogFilterPod("app", "ns", "new-uid", "True", nextReady))
	assert.Equal(t, nextReady, filter.cutoff("app", "ns", watchdogPodGVK))
}

func TestPodLogFilter_NamespacesAreIndependent(t *testing.T) {
	filter := newPodLogFilter()
	readyTime := time.Date(2026, 10, 1, 12, 0, 0, 123, time.UTC)
	filter.update(newLogFilterPod("app", "ready-ns", "uid", "True", readyTime))
	filter.update(newLogFilterPod("app", "starting-ns", "uid", "False", time.Time{}))
	assert.Equal(t, readyTime, filter.cutoff("app", "ready-ns", watchdogPodGVK))
	assert.True(t, filter.cutoff("app", "starting-ns", watchdogPodGVK).IsZero())
}

func TestPodLogFilter_PreservesReadinessSecond(t *testing.T) {
	filter := newPodLogFilter()
	readyTime := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	filter.update(newLogFilterPod("app", "ns", "uid", "True", readyTime))
	assert.Equal(t, readyTime.Add(time.Second-time.Nanosecond), filter.cutoff("app", "ns", watchdogPodGVK))
}

func TestPodLogFilter_MissingOrInvalidTransitionTime(t *testing.T) {
	for _, timestamp := range []string{"", "invalid"} {
		t.Run(timestamp, func(t *testing.T) {
			filter := newPodLogFilter()
			pod := newLogFilterPod("app", "ns", "uid", "True", time.Time{})
			conditions := pod.Object["status"].(map[string]any)["conditions"].([]any)
			conditions[0].(map[string]any)["lastTransitionTime"] = timestamp
			before := time.Now()
			filter.update(pod)
			after := time.Now()
			cutoff := filter.cutoff("app", "ns", watchdogPodGVK)
			assert.False(t, cutoff.Before(before))
			assert.False(t, cutoff.After(after))
		})
	}
}

func TestPodLogFilter_WatchesExistingPodsAndUpdatesUsingSharedInformer(t *testing.T) {
	ctx := t.Context()
	podGVR := schema.GroupVersionResource{Version: "v1", Resource: "pods"}
	readyTime := time.Date(2026, 10, 1, 12, 0, 0, 123, time.UTC)
	readyPod := newLogFilterPod("ready", "ns", "ready-uid", "True", readyTime)
	startingPod := newLogFilterPod("starting", "ns", "starting-uid", "False", time.Time{})
	client := fake.NewSimpleDynamicClientWithCustomListKinds(
		runtime.NewScheme(), map[schema.GroupVersionResource]string{podGVR: "PodList"}, readyPod, startingPod,
	)
	var watches atomic.Int32
	client.PrependWatchReactor("pods", func(clienttesting.Action) (bool, watch.Interface, error) {
		watches.Add(1)
		return false, nil, nil
	})
	factory := informer.NewConcurrentInformerFactory(ctx.Done(), make(chan error, 1), client,
		informer.ConcurrentInformerFactoryOptions{})

	// Start another consumer first, as kubedog does during resource tracking.
	var shared *kdutil.Concurrent[*informer.Informer]
	var err error
	factory.RWTransaction(func(factory *informer.InformerFactory) {
		shared, err = factory.ForNamespace(podGVR, "ns")
	})
	require.NoError(t, err)
	shared.RWTransaction(func(inf *informer.Informer) {
		_, err = inf.AddEventHandler(cache.ResourceEventHandlerFuncs{})
		if err == nil {
			inf.Run()
		}
	})
	require.NoError(t, err)

	filter := newPodLogFilter()
	require.NoError(t, filter.watch(factory, []trackTarget{
		{kind: "deploy", namespace: "ns"},
		{kind: "sts", namespace: "ns"},
		{kind: "job", namespace: "jobs"},
		{kind: "pvc", namespace: "storage"},
		{kind: "canary", namespace: "canaries"},
	}))
	require.Eventually(t, func() bool {
		return filter.cutoff("ready", "ns", watchdogPodGVK).Equal(readyTime)
	}, 5*time.Second, 10*time.Millisecond, "initial add must retain the pod's historical readiness time")
	assert.True(t, filter.cutoff("starting", "ns", watchdogPodGVK).IsZero())

	nextReady := readyTime.Add(time.Minute)
	startingPod = newLogFilterPod("starting", "ns", "starting-uid", "True", nextReady)
	_, err = client.Resource(podGVR).Namespace("ns").Update(ctx, startingPod, metav1.UpdateOptions{})
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		return filter.cutoff("starting", "ns", watchdogPodGVK).Equal(nextReady)
	}, 5*time.Second, 10*time.Millisecond, "updates must stop logs independently of the other pod")
	assert.EqualValues(t, 1, watches.Load(), "consumers and repeated targets must share one pod watch")
}
