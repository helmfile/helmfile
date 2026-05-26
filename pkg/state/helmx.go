package state

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/helmfile/chartify"
	"go.uber.org/zap"
	"helm.sh/helm/v4/pkg/storage/driver"

	"github.com/helmfile/helmfile/pkg/filesystem"
	"github.com/helmfile/helmfile/pkg/helmexec"
	"github.com/helmfile/helmfile/pkg/kubedog"
	"github.com/helmfile/helmfile/pkg/remote"
	"github.com/helmfile/helmfile/pkg/resource"
	"github.com/helmfile/helmfile/pkg/tmpl"
)

type Dependency struct {
	Chart   string `yaml:"chart"`
	Version string `yaml:"version"`
	Alias   string `yaml:"alias"`
}

func (st *HelmState) appendHelmXFlags(flags []string, release *ReleaseSpec) []string {
	for _, adopt := range release.Adopt {
		flags = append(flags, "--adopt", adopt)
	}

	return flags
}

func formatLabels(labels map[string]string) string {
	var labelsList, keys []string
	for k := range labels {
		if k == "" || slices.Contains(driver.GetSystemLabels(), k) {
			continue
		}

		keys = append(keys, k)
	}
	sort.Strings(keys)

	if len(keys) == 0 {
		return ""
	}

	for _, k := range keys {
		val := labels[k]
		labelsList = append(labelsList, fmt.Sprintf("%s=%s", k, val))
	}
	return strings.Join(labelsList, ",")
}

// append labels flags to helm flags, starting from helm v3.13.0
func (st *HelmState) appendLabelsFlags(flags []string, helm helmexec.Interface, release *ReleaseSpec, syncReleaseLabels bool) []string {
	if !helm.IsVersionAtLeast("3.13.0") {
		return flags
	}
	isSyncReleaseLabels := false
	switch {
	// Check if SyncReleaseLabels is true in the release spec.
	case release.SyncReleaseLabels != nil && *release.SyncReleaseLabels:
		isSyncReleaseLabels = true
	// Check if syncReleaseLabels argument is true.
	case syncReleaseLabels:
		isSyncReleaseLabels = true
	// Check if SyncReleaseLabels is true in HelmDefaults.
	case st.HelmDefaults.SyncReleaseLabels != nil && *st.HelmDefaults.SyncReleaseLabels:
		isSyncReleaseLabels = true
	}
	if isSyncReleaseLabels {
		labels := formatLabels(release.Labels)
		if labels != "" {
			flags = append(flags, "--labels", labels)
		}
	}
	return flags
}

// append post-renderer flags to helm flags
func (st *HelmState) appendPostRenderFlags(flags []string, release *ReleaseSpec, postRenderer string, helm helmexec.Interface) []string {
	var rendererPath string
	switch {
	// postRenderer arg comes from cmd flag.
	case release.PostRenderer != nil && *release.PostRenderer != "":
		rendererPath = *release.PostRenderer
	case postRenderer != "":
		rendererPath = postRenderer
	case st.HelmDefaults.PostRenderer != nil && *st.HelmDefaults.PostRenderer != "":
		rendererPath = *st.HelmDefaults.PostRenderer
	}

	if rendererPath != "" {
		// For Helm 4, convert the bash script path to a plugin name
		if helm != nil && !helm.IsHelm3() {
			// Check if this is a bash script path that needs conversion
			if strings.HasSuffix(rendererPath, ".bash") || strings.HasSuffix(rendererPath, ".sh") {
				// Extract the base name (e.g., "add-cm1" from "../../postrenderers/add-cm1.bash")
				baseName := filepath.Base(rendererPath)
				baseName = strings.TrimSuffix(baseName, filepath.Ext(baseName))

				// For Helm 4, use just the plugin name
				// From: ../../postrenderers/add-cm1.bash
				// To: add-cm1 (assuming the plugin is installed with this name)
				rendererPath = baseName
			}
		}
		flags = append(flags, "--post-renderer", rendererPath)
	}
	return flags
}

// append post-renderer-args flags to helm flags
func (st *HelmState) appendPostRenderArgsFlags(flags []string, release *ReleaseSpec, postRendererArgs []string) ([]string, error) {
	postRendererArgsFlags := []string{}
	switch {
	case len(release.PostRendererArgs) != 0:
		postRendererArgsFlags = release.PostRendererArgs
	case len(postRendererArgs) != 0:
		postRendererArgsFlags = postRendererArgs
	case len(st.HelmDefaults.PostRendererArgs) != 0:
		rendered, err := st.renderPostRendererArgs(release, st.HelmDefaults.PostRendererArgs)
		if err != nil {
			return nil, err
		}
		postRendererArgsFlags = rendered
	}
	for _, arg := range postRendererArgsFlags {
		if arg != "" {
			flags = append(flags, "--post-renderer-args="+arg)
		}
	}
	return flags, nil
}

