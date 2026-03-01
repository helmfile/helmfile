package kubedog

import (
	"time"

	"github.com/helmfile/helmfile/pkg/resource"
)

type TrackMode string

const (
	TrackModeHelm    TrackMode = "helm"
	TrackModeKubedog TrackMode = "kubedog"
)

type TrackOptions struct {
	Timeout   time.Duration
	Logs      bool
	LogsSince time.Duration
	Filter    *resource.FilterConfig
}

func NewTrackOptions() *TrackOptions {
	return &TrackOptions{
		Timeout:   5 * time.Minute,
		LogsSince: 10 * time.Minute,
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
