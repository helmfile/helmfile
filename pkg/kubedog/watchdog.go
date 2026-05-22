package kubedog

import (
	"context"
	"fmt"
	"time"

	"github.com/werf/kubedog/pkg/trackers/dyntracker/statestore"
	kdutil "github.com/werf/kubedog/pkg/trackers/dyntracker/util"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	"github.com/helmfile/helmfile/pkg/resource"
)

// failureWatchdogInterval is how often we re-scan the cluster for failing
// pods that dyntracker hasn't surfaced. 60s keeps API server pressure
// negligible (one LIST per tracked workload per minute) while still
// catching CrashLoopBackOff loops well before --track-timeout fires.
const failureWatchdogInterval = 60 * time.Second

var (
	watchdogPodGVK        = schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"}
	watchdogPodGVR        = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	watchdogReplicaSetGVR = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "replicasets"}
)

// watchdogWorkload identifies a workload whose pods we'll independently
// LIST against the live API.
type watchdogWorkload struct {
	kind      string
	gvk       schema.GroupVersionKind
	name      string
	namespace string
}

// runFailureWatchdog watches for pods that genuinely failed in the cluster
// but never made it into dyntracker's resource graph. This case happens
// when dyntracker loses the Pod-to-Deployment linkage race (Pod CREATE
// event arriving before its ReplicaSet CREATE event) — the Pod is
// completely invisible to kubedog's tracking pipeline: no progress row,
// no log stream, no failed-only-mode entry. Without this watchdog, a Pod
// in CrashLoopBackOff under a missed link would be silent until
// --track-timeout, which is a real safety gap for CI pipelines that rely
// on helmfile to surface failures.
//
// The watchdog runs alongside dyntracker, does not gate any decisions, and
// only emits warnings. It cannot fix the linkage — but it can make sure
// failures are at least visible to the operator.
func (t *Tracker) runFailureWatchdog(ctx context.Context, taskStore *kdutil.Concurrent[*statestore.TaskStore], resources []*resource.Resource) {
	workloads := watchdogWorkloads(resources)
	if len(workloads) == 0 {
		return
	}

	// warned tracks pod IDs we've already surfaced so we don't repeat the
	// same warning every minute for the same failing pod.
	warned := map[string]struct{}{}

	ticker := time.NewTicker(failureWatchdogInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			t.scanForMissedFailures(ctx, taskStore, workloads, warned)
		}
	}
}

// watchdogWorkloads filters tracked resources down to the kinds whose
// pods we can reasonably LIST by selector. PVCs have no pods; Canaries
// have complex multi-revision pod sets we shouldn't second-guess.
func watchdogWorkloads(resources []*resource.Resource) []watchdogWorkload {
	var out []watchdogWorkload
	for _, res := range resources {
		kind, gvk, ok := classifyResource(res.Kind)
		if !ok {
			continue
		}
		if kind == "pvc" || kind == "canary" {
			continue
		}
		out = append(out, watchdogWorkload{kind: kind, gvk: gvk, name: res.Name, namespace: res.Namespace})
	}
	return out
}

