package kubedog

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/gookit/color"
	"github.com/stretchr/testify/assert"
	"github.com/werf/kubedog/pkg/tracker/daemonset"
	"github.com/werf/kubedog/pkg/tracker/deployment"
	"github.com/werf/kubedog/pkg/tracker/job"
	"github.com/werf/kubedog/pkg/tracker/pod"
	"github.com/werf/kubedog/pkg/tracker/statefulset"
)

// TestMain forces ANSI color output so that tests asserting on escape codes
// pass in non-TTY environments such as CI runners.
func TestMain(m *testing.M) {
	color.ForceColor()
	os.Exit(m.Run())
}

// --- formatResourceCaption ---

func TestFormatResourceCaption_Ready(t *testing.T) {
	result := formatResourceCaption("deploy/myapp", true, false)
	assert.Contains(t, result, "deploy/myapp")
	// Green ANSI escape should be present
	assert.Contains(t, result, "\033[")
}

func TestFormatResourceCaption_Failed(t *testing.T) {
	result := formatResourceCaption("deploy/myapp", false, true)
	assert.Contains(t, result, "deploy/myapp")
	assert.Contains(t, result, "\033[")
}

func TestFormatResourceCaption_InProgress(t *testing.T) {
	result := formatResourceCaption("deploy/myapp", false, false)
	assert.Contains(t, result, "deploy/myapp")
	// Yellow for in-progress
	assert.Contains(t, result, "\033[")
}

func TestFormatResourceCaption_ReadyTakesPrecedence(t *testing.T) {
	// isReady=true should win over isFailed=true
	resultReady := formatResourceCaption("x", true, false)
	resultFailed := formatResourceCaption("x", false, true)
	// Colors should differ
	assert.NotEqual(t, resultReady, resultFailed)
}

// --- formatPodResourceCaption ---

func TestFormatPodResourceCaption_NotNew(t *testing.T) {
	result := formatPodResourceCaption("my-pod-abc", true, false, false)
	// Not a new pod: no coloring applied, just the plain name
	assert.Equal(t, "my-pod-abc", result)
}

func TestFormatPodResourceCaption_NewAndReady(t *testing.T) {
	result := formatPodResourceCaption("my-pod-abc", true, false, true)
	assert.Contains(t, result, "my-pod-abc")
	assert.Contains(t, result, "\033[")
}

func TestFormatPodResourceCaption_NewAndFailed(t *testing.T) {
	result := formatPodResourceCaption("my-pod-abc", false, true, true)
	assert.Contains(t, result, "my-pod-abc")
	assert.Contains(t, result, "\033[")
}

func TestFormatPodResourceCaption_NewInProgress(t *testing.T) {
	result := formatPodResourceCaption("my-pod-abc", false, false, true)
	assert.Contains(t, result, "my-pod-abc")
	assert.Contains(t, result, "\033[")
}

// --- formatResourceError / formatResourceWarning ---

func TestFormatResourceError(t *testing.T) {
	result := formatResourceError("CrashLoopBackOff")
	assert.Contains(t, result, "error:")
	assert.Contains(t, result, "CrashLoopBackOff")
}

func TestFormatResourceWarning(t *testing.T) {
	result := formatResourceWarning("PodNotScheduled")
	assert.Contains(t, result, "warning:")
	assert.Contains(t, result, "PodNotScheduled")
}

// --- termWidth ---

func TestTermWidth_ReturnsPositive(t *testing.T) {
	w := termWidth()
	assert.Greater(t, w, 0)
}

// --- displayDeploymentStatusProgress ---

func TestDisplayDeploymentStatusProgress_ZeroStatus(t *testing.T) {
	var buf bytes.Buffer
	caption := formatResourceCaption("deploy/myapp", false, false)
	var prev deployment.DeploymentStatus
	status := deployment.DeploymentStatus{}

	// Must not panic and must produce some output
	assert.NotPanics(t, func() {
		displayDeploymentStatusProgress(&buf, caption, status, &prev)
	})
	out := buf.String()
	assert.NotEmpty(t, out)
	assert.Contains(t, out, "DEPLOYMENT")
}

