package kubedog

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/werf/kubedog/pkg/informer"
	"github.com/werf/kubedog/pkg/trackers/dyntracker"
	"github.com/werf/kubedog/pkg/trackers/dyntracker/logstore"
	"github.com/werf/kubedog/pkg/trackers/dyntracker/statestore"
	kdutil "github.com/werf/kubedog/pkg/trackers/dyntracker/util"
	"go.uber.org/zap"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
	mapper        meta.ResettableRESTMapper
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
	mapper        meta.ResettableRESTMapper
	trackOptions  *TrackOptions
	filter        *resource.ResourceFilter
	namespace     string

	// upstreamDoneCh is closed when the calling code (e.g. helm.SyncRelease)
	// finishes. Per-resource freshness gates use it to give up waiting for a
	// generation bump that will never come (e.g. when helm produced no diff
	// for that resource).
	upstreamDoneCh chan struct{}
	upstreamOnce   sync.Once
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
		logger:         logger,
		clientSet:      cacheEntry.clientSet,
		dynamicClient:  cacheEntry.dynamicClient,
		discovery:      cacheEntry.discovery,
		mapper:         cacheEntry.mapper,
		trackOptions:   options,
		filter:         filter,
		namespace:      config.Namespace,
		upstreamDoneCh: make(chan struct{}),
	}, nil
}

// MarkUpstreamCompleted signals to any in-flight freshness gates that the
// upstream operation (helm upgrade) has finished. Resources whose baseline
// generation never bumped will then exit their gate without attaching kubedog
// rather than wait indefinitely. Safe to call multiple times.
func (t *Tracker) MarkUpstreamCompleted() {
	t.upstreamOnce.Do(func() {
		close(t.upstreamDoneCh)
	})
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
	gvk       schema.GroupVersionKind
}

// BaselineKey returns the map key used to associate a resource with its
// pre-change ResourceBaseline.
func BaselineKey(kind, namespace, name string) string {
	return kind + "/" + namespace + "/" + name
}

// CaptureBaselines fetches the current UID and metadata.generation for each
// resource via the dynamic client. Resources that don't exist yet are
// recorded with Exists=false so the freshness gate can detect first creation.
// Errors other than NotFound are logged and the baseline is omitted (the
// gate will then attach immediately rather than block on a degraded probe).
func (t *Tracker) CaptureBaselines(ctx context.Context, resources []*resource.Resource) map[string]ResourceBaseline {
	baselines := make(map[string]ResourceBaseline, len(resources))
	for _, res := range resources {
		ns := res.Namespace
		if ns == "" {
			ns = t.namespace
		}
		kind, gvk, ok := classifyResource(res.Kind)
		if !ok {
			continue
		}
		gvr, err := t.gvrFor(gvk)
		if err != nil {
			t.logger.Debugf("kubedog: cannot resolve GVR for %s/%s/%s baseline: %v", kind, ns, res.Name, err)
			continue
		}
		key := BaselineKey(kind, ns, res.Name)
		obj, err := t.dynamicClient.Resource(gvr).Namespace(ns).Get(ctx, res.Name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				baselines[key] = ResourceBaseline{Exists: false}
				continue
			}
			t.logger.Debugf("kubedog: baseline fetch for %s/%s/%s failed: %v", kind, ns, res.Name, err)
			continue
		}
		baselines[key] = ResourceBaseline{
			UID:        obj.GetUID(),
			Generation: obj.GetGeneration(),
			Exists:     true,
		}
	}
	return baselines
}

func (t *Tracker) gvrFor(gvk schema.GroupVersionKind) (schema.GroupVersionResource, error) {
	mapping, err := t.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return schema.GroupVersionResource{}, err
	}
	return mapping.Resource, nil
}

// errUpstreamDoneNoChange signals that the upstream operation (helm) finished
// without the resource ever changing — the goroutine should skip the dyntracker
// attach and exit cleanly.
var errUpstreamDoneNoChange = errors.New("upstream completed without resource change")