func (t *Tracker) scanForMissedFailures(ctx context.Context, taskStore *kdutil.Concurrent[*statestore.TaskStore], workloads []watchdogWorkload, warned map[string]struct{}) {
	for _, wl := range workloads {
		gvr, err := t.gvrFor(wl.gvk)
		if err != nil {
			continue
		}
		obj, err := t.dynamicClient.Resource(gvr).Namespace(wl.namespace).Get(ctx, wl.name, metav1.GetOptions{})
		if err != nil {
			if !apierrors.IsNotFound(err) {
				t.logger.Debugf("kubedog watchdog: GET %s/%s/%s failed: %v", wl.kind, wl.namespace, wl.name, err)
			}
			continue
		}
		workloadUID := obj.GetUID()

		selector := extractPodSelector(obj)
		if selector == "" {
			continue
		}

		pods, err := t.dynamicClient.Resource(watchdogPodGVR).Namespace(wl.namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
		if err != nil {
			t.logger.Debugf("kubedog watchdog: LIST pods for %s/%s/%s failed: %v", wl.kind, wl.namespace, wl.name, err)
			continue
		}

		// For Deployments, pods are not directly owned by the workload —
		// they're owned by a ReplicaSet that's in turn owned by the
		// Deployment. We lazy-load the RS UIDs only if we actually have a
		// failing pod that doesn't match the workload UID directly.
		var deploymentRSUIDs map[string]struct{}

		for i := range pods.Items {
			pod := &pods.Items[i]
			podName := pod.GetName()
			podID := kdutil.ResourceID(podName, wl.namespace, watchdogPodGVK)
			if _, seen := warned[podID]; seen {
				continue
			}
			reason := detectPodFailureReason(pod)
			if reason == "" {
				continue
			}
			if podIsInTaskStore(taskStore, podName, wl.namespace) {
				continue
			}
			// Ownership check: the pod must belong to the current workload,
			// not to a previous instance whose pods still share the same
			// labels (e.g., a failed pre-install hook whose Job hasn't been
			// cleaned up yet). Without this check, the watchdog cries wolf
			// every time a stale failing pod is still around at the start
			// of a new install.
			if !podOwnedBy(pod, workloadUID, wl.kind, func() map[string]struct{} {
				if deploymentRSUIDs == nil {
					deploymentRSUIDs = t.rsUIDsOwnedBy(ctx, wl.namespace, workloadUID)
				}
				return deploymentRSUIDs
			}) {
				continue
			}
			msg := fmt.Sprintf("[watchdog] Pod %s/%s is failing (%s) under %s/%s/%s but kubedog tracker is not tracking it. Inspect with: kubectl -n %s logs %s",
				wl.namespace, podName, reason, wl.kind, wl.namespace, wl.name, wl.namespace, podName)
			t.logger.Warnf("%s", StyleWarning(msg, t.trackOptions.Color))
			warned[podID] = struct{}{}
		}
	}
}

// podOwnedBy reports whether the pod's ownerReferences chain leads to the
// given workload UID. For Job/StatefulSet/DaemonSet the workload directly
// owns the pod, so a direct UID match suffices. For Deployment the pod is
// owned by an intermediate ReplicaSet — the caller passes a lazy lookup of
// "RS UIDs owned by this Deployment" so we only LIST RSes when we actually
// need to disambiguate.
func podOwnedBy(pod *unstructured.Unstructured, workloadUID types.UID, workloadKind string, deploymentRSUIDs func() map[string]struct{}) bool {
	for _, uid := range podOwnerUIDs(pod) {
		if uid == workloadUID {
			return true
		}
	}
	if workloadKind != "deploy" {
		return false
	}
	rsUIDs := deploymentRSUIDs()
	for _, uid := range podOwnerUIDs(pod) {
		if _, ok := rsUIDs[string(uid)]; ok {
			return true
		}
	}
	return false
}

func podOwnerUIDs(obj *unstructured.Unstructured) []types.UID {
	refs := obj.GetOwnerReferences()
	out := make([]types.UID, 0, len(refs))
	for _, r := range refs {
		out = append(out, r.UID)
	}
	return out
}

// rsUIDsOwnedBy returns the set of ReplicaSet UIDs in the namespace whose
// metadata.ownerReferences contain the given workload UID. Used to validate
// pod ownership for Deployments, whose pods are one indirection away from
// the workload (Pod → RS → Deployment).
func (t *Tracker) rsUIDsOwnedBy(ctx context.Context, namespace string, workloadUID types.UID) map[string]struct{} {
	rsList, err := t.dynamicClient.Resource(watchdogReplicaSetGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		t.logger.Debugf("kubedog watchdog: LIST replicasets in %s failed: %v", namespace, err)
		return map[string]struct{}{}
	}
	out := map[string]struct{}{}
	for i := range rsList.Items {
		rs := &rsList.Items[i]
		for _, uid := range podOwnerUIDs(rs) {
			if uid == workloadUID {
				out[string(rs.GetUID())] = struct{}{}
				break
			}
		}
	}
	return out
}

// extractPodSelector pulls the workload's pod label selector. Deployments,
// StatefulSets, DaemonSets, and Jobs all use spec.selector.matchLabels by
// convention; we ignore matchExpressions for simplicity (rare in practice
// and the watchdog is best-effort). Returns "" when nothing usable is
// present.
func extractPodSelector(obj *unstructured.Unstructured) string {
	matchLabels, found, _ := unstructured.NestedStringMap(obj.Object, "spec", "selector", "matchLabels")
	if !found || len(matchLabels) == 0 {
		return ""
	}
	return labels.SelectorFromSet(matchLabels).String()
}

// detectPodFailureReason returns a short reason string for the pod's
// failing state, or "" if the pod looks healthy. Mirrors the phases
// isFailingPodPhase classifies as failing, plus the additional
// terminated-with-error and OOMKilled signals available from container
// statuses.
func detectPodFailureReason(pod *unstructured.Unstructured) string {
	if phase, _, _ := unstructured.NestedString(pod.Object, "status", "phase"); phase == "Failed" {
		return "Failed"
	}

	containers, found, _ := unstructured.NestedSlice(pod.Object, "status", "containerStatuses")
	if found {
		if r := scanContainerStatusesForFailure(containers); r != "" {
			return r
		}
	}
	initContainers, found, _ := unstructured.NestedSlice(pod.Object, "status", "initContainerStatuses")
	if found {
		if r := scanContainerStatusesForFailure(initContainers); r != "" {
			return fmt.Sprintf("init: %s", r)
		}
	}
	return ""
}

func scanContainerStatusesForFailure(containers []any) string {
	for _, c := range containers {
		cMap, ok := c.(map[string]any)
		if !ok {
			continue
		}
		state, hasState, _ := unstructured.NestedMap(cMap, "state")
		if !hasState {
			continue
		}
		if waiting, ok := state["waiting"].(map[string]any); ok {
			if reason, _ := waiting["reason"].(string); isFailingPodPhase(reason) {
				return reason
			}
		}
		if terminated, ok := state["terminated"].(map[string]any); ok {
			if reason, _ := terminated["reason"].(string); reason == "OOMKilled" || reason == "Error" {
				return reason
			}
		}
	}
	return ""
}

// podIsInTaskStore returns true if dyntracker has any ResourceState for a
// Pod with the given name/namespace anywhere in the task store. A "true"
// result means kubedog already sees this pod and will surface its state
// through the normal progress/log pipeline; the watchdog should stay
// quiet.
func podIsInTaskStore(taskStore *kdutil.Concurrent[*statestore.TaskStore], podName, namespace string) bool {
	var present bool
	taskStore.RTransaction(func(s *statestore.TaskStore) {
		for _, taskC := range s.ReadinessTasksStates() {
			if present {
				return
			}
			taskC.RTransaction(func(ts *statestore.ReadinessTaskState) {
				for _, rsC := range ts.ResourceStates() {
					if present {
						return
					}
					rsC.RTransaction(func(rs *statestore.ResourceState) {
						if rs.GroupVersionKind().Kind == "Pod" && rs.Name() == podName && rs.Namespace() == namespace {
							present = true
						}
					})
				}
			})
		}
	})
	return present
}
