package kubedog

import (
	"context"
	"fmt"
	"time"

	"github.com/werf/kubedog/pkg/kube"
	"github.com/werf/kubedog/pkg/tracker"
	"github.com/werf/kubedog/pkg/trackers/rollout"
	"github.com/werf/kubedog/pkg/trackers/rollout/multitrack"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
)

type Tracker struct {
	logger       *zap.SugaredLogger
	clientSet    kubernetes.Interface
	trackOptions *TrackOptions
	kubeContext  string
	kubeconfig   string
}

type TrackerConfig struct {
	Logger       *zap.SugaredLogger
	Namespace    string
	KubeContext  string
	Kubeconfig   string
	TrackOptions *TrackOptions
}

func NewTracker(config *TrackerConfig) (*Tracker, error) {
	initOpts := kube.InitOptions{
		KubeConfigOptions: kube.KubeConfigOptions{
			Context:    config.KubeContext,
			ConfigPath: config.Kubeconfig,
		},
	}

	if kubeErr := kube.Init(initOpts); kubeErr != nil {
		return nil, fmt.Errorf("failed to initialize kubedog kube client: %w", kubeErr)
	}

	options := config.TrackOptions
	if options == nil {
		options = NewTrackOptions()
	}

	return &Tracker{
		logger:       config.Logger,
		clientSet:    kube.Kubernetes,
		trackOptions: options,
		kubeContext:  config.KubeContext,
		kubeconfig:   config.Kubeconfig,
	}, nil
}

func (t *Tracker) TrackResources(ctx context.Context, resources []*ResourceSpec) error {
	if len(resources) == 0 {
		t.logger.Info("No resources to track")
		return nil
	}

	t.logger.Infof("Tracking %d resources with kubedog", len(resources))

	specs := multitrack.MultitrackSpecs{}

	for _, res := range resources {
		switch res.Kind {
		case "deployment", "deploy":
			specs.Deployments = append(specs.Deployments, multitrack.MultitrackSpec{
				ResourceName: res.Name,
				Namespace:    res.Namespace,
				SkipLogs:     !t.trackOptions.Logs,
			})
		case "statefulset", "sts":
			specs.StatefulSets = append(specs.StatefulSets, multitrack.MultitrackSpec{
				ResourceName: res.Name,
				Namespace:    res.Namespace,
				SkipLogs:     !t.trackOptions.Logs,
			})
		case "daemonset", "ds":
			specs.DaemonSets = append(specs.DaemonSets, multitrack.MultitrackSpec{
				ResourceName: res.Name,
				Namespace:    res.Namespace,
				SkipLogs:     !t.trackOptions.Logs,
			})
		case "job":
			specs.Jobs = append(specs.Jobs, multitrack.MultitrackSpec{
				ResourceName: res.Name,
				Namespace:    res.Namespace,
				SkipLogs:     !t.trackOptions.Logs,
			})
		}
	}

	opts := multitrack.MultitrackOptions{
		Options: tracker.Options{
			ParentContext: ctx,
			Timeout:       t.trackOptions.Timeout,
			LogsFromTime:  time.Now().Add(-t.trackOptions.LogsSince),
		},
	}

	err := multitrack.Multitrack(t.clientSet, specs, opts)
	if err != nil {
		return fmt.Errorf("tracking failed: %w", err)
	}

	t.logger.Info("All resources tracked successfully")
	return nil
}

func (t *Tracker) TrackPod(ctx context.Context, podName, namespace string) error {
	t.logger.Infof("Tracking pod %s/%s with kubedog", namespace, podName)

	opts := tracker.Options{
		ParentContext: ctx,
		Timeout:       t.trackOptions.Timeout,
		LogsFromTime:  time.Now().Add(-t.trackOptions.LogsSince),
	}

	err := rollout.TrackPodTillReady(podName, namespace, t.clientSet, opts)
	if err != nil {
		return fmt.Errorf("pod tracking failed: %w", err)
	}

	t.logger.Infof("Pod %s tracked successfully", podName)
	return nil
}

func (t *Tracker) TrackDeployment(ctx context.Context, deploymentName, namespace string) error {
	t.logger.Infof("Tracking deployment %s/%s with kubedog", namespace, deploymentName)

	opts := tracker.Options{
		ParentContext: ctx,
		Timeout:       t.trackOptions.Timeout,
		LogsFromTime:  time.Now().Add(-t.trackOptions.LogsSince),
	}

	err := rollout.TrackDeploymentTillReady(deploymentName, namespace, t.clientSet, opts)
	if err != nil {
		return fmt.Errorf("deployment tracking failed: %w", err)
	}

	t.logger.Infof("Deployment %s tracked successfully", deploymentName)
	return nil
}

func (t *Tracker) TrackStatefulSet(ctx context.Context, stsName, namespace string) error {
	t.logger.Infof("Tracking statefulset %s/%s with kubedog", namespace, stsName)

	opts := tracker.Options{
		ParentContext: ctx,
		Timeout:       t.trackOptions.Timeout,
		LogsFromTime:  time.Now().Add(-t.trackOptions.LogsSince),
	}

	err := rollout.TrackStatefulSetTillReady(stsName, namespace, t.clientSet, opts)
	if err != nil {
		return fmt.Errorf("statefulset tracking failed: %w", err)
	}

	t.logger.Infof("StatefulSet %s tracked successfully", stsName)
	return nil
}

func (t *Tracker) TrackDaemonSet(ctx context.Context, dsName, namespace string) error {
	t.logger.Infof("Tracking daemonset %s/%s with kubedog", namespace, dsName)

	opts := tracker.Options{
		ParentContext: ctx,
		Timeout:       t.trackOptions.Timeout,
		LogsFromTime:  time.Now().Add(-t.trackOptions.LogsSince),
	}

	err := rollout.TrackDaemonSetTillReady(dsName, namespace, t.clientSet, opts)
	if err != nil {
		return fmt.Errorf("daemonset tracking failed: %w", err)
	}

	t.logger.Infof("DaemonSet %s tracked successfully", dsName)
	return nil
}

func (t *Tracker) TrackJob(ctx context.Context, jobName, namespace string) error {
	t.logger.Infof("Tracking job %s/%s with kubedog", namespace, jobName)

	opts := tracker.Options{
		ParentContext: ctx,
		Timeout:       t.trackOptions.Timeout,
		LogsFromTime:  time.Now().Add(-t.trackOptions.LogsSince),
	}

	err := rollout.TrackJobTillDone(jobName, namespace, t.clientSet, opts)
	if err != nil {
		return fmt.Errorf("job tracking failed: %w", err)
	}

	t.logger.Infof("Job %s tracked successfully", jobName)
	return nil
}

func (t *Tracker) Close() error {
	return nil
}
