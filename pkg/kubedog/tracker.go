package kubedog

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/werf/kubedog/pkg/kube"
	"github.com/werf/kubedog/pkg/tracker"
	"github.com/werf/kubedog/pkg/trackers/rollout/multitrack"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"

	"github.com/helmfile/helmfile/pkg/resource"
)

type cacheKey struct {
	kubeContext string
	kubeconfig  string
}

var (
	kubeInitMu  sync.Mutex
	clientCache = make(map[cacheKey]kubernetes.Interface)
)

type Tracker struct {
	logger       *zap.SugaredLogger
	clientSet    kubernetes.Interface
	trackOptions *TrackOptions
	filter       *resource.ResourceFilter
	namespace    string
}

type TrackerConfig struct {
	Logger       *zap.SugaredLogger
	Namespace    string
	KubeContext  string
	Kubeconfig   string
	TrackOptions *TrackOptions
}

func NewTracker(config *TrackerConfig) (*Tracker, error) {
	logger := config.Logger
	if logger == nil {
		logger = zap.NewNop().Sugar()
	}

	kubeconfig := config.Kubeconfig
	if kubeconfig == "" {
		kubeconfig = os.Getenv("KUBECONFIG")
	}

	clientSet, err := getOrCreateClient(config.KubeContext, kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize kubernetes client: %w", err)
	}

	options := config.TrackOptions
	if options == nil {
		options = NewTrackOptions()
	}

	var filter *resource.ResourceFilter
	if options.Filter != nil {
		filter = resource.NewResourceFilter(options.Filter, logger)
	}

	return &Tracker{
		logger:       logger,
		clientSet:    clientSet,
		trackOptions: options,
		filter:       filter,
		namespace:    config.Namespace,
	}, nil
}

func getOrCreateClient(kubeContext, kubeconfig string) (kubernetes.Interface, error) {
	key := cacheKey{
		kubeContext: kubeContext,
		kubeconfig:  kubeconfig,
	}

	kubeInitMu.Lock()
	defer kubeInitMu.Unlock()

	if client, ok := clientCache[key]; ok {
		return client, nil
	}

	initOpts := kube.InitOptions{
		KubeConfigOptions: kube.KubeConfigOptions{
			Context:    kubeContext,
			ConfigPath: kubeconfig,
		},
	}

	if err := kube.Init(initOpts); err != nil {
		return nil, err
	}

	client := kube.Kubernetes
	clientCache[key] = client

	return client, nil
}

func (t *Tracker) TrackResources(ctx context.Context, resources []*resource.Resource) error {
	if len(resources) == 0 {
		t.logger.Info("No resources to track")
		return nil
	}

	filtered := t.filterResources(resources)
	if len(filtered) == 0 {
		t.logger.Info("No resources to track after filtering")
		return nil
	}

	t.logger.Infof("Tracking %d resources with kubedog (filtered from %d total)", len(filtered), len(resources))

	specs := multitrack.MultitrackSpecs{}

	for _, res := range filtered {
		namespace := res.Namespace
		if namespace == "" {
			namespace = t.namespace
		}

		switch strings.ToLower(res.Kind) {
		case "deployment", "deploy":
			specs.Deployments = append(specs.Deployments, multitrack.MultitrackSpec{
				ResourceName: res.Name,
				Namespace:    namespace,
				SkipLogs:     !t.trackOptions.Logs,
			})
		case "statefulset", "sts":
			specs.StatefulSets = append(specs.StatefulSets, multitrack.MultitrackSpec{
				ResourceName: res.Name,
				Namespace:    namespace,
				SkipLogs:     !t.trackOptions.Logs,
			})
		case "daemonset", "ds":
			specs.DaemonSets = append(specs.DaemonSets, multitrack.MultitrackSpec{
				ResourceName: res.Name,
				Namespace:    namespace,
				SkipLogs:     !t.trackOptions.Logs,
			})
		case "job":
			specs.Jobs = append(specs.Jobs, multitrack.MultitrackSpec{
				ResourceName: res.Name,
				Namespace:    namespace,
				SkipLogs:     !t.trackOptions.Logs,
			})
		default:
			t.logger.Debugf("Skipping unsupported kind %s for resource %s/%s", res.Kind, namespace, res.Name)
		}
	}

	if len(specs.Deployments)+len(specs.StatefulSets)+len(specs.DaemonSets)+len(specs.Jobs) == 0 {
		t.logger.Info("No trackable resources found (only Deployment, StatefulSet, DaemonSet, and Job are supported)")
		return nil
	}

	opts := multitrack.MultitrackOptions{
		Options: tracker.Options{
			ParentContext: ctx,
			Timeout:       t.trackOptions.Timeout,
			LogsFromTime:  time.Now().Add(-t.trackOptions.LogsSince),
		},
		StatusProgressPeriod: 5 * time.Second,
	}

	err := multitrack.Multitrack(t.clientSet, specs, opts)
	if err != nil {
		return fmt.Errorf("tracking failed: %w", err)
	}

	t.logger.Info("All resources tracked successfully")
	return nil
}

func (t *Tracker) filterResources(resources []*resource.Resource) []*resource.Resource {
	if t.filter == nil {
		return resources
	}

	var result []*resource.Resource
	for _, res := range resources {
		if t.filter.ShouldTrack(res) {
			result = append(result, res)
		} else {
			t.logger.Debugf("Skipping resource %s/%s (kind: %s) based on configuration", res.Namespace, res.Name, res.Kind)
		}
	}
	return result
}