func (st *HelmState) renderPostRendererArgs(release *ReleaseSpec, args []string) ([]string, error) {
	vals := st.RenderedValues
	if vals == nil {
		vals = make(map[string]any)
	}

	fs := st.fs
	if fs == nil {
		fs = filesystem.DefaultFileSystem()
	}

	tmplData := st.createReleaseTemplateData(release, vals)
	renderer := tmpl.NewFileRenderer(fs, st.basePath, tmplData)

	result := make([]string, 0, len(args))
	for _, arg := range args {
		rendered, err := renderer.RenderTemplateContentToString([]byte(arg))
		if err != nil {
			return nil, fmt.Errorf("failed rendering postRendererArg %q for release %q: %w", arg, release.Name, err)
		}
		result = append(result, rendered)
	}

	return result, nil
}

// append skip-schema-validation flags to helm flags
func (st *HelmState) appendSkipSchemaValidationFlags(flags []string, release *ReleaseSpec, skipSchemaValidation bool) []string {
	if st.shouldSkipSchemaValidation(release, skipSchemaValidation) {
		flags = append(flags, "--skip-schema-validation")
	}
	return flags
}

func (st *HelmState) shouldSkipSchemaValidation(release *ReleaseSpec, skipSchemaValidation bool) bool {
	switch {
	// Check if SkipSchemaValidation is true in the release spec.
	case release.SkipSchemaValidation != nil && *release.SkipSchemaValidation:
		return true
	// Check if skipSchemaValidation argument is true.
	case skipSchemaValidation:
		return true
	// Check if SkipSchemaValidation is true in HelmDefaults.
	case st.HelmDefaults.SkipSchemaValidation != nil && *st.HelmDefaults.SkipSchemaValidation:
		return true
	default:
		return false
	}
}

// append suppress-output-line-regex flags to helm diff flags
func (st *HelmState) appendSuppressOutputLineRegexFlags(flags []string, release *ReleaseSpec, suppressOutputLineRegex []string) []string {
	suppressOutputLineRegexFlags := []string{}
	switch {
	case len(release.SuppressOutputLineRegex) != 0:
		suppressOutputLineRegexFlags = release.SuppressOutputLineRegex
	case len(suppressOutputLineRegex) != 0:
		suppressOutputLineRegexFlags = suppressOutputLineRegex
	case len(st.HelmDefaults.SuppressOutputLineRegex) != 0:
		suppressOutputLineRegexFlags = st.HelmDefaults.SuppressOutputLineRegex
	}
	for _, arg := range suppressOutputLineRegexFlags {
		if arg != "" {
			flags = append(flags, "--suppress-output-line-regex", arg)
		}
	}
	return flags
}

func (st *HelmState) appendWaitForJobsFlags(flags []string, release *ReleaseSpec, ops *SyncOpts) []string {
	if st.shouldUseKubedog(release, ops) {
		return flags
	}

	switch {
	case release.WaitForJobs != nil && *release.WaitForJobs:
		flags = append(flags, "--wait-for-jobs")
	case ops != nil && ops.WaitForJobs:
		flags = append(flags, "--wait-for-jobs")
	case release.WaitForJobs == nil && st.HelmDefaults.WaitForJobs:
		flags = append(flags, "--wait-for-jobs")
	}

	return flags
}

func (st *HelmState) shouldUseKubedog(release *ReleaseSpec, ops *SyncOpts) bool {
	return st.getTrackMode(release, ops) == string(kubedog.TrackModeKubedog)
}

func (st *HelmState) shouldFailOnTrackError(release *ReleaseSpec, ops *SyncOpts) bool {
	if release.TrackFailOnError != nil {
		return *release.TrackFailOnError
	}
	if ops != nil {
		return ops.TrackFailOnError
	}
	return false
}

// trackReleaseIfEnabled performs kubedog tracking for a release if trackMode is "kubedog".
// It returns a ReleaseError if tracking fails and shouldFailOnTrackError is true.
// The caller is responsible for mutating affectedReleases when needed.
func (st *HelmState) trackReleaseIfEnabled(ctx context.Context, release *ReleaseSpec, helm helmexec.Interface, opts *SyncOpts) *ReleaseError {
	if !st.shouldUseKubedog(release, opts) {
		return nil
	}
	if trackErr := st.trackWithKubedog(ctx, release, helm, opts); trackErr != nil {
		st.logger.Warnf("kubedog tracking failed for release %s: %v", release.Name, trackErr)
		if st.shouldFailOnTrackError(release, opts) {
			return newReleaseFailedError(release, trackErr)
		}
	}
	return nil
}

