package kubedog

import (
	"time"

	"github.com/helmfile/helmfile/pkg/resource"
)

type TrackMode string

const (
	TrackModeHelm       TrackMode = "helm"
	TrackModeHelmLegacy TrackMode = "helm-legacy"
	TrackModeKubedog    TrackMode = "kubedog"
)

type TrackOptions struct {
	Timeout   time.Duration
	Logs      bool
	LogsSince time.Duration
	Filter    *resource.FilterConfig
	QPS       float32
	Burst     int
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
