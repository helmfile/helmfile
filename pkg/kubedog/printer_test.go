package kubedog

import (
	"bytes"
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/werf/kubedog/pkg/trackers/dyntracker/logstore"
	"github.com/werf/kubedog/pkg/trackers/dyntracker/statestore"
	kdutil "github.com/werf/kubedog/pkg/trackers/dyntracker/util"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// newBufferedLogger returns a zap.SugaredLogger whose Info-level writes are
// captured into a buffer for inspection in tests.
func newBufferedLogger(t *testing.T) (*zap.SugaredLogger, *bytes.Buffer, *sync.Mutex) {
	t.Helper()
	buf := &bytes.Buffer{}
	mu := &sync.Mutex{}
	w := &syncedBuffer{buf: buf, mu: mu}

	var cfg zapcore.EncoderConfig
	cfg.MessageKey = "message"
	core := zapcore.NewCore(zapcore.NewConsoleEncoder(cfg), zapcore.AddSync(w), zapcore.InfoLevel)
	return zap.New(core).Sugar(), buf, mu
}

type syncedBuffer struct {
	buf *bytes.Buffer
	mu  *sync.Mutex
}

func (s *syncedBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func capturedOutput(buf *bytes.Buffer, mu *sync.Mutex) string {
	mu.Lock()
	defer mu.Unlock()
	return buf.String()
}

var (
	deploymentGVK = schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}
	podGVK        = schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"}
	jobGVK        = schema.GroupVersionKind{Group: "batch", Version: "v1", Kind: "Job"}
)

// addPodChild attaches a pod resource state under a task and sets its status
// + optional pod-phase attribute. Returns the wrapped ResourceState.
func addPodChild(t *testing.T, ts *statestore.ReadinessTaskState, podName, namespace string,
	status statestore.ResourceStatus, statusAttr string) {
	t.Helper()
	ts.AddResourceState(podName, namespace, podGVK)
	ts.AddDependency(ts.Name(), ts.Namespace(), ts.GroupVersionKind(), podName, namespace, podGVK)
	rs := ts.ResourceState(podName, namespace, podGVK)
	rs.RWTransaction(func(rs *statestore.ResourceState) {
		rs.SetStatus(status)
		if statusAttr != "" {
			rs.AddAttribute(statestore.NewAttribute(statestore.AttributeNameStatus, statusAttr))
		}
	})
}

// setRequiredReplicas writes the AttributeNameRequiredReplicas on the root
// resource state of a task — what kubedog's deployment handler populates from
// Status.Replicas in real runs.
func setRequiredReplicas(t *testing.T, ts *statestore.ReadinessTaskState, count int) {
	t.Helper()
	root := ts.ResourceState(ts.Name(), ts.Namespace(), ts.GroupVersionKind())
	root.RWTransaction(func(rs *statestore.ResourceState) {
		rs.AddAttribute(statestore.NewAttribute(statestore.AttributeNameRequiredReplicas, count))
	})
}

func TestStatusColor_FailingPhaseOverridesReady(t *testing.T) {
	p := &progressPrinter{useColor: true}

	// kubedog reports the pod's ResourceStatus as "ready" because the parent
	// task is ready, but the pod's actual phase is "Error" — must render red.
	assert.Equal(t, ansiRed, p.statusColor("ready (Error)", ""))
	assert.Equal(t, ansiRed, p.statusColor("progressing (CrashLoopBackOff)", ""))
	assert.Equal(t, ansiRed, p.statusColor("ready (ImagePullBackOff)", ""))
	assert.Equal(t, ansiRed, p.statusColor("ready (OOMKilled)", ""))
}

func TestStatusColor_LeadingState(t *testing.T) {
	p := &progressPrinter{useColor: true}
	assert.Equal(t, ansiGreen, p.statusColor("ready (1/1)", ""))
	assert.Equal(t, ansiGreen, p.statusColor("ready (Running)", ""))
	assert.Equal(t, ansiYellow, p.statusColor("progressing (0/1)", ""))
	assert.Equal(t, ansiRed, p.statusColor("failed", ""))
	assert.Equal(t, ansiCyan, p.statusColor("waiting for update (uid=abc gen=1)", ""))
	assert.Equal(t, ansiGray, p.statusColor("unknown", ""))
}

func TestStatusColor_PodPhaseFallback(t *testing.T) {
	p := &progressPrinter{useColor: true}
	// Bare pod phases (no leading kubedog state word), no parent context —
	// defaults match the long-running workload assumptions (Running = green).
	assert.Equal(t, ansiGreen, p.statusColor("Running", ""))
	assert.Equal(t, ansiGreen, p.statusColor("Completed", ""))
	assert.Equal(t, ansiYellow, p.statusColor("ContainerCreating", ""))
	assert.Equal(t, ansiYellow, p.statusColor("Pending", ""))
	assert.Equal(t, ansiYellow, p.statusColor("Init:0/2", ""))
	assert.Equal(t, ansiGray, p.statusColor("Terminating", ""))
}

func TestStatusColor_JobChildRunningIsYellow(t *testing.T) {
	p := &progressPrinter{useColor: true}
	// A Job pod in Running phase is still working toward Completed, so the
	// row should render in-progress yellow instead of steady-state green.
	assert.Equal(t, ansiYellow, p.statusColor("Running", "Job"))
	// Once the Job pod hits Completed it's done — green.
	assert.Equal(t, ansiGreen, p.statusColor("ready (Completed)", "Job"))
	assert.Equal(t, ansiGreen, p.statusColor("Completed", "Job"))
	// Other workload kinds keep the steady-state semantics for Running.
	assert.Equal(t, ansiGreen, p.statusColor("Running", "Deployment"))
	assert.Equal(t, ansiGreen, p.statusColor("Running", "StatefulSet"))
}

func TestColorize_RespectsUseColor(t *testing.T) {
	with := &progressPrinter{useColor: true}
	assert.Equal(t, ansiGreen+"ready"+ansiReset, with.colorize("ready", ansiGreen))

	without := &progressPrinter{useColor: false}
	assert.Equal(t, "ready", without.colorize("ready", ansiGreen),
		"colorize must be a no-op when useColor is false")
}

func TestDescribeChildStatus(t *testing.T) {
	tests := []struct {
		name     string
		kd, attr string
		want     string
	}{
		{"both empty", "", "", "unknown"},
		{"attr only", "", "Running", "Running"},
		{"kd only", "ready", "", "ready"},
		{"unknown plus attr drops unknown", "unknown", "ContainerCreating", "ContainerCreating"},
		{"both meaningful", "ready", "Running", "ready (Running)"},
		{"failed plus error", "failed", "CrashLoopBackOff", "failed (CrashLoopBackOff)"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, describeChildStatus(tc.kd, tc.attr))
		})
	}
}