// kubedogTrackingHandle bundles the closures the caller needs to coordinate
// parallel kubedog tracking with helm execution. See
// startBackgroundKubedogTracking for the lifecycle.
type kubedogTrackingHandle struct {
	// Helm is the helm.Interface the caller MUST use for SyncRelease and any
	// follow-up commands (e.g. listReleases) while tracking is running. It's
	// a logger-scoped clone of the original helm that captures all its output
	// into the per-release buffer so it doesn't interleave with kubedog
	// progress on stdout.
	Helm helmexec.Interface
	// Wait blocks until the tracker exits and returns the resulting error
	// (already shaped by trackFailOnError policy).
	Wait func() *ReleaseError
	// Cancel cancels the tracker; call when helm itself fails.
	Cancel func()
	// NotifyHelmDone signals to in-flight freshness gates that helm finished
	// so unchanged resources can stop waiting for a generation bump.
	NotifyHelmDone func()
	// FlushBufferedHelmOutput emits the captured helm output (upgrades, list,
	// etc.) through the real logger in a single block. Safe to call multiple
	// times — second call is a no-op.
	FlushBufferedHelmOutput func()
	// WasHelmKilled reports whether the safety-valve helm-killer fired during
	// tracking. When true, the caller MUST treat any error returned by helm
	// SyncRelease as success — helm was deliberately interrupted because the
	// cluster confirmed convergence and helm wedged on its hook waiter.
	WasHelmKilled func() bool
}

