package kubedog

import (
	"context"
	"fmt"
	"math"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/werf/kubedog/pkg/informer"
	"github.com/werf/kubedog/pkg/tracker"
	"github.com/werf/kubedog/pkg/tracker/canary"
	"github.com/werf/kubedog/pkg/tracker/daemonset"
	"github.com/werf/kubedog/pkg/tracker/deployment"
	"github.com/werf/kubedog/pkg/tracker/job"
	"github.com/werf/kubedog/pkg/tracker/statefulset"
	"github.com/werf/kubedog/pkg/trackers/dyntracker/util"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	watchtools "k8s.io/client-go/tools/watch"

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

type trackTarget struct {
	kind      string
	name      string
	namespace string
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

	targets := t.buildTargets(filtered)
	if len(targets) == 0 {
		t.logger.Info("No trackable resources found (only Deployment, StatefulSet, DaemonSet, Job, and Canary are supported)")
		return nil
	}

	t.logger.Infof("Tracking breakdown: %s", t.targetSummary(targets))

	watchErrCh := make(chan error, len(targets))
	informerFactory := informer.NewConcurrentInformerFactory(
		ctx.Done(),
		watchErrCh,
		t.dynamicClient,
		informer.ConcurrentInformerFactoryOptions{},
	)

	opts := tracker.Options{
		ParentContext: ctx,
		Timeout:       t.trackOptions.Timeout,
		LogsFromTime:  time.Now().Add(-t.trackOptions.LogsSince),
	}

	var wg sync.WaitGroup
	errCh := make(chan error, len(targets))

	for _, target := range targets {
		wg.Add(1)
		go func(tgt trackTarget) {
			defer wg.Done()
			if err := t.trackSingleResource(tgt, informerFactory, opts); err != nil {
				errCh <- fmt.Errorf("%s/%s tracking failed: %w", tgt.kind, tgt.name, err)
			}
		}(target)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("tracking failed: %w", err)
	case <-done:
		t.logger.Info("All resources tracked successfully")
		return nil
	case <-ctx.Done():
		return fmt.Errorf("tracking canceled: %w", ctx.Err())
	}
}

func (t *Tracker) trackSingleResource(target trackTarget, informerFactory *util.Concurrent[*informer.InformerFactory], opts tracker.Options) error {
	parentContext := opts.ParentContext
	if parentContext == nil {
		parentContext = context.Background()
	}
	ctx, cancel := watchtools.ContextWithOptionalTimeout(parentContext, opts.Timeout)
	defer cancel()

	trackErrCh := make(chan error, 1)
	doneCh := make(chan struct{})

	switch target.kind {
	case "deploy":
		tr := deployment.NewTracker(target.name, target.namespace, t.clientSet, informerFactory, opts)
		go t.runDeploymentTracker(ctx, tr, trackErrCh, doneCh)
		return t.waitDeploymentTracker(ctx, tr, trackErrCh, doneCh)
	case "sts":
		tr := statefulset.NewTracker(target.name, target.namespace, t.clientSet, informerFactory, opts)
		go t.runStatefulSetTracker(ctx, tr, trackErrCh, doneCh)
		return t.waitStatefulSetTracker(ctx, tr, trackErrCh, doneCh)
	case "ds":
		tr := daemonset.NewTracker(target.name, target.namespace, t.clientSet, informerFactory, opts)
		go t.runDaemonSetTracker(ctx, tr, trackErrCh, doneCh)
		return t.waitDaemonSetTracker(ctx, tr, trackErrCh, doneCh)
	case "job":
		tr := job.NewTracker(target.name, target.namespace, t.clientSet, informerFactory, opts)
		go t.runJobTracker(ctx, tr, trackErrCh, doneCh)
		return t.waitJobTracker(ctx, tr, trackErrCh, doneCh)
	case "canary":
		tr := canary.NewTracker(target.name, target.namespace, t.clientSet, t.dynamicClient, informerFactory, opts)
		go t.runCanaryTracker(ctx, tr, trackErrCh, doneCh)
		return t.waitCanaryTracker(ctx, tr, trackErrCh, doneCh)
	default:
		return fmt.Errorf("unsupported resource kind: %s", target.kind)
	}
}

func (t *Tracker) runDeploymentTracker(ctx context.Context, tr *deployment.Tracker, errCh chan<- error, doneCh chan<- struct{}) {
	if err := tr.Track(ctx); err != nil {
		errCh <- err
	} else {
		close(doneCh)
	}
}

func (t *Tracker) waitDeploymentTracker(ctx context.Context, tr *deployment.Tracker, trackErrCh <-chan error, doneCh <-chan struct{}) error {
	for {
		select {
		case <-tr.Ready:
			t.logger.Debugf("Deployment %s/%s is ready", tr.Namespace, tr.ResourceName)
			return nil
		case status := <-tr.Failed:
			return fmt.Errorf("deployment %s/%s failed: %s", tr.Namespace, tr.ResourceName, status.FailedReason)
		case err := <-trackErrCh:
			return err
		case <-doneCh:
			return nil
		case <-ctx.Done():
			return fmt.Errorf("tracking canceled for deployment %s/%s: %w", tr.Namespace, tr.ResourceName, ctx.Err())
		}
	}
}

func (t *Tracker) runStatefulSetTracker(ctx context.Context, tr *statefulset.Tracker, errCh chan<- error, doneCh chan<- struct{}) {
	if err := tr.Track(ctx); err != nil {
		errCh <- err
	} else {
		close(doneCh)
	}
}

func (t *Tracker) waitStatefulSetTracker(ctx context.Context, tr *statefulset.Tracker, trackErrCh <-chan error, doneCh <-chan struct{}) error {
	for {
		select {
		case <-tr.Ready:
			t.logger.Debugf("StatefulSet %s/%s is ready", tr.Namespace, tr.ResourceName)
			return nil
		case status := <-tr.Failed:
			return fmt.Errorf("statefulset %s/%s failed: %s", tr.Namespace, tr.ResourceName, status.FailedReason)
		case err := <-trackErrCh:
			return err
		case <-doneCh:
			return nil
		case <-ctx.Done():
			return fmt.Errorf("tracking canceled for statefulset %s/%s: %w", tr.Namespace, tr.ResourceName, ctx.Err())
		}
	}
}

func (t *Tracker) runDaemonSetTracker(ctx context.Context, tr *daemonset.Tracker, errCh chan<- error, doneCh chan<- struct{}) {
	if err := tr.Track(ctx); err != nil {
		errCh <- err
	} else {
		close(doneCh)
	}
}

func (t *Tracker) waitDaemonSetTracker(ctx context.Context, tr *daemonset.Tracker, trackErrCh <-chan error, doneCh <-chan struct{}) error {
	for {
		select {
		case <-tr.Ready:
			t.logger.Debugf("DaemonSet %s/%s is ready", tr.Namespace, tr.ResourceName)
			return nil
		case status := <-tr.Failed:
			return fmt.Errorf("daemonset %s/%s failed: %s", tr.Namespace, tr.ResourceName, status.FailedReason)
		case err := <-trackErrCh:
			return err
		case <-doneCh:
			return nil
		case <-ctx.Done():
			return fmt.Errorf("tracking canceled for daemonset %s/%s: %w", tr.Namespace, tr.ResourceName, ctx.Err())
		}
	}
}

func (t *Tracker) runJobTracker(ctx context.Context, tr *job.Tracker, errCh chan<- error, doneCh chan<- struct{}) {
	if err := tr.Track(ctx); err != nil {
		errCh <- err
	} else {
		close(doneCh)
	}
}

func (t *Tracker) waitJobTracker(ctx context.Context, tr *job.Tracker, trackErrCh <-chan error, doneCh <-chan struct{}) error {
	for {
		select {
		case <-tr.Succeeded:
			t.logger.Debugf("Job %s/%s succeeded", tr.Namespace, tr.ResourceName)
			return nil
		case status := <-tr.Failed:
			return fmt.Errorf("job %s/%s failed: %s", tr.Namespace, tr.ResourceName, status.FailedReason)
		case err := <-trackErrCh:
			return err
		case <-doneCh:
			return nil
		case <-ctx.Done():
			return fmt.Errorf("tracking canceled for job %s/%s: %w", tr.Namespace, tr.ResourceName, ctx.Err())
		}
	}
}

func (t *Tracker) runCanaryTracker(ctx context.Context, tr *canary.Tracker, errCh chan<- error, doneCh chan<- struct{}) {
	if err := tr.Track(ctx); err != nil {
		errCh <- err
	} else {
		close(doneCh)
	}
}

func (t *Tracker) waitCanaryTracker(ctx context.Context, tr *canary.Tracker, trackErrCh <-chan error, doneCh <-chan struct{}) error {
	for {
		select {
		case <-tr.Succeeded:
			t.logger.Debugf("Canary %s/%s succeeded", tr.Namespace, tr.ResourceName)
			return nil
		case status := <-tr.Failed:
			return fmt.Errorf("canary %s/%s failed: %s", tr.Namespace, tr.ResourceName, status.FailedReason)
		case err := <-trackErrCh:
			return err
		case <-doneCh:
			return nil
		case <-ctx.Done():
			return fmt.Errorf("tracking canceled for canary %s/%s: %w", tr.Namespace, tr.ResourceName, ctx.Err())
		}
	}
}

func (t *Tracker) buildTargets(resources []*resource.Resource) []trackTarget {
	var targets []trackTarget
	for _, res := range resources {
		namespace := res.Namespace
		if namespace == "" {
			namespace = t.namespace
		}

		kind := ""
		switch strings.ToLower(res.Kind) {
		case "deployment", "deploy":
			kind = "deploy"
		case "statefulset", "sts":
			kind = "sts"
		case "daemonset", "ds":
			kind = "ds"
		case "job":
			kind = "job"
		case "canary":
			kind = "canary"
		default:
			t.logger.Debugf("Skipping unsupported kind %s for resource %s/%s", res.Kind, namespace, res.Name)
			continue
		}

		targets = append(targets, trackTarget{
			kind:      kind,
			name:      res.Name,
			namespace: namespace,
		})
	}
	return targets
}

func (t *Tracker) targetSummary(targets []trackTarget) string {
	counts := make(map[string]int)
	for _, tgt := range targets {
		counts[tgt.kind]++
	}
	parts := make([]string, 0, len(counts))
	for kind, count := range counts {
		parts = append(parts, fmt.Sprintf("%ss=%d", kind, count))
	}
	return strings.Join(parts, ", ")
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