func TestIsFailingPodPhase(t *testing.T) {
	for _, p := range []string{"Error", "Failed", "CrashLoopBackOff", "ImagePullBackOff",
		"ErrImagePull", "OOMKilled", "CreateContainerConfigError", "CreateContainerError",
		"InvalidImageName"} {
		assert.True(t, isFailingPodPhase(p), "%q should be classified as failing", p)
	}
	for _, p := range []string{"Running", "Completed", "ContainerCreating", "Pending",
		"PodInitializing", "Terminating", ""} {
		assert.False(t, isFailingPodPhase(p), "%q should not be classified as failing", p)
	}
}

func TestShortKind(t *testing.T) {
	assert.Equal(t, "deploy", shortKind("Deployment"))
	assert.Equal(t, "sts", shortKind("StatefulSet"))
	assert.Equal(t, "ds", shortKind("DaemonSet"))
	assert.Equal(t, "job", shortKind("Job"))
	assert.Equal(t, "canary", shortKind("Canary"))
	// Anything else lowercases for safety.
	assert.Equal(t, "pod", shortKind("Pod"))
}

func TestFlushProgress_HidesStaleReadyPodWithoutAttr(t *testing.T) {
	taskStore := kdutil.NewConcurrent(statestore.NewTaskStore())
	logStore := kdutil.NewConcurrent(logstore.NewLogStore())

	ts := statestore.NewReadinessTaskState("app", "ns", deploymentGVK, statestore.ReadinessTaskStateOptions{})
	setRequiredReplicas(t, ts, 1)
	// New pod that kubedog has observed (status attr set).
	addPodChild(t, ts, "app-new-xyz", "ns", statestore.ResourceStatusReady, "Running")
	// Stale leftover from previous ReplicaSet: ready but no Status attribute.
	addPodChild(t, ts, "app-old-abc", "ns", statestore.ResourceStatusReady, "")

	taskC := kdutil.NewConcurrent(ts)
	taskStore.RWTransaction(func(s *statestore.TaskStore) { s.AddReadinessTaskState(taskC) })

	logger, buf, mu := newBufferedLogger(t)
	p := newProgressPrinter(logger, "", taskStore, logStore, true /*skipLogs*/, false, newGateStatuses(), newSkippedKeys(), false)
	p.flushProgress()

	out := capturedOutput(buf, mu)
	assert.Contains(t, out, "Deployment/ns/app")
	assert.Contains(t, out, "ready (1/1)", "ready count must be capped to required replicas")
	assert.Contains(t, out, "Pod/ns/app-new-xyz")
	assert.NotContains(t, out, "app-old-abc", "stale ready pod without status attribute must be hidden")
}