// startBackgroundKubedogTracking templates the release upfront and starts a
// kubedog tracker in a goroutine so it runs in parallel with helm. It returns
// a handle whose Helm field MUST be used for the helm calls during the
// tracking window — that helm clone buffers output for clean ordering.
//
// When started is false the caller must fall back to the sequential
// trackReleaseIfEnabled path (e.g. templating failed before helm ran, so we
// retry after helm finishes to preserve the previous behavior).
func (st *HelmState) startBackgroundKubedogTracking(
	ctx context.Context, release *ReleaseSpec, helm helmexec.Interface, opts *SyncOpts,
) (h *kubedogTrackingHandle, started bool) {
	noop := &kubedogTrackingHandle{
		Helm:                    helm,
		Wait:                    func() *ReleaseError { return nil },
		Cancel:                  func() {},
		NotifyHelmDone:          func() {},
		FlushBufferedHelmOutput: func() {},
		WasHelmKilled:           func() bool { return false },
	}

	if !st.shouldUseKubedog(release, opts) {
		return noop, false
	}

	useColor := false
	if opts != nil {
		useColor = opts.Color && !opts.NoColor
	}

	// Capture the per-release preamble (header + helm template "Templating
	// release=" / "wrote ..." lines + "Found N resources" line) into one
	// buffer and emit it via st.logger as a single atomic entry. Without
	// this, parallel releases interleave their template output and it
	// becomes very hard to follow which `wrote ...` line belongs to which
	// chart. Helm output during install/upgrade is buffered separately
	// (helmOutputBuf below) and flushed after tracking finishes.
	preambleBuf := &bytes.Buffer{}
	preambleLogger := helmexec.NewLogger(preambleBuf, "info")
	preambleLogger.Infof("\n%s", kubedog.HeaderDividerStyled(fmt.Sprintf("Release '%s'", release.Name), useColor))

	tmplHelm := helm
	if swapper, ok := helm.(helmexec.LoggerSwapper); ok {
		tmplHelm = swapper.WithLogger(preambleLogger)
	}

	flushPreamble := func() {
		if preambleBuf.Len() == 0 {
			return
		}
		st.logger.Infof("%s", strings.TrimRight(preambleBuf.String(), "\n"))
		preambleBuf.Reset()
	}

	resources, err := st.getReleaseResources(ctx, release, tmplHelm, preambleLogger)
	if err != nil {
		flushPreamble()
		st.logger.Warnf("kubedog: failed to template release %s for parallel tracking, falling back to post-helm tracking: %v", release.Name, err)
		return noop, false
	}
	if len(resources) == 0 {
		flushPreamble()
		st.logger.Infof("kubedog: no trackable resources templated for release %s", release.Name)
		return noop, true
	}
	flushPreamble()

	timeout := 5 * time.Minute
	if release.TrackTimeout != nil && *release.TrackTimeout > 0 {
		timeout = time.Duration(*release.TrackTimeout) * time.Second
	} else if opts != nil && opts.TrackTimeout > 0 {
		timeout = time.Duration(opts.TrackTimeout) * time.Second
	}

	trackLogs := release.TrackLogs != nil && *release.TrackLogs
	if release.TrackLogs == nil && opts != nil {
		trackLogs = opts.TrackLogs
	}

	trackFailedLogs := release.TrackFailedLogs != nil && *release.TrackFailedLogs
	if release.TrackFailedLogs == nil && opts != nil {
		trackFailedLogs = opts.TrackFailedLogs
	}

	filterConfig := &resource.FilterConfig{
		TrackKinds:     release.TrackKinds,
		SkipKinds:      release.SkipKinds,
		TrackResources: convertTrackResources(release.TrackResources),
	}

	trackOpts := kubedog.NewTrackOptions().
		WithTimeout(timeout).
		WithLogs(trackLogs).
		WithFailedLogsOnly(trackFailedLogs).
		WithFilterConfig(filterConfig).
		WithColor(useColor)

	tracker, err := kubedog.NewTracker(&kubedog.TrackerConfig{
		Logger:       st.logger,
		Namespace:    release.Namespace,
		KubeContext:  st.getKubeContext(release),
		Kubeconfig:   st.kubeconfig,
		ReleaseName:  release.Name,
		TrackOptions: trackOpts,
		KubedogQPS:   release.KubedogQPS,
		KubedogBurst: release.KubedogBurst,
	})
	if err != nil {
		st.logger.Warnf("kubedog: failed to initialize tracker for release %s, falling back to post-helm tracking: %v", release.Name, err)
		return noop, false
	}

	// Snapshot UID + generation BEFORE handing off to helm so each tracker
	// goroutine can wait until the resource actually changes. Without this,
	// kubedog observes the pre-upgrade "ready" state and exits immediately.
	trackOpts.WithBaselines(tracker.CaptureBaselines(ctx, resources))

	// Buffer helm output so it doesn't interleave with kubedog progress on
	// stdout. We swap the helm logger out for one that writes into the buffer;
	// after tracking finishes we replay the buffer through the real logger as
	// a single block.
	helmOutputBuf := &bytes.Buffer{}
	bufHelm := helm
	if swapper, ok := helm.(helmexec.LoggerSwapper); ok {
		bufHelm = swapper.WithLogger(helmexec.NewLogger(helmOutputBuf, "info"))
	} else {
		st.logger.Debugf("kubedog: helm implementation does not support logger swap; output will interleave for release %s", release.Name)
	}

	// Derive a release-scoped context for the helm subprocess so the safety
	// valve can SIGINT helm directly when the cluster confirms convergence
	// but helm is wedged on its hook waiter. Without this, the only escape
	// from a wedged helm is --track-timeout (hours). When the helmexec
	// implementation supports ContextSwapper (the real one does; mocks
	// generally don't), we plumb releaseCtx through so cancelling it kicks
	// the existing ShellRunner SIGINT path. Otherwise releaseCancel is just
	// bookkeeping with no effect on helm.
	releaseCtx, releaseCancel := context.WithCancel(ctx)
	if swapper, ok := bufHelm.(helmexec.ContextSwapper); ok {
		bufHelm = swapper.WithContext(releaseCtx)
	}

	trackCtx, trackCancel := context.WithCancel(ctx)
	resultCh := make(chan error, 1)

	st.logger.Infof("Tracking %d resources from release %s with kubedog (in parallel with helm)", len(resources), release.Name)

	go func() {
		resultCh <- tracker.TrackResources(trackCtx, resources)
	}()

	// Two related safety valves run alongside the tracker. Both verify cluster
	// state via the live API rather than trusting kubedog's resource graph.
	//
	// 1. Tracker-race safety valve (always runs): once helm signals success,
	//    wait a grace period and then poll. If the cluster confirms every
	//    tracked resource converged but the dyntracker is still wedged
	//    (kubedog race where a fast Job completion never flips ResourceStatus
	//    to Ready), cancel the tracker so wait() can return success instead
	//    of blocking until --track-timeout.
	//
	// 2. Helm-stuck killer (opt-in via helmStuckGrace): poll alongside helm.
	//    If the cluster stays converged for helmStuckGrace while helm is
	//    still running, send SIGINT to helm — that's the helm v4 hook waiter
	//    wedge (statuswait.go:195 or legacy wait.go:263), recoverable only by
	//    interrupting the helm subprocess.
	helmDoneCh := make(chan struct{})
	var helmDoneOnce sync.Once
	var safetyValveTriggered atomic.Bool
	var helmKilledByUs atomic.Bool
	const (
		safetyValveGrace = 60 * time.Second
		safetyValveCheck = 10 * time.Second
	)
	helmStuckGrace := st.getHelmStuckGrace(release, opts)

	go func() {
		select {
		case <-helmDoneCh:
		case <-trackCtx.Done():
			return
		}
		select {
		case <-time.After(safetyValveGrace):
		case <-trackCtx.Done():
			return
		}
		ticker := time.NewTicker(safetyValveCheck)
		defer ticker.Stop()
		for {
			if tracker.VerifyAllConverged(trackCtx, resources) {
				st.logger.Infof("kubedog: cluster confirms all tracked resources converged for release %s; cancelling tracker (worked around dyntracker race)", release.Name)
				safetyValveTriggered.Store(true)
				trackCancel()
				return
			}
			select {
			case <-trackCtx.Done():
				return
			case <-ticker.C:
			}
		}
	}()

	if helmStuckGrace > 0 {
		go func() {
			ticker := time.NewTicker(safetyValveCheck)
			defer ticker.Stop()
			var firstConvergedAt time.Time
			for {
				select {
				case <-trackCtx.Done():
					return
				case <-helmDoneCh:
					return // helm finished on its own; killer not needed
				case <-ticker.C:
				}
				if !tracker.VerifyAllConverged(trackCtx, resources) {
					firstConvergedAt = time.Time{}
					continue
				}
				if firstConvergedAt.IsZero() {
					firstConvergedAt = time.Now()
					continue
				}
				if time.Since(firstConvergedAt) < helmStuckGrace {
					continue
				}
				// Cluster has been converged for >= helmStuckGrace while helm
				// is still running. Treat as the helm v4 hook waiter wedge
				// and send SIGINT via the release-scoped context. Style the
				// whole block in bold+yellow so it stands out from the regular
				// info chatter in CI logs.
				block := fmt.Sprintf("%s\nCluster has confirmed convergence for %s but helm subprocess is still running.\nSending SIGINT to recover from helm v4 hook waiter wedge.\nRelease secret may need manual cleanup: kubectl -n %s delete secret sh.helm.release.v1.%s.<rev>",
					kubedog.HeaderDivider(fmt.Sprintf("WARNING: Release '%s' — helm-killer fired", release.Name)),
					time.Since(firstConvergedAt).Round(time.Second), release.Namespace, release.Name)
				st.logger.Warnf("\n%s", kubedog.StyleWarning(block, useColor))
				helmKilledByUs.Store(true)
				releaseCancel()
				return
			}
		}()
	}

	var flushOnce sync.Once
	flush := func() {
		flushOnce.Do(func() {
			payload := strings.TrimSpace(helmOutputBuf.String())
			if payload == "" {
				return
			}
			header := kubedog.HeaderDividerStyled(fmt.Sprintf("Helm output for release '%s'", release.Name), useColor)
			st.logger.Infof("\n%s\n%s", header, payload)
		})
	}

	canceled := false
	wait := func() *ReleaseError {
		trackErr := <-resultCh
		trackCancel()
		releaseCancel()
		flush()
		if canceled {
			// Helm failed and we cancelled the tracker; treat as no-op.
			return nil
		}
		if safetyValveTriggered.Load() || helmKilledByUs.Load() {
			// We deliberately cancelled the tracker after verifying via the
			// live API that everything was healthy. The resulting trackErr
			// (typically context.Canceled) is not a real failure.
			return nil
		}
		if trackErr != nil {
			st.logger.Warnf("kubedog tracking failed for release %s: %v", release.Name, trackErr)
			if st.shouldFailOnTrackError(release, opts) {
				return newReleaseFailedError(release, trackErr)
			}
		}
		return nil
	}
	cancel := func() {
		canceled = true
		trackCancel()
		releaseCancel()
	}
	notifyHelmDone := func() {
		tracker.MarkUpstreamCompleted()
		helmDoneOnce.Do(func() { close(helmDoneCh) })
	}
	wasHelmKilled := func() bool {
		return helmKilledByUs.Load()
	}

	return &kubedogTrackingHandle{
		Helm:                    bufHelm,
		Wait:                    wait,
		Cancel:                  cancel,
		NotifyHelmDone:          notifyHelmDone,
		FlushBufferedHelmOutput: flush,
		WasHelmKilled:           wasHelmKilled,
	}, true
}

