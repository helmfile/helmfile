package kubedog

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/werf/kubedog/pkg/trackers/dyntracker/logstore"
	"github.com/werf/kubedog/pkg/trackers/dyntracker/statestore"
	kdutil "github.com/werf/kubedog/pkg/trackers/dyntracker/util"
	"go.uber.org/zap"
)

const (
	progressInterval  = 10 * time.Second
	logsInterval      = 10 * time.Second
	heartbeatInterval = 2 * time.Minute
)

// ANSI escape codes. Hard-coded to keep the dependency list small — these are
// the standard 16-color SGR sequences and work on any reasonable terminal.
const (
	ansiReset  = "\x1b[0m"
	ansiGreen  = "\x1b[32m"
	ansiYellow = "\x1b[33m"
	ansiRed    = "\x1b[31m"
	ansiCyan   = "\x1b[36m"
	ansiBlue   = "\x1b[34m"
	ansiGray   = "\x1b[90m"
	ansiBold   = "\x1b[1m"
)

// HeaderDivider returns a section header decorated so it stays visually
// prominent even in CI log viewers that collapse leading newlines (e.g.
// GitLab CI). The "==" borders survive newline-stripping because they live
// on the same line as the title.
func HeaderDivider(title string) string {
	return "========== " + title + " =========="
}

// HeaderDividerStyled is HeaderDivider with the bold+blue style nelm uses for
// its section titles. When useColor is false it returns the plain divider so
// non-TTY/CI-without-color callers see identical bytes.
func HeaderDividerStyled(title string, useColor bool) string {
	h := HeaderDivider(title)
	if !useColor {
		return h
	}
	return ansiBold + ansiBlue + h + ansiReset
}

// StyleWarning wraps text in bold+yellow so it stands out among the regular
// info lines in CI logs without screaming "everything is on fire" — that's
// what red is for. Use this for messages an operator should notice and act
// on (e.g. orphan resource cleanup) but that don't represent a release-level
// failure. No-op when useColor is false so non-TTY consumers see clean text.
func StyleWarning(s string, useColor bool) string {
	if !useColor {
		return s
	}
	return ansiBold + ansiYellow + s + ansiReset
}

// gateStatuses holds per-resource "waiting for freshness" messages keyed by
// BaselineKey. Producers (tracker goroutines) call set/clear; the printer
// reads via snapshot.
type gateStatuses struct {
	mu sync.RWMutex
	m  map[string]string
}

func newGateStatuses() *gateStatuses {
	return &gateStatuses{m: make(map[string]string)}
}

func (g *gateStatuses) set(key, status string) {
	g.mu.Lock()
	g.m[key] = status
	g.mu.Unlock()
}

func (g *gateStatuses) clear(key string) {
	g.mu.Lock()
	delete(g.m, key)
	g.mu.Unlock()
}

func (g *gateStatuses) snapshot() map[string]string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	out := make(map[string]string, len(g.m))
	for k, v := range g.m {
		out[k] = v
	}
	return out
}

// skippedKeys records the ResourceIDs of tasks the printer should hide
// (typically because the upstream operation finished without changing the
// resource and the task's "progressing" state would be misleading).
type skippedKeys struct {
	mu sync.RWMutex
	m  map[string]struct{}
}

func newSkippedKeys() *skippedKeys {
	return &skippedKeys{m: make(map[string]struct{})}
}

func (s *skippedKeys) add(id string) {
	s.mu.Lock()
	s.m[id] = struct{}{}
	s.mu.Unlock()
}

func (s *skippedKeys) has(id string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.m[id]
	return ok
}

// kubedog's util.ResourceID returns "<ns>:<group>:<kind>:<name>".

// formatResourceLabel renders a resource identifier, omitting the namespace
// when it matches the printer-wide common namespace. Cluster-scoped
// resources (empty namespace) always render as Kind/name. The goal is to
// stop repeating the same namespace on every row of a single-namespace
// release without losing the namespace when releases genuinely span
// multiple namespaces.
func formatResourceLabel(kind, ns, name, commonNS string) string {
	if ns == "" || (commonNS != "" && ns == commonNS) {
		return fmt.Sprintf("%s/%s", kind, name)
	}
	return fmt.Sprintf("%s/%s/%s", kind, ns, name)
}

