package kubedog

import (
	"time"

	"k8s.io/apimachinery/pkg/types"

	"github.com/helmfile/helmfile/pkg/resource"
)

// ResourceBaseline records a resource's identity and generation captured
// before an in-flight change (e.g. helm upgrade). The tracker uses it to
// distinguish "still observing the pre-change state" from "the change has
// landed in the cluster" and only then attaches its readiness logic.
type ResourceBaseline struct {
	UID        types.UID
	Generation int64
	Exists     bool
}

type TrackMode string

const (
	TrackModeHelm       TrackMode = "helm"
	TrackModeHelmLegacy TrackMode = "helm-legacy"
	TrackModeKubedog    TrackMode = "kubedog"
)

type TrackOptions struct {
	Timeout time.Duration
	// Logs enables emitting logs for every pod kubedog observes.
	Logs bool
	// FailedLogsOnly enables capturing logs in the background and emitting
	// them only for pods that enter a failed state (CrashLoopBackOff, Error,
	// ImagePullBackOff, etc.). Has no effect when Logs is true.
	FailedLogsOnly bool
	LogsSince      time.Duration
	Filter         *resource.FilterConfig
	QPS            float32
	Burst          int
	// Baselines holds the pre-change state of each resource keyed by
	// "Kind/Namespace/Name". When set, the tracker delays attaching kubedog
	// to a resource until its UID changes or its generation increments past
	// the recorded baseline — preventing false "ready" verdicts that would
	// otherwise come from observing the old rolled-out state.
	Baselines map[string]ResourceBaseline
	// Color enables ANSI color escapes in the progress printer output.
	// When false the printer emits plain text regardless of TTY detection.
	Color bool
}

func NewTrackOptions() *TrackOptions {
	return &TrackOptions{
		Timeout:   5 * time.Minute,
		LogsSince: 10 * time.Minute,
		QPS:       100,
		Burst:     200,
	}
}

func (o *TrackOptions) WithTimeout(timeout time.Duration) *TrackOptions {
	o.Timeout = timeout
	return o
}

func (o *TrackOptions) WithLogs(logs bool) *TrackOptions {
	o.Logs = logs
	return o
}

func (o *TrackOptions) WithFilterConfig(config *resource.FilterConfig) *TrackOptions {
	o.Filter = config
	return o
}

func (o *TrackOptions) WithQPS(qps float32) *TrackOptions {
	o.QPS = qps
	return o
}

func (o *TrackOptions) WithBurst(burst int) *TrackOptions {
	o.Burst = burst
	return o
}

func (o *TrackOptions) WithBaselines(baselines map[string]ResourceBaseline) *TrackOptions {
	o.Baselines = baselines
	return o
}

func (o *TrackOptions) WithColor(color bool) *TrackOptions {
	o.Color = color
	return o
}

func (o *TrackOptions) WithFailedLogsOnly(v bool) *TrackOptions {
	o.FailedLogsOnly = v
	return o
}