// getHelmStuckGrace returns the configured helm-stuck grace period — how long
// the cluster must be confirmed converged via the live API while helm is
// still running before we send SIGINT to helm. Zero disables the killer.
// Lookup order: per-release HelmStuckGrace, SyncOpts.HelmStuckGrace,
// HelmDefaults.HelmStuckGrace.
func (st *HelmState) getHelmStuckGrace(release *ReleaseSpec, ops *SyncOpts) time.Duration {
	if release.HelmStuckGrace != nil && *release.HelmStuckGrace > 0 {
		return time.Duration(*release.HelmStuckGrace) * time.Second
	}
	if ops != nil && ops.HelmStuckGrace > 0 {
		return time.Duration(ops.HelmStuckGrace) * time.Second
	}
	if st.HelmDefaults.HelmStuckGrace > 0 {
		return time.Duration(st.HelmDefaults.HelmStuckGrace) * time.Second
	}
	return 0
}

func (st *HelmState) getTrackMode(release *ReleaseSpec, ops *SyncOpts) string {
	trackMode := release.TrackMode
	if trackMode == "" && ops != nil && ops.TrackMode != "" {
		trackMode = ops.TrackMode
	}
	if trackMode == "" {
		trackMode = st.HelmDefaults.TrackMode
	}
	if trackMode == "" {
		trackMode = string(kubedog.TrackModeHelm)
	}
	return trackMode
}