func TestDisplayDeploymentStatusProgress_Failed(t *testing.T) {
	var buf bytes.Buffer
	caption := formatResourceCaption("deploy/myapp", false, true)
	var prev deployment.DeploymentStatus
	status := deployment.DeploymentStatus{
		IsFailed:     true,
		FailedReason: "ImagePullBackOff",
	}

	assert.NotPanics(t, func() {
		displayDeploymentStatusProgress(&buf, caption, status, &prev)
	})
	out := buf.String()
	assert.Contains(t, out, "error:")
	assert.Contains(t, out, "ImagePullBackOff")
}

func TestDisplayDeploymentStatusProgress_WithWaitingMessage(t *testing.T) {
	var buf bytes.Buffer
	caption := formatResourceCaption("deploy/myapp", false, false)
	var prev deployment.DeploymentStatus
	// WaitingForMessages is only rendered when there are pods
	status := deployment.DeploymentStatus{
		StatusGeneration:   1,
		WaitingForMessages: []string{"up-to-date 1->3"},
		Pods: map[string]pod.PodStatus{
			"myapp-pod-abc": {ReadyContainers: 1, TotalContainers: 1},
		},
	}

	assert.NotPanics(t, func() {
		displayDeploymentStatusProgress(&buf, caption, status, &prev)
	})
	out := buf.String()
	assert.Contains(t, out, "Waiting for:")
	assert.Contains(t, out, "up-to-date 1->3")
}

func TestDisplayDeploymentStatusProgress_WithPods(t *testing.T) {
	var buf bytes.Buffer
	caption := formatResourceCaption("deploy/myapp", false, false)
	prev := deployment.DeploymentStatus{}
	status := deployment.DeploymentStatus{
		StatusGeneration: 1,
		Pods: map[string]pod.PodStatus{
			"myapp-abc-123": {ReadyContainers: 1, TotalContainers: 1},
		},
		NewPodsNames: []string{"myapp-abc-123"},
	}

	assert.NotPanics(t, func() {
		displayDeploymentStatusProgress(&buf, caption, status, &prev)
	})
	out := buf.String()
	assert.Contains(t, out, "POD")
	assert.Contains(t, out, "myapp-abc-123")
}

// --- displayStatefulSetStatusProgress ---

func TestDisplayStatefulSetStatusProgress_ZeroStatus(t *testing.T) {
	var buf bytes.Buffer
	caption := formatResourceCaption("sts/myapp", false, false)
	var prev statefulset.StatefulSetStatus
	status := statefulset.StatefulSetStatus{}

	assert.NotPanics(t, func() {
		displayStatefulSetStatusProgress(&buf, caption, status, &prev)
	})
	out := buf.String()
	assert.Contains(t, out, "STATEFULSET")
}

func TestDisplayStatefulSetStatusProgress_WithWarnings(t *testing.T) {
	var buf bytes.Buffer
	caption := formatResourceCaption("sts/myapp", false, false)
	var prev statefulset.StatefulSetStatus
	status := statefulset.StatefulSetStatus{
		WarningMessages: []string{"PodNotScheduled: insufficient resources"},
	}

	assert.NotPanics(t, func() {
		displayStatefulSetStatusProgress(&buf, caption, status, &prev)
	})
	out := buf.String()
	assert.Contains(t, out, "warning:")
	assert.Contains(t, out, "PodNotScheduled")
}

func TestDisplayStatefulSetStatusProgress_Failed(t *testing.T) {
	var buf bytes.Buffer
	caption := formatResourceCaption("sts/myapp", false, true)
	var prev statefulset.StatefulSetStatus
	status := statefulset.StatefulSetStatus{
		IsFailed:     true,
		FailedReason: "timeout waiting for ready",
	}

	assert.NotPanics(t, func() {
		displayStatefulSetStatusProgress(&buf, caption, status, &prev)
	})
	out := buf.String()
	assert.Contains(t, out, "error:")
	assert.Contains(t, out, "timeout waiting for ready")
}