type progressPrinter struct {
	logger         *zap.SugaredLogger
	releaseName    string
	taskStore      *kdutil.Concurrent[*statestore.TaskStore]
	logStore       *kdutil.Concurrent[*logstore.LogStore]
	skipLogs       bool
	failedLogsOnly bool
	gates          *gateStatuses
	skipped        *skippedKeys
	useColor       bool
	startTime      time.Time
	// lastEmit tracks the most recent time we printed *anything* to the
	// logger (progress block or log stream). The heartbeat ticker uses it to
	// suppress its own output while the printer is actively chatty; it only
	// emits when there's been a silent gap of at least heartbeatInterval.
	lastEmit   time.Time
	lastStatus map[string]string
	lastCounts map[string]int
	// lastLogSource is the "<resourceKey>|<source>" emitted at the tail of
	// the previous flushLogs call. Carrying it across flushes lets us drop
	// the redundant "logs <pod> <container>" header when consecutive flushes
	// continue from the same source.
	lastLogSource string
}

func newProgressPrinter(
	logger *zap.SugaredLogger,
	releaseName string,
	taskStore *kdutil.Concurrent[*statestore.TaskStore],
	logStore *kdutil.Concurrent[*logstore.LogStore],
	skipLogs bool,
	failedLogsOnly bool,
	gates *gateStatuses,
	skipped *skippedKeys,
	useColor bool,
) *progressPrinter {
	return &progressPrinter{
		logger:         logger,
		releaseName:    releaseName,
		taskStore:      taskStore,
		logStore:       logStore,
		skipLogs:       skipLogs,
		failedLogsOnly: failedLogsOnly,
		gates:          gates,
		skipped:        skipped,
		useColor:       useColor,
		startTime:      time.Now(),
		lastEmit:       time.Now(),
		lastStatus:     make(map[string]string),
		lastCounts:     make(map[string]int),
	}
}

func (p *progressPrinter) run(ctx context.Context, done <-chan struct{}) {
	progressTicker := time.NewTicker(progressInterval)
	defer progressTicker.Stop()
	logsTicker := time.NewTicker(logsInterval)
	defer logsTicker.Stop()
	heartbeatTicker := time.NewTicker(heartbeatInterval)
	defer heartbeatTicker.Stop()

	p.flushProgress()

	for {
		select {
		case <-progressTicker.C:
			p.flushProgress()
			if !p.skipLogs {
				p.flushLogs()
			}
		case <-logsTicker.C:
			if !p.skipLogs {
				p.flushLogs()
			}
		case <-heartbeatTicker.C:
			p.flushHeartbeat()
		case <-done:
			p.flushProgress()
			if !p.skipLogs {
				p.flushLogs()
			}
			return
		case <-ctx.Done():
			return
		}
	}
}

// commonNamespace returns the namespace shared by every tracked task, or
// the empty string when tasks span multiple namespaces or there are no
// tasks. When non-empty, callers strip the namespace from each row's label
// and add it once to the block title via the formatResourceLabel helper.
func (p *progressPrinter) commonNamespace() string {
	var ns string
	multiple := false
	p.taskStore.RTransaction(func(s *statestore.TaskStore) {
		for _, taskC := range s.ReadinessTasksStates() {
			if multiple {
				return
			}
			taskC.RTransaction(func(ts *statestore.ReadinessTaskState) {
				taskNS := ts.Namespace()
				if taskNS == "" {
					return
				}
				if ns == "" {
					ns = taskNS
				} else if ns != taskNS {
					multiple = true
				}
			})
		}
	})
	if multiple {
		return ""
	}
	return ns
}

func (p *progressPrinter) colorize(s, code string) string {
	if !p.useColor || code == "" {
		return s
	}
	return code + s + ansiReset
}

