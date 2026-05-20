package kubedog

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/werf/kubedog/pkg/tracker/daemonset"
	"github.com/werf/kubedog/pkg/tracker/deployment"
	"github.com/werf/kubedog/pkg/tracker/indicators"
	"github.com/werf/kubedog/pkg/tracker/job"
	"github.com/werf/kubedog/pkg/tracker/pod"
	"github.com/werf/kubedog/pkg/tracker/statefulset"
	"github.com/werf/kubedog/pkg/utils"
	"golang.org/x/term"
)

var statusProgressTableRatio = []float64{.58, .11, .12, .19}
var statusProgressSubTableRatio = []float64{.40, .15, .20, .25}

func writeOut(out io.Writer, s string) {
	_, _ = fmt.Fprint(out, s)
}

func displayDeploymentStatusProgress(out io.Writer, resourceCaption string, status deployment.DeploymentStatus, prevStatus *deployment.DeploymentStatus) {
	t := utils.NewTable(statusProgressTableRatio...)
	t.SetWidth(termWidth())

	showProgress := status.StatusGeneration > prevStatus.StatusGeneration

	replicas := "-"
	if status.ReplicasIndicator != nil {
		replicas = status.ReplicasIndicator.FormatTableElem(prevStatus.ReplicasIndicator, indicators.FormatTableElemOptions{
			ShowProgress:    showProgress,
			WithTargetValue: true,
		})
	}
	available := "-"
	if status.AvailableIndicator != nil {
		available = status.AvailableIndicator.FormatTableElem(prevStatus.AvailableIndicator, indicators.FormatTableElemOptions{
			ShowProgress: showProgress,
		})
	}
	uptodate := "-"
	if status.UpToDateIndicator != nil {
		uptodate = status.UpToDateIndicator.FormatTableElem(prevStatus.UpToDateIndicator, indicators.FormatTableElemOptions{
			ShowProgress: showProgress,
		})
	}

	t.Header("DEPLOYMENT", "REPLICAS", "AVAILABLE", "UP-TO-DATE")

	args := []interface{}{resourceCaption, replicas, available, uptodate}
	if status.IsFailed {
		args = append(args, formatResourceError(status.FailedReason))
	}
	t.Row(args...)

	displayChildPodsAndWaiting(&t, prevStatus.Pods, status.Pods, status.NewPodsNames, status.WaitingForMessages)

	writeOut(out, t.Render())
}

func displayStatefulSetStatusProgress(out io.Writer, resourceCaption string, status statefulset.StatefulSetStatus, prevStatus *statefulset.StatefulSetStatus) {
	t := utils.NewTable(statusProgressTableRatio...)
	t.SetWidth(termWidth())

	showProgress := status.StatusGeneration > prevStatus.StatusGeneration

	replicas := "-"
	if status.ReplicasIndicator != nil {
		replicas = status.ReplicasIndicator.FormatTableElem(prevStatus.ReplicasIndicator, indicators.FormatTableElemOptions{
			ShowProgress:    showProgress,
			WithTargetValue: true,
		})
	}
	ready := "-"
	if status.ReadyIndicator != nil {
		ready = status.ReadyIndicator.FormatTableElem(prevStatus.ReadyIndicator, indicators.FormatTableElemOptions{
			ShowProgress: showProgress,
		})
	}
	uptodate := "-"
	if status.UpToDateIndicator != nil {
		uptodate = status.UpToDateIndicator.FormatTableElem(prevStatus.UpToDateIndicator, indicators.FormatTableElemOptions{
			ShowProgress: showProgress,
		})
	}

	t.Header("STATEFULSET", "REPLICAS", "READY", "UP-TO-DATE")

	args := []interface{}{resourceCaption, replicas, ready, uptodate}
	if status.IsFailed {
		args = append(args, formatResourceError(status.FailedReason))
	} else {
		for _, w := range status.WarningMessages {
			args = append(args, formatResourceWarning(w))
		}
	}
	t.Row(args...)

	displayChildPodsAndWaiting(&t, prevStatus.Pods, status.Pods, status.NewPodsNames, status.WaitingForMessages)

	writeOut(out, t.Render())
}

func displayDaemonSetStatusProgress(out io.Writer, resourceCaption string, status daemonset.DaemonSetStatus, prevStatus *daemonset.DaemonSetStatus) {
	t := utils.NewTable(statusProgressTableRatio...)
	t.SetWidth(termWidth())

	showProgress := status.StatusGeneration > prevStatus.StatusGeneration

	replicas := "-"
	if status.ReplicasIndicator != nil {
		replicas = status.ReplicasIndicator.FormatTableElem(prevStatus.ReplicasIndicator, indicators.FormatTableElemOptions{
			ShowProgress:    showProgress,
			WithTargetValue: true,
		})
	}
	available := "-"
	if status.AvailableIndicator != nil {
		available = status.AvailableIndicator.FormatTableElem(prevStatus.AvailableIndicator, indicators.FormatTableElemOptions{
			ShowProgress: showProgress,
		})
	}
	uptodate := "-"
	if status.UpToDateIndicator != nil {
		uptodate = status.UpToDateIndicator.FormatTableElem(prevStatus.UpToDateIndicator, indicators.FormatTableElemOptions{
			ShowProgress: showProgress,
		})
	}

	t.Header("DAEMONSET", "REPLICAS", "AVAILABLE", "UP-TO-DATE")

	args := []interface{}{resourceCaption, replicas, available, uptodate}
	if status.IsFailed {
		args = append(args, formatResourceError(status.FailedReason))
	}
	t.Row(args...)

	displayChildPodsAndWaiting(&t, prevStatus.Pods, status.Pods, status.NewPodsNames, status.WaitingForMessages)

	writeOut(out, t.Render())
}