func (st *HelmState) appendWaitFlags(flags []string, helm helmexec.Interface, release *ReleaseSpec, ops *SyncOpts) []string {
	shouldWait := false
	switch {
	case release.Wait != nil && *release.Wait:
		shouldWait = true
	case ops != nil && ops.Wait:
		shouldWait = true
	case release.Wait == nil && st.HelmDefaults.Wait:
		shouldWait = true
	}

	if shouldWait {
		trackMode := st.getTrackMode(release, ops)
		if trackMode == string(kubedog.TrackModeHelmLegacy) {
			if helm != nil && helm.IsHelm4() {
				flags = append(flags, "--wait=legacy")
			} else {
				if st.logger != nil {
					st.logger.Warnf("trackMode 'helm-legacy' requires Helm v4, falling back to regular --wait for release %s", release.Name)
				}
				flags = append(flags, "--wait")
			}
		} else {
			flags = append(flags, "--wait")
		}
	}

	return flags
}

// append post-renderer flags to helm flags
func (st *HelmState) appendCascadeFlags(flags []string, helm helmexec.Interface, release *ReleaseSpec, cascade string) []string {
	// see https://github.com/helm/helm/releases/tag/v3.12.1
	if !helm.IsVersionAtLeast("3.12.1") {
		return flags
	}
	switch {
	// postRenderer arg comes from cmd flag.
	case release.Cascade != nil && *release.Cascade != "":
		flags = append(flags, "--cascade", *release.Cascade)
	case cascade != "":
		flags = append(flags, "--cascade", cascade)
	case st.HelmDefaults.Cascade != nil && *st.HelmDefaults.Cascade != "":
		flags = append(flags, "--cascade", *st.HelmDefaults.Cascade)
	}
	return flags
}

// append hide-notes flags to helm flags
func (st *HelmState) appendHideNotesFlags(flags []string, helm helmexec.Interface, ops *SyncOpts) []string {
	if ops == nil {
		return flags
	}
	// see https://github.com/helm/helm/releases/tag/v3.16.0
	if !helm.IsVersionAtLeast("3.16.0") {
		return flags
	}
	switch {
	case ops.HideNotes:
		flags = append(flags, "--hide-notes")
	}
	return flags
}

// append take-ownership flags to helm flags
func (st *HelmState) appendTakeOwnershipFlagsForUpgrade(flags []string, helm helmexec.Interface, release *ReleaseSpec, takeOwnership bool) []string {
	// see https://github.com/helm/helm/releases/tag/v3.17.0
	if !helm.IsVersionAtLeast("3.17.0") {
		return flags
	}
	switch {
	case release.TakeOwnership != nil && *release.TakeOwnership:
		flags = append(flags, "--take-ownership")
	case takeOwnership:
		flags = append(flags, "--take-ownership")
	case st.HelmDefaults.TakeOwnership != nil && *st.HelmDefaults.TakeOwnership:
		flags = append(flags, "--take-ownership")
	}
	return flags
}

// append show-only flags to helm flags
func (st *HelmState) appendShowOnlyFlags(flags []string, showOnly []string) []string {
	showOnlyFlags := []string{}
	if len(showOnly) != 0 {
		showOnlyFlags = showOnly
	}
	for _, arg := range showOnlyFlags {
		if arg != "" {
			flags = append(flags, "--show-only", arg)
		}
	}
	return flags
}

type Chartify struct {
	Opts                     *chartify.ChartifyOpts
	Clean                    func()
	NeedsChartifyForLocalDir bool
}

func (st *HelmState) downloadChartWithGoGetter(r *ReleaseSpec) (string, error) {
	var pathElems []string

	if r.Namespace != "" {
		pathElems = append(pathElems, r.Namespace)
	}

	if r.KubeContext != "" {
		pathElems = append(pathElems, r.KubeContext)
	}

	pathElems = append(pathElems, r.Name)

	cacheDir := filepath.Join(pathElems...)

	return st.goGetterChart(r.Chart, r.Directory, cacheDir, r.ForceGoGetter)
}

func (st *HelmState) goGetterChart(chart, dir, cacheDir string, force bool) (string, error) {
	if dir != "" && chart == "" {
		chart = dir
	}

	_, err := remote.Parse(chart)
	if err != nil {
		if force {
			return "", fmt.Errorf("Parsing url from dir failed due to error %q.\nContinuing the process assuming this is a regular Helm chart or a local dir.", err.Error())
		}
	} else {
		r := remote.NewRemote(st.logger, "", st.fs)

		fetchedDir, err := r.Fetch(chart, cacheDir)
		if err != nil {
			return "", fmt.Errorf("fetching %q: %v", chart, err)
		}

		chart = fetchedDir
	}

	return chart, nil
}