func TestFlushProgress_HidesEmptyNameChildPlaceholder(t *testing.T) {
	taskStore := kdutil.NewConcurrent(statestore.NewTaskStore())
	logStore := kdutil.NewConcurrent(logstore.NewLogStore())

	ts := statestore.NewReadinessTaskState("init", "ns", jobGVK, statestore.ReadinessTaskStateOptions{})
	addPodChild(t, ts, "real-pod", "ns", statestore.ResourceStatusReady, "Running")
	// Placeholder child that dyntracker can insert when it has observed a
	// reference to a pod but hasn't ingested the object yet — empty name,
	// no status, no attributes. It must not be rendered.
	addPodChild(t, ts, "", "ns", statestore.ResourceStatusUnknown, "")
	taskStore.RWTransaction(func(s *statestore.TaskStore) {
		s.AddReadinessTaskState(kdutil.NewConcurrent(ts))
	})

	logger, buf, mu := newBufferedLogger(t)
	p := newProgressPrinter(logger, "", taskStore, logStore, true, false, newGateStatuses(), newSkippedKeys(), false)
	p.flushProgress()

	out := capturedOutput(buf, mu)
	assert.Contains(t, out, "Pod/ns/real-pod")
	// "Pod/ns/" with no name suffix is the visible footprint of the
	// placeholder. It must not appear in the rendered output.
	assert.NotContains(t, out, "Pod/ns/ ",
		"empty-name placeholder child must not be rendered")
	assert.NotContains(t, out, "Pod/ns/\n",
		"empty-name placeholder child must not be rendered as a trailing row")
}

func TestFlushProgress_HeaderUsesReleaseName(t *testing.T) {
	taskStore := kdutil.NewConcurrent(statestore.NewTaskStore())
	logStore := kdutil.NewConcurrent(logstore.NewLogStore())
	ts := statestore.NewReadinessTaskState("app", "ns", deploymentGVK, statestore.ReadinessTaskStateOptions{})
	taskStore.RWTransaction(func(s *statestore.TaskStore) {
		s.AddReadinessTaskState(kdutil.NewConcurrent(ts))
	})

	logger, buf, mu := newBufferedLogger(t)
	p := newProgressPrinter(logger, "vray", taskStore, logStore, true, false, newGateStatuses(), newSkippedKeys(), false)
	p.flushProgress()
	out := capturedOutput(buf, mu)
	assert.Contains(t, out, "Release 'vray' progress:",
		"header must use release name when set")
	assert.Contains(t, out, "==========",
		"header must include divider that survives CI newline stripping")
}

func TestFlushProgress_HeaderFallsBackWithoutReleaseName(t *testing.T) {
	taskStore := kdutil.NewConcurrent(statestore.NewTaskStore())
	logStore := kdutil.NewConcurrent(logstore.NewLogStore())
	ts := statestore.NewReadinessTaskState("app", "ns", deploymentGVK, statestore.ReadinessTaskStateOptions{})
	taskStore.RWTransaction(func(s *statestore.TaskStore) {
		s.AddReadinessTaskState(kdutil.NewConcurrent(ts))
	})

	logger, buf, mu := newBufferedLogger(t)
	p := newProgressPrinter(logger, "", taskStore, logStore, true, false, newGateStatuses(), newSkippedKeys(), false)
	p.flushProgress()
	out := capturedOutput(buf, mu)
	assert.Contains(t, out, "kubedog progress:")
	assert.NotContains(t, out, "Release ''", "empty release name must not produce 'Release '' progress:'")
}