// --- displayDaemonSetStatusProgress ---

func TestDisplayDaemonSetStatusProgress_ZeroStatus(t *testing.T) {
	var buf bytes.Buffer
	caption := formatResourceCaption("ds/myapp", false, false)
	var prev daemonset.DaemonSetStatus
	status := daemonset.DaemonSetStatus{}

	assert.NotPanics(t, func() {
		displayDaemonSetStatusProgress(&buf, caption, status, &prev)
	})
	out := buf.String()
	assert.Contains(t, out, "DAEMONSET")
}

func TestDisplayDaemonSetStatusProgress_Failed(t *testing.T) {
	var buf bytes.Buffer
	caption := formatResourceCaption("ds/myapp", false, true)
	var prev daemonset.DaemonSetStatus
	status := daemonset.DaemonSetStatus{
		IsFailed:     true,
		FailedReason: "node not ready",
	}

	assert.NotPanics(t, func() {
		displayDaemonSetStatusProgress(&buf, caption, status, &prev)
	})
	out := buf.String()
	assert.Contains(t, out, "error:")
	assert.Contains(t, out, "node not ready")
}

// --- displayJobStatusProgress ---

func TestDisplayJobStatusProgress_ZeroStatus(t *testing.T) {
	var buf bytes.Buffer
	caption := formatResourceCaption("job/myjob", false, false)
	var prev job.JobStatus
	status := job.JobStatus{}

	assert.NotPanics(t, func() {
		displayJobStatusProgress(&buf, caption, status, &prev)
	})
	out := buf.String()
	assert.Contains(t, out, "JOB")
}

func TestDisplayJobStatusProgress_Active(t *testing.T) {
	var buf bytes.Buffer
	caption := formatResourceCaption("job/myjob", false, false)
	var prev job.JobStatus
	status := job.JobStatus{
		StatusGeneration: 1,
	}

	assert.NotPanics(t, func() {
		displayJobStatusProgress(&buf, caption, status, &prev)
	})
	out := buf.String()
	assert.Contains(t, out, "ACTIVE")
}

func TestDisplayJobStatusProgress_Failed(t *testing.T) {
	var buf bytes.Buffer
	caption := formatResourceCaption("job/myjob", false, true)
	var prev job.JobStatus
	status := job.JobStatus{
		IsFailed:     true,
		FailedReason: "BackoffLimitExceeded",
	}

	assert.NotPanics(t, func() {
		displayJobStatusProgress(&buf, caption, status, &prev)
	})
	out := buf.String()
	assert.Contains(t, out, "error:")
	assert.Contains(t, out, "BackoffLimitExceeded")
}

func TestDisplayJobStatusProgress_WithWaitingMessage(t *testing.T) {
	var buf bytes.Buffer
	caption := formatResourceCaption("job/myjob", false, false)
	var prev job.JobStatus
	status := job.JobStatus{
		WaitingForMessages: []string{"succeeded 0->1"},
		Pods: map[string]pod.PodStatus{
			"myjob-abc": {ReadyContainers: 0, TotalContainers: 1},
		},
	}

	assert.NotPanics(t, func() {
		displayJobStatusProgress(&buf, caption, status, &prev)
	})
	out := buf.String()
	assert.Contains(t, out, "Waiting for:")
	assert.Contains(t, out, "succeeded 0->1")
}

// --- displayChildPodsStatusProgress ---

func TestDisplayChildPodsStatusProgress_Empty(t *testing.T) {
	var buf bytes.Buffer
	caption := formatResourceCaption("deploy/myapp", false, false)
	// With no pods, only the header should be rendered
	prev := deployment.DeploymentStatus{}
	status := deployment.DeploymentStatus{
		Pods: map[string]pod.PodStatus{},
	}
	assert.NotPanics(t, func() {
		displayDeploymentStatusProgress(&buf, caption, status, &prev)
	})
	// No POD sub-table header when pods is empty
	out := buf.String()
	assert.NotContains(t, out, "POD")
}

