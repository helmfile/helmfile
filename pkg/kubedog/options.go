package kubedog

import "time"

type TrackMode string

const (
	TrackModeHelm    TrackMode = "helm"
	TrackModeKubedog TrackMode = "kubedog"
)

type ResourceSpec struct {
	Name      string
	Namespace string
	Kind      string
}

type TrackOptions struct {
	Timeout       time.Duration
	Logs          bool
	LogsSince     time.Duration
	ContainerLogs []string
	Namespace     string
	KubeContext   string
	Kubeconfig    string
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

func (o *TrackOptions) WithLogsSince(since time.Duration) *TrackOptions {
	o.LogsSince = since
	return o
}

func (o *TrackOptions) WithContainerLogs(containers []string) *TrackOptions {
	o.ContainerLogs = containers
	return o
}

func (o *TrackOptions) WithNamespace(namespace string) *TrackOptions {
	o.Namespace = namespace
	return o
}

func (o *TrackOptions) WithKubeContext(context string) *TrackOptions {
	o.KubeContext = context
	return o
}

func (o *TrackOptions) WithKubeconfig(kubeconfig string) *TrackOptions {
	o.Kubeconfig = kubeconfig
	return o
}