func TestFlushProgress_NestingAndAlignment(t *testing.T) {
	taskStore := kdutil.NewConcurrent(statestore.NewTaskStore())
	logStore := kdutil.NewConcurrent(logstore.NewLogStore())

	ts := statestore.NewReadinessTaskState("svc", "default", deploymentGVK, statestore.ReadinessTaskStateOptions{})
	setRequiredReplicas(t, ts, 2)
	addPodChild(t, ts, "svc-pod-aaa", "default", statestore.ResourceStatusReady, "Running")
	addPodChild(t, ts, "svc-pod-bbb", "default", statestore.ResourceStatusUnknown, "ContainerCreating")

	taskStore.RWTransaction(func(s *statestore.TaskStore) {
		s.AddReadinessTaskState(kdutil.NewConcurrent(ts))
	})

	logger, buf, mu := newBufferedLogger(t)
	p := newProgressPrinter(logger, "", taskStore, logStore, true, false, newGateStatuses(), newSkippedKeys(), false)
	p.flushProgress()

	out := capturedOutput(buf, mu)
	require.Contains(t, out, "Deployment/default/svc")
	require.Contains(t, out, "  • Pod/default/svc-pod-aaa", "pod rows must be indented with a bullet")
	require.Contains(t, out, "Running")
	require.Contains(t, out, "ContainerCreating")
	// readyChildren counts only Ready pods; one of two is ready.
	require.Contains(t, out, "progressing (1/2)")
}

func TestFlushProgress_RespectsGateAndSkip(t *testing.T) {
	taskStore := kdutil.NewConcurrent(statestore.NewTaskStore())
	logStore := kdutil.NewConcurrent(logstore.NewLogStore())

	gated := statestore.NewReadinessTaskState("gated", "ns", deploymentGVK, statestore.ReadinessTaskStateOptions{})
	skipped := statestore.NewReadinessTaskState("skipped", "ns", deploymentGVK, statestore.ReadinessTaskStateOptions{})
	visible := statestore.NewReadinessTaskState("visible", "ns", deploymentGVK, statestore.ReadinessTaskStateOptions{})
	setRequiredReplicas(t, visible, 1)
	addPodChild(t, visible, "visible-aaa", "ns", statestore.ResourceStatusReady, "Running")

	for _, ts := range []*statestore.ReadinessTaskState{gated, skipped, visible} {
		taskStore.RWTransaction(func(s *statestore.TaskStore) {
			s.AddReadinessTaskState(kdutil.NewConcurrent(ts))
		})
	}

	gates := newGateStatuses()
	gates.set(BaselineKey("deploy", "ns", "gated"), "waiting for update (uid=abc gen=2)")
	skips := newSkippedKeys()
	skips.add(kdutil.ResourceID("skipped", "ns", deploymentGVK))

	logger, buf, mu := newBufferedLogger(t)
	p := newProgressPrinter(logger, "", taskStore, logStore, true, false, gates, skips, false)
	p.flushProgress()

	out := capturedOutput(buf, mu)
	assert.Contains(t, out, "Deployment/ns/gated")
	assert.Contains(t, out, "waiting for update (uid=abc gen=2)")
	assert.NotContains(t, out, "Deployment/ns/skipped", "skipped tasks must be omitted")
	assert.Contains(t, out, "Deployment/ns/visible")
	assert.Contains(t, out, "ready (1/1)")
}

