package kubedog

import (
	"fmt"
	"sync"
	"time"

	"github.com/werf/kubedog/pkg/informer"
	kdutil "github.com/werf/kubedog/pkg/trackers/dyntracker/util"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
)

type podLogCutoff struct {
	uid  types.UID
	time time.Time
}

// podLogFilter remembers the first Ready transition independently of kubedog's
// workload readiness. It only filters output; pod tracking and log capture continue.
type podLogFilter struct {
	mu      sync.RWMutex
	cutoffs map[string]podLogCutoff
}

func newPodLogFilter() *podLogFilter {
	return &podLogFilter{cutoffs: make(map[string]podLogCutoff)}
}

func (f *podLogFilter) watch(factory *kdutil.Concurrent[*informer.InformerFactory], targets []trackTarget) error {
	namespaces := make(map[string]struct{})
	for _, target := range targets {
		switch target.kind {
		case "deploy", "sts", "ds":
		default:
			continue
		}
		if _, watched := namespaces[target.namespace]; watched {
			continue
		}
		var podInformer *kdutil.Concurrent[*informer.Informer]
		var err error
		factory.RWTransaction(func(factory *informer.InformerFactory) {
			podInformer, err = factory.ForNamespace(schema.GroupVersionResource{Version: "v1", Resource: "pods"}, target.namespace)
		})
		if err != nil {
			return fmt.Errorf("create pod log informer: %w", err)
		}
		podInformer.RWTransaction(func(inf *informer.Informer) {
			_, err = inf.AddEventHandler(cache.ResourceEventHandlerFuncs{
				AddFunc: f.update,
				UpdateFunc: func(_, obj any) {
					f.update(obj)
				},
			})
			if err == nil {
				inf.Run()
			}
		})
		if err != nil {
			return fmt.Errorf("watch pod readiness for logs: %w", err)
		}
		namespaces[target.namespace] = struct{}{}
	}
	return nil
}

func (f *podLogFilter) update(obj any) {
	pod, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return
	}
	id := kdutil.ResourceID(pod.GetName(), pod.GetNamespace(), watchdogPodGVK)
	f.mu.Lock()
	defer f.mu.Unlock()
	if cutoff, found := f.cutoffs[id]; found {
		if cutoff.uid == pod.GetUID() {
			return
		}
		delete(f.cutoffs, id)
	}
	// Job pods can be Ready while still executing; keep their complete output.
	for _, owner := range pod.GetOwnerReferences() {
		if owner.Kind == "Job" {
			return
		}
	}
	conditions, _, err := unstructured.NestedSlice(pod.Object, "status", "conditions")
	if err != nil {
		return
	}
	for _, raw := range conditions {
		condition, ok := raw.(map[string]any)
		if !ok || condition["type"] != "Ready" || condition["status"] != "True" {
			continue
		}
		timestamp, _ := condition["lastTransitionTime"].(string)
		readyTime, err := time.Parse(time.RFC3339Nano, timestamp)
		if err != nil {
			readyTime = time.Now()
		} else if readyTime.Nanosecond() == 0 {
			// Kubernetes rounds Ready timestamps to seconds. Preserve startup
			// lines within that second too, even when delivered after readiness.
			readyTime = readyTime.Add(time.Second - time.Nanosecond)
		}
		f.cutoffs[id] = podLogCutoff{uid: pod.GetUID(), time: readyTime}
		return
	}
}

func (f *podLogFilter) cutoff(name, namespace string, gvk schema.GroupVersionKind) time.Time {
	if f == nil {
		return time.Time{}
	}
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.cutoffs[kdutil.ResourceID(name, namespace, gvk)].time
}