// statusColor maps a status line to an ANSI color code. Failing pod-phase
// substrings ("Error", "CrashLoopBackOff", …) take priority — a Job whose
// pod failed but whose overall task is marked Ready (because a sibling pod
// completed) would otherwise render the failed pod's row in green. After
// that we check the leading kubedog state, then fall back to other known
// pod phases.
//
// parentKind is the kind of the workload owning this row (empty for the
// root row of a task). It only affects how "Running" is colored: under a
// Job parent, Running means "still executing toward Completed" so it's
// yellow; everywhere else (Deployment/StatefulSet/DaemonSet) Running is
// the steady state, so green.
func (p *progressPrinter) statusColor(status, parentKind string) string {
	if containsAny(status, "Error", "CrashLoopBackOff", "ImagePullBackOff", "ErrImagePull", "Failed", "OOMKilled",
		"CreateContainerConfigError", "CreateContainerError", "InvalidImageName") {
		return ansiRed
	}

	head := status
	if idx := strings.Index(status, " "); idx >= 0 {
		head = status[:idx]
	}
	switch head {
	case "ready":
		return ansiGreen
	case "progressing":
		return ansiYellow
	case "failed":
		return ansiRed
	case "waiting":
		return ansiCyan
	case "created":
		return ansiCyan
	case "deleted":
		return ansiGray
	case "unknown":
		return ansiGray
	}

	// Job pods in Running state haven't completed yet — treat as in-progress.
	if parentKind == "Job" && containsAny(status, "Running") && !containsAny(status, "Completed") {
		return ansiYellow
	}

	switch {
	case containsAny(status, "Completed"):
		return ansiGreen
	case containsAny(status, "Running"):
		return ansiGreen
	case containsAny(status, "ContainerCreating", "PodInitializing", "Pending", "Init:", "ContainerStarting"):
		return ansiYellow
	case containsAny(status, "Terminating"):
		return ansiGray
	}
	return ""
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// describeChildStatus collapses kubedog's bookkeeping status with the
// human-readable status attribute. When the bookkeeping value is "unknown"
// (which it usually is for transient pod phases) the attribute alone is
// shown; otherwise both are joined.
//
// If the attribute is a pre-ready pod phase (Pending, ContainerCreating,
// PodInitializing, Init:*) the attribute alone is shown even when kubedog
// reports the pod as ready — the two states are contradictory and the pod
// phase is what kubectl would show, so we treat it as authoritative for
// display purposes.
func describeChildStatus(kdStatus, statusAttr string) string {
	if isPreReadyPodPhase(statusAttr) {
		return statusAttr
	}
	switch {
	case statusAttr == "" && kdStatus == "":
		return "unknown"
	case statusAttr == "":
		return kdStatus
	case kdStatus == "" || kdStatus == "unknown":
		return statusAttr
	default:
		return fmt.Sprintf("%s (%s)", kdStatus, statusAttr)
	}
}

// isPreReadyPodPhase returns true for kubelet-reported phases that indicate
// the pod cannot yet be ready — startup transitions before the Ready
// condition has had a chance to flip. Used to override kubedog's
// ResourceStatus when the two disagree, so a pod reporting ContainerCreating
// in the attribute is never displayed or counted as ready.
func isPreReadyPodPhase(phase string) bool {
	switch phase {
	case "Pending", "ContainerCreating", "PodInitializing", "ContainerStarting":
		return true
	}
	// Init:0/3, Init:1/3, etc. — pod is still running init containers.
	if strings.HasPrefix(phase, "Init:") {
		return true
	}
	return false
}

type progressRow struct {
	sortKey string
	label   string
	status  string
	indent  string
	// parentKind is the Kind of the workload the row belongs to. Set
	// only for child rows; empty for root rows. Used to bias the status
	// color so a Running Job pod renders yellow (still in flight) while
	// a Running Deployment pod renders green (steady state).
	parentKind string
}

// podChildRow is a child resource (typically a pod) summarized for display in
// the progress block.
type podChildRow struct {
	label   string
	status  string
	ready   bool
	hasAttr bool // has a populated AttributeNameStatus
}

// filterStaleReadyPods drops Ready pods that no longer carry a status
// attribute. They are leftovers from a previous ReplicaSet that kubedog never
// updated past Ready: they survive in the resource graph but don't reflect the
// current spec. Only applied when a required-replica count is known.
func filterStaleReadyPods(children []podChildRow, required int) []podChildRow {
	if required <= 0 {
		return children
	}
	filtered := children[:0]
	for _, c := range children {
		if c.ready && !c.hasAttr {
			continue
		}
		filtered = append(filtered, c)
	}
	return filtered
}

// countReady counts Ready children, capped at required so a stray stale pod
// can never produce a "ready (2/1)" display.
func countReady(children []podChildRow, required int) int {
	readyChildren := 0
	for _, c := range children {
		if c.ready {
			readyChildren++
		}
	}
	if required > 0 && readyChildren > required {
		readyChildren = required
	}
	return readyChildren
}

// rowsForTask builds the progress rows for a single tracked resource and its
// children. Extracted from flushProgress to keep that method's cognitive
// complexity manageable.
func (p *progressPrinter) rowsForTask(ts *statestore.ReadinessTaskState, gateSnapshot map[string]string, commonNS string) []progressRow {
	kind := ts.GroupVersionKind().Kind
	name := ts.Name()
	ns := ts.Namespace()
	rootLabel := formatResourceLabel(kind, ns, name, commonNS)
	// sortKey uses the full path so rows still sort deterministically
	// regardless of how the displayed label was abbreviated.
	rootSortKey := fmt.Sprintf("%s/%s/%s", kind, ns, name)
	rootID := kdutil.ResourceID(name, ns, ts.GroupVersionKind())
	gateKey := BaselineKey(shortKind(kind), ns, name)

	if p.skipped != nil && p.skipped.has(rootID) {
		return nil
	}

	if gateMsg, gated := gateSnapshot[gateKey]; gated {
		return []progressRow{{
			sortKey: rootSortKey,
			label:   rootLabel,
			status:  gateMsg,
		}}
	}

	var children []podChildRow
	var required int
	var rootStatusAttr string
	var rootHasFailed bool

	for _, rsC := range ts.ResourceStates() {
		rsC.RTransaction(func(rs *statestore.ResourceState) {
			if rs.ID() == rootID {
				for _, attr := range rs.Attributes() {
					switch attr.Name() {
					case statestore.AttributeNameRequiredReplicas:
						if a, ok := attr.(*statestore.Attribute[int]); ok {
							required = a.Value
						}
					case statestore.AttributeNameStatus:
						if a, ok := attr.(*statestore.Attribute[string]); ok {
							rootStatusAttr = a.Value
						}
					}
				}
				if rs.Status() == statestore.ResourceStatusFailed {
					rootHasFailed = true
				}
				return
			}
			// Skip placeholder children that dyntracker inserts when
			// it has observed a reference to a resource but hasn't
			// yet ingested the full object — they have no name and
			// no status, so they can't tell the operator anything.
			if rs.Name() == "" {
				return
			}
			label := formatResourceLabel(rs.GroupVersionKind().Kind, rs.Namespace(), rs.Name(), commonNS)
			var podStatusAttr string
			for _, attr := range rs.Attributes() {
				if attr.Name() == statestore.AttributeNameStatus {
					if a, ok := attr.(*statestore.Attribute[string]); ok {
						podStatusAttr = a.Value
					}
				}
			}
			// A pre-ready phase in the attribute overrides kubedog's
			// ResourceStatus for counting purposes — without this, a
			// pod whose attribute is ContainerCreating but whose
			// ResourceStatus has been flipped to Ready by some other
			// code path would inflate the (N/M) count even though
			// the display correctly shows it as ContainerCreating.
			ready := rs.Status() == statestore.ResourceStatusReady && !isPreReadyPodPhase(podStatusAttr)
			children = append(children, podChildRow{
				label:   label,
				status:  describeChildStatus(string(rs.Status()), podStatusAttr),
				ready:   ready,
				hasAttr: podStatusAttr != "",
			})
		})
	}

	children = filterStaleReadyPods(children, required)
	readyChildren := countReady(children, required)

	var rootStatus string
	switch {
	case rootHasFailed:
		rootStatus = "failed"
	case required > 0:
		if readyChildren >= required {
			rootStatus = fmt.Sprintf("ready (%d/%d)", readyChildren, required)
		} else {
			rootStatus = fmt.Sprintf("progressing (%d/%d)", readyChildren, required)
		}
	default:
		rootStatus = string(ts.Status())
	}
	if rootStatusAttr != "" {
		rootStatus = fmt.Sprintf("%s %s", rootStatus, rootStatusAttr)
	}

	out := make([]progressRow, 0, 1+len(children))
	out = append(out, progressRow{
		sortKey: rootSortKey,
		label:   rootLabel,
		status:  rootStatus,
	})
	sort.Slice(children, func(i, j int) bool { return children[i].label < children[j].label })
	for _, c := range children {
		out = append(out, progressRow{
			sortKey:    rootSortKey + "/" + c.label,
			label:      c.label,
			status:     c.status,
			indent:     "  • ",
			parentKind: kind,
		})
	}
	return out
}

func (p *progressPrinter) flushProgress() {
	gateSnapshot := map[string]string{}
	if p.gates != nil {
		gateSnapshot = p.gates.snapshot()
	}
	commonNS := p.commonNamespace()

	var rows []progressRow
	p.taskStore.RTransaction(func(s *statestore.TaskStore) {
		for _, taskC := range s.ReadinessTasksStates() {
			taskC.RTransaction(func(ts *statestore.ReadinessTaskState) {
				rows = append(rows, p.rowsForTask(ts, gateSnapshot, commonNS)...)
			})
		}
	})

	sort.Slice(rows, func(i, j int) bool { return rows[i].sortKey < rows[j].sortKey })

	current := make(map[string]string, len(rows))
	for _, r := range rows {
		current[r.indent+r.label] = r.status
	}
	changed := len(current) != len(p.lastStatus)
	if !changed {
		for k, v := range current {
			if p.lastStatus[k] != v {
				changed = true
				break
			}
		}
	}
	if !changed {
		return
	}
	p.lastStatus = current

	if len(rows) == 0 {
		return
	}

	// Auto-size label column to the widest row so long pod names don't
	// run into the status text. We use rune count (not byte length) because
	// the child indent contains "•" (3 bytes, 1 visual column) — counting
	// bytes would over-estimate child row widths and under-pad parent rows
	// by the difference.
	visibleWidth := func(s string) int { return utf8.RuneCountInString(s) }
	maxLabelWidth := 0
	for _, r := range rows {
		w := visibleWidth(r.indent) + visibleWidth(r.label)
		if w > maxLabelWidth {
			maxLabelWidth = w
		}
	}
	const statusGap = 2

	elapsed := time.Since(p.startTime).Round(time.Second)
	title := fmt.Sprintf("kubedog progress (%s)", elapsed)
	if p.releaseName != "" {
		title = fmt.Sprintf("Release '%s' progress (%s)", p.releaseName, elapsed)
	}
	if commonNS != "" {
		title = fmt.Sprintf("%s in '%s'", title, commonNS)
	}

	var sb strings.Builder
	// Leading newline visually separates each progress block from helm output
	// and from any log lines that came right before. It may be collapsed in
	// some CI log viewers — the "==" decoration in the header survives.
	sb.WriteString("\n")
	sb.WriteString(HeaderDividerStyled(title, p.useColor))
	for _, r := range rows {
		labelVisual := visibleWidth(r.label)
		// Target column for the status = maxLabelWidth + statusGap +
		// indentVisual. After subtracting the row's actual prefix width
		// (indentVisual + labelVisual), indentVisual cancels and we're left
		// with padding = maxLabelWidth - labelVisual + statusGap. This nests
		// child statuses further right than their parent's by exactly the
		// child indent width.
		pad := maxLabelWidth - labelVisual + statusGap
		padding := strings.Repeat(" ", pad)
		fmt.Fprintf(&sb, "\n  %s%s%s%s",
			r.indent,
			r.label,
			padding,
			p.colorize(r.status, p.statusColor(r.status, r.parentKind)),
		)
	}
	p.logger.Info(sb.String())
	p.lastEmit = time.Now()
	// A progress block visually breaks any in-flight log stream, so the next
	// flushLogs call should re-emit the "logs <pod> <container>:" header even
	// if it continues from the same source.
	p.lastLogSource = ""
}

func (p *progressPrinter) flushLogs() {
	// failedPodIDs holds the kdutil.ResourceID of every pod currently in a
	// failed state. Populated only in failed-only mode; otherwise we don't
	// need it.
	var failedPodIDs map[string]struct{}
	if p.failedLogsOnly {
		failedPodIDs = p.collectFailedPodIDs()
	}

	commonNS := p.commonNamespace()

	type entry struct {
		// resourceKey is the full Kind/ns/name path used for cursor
		// uniqueness across flushes. displayLabel is the ns-stripped form
		// rendered in the per-source header.
		resourceKey  string
		displayLabel string
		source       string
		line         string
		ts           time.Time
	}
	var pending []entry

	p.logStore.RTransaction(func(s *logstore.LogStore) {
		for _, rlC := range s.ResourcesLogs() {
			rlC.RTransaction(func(rl *logstore.ResourceLogs) {
				resourceKey := fmt.Sprintf("%s/%s/%s", rl.GroupVersionKind().Kind, rl.Namespace(), rl.Name())
				displayLabel := formatResourceLabel(rl.GroupVersionKind().Kind, rl.Namespace(), rl.Name(), commonNS)
				// In failed-only mode, gate emission on the pod's current
				// status. We keep the cursor frozen for non-failed pods so
				// their accumulated lines stay queued and get printed in full
				// if/when the pod transitions to failed later.
				if p.failedLogsOnly {
					podID := kdutil.ResourceID(rl.Name(), rl.Namespace(), rl.GroupVersionKind())
					if _, failed := failedPodIDs[podID]; !failed {
						return
					}
				}
				for source, lines := range rl.LogLines() {
					cursorKey := resourceKey + "|" + source
					start := p.lastCounts[cursorKey]
					if start >= len(lines) {
						continue
					}
					for _, ll := range lines[start:] {
						pending = append(pending, entry{
							resourceKey:  resourceKey,
							displayLabel: displayLabel,
							source:       source,
							line:         ll.Line,
							ts:           ll.Time,
						})
					}
					p.lastCounts[cursorKey] = len(lines)
				}
			})
		}
	})

	sort.Slice(pending, func(i, j int) bool { return pending[i].ts.Before(pending[j].ts) })

	if len(pending) == 0 {
		return
	}

	// Group consecutive lines from the same pod/container into a single
	// emission so the per-source header is only printed once and the lines
	// look like an excerpt rather than scattered noise.
	var sb strings.Builder
	prevKey := p.lastLogSource
	startedWithHeader := false
	lastEmitted := ""
	for _, e := range pending {
		line := strings.TrimRight(e.line, "\n")
		if line == "" {
			continue
		}
		key := e.resourceKey + "|" + e.source
		if key != prevKey {
			// On a source change we emit a "Logs <pod> <container>:" header
			// (with "==" decoration for CI log readability) preceded by a
			// blank line so it visually detaches from whatever came before.
			// Continuation flushes (same source) skip both the blank line AND
			// the header to avoid stacking newlines from zap.
			if sb.Len() > 0 {
				sb.WriteString("\n\n")
			} else {
				sb.WriteString("\n")
				startedWithHeader = true
			}
			sb.WriteString(HeaderDividerStyled(fmt.Sprintf("Logs %s %s", e.displayLabel, e.source), p.useColor))
			prevKey = key
		}
		// Indent each line; the first line of a continuation flush has no
		// leading newline so zap's own message-boundary newline alone
		// separates it from the previous flush.
		if startedWithHeader || sb.Len() > 0 {
			sb.WriteString("\n  ")
		} else {
			sb.WriteString("  ")
		}
		sb.WriteString(line)
		lastEmitted = key
	}
	if sb.Len() > 0 {
		p.logger.Info(sb.String())
		p.lastEmit = time.Now()
	}
	if lastEmitted != "" {
		p.lastLogSource = lastEmitted
	}
}

// flushHeartbeat emits a single-line "still tracking" digest when nothing
// else has been printed for at least heartbeatInterval. It's the only signal
// CI operators get during long silent stretches (e.g. an hour-long Job that
// never changes state and never logs) that helmfile is alive rather than
// hung. Suppressed when (a) the printer recently produced output already, or
// (b) every tracked task is Ready — in which case there's no useful content.
func (p *progressPrinter) flushHeartbeat() {
	// Suppress only if some *change-driven* output (progress block or log
	// stream) happened recently. We deliberately do NOT update p.lastEmit
	// from this function: two consecutive heartbeats during a silent stretch
	// are exactly what the operator needs. Updating lastEmit on every
	// heartbeat caused the next tick (which fires on a fixed wall-clock
	// schedule) to land microseconds short of heartbeatInterval after our
	// processing delay, so it got suppressed — observed effect was a tick
	// emitted every 4 minutes instead of 2.
	if time.Since(p.lastEmit) < heartbeatInterval {
		return
	}

	gateSnapshot := map[string]string{}
	if p.gates != nil {
		gateSnapshot = p.gates.snapshot()
	}
	commonNS := p.commonNamespace()

	var progressing, waiting []string
	p.taskStore.RTransaction(func(s *statestore.TaskStore) {
		for _, taskC := range s.ReadinessTasksStates() {
			taskC.RTransaction(func(ts *statestore.ReadinessTaskState) {
				if ts.Status() == statestore.ReadinessTaskStatusReady {
					return
				}
				if p.skipped != nil && p.skipped.has(kdutil.ResourceID(ts.Name(), ts.Namespace(), ts.GroupVersionKind())) {
					return
				}
				kind := ts.GroupVersionKind().Kind
				name := ts.Name()
				ns := ts.Namespace()
				label := formatResourceLabel(kind, ns, name, commonNS)
				if _, gated := gateSnapshot[BaselineKey(shortKind(kind), ns, name)]; gated {
					waiting = append(waiting, label)
				} else {
					progressing = append(progressing, label)
				}
			})
		}
	})

	if len(progressing) == 0 && len(waiting) == 0 {
		return
	}

	sort.Strings(progressing)
	sort.Strings(waiting)

	elapsed := time.Since(p.startTime).Round(time.Second)
	timestamp := time.Now().Format("15:04:05")
	label := "kubedog"
	if p.releaseName != "" {
		label = fmt.Sprintf("Release '%s'", p.releaseName)
	}
	if commonNS != "" {
		label = fmt.Sprintf("%s in '%s'", label, commonNS)
	}

	// Render the progressing group with its names (capped); the waiting group
	// is summarized as a count only because listing every queued resource
	// makes the line too long for one CI log row. Both groups carry their own
	// count so the split is obvious at a glance.
	var parts []string
	if len(progressing) > 0 {
		const maxShown = 3
		names := progressing
		overflow := ""
		if len(names) > maxShown {
			overflow = fmt.Sprintf(", +%d", len(names)-maxShown)
			names = names[:maxShown]
		}
		parts = append(parts, fmt.Sprintf("%d progressing (%s%s)",
			len(progressing), strings.Join(names, ", "), overflow))
	}
	if len(waiting) > 0 {
		parts = append(parts, fmt.Sprintf("%d waiting", len(waiting)))
	}

	line := fmt.Sprintf("[%s] %s still tracking (%s) — %s",
		timestamp, label, elapsed, strings.Join(parts, ", "))
	p.logger.Info(p.colorize(line, ansiCyan))
}

// collectFailedPodIDs walks the task store and returns the set of pod
// resource IDs that are in a failed-ish state — either marked as
// ResourceStatusFailed, carrying recorded errors, or whose status attribute
// matches a known pod-phase failure (CrashLoopBackOff, ImagePullBackOff,
// Error, etc.). Used by failed-only log mode to gate emission.
func (p *progressPrinter) collectFailedPodIDs() map[string]struct{} {
	out := map[string]struct{}{}
	p.taskStore.RTransaction(func(s *statestore.TaskStore) {
		for _, taskC := range s.ReadinessTasksStates() {
			taskC.RTransaction(func(ts *statestore.ReadinessTaskState) {
				for _, rsC := range ts.ResourceStates() {
					rsC.RTransaction(func(rs *statestore.ResourceState) {
						// We only filter pod logs, so only collect Pod IDs.
						if rs.GroupVersionKind().Kind != "Pod" {
							return
						}
						if rs.Status() == statestore.ResourceStatusFailed {
							out[rs.ID()] = struct{}{}
							return
						}
						if len(rs.Errors()) > 0 {
							out[rs.ID()] = struct{}{}
							return
						}
						for _, attr := range rs.Attributes() {
							if attr.Name() != statestore.AttributeNameStatus {
								continue
							}
							a, ok := attr.(*statestore.Attribute[string])
							if !ok {
								continue
							}
							if isFailingPodPhase(a.Value) {
								out[rs.ID()] = struct{}{}
							}
						}
					})
				}
			})
		}
	})
	return out
}

// isFailingPodPhase returns true for kubelet-reported pod phases that
// indicate the pod is in a broken state, even if kubedog hasn't yet flipped
// the underlying ResourceStatus to Failed.
func isFailingPodPhase(phase string) bool {
	switch phase {
	case "Error", "Failed",
		"CrashLoopBackOff",
		"ImagePullBackOff", "ErrImagePull",
		"OOMKilled",
		"CreateContainerConfigError", "CreateContainerError",
		"InvalidImageName":
		return true
	}
	return false
}

// shortKind maps full Kind names (as reported by the task store) back to the
// short identifiers used as the BaselineKey prefix.
func shortKind(kind string) string {
	switch kind {
	case "Deployment":
		return "deploy"
	case "StatefulSet":
		return "sts"
	case "DaemonSet":
		return "ds"
	case "Job":
		return "job"
	case "Canary":
		return "canary"
	case "PersistentVolumeClaim":
		return "pvc"
	}
	return strings.ToLower(kind)
}