func displayJobStatusProgress(out io.Writer, resourceCaption string, status job.JobStatus, prevStatus *job.JobStatus) {
	t := utils.NewTable(statusProgressTableRatio...)
	t.SetWidth(termWidth())

	showProgress := status.StatusGeneration > prevStatus.StatusGeneration

	succeeded := "-"
	if status.SucceededIndicator != nil {
		succeeded = status.SucceededIndicator.FormatTableElem(prevStatus.SucceededIndicator, indicators.FormatTableElemOptions{
			ShowProgress: showProgress,
		})
	}

	t.Header("JOB", "ACTIVE", "DURATION", "SUCCEEDED/FAILED")

	var active interface{} = "-"
	if status.Active != 0 {
		active = status.Active
	}
	failed := fmt.Sprintf("%d", status.Failed)

	args := []interface{}{resourceCaption, active, status.Age, strings.Join([]string{succeeded, failed}, "/")}
	if status.IsFailed {
		args = append(args, formatResourceError(status.FailedReason))
	}
	t.Row(args...)

	if len(status.Pods) > 0 {
		st := displayChildPodsStatusProgress(&t, prevStatus.Pods, status.Pods, nil, showProgress)
		extraMsg := ""
		if len(status.WaitingForMessages) > 0 {
			extraMsg += "---\n"
			extraMsg += utils.BlueF("Waiting for: %s", strings.Join(status.WaitingForMessages, ", "))
		}
		st.Commit(extraMsg)
	}

	writeOut(out, t.Render())
}

func displayChildPodsAndWaiting(t *utils.Table, prevPods, pods map[string]pod.PodStatus, newPodsNames []string, waitingForMessages []string) {
	if len(pods) > 0 {
		st := displayChildPodsStatusProgress(t, prevPods, pods, newPodsNames, true)
		extraMsg := ""
		if len(waitingForMessages) > 0 {
			extraMsg += "---\n"
			extraMsg += utils.BlueF("Waiting for: %s", strings.Join(waitingForMessages, ", "))
		}
		st.Commit(extraMsg)
	}
}

func displayChildPodsStatusProgress(t *utils.Table, prevPods, pods map[string]pod.PodStatus, newPodsNames []string, showProgress bool) *utils.Table {
	subT := t.SubTable(statusProgressSubTableRatio...)
	st := &subT

	st.Header("POD", "READY", "RESTARTS", "STATUS")

	podsNames := make([]string, 0, len(pods))
	for podName := range pods {
		podsNames = append(podsNames, podName)
	}
	sort.Strings(podsNames)

	var podRows [][]interface{}

	newPodSet := make(map[string]struct{}, len(newPodsNames))
	for _, name := range newPodsNames {
		newPodSet[name] = struct{}{}
	}

	for _, podName := range podsNames {
		var podRow []interface{}

		_, isPodNew := newPodSet[podName]

		prevPodStatus := prevPods[podName]
		podStatus := pods[podName]

		isReady := false
		if podStatus.StatusIndicator != nil {
			isReady = podStatus.StatusIndicator.IsReady()
		}

		resource := formatPodResourceCaption(podName, isReady, podStatus.IsFailed, isPodNew)
		ready := fmt.Sprintf("%d/%d", podStatus.ReadyContainers, podStatus.TotalContainers)

		status := "-"
		if podStatus.StatusIndicator != nil {
			status = podStatus.StatusIndicator.FormatTableElem(prevPodStatus.StatusIndicator, indicators.FormatTableElemOptions{
				ShowProgress:  showProgress,
				IsResourceNew: isPodNew,
			})
		}

		podRow = append(podRow, resource, ready, podStatus.Restarts, status)
		if podStatus.IsFailed {
			podRow = append(podRow, formatResourceError(podStatus.FailedReason))
		}

		podRows = append(podRows, podRow)
	}

	st.Rows(podRows...)

	return st
}

func formatResourceCaption(caption string, isReady, isFailed bool) string {
	switch {
	case isReady:
		return utils.GreenF("%s", caption)
	case isFailed:
		return utils.RedF("%s", caption)
	default:
		return utils.YellowF("%s", caption)
	}
}

func formatPodResourceCaption(podName string, isReady, isFailed, isNew bool) string {
	if !isNew {
		return podName
	}
	return formatResourceCaption(podName, isReady, isFailed)
}

func formatResourceError(reason string) string {
	return utils.RedF("error: %s", reason)
}

func formatResourceWarning(reason string) string {
	return utils.YellowF("warning: %s", reason)
}

func termWidth() int {
	if w, _, err := term.GetSize(int(os.Stderr.Fd())); err == nil && w > 0 {
		return w
	}
	return 140
}

func displayCanaryStatus(out io.Writer, resourceCaption string, status CanaryStatusView) {
	var parts []string
	if status.Phase != "" {
		parts = append(parts, fmt.Sprintf("phase %s", status.Phase))
	}
	if status.Age != "" {
		parts = append(parts, fmt.Sprintf("age %s", status.Age))
	}
	msg := fmt.Sprintf("%s: %s", resourceCaption, strings.Join(parts, ", "))
	if status.IsFailed {
		msg = utils.RedF("%s", msg)
	}
	_, _ = fmt.Fprintln(out, msg)
}

type CanaryStatusView struct {
	Phase    string
	Age      string
	IsFailed bool
}

func statusOutput() io.Writer {
	return os.Stderr
}