func (st *HelmState) PrepareChartify(helm helmexec.Interface, release *ReleaseSpec, chart string, workerIndex int) (*Chartify, func(), error) {
	c := &Chartify{
		Opts: &chartify.ChartifyOpts{
			WorkaroundOutputDirIssue:    true,
			EnableKustomizeAlphaPlugins: true,
			ChartVersion:                release.Version,
			Namespace:                   release.Namespace,
			ID:                          ReleaseToID(release),
		},
	}

	var filesNeedCleaning []string

	clean := func() {
		st.removeFiles(filesNeedCleaning)
	}

	var shouldRun bool

	dir := chart
	if !filepath.IsAbs(chart) {
		dir = filepath.Join(st.basePath, chart)
	}
	if stat, _ := os.Stat(dir); stat != nil && stat.IsDir() {
		if exists, err := st.fs.FileExists(filepath.Join(dir, "Chart.yaml")); err == nil && !exists {
			shouldRun = true
			c.NeedsChartifyForLocalDir = true
		}
	}

	for _, d := range release.Dependencies {
		chart := d.Chart
		normalizedChart := normalizeChart(st.basePath, chart)
		if st.fs.DirectoryExistsAt(normalizedChart) {
			var err error

			// Otherwise helm-dependency-up on the temporary chart generated by chartify ends up errors like:
			//   Error: directory /tmp/chartify945964195/myapp-57fb4495cf/test/integration/charts/httpbin not found]
			// which is due to that the temporary chart is generated outside of the current working directory/basePath,
			// and therefore the relative path in `chart` points to somewhere inexistent.
			chart, err = filepath.Abs(filepath.Join(st.basePath, chart))
			if err != nil {
				return nil, clean, err
			}
		} else if rewritten, ok := st.resolveOCIAdhocDepChart(d.Chart); ok {
			st.logger.Debugf("ad-hoc dependency %q rewritten to %q (matched OCI repo entry)", d.Chart, rewritten)
			chart = rewritten
		}

		c.Opts.AdhocChartDependencies = append(c.Opts.AdhocChartDependencies, chartify.ChartDependency{
			Alias:   d.Alias,
			Chart:   chart,
			Version: d.Version,
		})

		shouldRun = true
	}

	jsonPatches := release.JSONPatches
	if len(jsonPatches) > 0 {
		generatedFiles, err := st.generateTemporaryReleaseValuesFiles(release, jsonPatches)
		if err != nil {
			return nil, clean, err
		}

		filesNeedCleaning = append(filesNeedCleaning, generatedFiles...)

		c.Opts.JsonPatches = append(c.Opts.JsonPatches, generatedFiles...)

		shouldRun = true
	}

	strategicMergePatches := release.StrategicMergePatches
	if len(strategicMergePatches) > 0 {
		generatedFiles, err := st.generateTemporaryReleaseValuesFiles(release, strategicMergePatches)
		if err != nil {
			return nil, clean, err
		}

		c.Opts.StrategicMergePatches = append(c.Opts.StrategicMergePatches, generatedFiles...)

		filesNeedCleaning = append(filesNeedCleaning, generatedFiles...)

		shouldRun = true
	}

	transformers := release.Transformers
	if len(transformers) > 0 {
		generatedFiles, err := st.generateTemporaryReleaseValuesFiles(release, transformers)
		if err != nil {
			return nil, clean, err
		}

		c.Opts.Transformers = append(c.Opts.Transformers, generatedFiles...)

		filesNeedCleaning = append(filesNeedCleaning, generatedFiles...)

		shouldRun = true
	}

	if release.ForceNamespace != "" {
		c.Opts.OverrideNamespace = release.ForceNamespace

		shouldRun = true
	}

	if shouldRun {
		st.logger.Debugf("Chartify process for %s", dir)
		generatedFiles, err := st.generateValuesFiles(helm, release, workerIndex)
		if err != nil {
			return nil, clean, err
		}

		filesNeedCleaning = append(filesNeedCleaning, generatedFiles...)

		c.Opts.ValuesFiles = generatedFiles
		setFlags, err := st.setFlags(release.SetValues)
		if err != nil {
			return nil, clean, fmt.Errorf("rendering set value entry for release %s: %v", release.Name, err)
		}
		c.Opts.SetFlags = setFlags
		c.Opts.TemplateData = st.newReleaseTemplateData(release)
		c.Opts.TemplateFuncs = st.newReleaseTemplateFuncMap(dir)

		return c, clean, nil
	}

	return nil, clean, nil
}

