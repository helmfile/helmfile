package kubedog

import (
	"context"
	"fmt"
	"math"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/werf/kubedog-for-werf-helm/pkg/tracker"
	"github.com/werf/kubedog-for-werf-helm/pkg/trackers/rollout/multitrack"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/helmfile/helmfile/pkg/resource"
)

type cacheKey struct {
	kubeContext string
	kubeconfig  string
	qps         float32
	burst       int
}

type clientCacheEntry struct {
	clientSet     kubernetes.Interface
	dynamicClient dynamic.Interface
	restConfig    *rest.Config
	discovery     discovery.CachedDiscoveryInterface
	mapper        meta.RESTMapper
}

var (
	kubeInitMu  sync.Mutex
	clientCache = make(map[cacheKey]clientCacheEntry)
)

type Tracker struct {
	logger        *zap.SugaredLogger
	clientSet     kubernetes.Interface
	dynamicClient dynamic.Interface
	discovery     discovery.CachedDiscoveryInterface
	mapper        meta.RESTMapper
	trackOptions  *TrackOptions
	filter        *resource.ResourceFilter
	namespace     string
}

type TrackerConfig struct {
	Logger       *zap.SugaredLogger
	Namespace    string
	KubeContext  string
	Kubeconfig   string
	TrackOptions *TrackOptions
	KubedogQPS   *float32
	KubedogBurst *int
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

	options := config.TrackOptions
	if options == nil {
		options = NewTrackOptions()
	}

	qps := options.QPS
	if config.KubedogQPS != nil {
		qps = *config.KubedogQPS
	}

	burst := options.Burst
	if config.KubedogBurst != nil {
		burst = *config.KubedogBurst
	}

	if qps <= 0 || math.IsInf(float64(qps), 0) || math.IsNaN(float64(qps)) {
		return nil, fmt.Errorf("invalid kubedog QPS %v: must be > 0 and finite", qps)
	}
	if burst < 1 {
		return nil, fmt.Errorf("invalid kubedog burst %v: must be >= 1", burst)
	}

	cacheEntry, err := getOrCreateClients(config.KubeContext, kubeconfig, qps, burst)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize kubernetes clients: %w", err)
	}

	var filter *resource.ResourceFilter
	if options.Filter != nil {
		filter = resource.NewResourceFilter(options.Filter, logger)
	}

	return &Tracker{
		logger:        logger,
		clientSet:     cacheEntry.clientSet,
		dynamicClient: cacheEntry.dynamicClient,
		discovery:     cacheEntry.discovery,
		mapper:        cacheEntry.mapper,
		trackOptions:  options,
		filter:        filter,
		namespace:     config.Namespace,
	}, nil
}

func getOrCreateClients(kubeContext, kubeconfig string, qps float32, burst int) (clientCacheEntry, error) {
	key := cacheKey{
		kubeContext: kubeContext,
		kubeconfig:  kubeconfig,
		qps:         qps,
		burst:       burst,
	}

	kubeInitMu.Lock()
	if cache, ok := clientCache[key]; ok {
		kubeInitMu.Unlock()
		return cache, nil
	}
	kubeInitMu.Unlock()

	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if kubeconfig != "" {
		loadingRules.ExplicitPath = kubeconfig
	}

	overrides := &clientcmd.ConfigOverrides{}
	if kubeContext != "" {
		overrides.CurrentContext = kubeContext
	}

	cc := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides)
	restConfig, err := cc.ClientConfig()
	if err != nil {
		return clientCacheEntry{}, fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	restConfig.QPS = qps
	restConfig.Burst = burst

	clientSet, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return clientCacheEntry{}, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return clientCacheEntry{}, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	discoveryClient := memory.NewMemCacheClient(clientSet.Discovery())
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(discoveryClient)

	cache := clientCacheEntry{
		clientSet:     clientSet,
		dynamicClient: dynamicClient,
		restConfig:    restConfig,
		discovery:     discoveryClient,
		mapper:        mapper,
	}

	kubeInitMu.Lock()
	defer kubeInitMu.Unlock()

	if existingCache, ok := clientCache[key]; ok {
		return existingCache, nil
	}

	clientCache[key] = cache

	return cache, nil
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
		case "canary":
			specs.Canaries = append(specs.Canaries, multitrack.MultitrackSpec{
				ResourceName: res.Name,
				Namespace:    namespace,
				SkipLogs:     !t.trackOptions.Logs,
			})
		default:
			t.logger.Debugf("Skipping unsupported kind %s for resource %s/%s", res.Kind, namespace, res.Name)
		}
	}

	totalResources := len(specs.Deployments) + len(specs.StatefulSets) +
		len(specs.DaemonSets) + len(specs.Jobs) + len(specs.Canaries)

	if totalResources == 0 {
		t.logger.Info("No trackable resources found (only Deployment, StatefulSet, DaemonSet, Job, and Canary are supported)")
		return nil
	}

	t.logger.Infof("Tracking breakdown: Deployments=%d, StatefulSets=%d, DaemonSets=%d, Jobs=%d, Canaries=%d",
		len(specs.Deployments), len(specs.StatefulSets), len(specs.DaemonSets),
		len(specs.Jobs), len(specs.Canaries))

	opts := multitrack.MultitrackOptions{
		Options: tracker.Options{
			ParentContext: ctx,
			Timeout:       t.trackOptions.Timeout,
			LogsFromTime:  time.Now().Add(-t.trackOptions.LogsSince),
		},
		StatusProgressPeriod: 5 * time.Second,
		DynamicClient:        t.dynamicClient,
		DiscoveryClient:      t.discovery,
		Mapper:               t.mapper,
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