func TestFlushLogs_FailedOnlyMode_GatesUnchanged(t *testing.T) {
	taskStore := kdutil.NewConcurrent(statestore.NewTaskStore())
	logStore := kdutil.NewConcurrent(logstore.NewLogStore())

	// Job with two pods: one Completed (success), one Error (failed).
	ts := statestore.NewReadinessTaskState("init", "ns", jobGVK, statestore.ReadinessTaskStateOptions{})
	addPodChild(t, ts, "init-good", "ns", statestore.ResourceStatusReady, "Completed")
	addPodChild(t, ts, "init-bad", "ns", statestore.ResourceStatusReady, "Error")
	taskStore.RWTransaction(func(s *statestore.TaskStore) {
		s.AddReadinessTaskState(kdutil.NewConcurrent(ts))
	})

	// Both pods have log lines accumulated in the log store.
	addPodLogs(t, logStore, "init-good", "ns", "container/main", "good line 1", "good line 2")
	addPodLogs(t, logStore, "init-bad", "ns", "container/main", "bad line 1", "panic: boom")

	logger, buf, mu := newBufferedLogger(t)
	p := newProgressPrinter(logger, "", taskStore, logStore, false /*skipLogs*/, true /*failedLogsOnly*/, newGateStatuses(), newSkippedKeys(), false)
	p.flushLogs()

	out := capturedOutput(buf, mu)
	assert.NotContains(t, out, "good line", "successful-pod logs must be hidden in failed-only mode")
	assert.Contains(t, out, "bad line 1")
	assert.Contains(t, out, "panic: boom")
	assert.Contains(t, out, "Pod/ns/init-bad")
}

func TestFlushLogs_FailedOnlyMode_DoesNotAdvanceCursorForSuccess(t *testing.T) {
	taskStore := kdutil.NewConcurrent(statestore.NewTaskStore())
	logStore := kdutil.NewConcurrent(logstore.NewLogStore())

	// Pod is currently classified as successful (Completed) — its lines must
	// stay queued. When the pod transitions to a failing phase, all lines
	// since the start should be emitted.
	ts := statestore.NewReadinessTaskState("flaky", "ns", jobGVK, statestore.ReadinessTaskStateOptions{})
	addPodChild(t, ts, "flaky-pod", "ns", statestore.ResourceStatusReady, "Completed")
	taskStore.RWTransaction(func(s *statestore.TaskStore) {
		s.AddReadinessTaskState(kdutil.NewConcurrent(ts))
	})
	addPodLogs(t, logStore, "flaky-pod", "ns", "container/main", "early line A", "early line B")

	logger, buf, mu := newBufferedLogger(t)
	p := newProgressPrinter(logger, "", taskStore, logStore, false, true, newGateStatuses(), newSkippedKeys(), false)

	p.flushLogs()
	assert.NotContains(t, capturedOutput(buf, mu), "early line",
		"flushLogs must skip successful pods in failed-only mode")

	// Pod transitions to failing — re-flush should now drain the buffered lines.
	flipPodPhase(t, ts, "flaky-pod", "ns", "CrashLoopBackOff")
	p.flushLogs()

	out := capturedOutput(buf, mu)
	assert.Contains(t, out, "early line A", "lines accumulated before failure must be emitted on flip")
	assert.Contains(t, out, "early line B")
}

func TestFlushLogs_AllPodsMode_EmitsEverything(t *testing.T) {
	taskStore := kdutil.NewConcurrent(statestore.NewTaskStore())
	logStore := kdutil.NewConcurrent(logstore.NewLogStore())

	ts := statestore.NewReadinessTaskState("init", "ns", jobGVK, statestore.ReadinessTaskStateOptions{})
	addPodChild(t, ts, "init-good", "ns", statestore.ResourceStatusReady, "Completed")
	addPodChild(t, ts, "init-bad", "ns", statestore.ResourceStatusReady, "Error")
	taskStore.RWTransaction(func(s *statestore.TaskStore) {
		s.AddReadinessTaskState(kdutil.NewConcurrent(ts))
	})
	addPodLogs(t, logStore, "init-good", "ns", "container/main", "good line 1")
	addPodLogs(t, logStore, "init-bad", "ns", "container/main", "bad line 1")

	logger, buf, mu := newBufferedLogger(t)
	p := newProgressPrinter(logger, "", taskStore, logStore, false, false /*failedLogsOnly=off*/, newGateStatuses(), newSkippedKeys(), false)
	p.flushLogs()

	out := capturedOutput(buf, mu)
	assert.Contains(t, out, "good line 1")
	assert.Contains(t, out, "bad line 1")
}