// waitForFreshness blocks until the resource has either appeared (when it
// didn't exist at baseline), been recreated (UID changed), or had its spec
// updated (generation > baseline). Returns nil once the resource is fresh,
// errUpstreamDoneNoChange when the upstream operation completed and no change
// was ever observed, or ctx.Err() on cancellation.
func (t *Tracker) waitForFreshness(ctx context.Context, tgt trackTarget, baseline ResourceBaseline, statusCb func(string)) error {
	gvr, err := t.gvrFor(tgt.gvk)
	if err != nil {
		t.logger.Debugf("kubedog: cannot resolve GVR for %s/%s/%s, skipping freshness gate: %v", tgt.kind, tgt.namespace, tgt.name, err)
		return nil
	}

	if statusCb != nil {
		if baseline.Exists {
			statusCb(fmt.Sprintf("waiting for update (uid=%s gen=%d)", baseline.UID, baseline.Generation))
		} else {
			statusCb("waiting for creation")
		}
	}

	const (
		pollInterval = 500 * time.Millisecond
		// Grace window after helm finishes: keep checking briefly in case the
		// API server lags. If nothing observed in this window, give up.
		postUpstreamGrace = 3 * time.Second
	)
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	probe := func() (fresh bool) {
		obj, err := t.dynamicClient.Resource(gvr).Namespace(tgt.namespace).Get(ctx, tgt.name, metav1.GetOptions{})
		if err != nil {
			if !apierrors.IsNotFound(err) {
				t.logger.Debugf("kubedog: freshness probe for %s/%s/%s failed: %v", tgt.kind, tgt.namespace, tgt.name, err)
			}
			return false
		}
		if !baseline.Exists {
			return true
		}
		if obj.GetUID() != baseline.UID {
			return true
		}
		if obj.GetGeneration() > baseline.Generation {
			return true
		}
		return false
	}

	var upstreamDoneAt time.Time
	for {
		if probe() {
			return nil
		}
		if !upstreamDoneAt.IsZero() && time.Since(upstreamDoneAt) >= postUpstreamGrace {
			return errUpstreamDoneNoChange
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.upstreamDoneCh:
			if upstreamDoneAt.IsZero() {
				upstreamDoneAt = time.Now()
			}
			// Fast retry once helm finishes, then fall through to the next
			// ticker to enforce the grace window.
		case <-ticker.C:
		}
	}
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

	trackCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	taskStore := kdutil.NewConcurrent(statestore.NewTaskStore())
	logStore := kdutil.NewConcurrent(logstore.NewLogStore())

	watchErrCh := make(chan error, 16)
	informerFactory := informer.NewConcurrentInformerFactory(
		trackCtx.Done(), watchErrCh, t.dynamicClient,
		informer.ConcurrentInformerFactoryOptions{},
	)

	// Drain informer watch errors so the factory doesn't block.
	go func() {
		for {
			select {
			case err, ok := <-watchErrCh:
				if !ok {
					return
				}
				if err != nil {
					t.logger.Warnf("kubedog informer watch error: %v", err)
				}
			case <-trackCtx.Done():
				return
			}
		}
	}()

	captureLogsFromTime := time.Now().Add(-t.trackOptions.LogsSince)
	ignoreLogs := !t.trackOptions.Logs

	type trackerEntry struct {
		target    trackTarget
		taskState *kdutil.Concurrent[*statestore.ReadinessTaskState]
	}
	entries := make([]trackerEntry, 0, len(targets))

	for _, tgt := range targets {
		taskState := kdutil.NewConcurrent(
			statestore.NewReadinessTaskState(tgt.name, tgt.namespace, tgt.gvk, statestore.ReadinessTaskStateOptions{}),
		)
		taskStore.RWTransaction(func(s *statestore.TaskStore) {
			s.AddReadinessTaskState(taskState)
		})
		entries = append(entries, trackerEntry{target: tgt, taskState: taskState})
	}

	gateStatuses := newGateStatuses()
	skippedKeys := newSkippedKeys()
	printer := newProgressPrinter(t.logger, taskStore, logStore, ignoreLogs, gateStatuses, skippedKeys, t.trackOptions.Color)
	printerDone := make(chan struct{})

	var trackerWg sync.WaitGroup
	errCh := make(chan error, len(entries))

	for _, entry := range entries {
		trackerWg.Add(1)
		tgt := entry.target
		ts := entry.taskState
		baselineKey := BaselineKey(tgt.kind, tgt.namespace, tgt.name)
		baseline, hasBaseline := t.trackOptions.Baselines[baselineKey]
		go func() {
			defer trackerWg.Done()

			if hasBaseline {
				err := t.waitForFreshness(trackCtx, tgt, baseline, func(msg string) {
					gateStatuses.set(baselineKey, msg)
				})
				gateStatuses.clear(baselineKey)
				switch {
				case err == nil:
					// resource changed; proceed to attach the tracker
				case errors.Is(err, errUpstreamDoneNoChange):
					t.logger.Debugf("kubedog: %s/%s/%s unchanged by upstream; skipping tracker", tgt.kind, tgt.namespace, tgt.name)
					// Hide this task from the printer output: it never changed,
					// so the persistent "progressing" we'd otherwise show is
					// misleading.
					skippedKeys.add(kdutil.ResourceID(tgt.name, tgt.namespace, tgt.gvk))
					return
				case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
					return
				default:
					errCh <- fmt.Errorf("%s/%s freshness gate failed: %w", tgt.kind, tgt.name, err)
					return
				}
			}

			dt, err := dyntracker.NewDynamicReadinessTracker(
				trackCtx, ts, logStore, informerFactory,
				t.clientSet, t.dynamicClient, t.discovery, t.mapper,
				dyntracker.DynamicReadinessTrackerOptions{
					Timeout:             t.trackOptions.Timeout,
					CaptureLogsFromTime: captureLogsFromTime,
					IgnoreLogs:          ignoreLogs,
					// Kubedog uses this as a "stream logs for at most N replicas"
					// cap. The zero value means "0 replicas", which silently disables
					// log streaming even when IgnoreLogs is false. Use a high value
					// so all replicas in any realistic deployment stream their logs.
					SaveLogsOnlyForNumberOfReplicas: math.MaxInt32,
				},
			)
			if err != nil {
				errCh <- fmt.Errorf("create dynamic tracker for %s/%s: %w", tgt.kind, tgt.name, err)
				return
			}
			if err := dt.Track(trackCtx); err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					return
				}
				errCh <- fmt.Errorf("%s/%s tracking failed: %w", tgt.kind, tgt.name, err)
			}
		}()
	}

	trackersDone := make(chan struct{})
	go func() {
		trackerWg.Wait()
		close(trackersDone)
	}()

	go func() {
		printer.run(trackCtx, trackersDone)
		close(printerDone)
	}()

	var firstErr error
	select {
	case err := <-errCh:
		firstErr = err
		cancel()
	case <-trackersDone:
	case <-trackCtx.Done():
		firstErr = trackCtx.Err()
	}

	cancel()
	<-trackersDone
	<-printerDone

	// Drain remaining tracker errors so they're surfaced in logs.
	close(errCh)
	for err := range errCh {
		if firstErr == nil {
			firstErr = err
		} else {
			t.logger.Warnf("additional tracking error: %v", err)
		}
	}

	if firstErr != nil {
		return firstErr
	}

	t.logger.Info("All resources tracked successfully")
	return nil
}