func (st *HelmState) trackWithKubedog(ctx context.Context, release *ReleaseSpec, helm helmexec.Interface, ops *SyncOpts) error {
	timeout := 5 * time.Minute
	if release.TrackTimeout != nil && *release.TrackTimeout > 0 {
		timeout = time.Duration(*release.TrackTimeout) * time.Second
	} else if ops != nil && ops.TrackTimeout > 0 {
		timeout = time.Duration(ops.TrackTimeout) * time.Second
	}

	trackLogs := release.TrackLogs != nil && *release.TrackLogs
	if release.TrackLogs == nil && ops != nil {
		trackLogs = ops.TrackLogs
	}

	trackFailedLogs := release.TrackFailedLogs != nil && *release.TrackFailedLogs
	if release.TrackFailedLogs == nil && ops != nil {
		trackFailedLogs = ops.TrackFailedLogs
	}

	filterConfig := &resource.FilterConfig{
		TrackKinds:     release.TrackKinds,
		SkipKinds:      release.SkipKinds,
		TrackResources: convertTrackResources(release.TrackResources),
	}

	kubeContext := st.getKubeContext(release)

	trackOpts := kubedog.NewTrackOptions().
		WithTimeout(timeout).
		WithLogs(trackLogs).
		WithFailedLogsOnly(trackFailedLogs).
		WithFilterConfig(filterConfig)

	tracker, err := kubedog.NewTracker(&kubedog.TrackerConfig{
		Logger:       st.logger,
		Namespace:    release.Namespace,
		KubeContext:  kubeContext,
		Kubeconfig:   st.kubeconfig,
		ReleaseName:  release.Name,
		TrackOptions: trackOpts,
		KubedogQPS:   release.KubedogQPS,
		KubedogBurst: release.KubedogBurst,
	})
	if err != nil {
		return fmt.Errorf("failed to create kubedog tracker: %w", err)
	}

	resources, err := st.getReleaseResources(ctx, release, helm, nil)
	if err != nil {
		return fmt.Errorf("failed to get release resources: %w", err)
	}

	if len(resources) == 0 {
		st.logger.Infof("No trackable resources found for release %s", release.Name)
		return nil
	}

	st.logger.Infof("Tracking %d resources from release %s with kubedog", len(resources), release.Name)

	if err := tracker.TrackResources(ctx, resources); err != nil {
		return fmt.Errorf("kubedog tracking failed for release %s: %w", release.Name, err)
	}

	return nil
}

// getReleaseResources templates a release and returns its parsed resources.
// outLogger is the logger to use for the user-visible "Found N resources"
// (and No-manifest / No-resources) messages — pass a buffered logger when
// the caller wants to flush all per-release preamble output as one atomic
// block, otherwise pass nil to use st.logger directly.
func (st *HelmState) getReleaseResources(_ context.Context, release *ReleaseSpec, helm helmexec.Interface, outLogger *zap.SugaredLogger) ([]*resource.Resource, error) {
	if outLogger == nil {
		outLogger = st.logger
	}
	st.logger.Debugf("Getting resources for release %s", release.Name)

	manifest, namespace, err := st.getReleaseManifest(release, helm)
	if err != nil {
		return nil, fmt.Errorf("failed to get release manifest: %w", err)
	}

	if len(manifest) == 0 {
		outLogger.Infof("No manifest found for release %s", release.Name)
		return nil, nil
	}

	defaultNs := namespace
	if defaultNs == "" {
		defaultNs = "default"
	}

	resources, err := resource.ParseManifest(manifest, defaultNs, st.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to parse release resources from manifest: %w", err)
	}

	if len(resources) == 0 {
		outLogger.Infof("No resources found in manifest for release %s", release.Name)
		return nil, nil
	}

	outLogger.Infof("Found %d resources in manifest for release %s", len(resources), release.Name)

	result := make([]*resource.Resource, len(resources))
	for i := range resources {
		result[i] = &resources[i]
	}

	return result, nil
}

func (st *HelmState) getReleaseManifest(release *ReleaseSpec, helm helmexec.Interface) ([]byte, string, error) {
	var tempDir string
	var err error

	if st.tempDir != nil {
		tempDir, err = st.tempDir("", "helmfile-template-")
	} else {
		tempDir, err = os.MkdirTemp("", "helmfile-template-")
	}

	if err != nil {
		return nil, "", fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			st.logger.Warnf("Failed to remove temp directory %s: %v", tempDir, err)
		}
	}()

	releaseCopy := *release
	st.ApplyOverrides(&releaseCopy)

	flags, files, err := st.flagsForTemplate(helm, &releaseCopy, 0, &TemplateOpts{})
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate template flags: %w", err)
	}
	defer st.removeFiles(files)

	flags = append(flags, "--output-dir", tempDir)

	if err := helm.TemplateRelease(releaseCopy.Name, releaseCopy.ChartPathOrName(), flags...); err != nil {
		return nil, "", fmt.Errorf("failed to run helm template: %w", err)
	}

	var manifest []byte

	err = filepath.Walk(tempDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		if !strings.HasSuffix(info.Name(), ".yaml") && !strings.HasSuffix(info.Name(), ".yml") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read file %s: %w", path, err)
		}

		if len(manifest) > 0 {
			manifest = append(manifest, []byte("\n---\n")...)
		}
		manifest = append(manifest, content...)

		return nil
	})

	if err != nil {
		return nil, "", fmt.Errorf("failed to walk template output directory: %w", err)
	}

	return manifest, releaseCopy.Namespace, nil
}

func convertTrackResources(resources []TrackResourceSpec) []resource.Resource {
	if len(resources) == 0 {
		return nil
	}
	result := make([]resource.Resource, len(resources))
	for i, r := range resources {
		result[i] = resource.Resource{
			Kind:      r.Kind,
			Name:      r.Name,
			Namespace: r.Namespace,
		}
	}
	return result
}