func TestFlushLogs_HeaderDedupedAcrossFlushes(t *testing.T) {
	taskStore := kdutil.NewConcurrent(statestore.NewTaskStore())
	logStore := kdutil.NewConcurrent(logstore.NewLogStore())

	ts := statestore.NewReadinessTaskState("app", "ns", deploymentGVK, statestore.ReadinessTaskStateOptions{})
	addPodChild(t, ts, "app-pod", "ns", statestore.ResourceStatusReady, "Running")
	taskStore.RWTransaction(func(s *statestore.TaskStore) {
		s.AddReadinessTaskState(kdutil.NewConcurrent(ts))
	})

	logger, buf, mu := newBufferedLogger(t)
	p := newProgressPrinter(logger, "", taskStore, logStore, false, false, newGateStatuses(), newSkippedKeys(), false)

	addPodLogs(t, logStore, "app-pod", "ns", "container/main", "line 1", "line 2")
	p.flushLogs()
	addPodLogs(t, logStore, "app-pod", "ns", "container/main", "line 3", "line 4")
	p.flushLogs()

	out := capturedOutput(buf, mu)
	// The header should appear exactly once — second flush is a continuation
	// of the same source.
	assert.Equal(t, 1, strings.Count(out, "Logs Pod/ns/app-pod container/main:"),
		"continuation flushes must not re-emit the per-source header")
	assert.Contains(t, out, "line 1")
	assert.Contains(t, out, "line 4")
}

func TestHeaderDivider_FormatsBothBorders(t *testing.T) {
	assert.Equal(t, "========== title: ==========", HeaderDivider("title:"))
}

func TestProgressPrinter_FullRun_CancelsAndDrainsOnContextDone(t *testing.T) {
	// Smoke test that the run loop terminates cleanly on context cancel.
	taskStore := kdutil.NewConcurrent(statestore.NewTaskStore())
	logStore := kdutil.NewConcurrent(logstore.NewLogStore())
	logger, _, _ := newBufferedLogger(t)
	p := newProgressPrinter(logger, "", taskStore, logStore, true, false, newGateStatuses(), newSkippedKeys(), false)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	returned := make(chan struct{})
	go func() {
		p.run(ctx, done)
		close(returned)
	}()
	cancel()
	select {
	case <-returned:
	case <-time.After(2 * time.Second):
		t.Fatal("printer.run did not return after ctx cancel")
	}
}

// addPodLogs appends log lines to a pod's ResourceLogs entry in the log
// store, creating the entry if needed. Uses an incrementing timestamp so
// the printer's chronological sort is deterministic.
func addPodLogs(t *testing.T, logStore *kdutil.Concurrent[*logstore.LogStore], podName, namespace, source string, lines ...string) {
	t.Helper()
	logStore.RWTransaction(func(s *logstore.LogStore) {
		var existing *kdutil.Concurrent[*logstore.ResourceLogs]
		for _, rlC := range s.ResourcesLogs() {
			rlC.RTransaction(func(rl *logstore.ResourceLogs) {
				if rl.Name() == podName && rl.Namespace() == namespace {
					existing = rlC
				}
			})
			if existing != nil {
				break
			}
		}
		if existing == nil {
			existing = kdutil.NewConcurrent(logstore.NewResourceLogs(podName, namespace, podGVK))
			s.AddResourceLogs(existing)
		}
		base := time.Unix(0, 0)
		existing.RWTransaction(func(rl *logstore.ResourceLogs) {
			for i, line := range lines {
				rl.AddLogLine(line, source, base.Add(time.Duration(i)*time.Millisecond))
			}
		})
	})
}

// flipPodPhase mutates an existing pod child's Status attribute in place.
// Used to simulate transitions like "Completed" -> "CrashLoopBackOff".
func flipPodPhase(t *testing.T, ts *statestore.ReadinessTaskState, podName, namespace, newPhase string) {
	t.Helper()
	rs := ts.ResourceState(podName, namespace, podGVK)
	rs.RWTransaction(func(rs *statestore.ResourceState) {
		// Find existing Status attribute and rewrite its Value, if it exists.
		for _, attr := range rs.Attributes() {
			if attr.Name() == statestore.AttributeNameStatus {
				if a, ok := attr.(*statestore.Attribute[string]); ok {
					a.Value = newPhase
					return
				}
			}
		}
		// Otherwise add one.
		rs.AddAttribute(statestore.NewAttribute(statestore.AttributeNameStatus, newPhase))
	})
}