func (t *Tracker) buildTargets(resources []*resource.Resource) []trackTarget {
	var targets []trackTarget
	for _, res := range resources {
		namespace := res.Namespace
		if namespace == "" {
			namespace = t.namespace
		}

		kind, gvk, ok := classifyResource(res.Kind)
		if !ok {
			t.logger.Debugf("Skipping unsupported kind %s for resource %s/%s", res.Kind, namespace, res.Name)
			continue
		}

		targets = append(targets, trackTarget{
			kind:      kind,
			name:      res.Name,
			namespace: namespace,
			gvk:       gvk,
		})
	}
	return targets
}

func classifyResource(rawKind string) (string, schema.GroupVersionKind, bool) {
	switch strings.ToLower(rawKind) {
	case "deployment", "deploy":
		return "deploy", schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}, true
	case "statefulset", "sts":
		return "sts", schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "StatefulSet"}, true
	case "daemonset", "ds":
		return "ds", schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "DaemonSet"}, true
	case "job":
		return "job", schema.GroupVersionKind{Group: "batch", Version: "v1", Kind: "Job"}, true
	case "canary":
		return "canary", schema.GroupVersionKind{Group: "flagger.app", Version: "v1beta1", Kind: "Canary"}, true
	}
	return "", schema.GroupVersionKind{}, false
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
	sort.Strings(parts)
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
