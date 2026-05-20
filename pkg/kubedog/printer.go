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
	progressInterval = 10 * time.Second
	logsInterval     = 10 * time.Second
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
	lastStatus     map[string]string
	lastCounts     map[string]int
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
		lastStatus:     make(map[string]string),
		lastCounts:     make(map[string]int),
	}
}

func (p *progressPrinter) run(ctx context.Context, done <-chan struct{}) {
	progressTicker := time.NewTicker(progressInterval)
	defer progressTicker.Stop()
	logsTicker := time.NewTicker(logsInterval)
	defer logsTicker.Stop()

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
func describeChildStatus(kdStatus, statusAttr string) string {
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

func (p *progressPrinter) flushProgress() {
	type row struct {
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
	var rows []row

	gateSnapshot := map[string]string{}
	if p.gates != nil {
		gateSnapshot = p.gates.snapshot()
	}

	p.taskStore.RTransaction(func(s *statestore.TaskStore) {
		for _, taskC := range s.ReadinessTasksStates() {
			taskC.RTransaction(func(ts *statestore.ReadinessTaskState) {
				kind := ts.GroupVersionKind().Kind
				name := ts.Name()
				ns := ts.Namespace()
				rootLabel := fmt.Sprintf("%s/%s/%s", kind, ns, name)
				rootID := kdutil.ResourceID(name, ns, ts.GroupVersionKind())
				gateKey := BaselineKey(shortKind(kind), ns, name)

				if p.skipped != nil && p.skipped.has(rootID) {
					return
				}

				if gateMsg, gated := gateSnapshot[gateKey]; gated {
					rows = append(rows, row{
						sortKey: rootLabel,
						label:   rootLabel,
						status:  gateMsg,
					})
					return
				}

				type childRow struct {
					label    string
					status   string
					ready    bool
					hasAttr  bool // has a populated AttributeNameStatus
				}
				var children []childRow
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
						label := fmt.Sprintf("%s/%s/%s", rs.GroupVersionKind().Kind, rs.Namespace(), rs.Name())
						var podStatusAttr string
						for _, attr := range rs.Attributes() {
							if attr.Name() == statestore.AttributeNameStatus {
								if a, ok := attr.(*statestore.Attribute[string]); ok {
									podStatusAttr = a.Value
								}
							}
						}
						children = append(children, childRow{
							label:   label,
							status:  describeChildStatus(string(rs.Status()), podStatusAttr),
							ready:   rs.Status() == statestore.ResourceStatusReady,
							hasAttr: podStatusAttr != "",
						})
					})
				}

				// Filter out stale pods — Ready pods that no longer carry a
				// status attribute are leftovers from a previous ReplicaSet
				// that kubedog never updated past Ready. They survive in the
				// resource graph but don't reflect the current spec.
				if required > 0 {
					filtered := children[:0]
					for _, c := range children {
						if c.ready && !c.hasAttr {
							continue
						}
						filtered = append(filtered, c)
					}
					children = filtered
				}

				readyChildren := 0
				for _, c := range children {
					if c.ready {
						readyChildren++
					}
				}
				// Cap the displayed ready count at required so "ready (2/1)"
				// can't happen even if a stale pod survived the filter above.
				if required > 0 && readyChildren > required {
					readyChildren = required
				}

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

				rows = append(rows, row{
					sortKey: rootLabel,
					label:   rootLabel,
					status:  rootStatus,
				})
				sort.Slice(children, func(i, j int) bool { return children[i].label < children[j].label })
				for _, c := range children {
					rows = append(rows, row{
						sortKey:    rootLabel + "/" + c.label,
						label:      c.label,
						status:     c.status,
						indent:     "  • ",
						parentKind: kind,
					})
				}
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
		sb.WriteString(fmt.Sprintf("\n  %s%s%s%s",
			r.indent,
			r.label,
			padding,
			p.colorize(r.status, p.statusColor(r.status, r.parentKind)),
		))
	}
	p.logger.Info(sb.String())
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

	type entry struct {
		resourceKey string
		source      string
		line        string
		ts          time.Time
	}
	var pending []entry

	p.logStore.RTransaction(func(s *logstore.LogStore) {
		for _, rlC := range s.ResourcesLogs() {
			rlC.RTransaction(func(rl *logstore.ResourceLogs) {
				resourceKey := fmt.Sprintf("%s/%s/%s", rl.GroupVersionKind().Kind, rl.Namespace(), rl.Name())
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
							resourceKey: resourceKey,
							source:      source,
							line:        ll.Line,
							ts:          ll.Time,
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
			sb.WriteString(HeaderDividerStyled(fmt.Sprintf("Logs %s %s", e.resourceKey, e.source), p.useColor))
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
	}
	if lastEmitted != "" {
		p.lastLogSource = lastEmitted
	}
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