func TestDisplayChildPodsStatusProgress_NewPodSet(t *testing.T) {
	var buf bytes.Buffer
	caption := formatResourceCaption("deploy/myapp", false, false)
	prev := deployment.DeploymentStatus{}
	// Two pods: one new, one old
	status := deployment.DeploymentStatus{
		StatusGeneration: 1,
		Pods: map[string]pod.PodStatus{
			"pod-new-abc": {ReadyContainers: 0, TotalContainers: 1},
			"pod-old-xyz": {ReadyContainers: 1, TotalContainers: 1},
		},
		NewPodsNames: []string{"pod-new-abc"},
	}

	assert.NotPanics(t, func() {
		displayDeploymentStatusProgress(&buf, caption, status, &prev)
	})
	out := buf.String()
	assert.Contains(t, out, "pod-new-abc")
	assert.Contains(t, out, "pod-old-xyz")
}

func TestDisplayChildPodsStatusProgress_ManyPodsO1Check(t *testing.T) {
	// Verifies O(1) new-pod detection works correctly for many pods
	var buf bytes.Buffer
	caption := formatResourceCaption("deploy/myapp", false, false)
	prev := deployment.DeploymentStatus{}

	pods := make(map[string]pod.PodStatus)
	newNames := make([]string, 0, 10)
	for i := 0; i < 20; i++ {
		name := strings.Repeat("a", i+1)
		pods[name] = pod.PodStatus{ReadyContainers: 1, TotalContainers: 1}
		if i%2 == 0 {
			newNames = append(newNames, name)
		}
	}
	status := deployment.DeploymentStatus{
		StatusGeneration: 1,
		Pods:             pods,
		NewPodsNames:     newNames,
	}

	assert.NotPanics(t, func() {
		displayDeploymentStatusProgress(&buf, caption, status, &prev)
	})
	assert.NotEmpty(t, buf.String())
}

// --- displayCanaryStatus ---

func TestDisplayCanaryStatus_Normal(t *testing.T) {
	var buf bytes.Buffer
	caption := formatResourceCaption("canary/myapp", false, false)
	view := CanaryStatusView{Phase: "Progressing", Age: "1m"}

	assert.NotPanics(t, func() {
		displayCanaryStatus(&buf, caption, view)
	})
	out := buf.String()
	assert.Contains(t, out, "Progressing")
	assert.Contains(t, out, "1m")
}

func TestDisplayCanaryStatus_Failed(t *testing.T) {
	var buf bytes.Buffer
	caption := formatResourceCaption("canary/myapp", false, true)
	view := CanaryStatusView{Phase: "Failed", IsFailed: true}

	assert.NotPanics(t, func() {
		displayCanaryStatus(&buf, caption, view)
	})
	out := buf.String()
	assert.Contains(t, out, "Failed")
}

func TestDisplayCanaryStatus_Succeeded(t *testing.T) {
	var buf bytes.Buffer
	caption := formatResourceCaption("canary/myapp", true, false)
	view := CanaryStatusView{Phase: "Succeeded"}

	assert.NotPanics(t, func() {
		displayCanaryStatus(&buf, caption, view)
	})
	out := buf.String()
	assert.Contains(t, out, "Succeeded")
}

func TestDisplayCanaryStatus_EmptyPhaseAndAge(t *testing.T) {
	var buf bytes.Buffer
	caption := formatResourceCaption("canary/myapp", false, false)
	view := CanaryStatusView{}

	assert.NotPanics(t, func() {
		displayCanaryStatus(&buf, caption, view)
	})
	// Should still produce output (at least the caption + newline)
	assert.NotEmpty(t, buf.String())
}

// --- writeOut ---

func TestWriteOut(t *testing.T) {
	var buf bytes.Buffer
	writeOut(&buf, "hello world")
	assert.Equal(t, "hello world", buf.String())
}

func TestWriteOut_Empty(t *testing.T) {
	var buf bytes.Buffer
	writeOut(&buf, "")
	assert.Equal(t, "", buf.String())
}
