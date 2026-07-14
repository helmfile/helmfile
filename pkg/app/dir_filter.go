package app

import (
	"path/filepath"
	"strings"

	"github.com/helmfile/helmfile/pkg/remote"
	"github.com/helmfile/helmfile/pkg/state"
)

// RootHelmfileDir returns the absolute directory of the top-level helmfile,
// or "" when the root is a remote URL or otherwise unresolvable. Callers
// treat "" as "dir-based filtering not available". The value is cached by
// resolveRootHelmfileDir; it must be invoked before any goroutine fan-out
// or working-directory change so that the CWD-sensitive resolution lands
// against the process CWD at command start, not the (possibly chdir'd)
// directory of a leaf helmfile being processed.
func (a *App) RootHelmfileDir() string {
	return a.rootHelmfileDir
}

// resolveRootHelmfileDir computes and caches the root directory anchor for
// dir-based filtering. Idempotent; safe to call multiple times. Must be
// called from the top-level entry of each command, before within() or any
// goroutine spawn.
func (a *App) resolveRootHelmfileDir() {
	if a.rootHelmfileDirResolved {
		return
	}
	a.rootHelmfileDirResolved = true
	a.rootHelmfileDir = a.computeRootHelmfileDir()
}

func (a *App) computeRootHelmfileDir() string {
	if a.FileOrDir == "" {
		wd, err := a.fs.Getwd()
		if err != nil {
			return ""
		}
		return wd
	}
	if remote.IsRemote(a.FileOrDir) {
		return ""
	}
	absPath, err := a.fs.Abs(a.FileOrDir)
	if err != nil {
		return ""
	}
	if a.fs.DirectoryExistsAt(absPath) {
		return absPath
	}
	return filepath.Dir(absPath)
}

// dirSelectorGroup holds the positive dir= target values inside one -l
// argument. An empty targets slice means the group has no dir= constraint
// and is therefore path-permissive during traversal-skip.
type dirSelectorGroup struct {
	targets []string
}

// extractDirSelectorTargets returns one dirSelectorGroup per -l argument
// with that group's positive dir= values. Negative dir!= is intentionally
// dropped: skipping a branch on a negative constraint would require proving
// it contains only excluded releases, which generally requires loading it.
// Malformed groups are treated as path-permissive so traversal does not
// short-circuit on selector strings the filter will later report on.
// Returns nil when no positive dir= appears, so callers can skip the check.
func extractDirSelectorTargets(selectors []string) []dirSelectorGroup {
	if len(selectors) == 0 {
		return nil
	}
	groups := make([]dirSelectorGroup, 0, len(selectors))
	anyDir := false
	for _, s := range selectors {
		targets, err := state.PositiveDirTargets(s)
		if err != nil {
			groups = append(groups, dirSelectorGroup{})
			continue
		}
		groups = append(groups, dirSelectorGroup{targets: targets})
		if len(targets) > 0 {
			anyDir = true
		}
	}
	if !anyDir {
		return nil
	}
	return groups
}

// shouldDescendForDirFilter returns true when the sub-helmfile at entryPath
// could contain a release matching at least one of the dir-selector groups.
// On any path-computation failure or when entryPath escapes rootDir, returns
// true: skipping a branch we cannot reason about would silently drop work.
func shouldDescendForDirFilter(rootDir, stateDir, entryPath string, groups []dirSelectorGroup) bool {
	if len(groups) == 0 {
		return true
	}

	rel, ok := relativeEntryPath(rootDir, stateDir, entryPath)
	if !ok {
		return true
	}
	parent := filepath.ToSlash(filepath.Dir(rel))
	if parent == "." || parent == "" {
		return true
	}

	for _, g := range groups {
		if len(g.targets) == 0 {
			return true
		}
		matched := true
		for _, target := range g.targets {
			if !couldContainDirMatch(rel, parent, target) {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}

// relativeEntryPath resolves entryPath (possibly relative to stateDir, or
// already absolute) into a path relative to rootDir, in slash form. Returns
// (_, false) when the entry escapes rootDir (rel begins with "..") or when
// path computation fails, signaling the caller to descend conservatively.
func relativeEntryPath(rootDir, stateDir, entryPath string) (string, bool) {
	abs := entryPath
	if !filepath.IsAbs(abs) {
		base := stateDir
		if !filepath.IsAbs(base) {
			absBase, err := filepath.Abs(base)
			if err != nil {
				return "", false
			}
			base = absBase
		}
		abs = filepath.Join(base, entryPath)
	}
	rel, err := filepath.Rel(rootDir, abs)
	if err != nil {
		return "", false
	}
	rel = filepath.ToSlash(rel)
	if strings.HasPrefix(rel, "..") {
		return "", false
	}
	return rel, true
}

// couldContainDirMatch reports whether a release defined under the file at
// entryPath (with parent directory entryParent) could possibly match a
// dir=target selector, given target's directory-prefix semantics:
//
//   - target matches anything at or below target/, so descending into a
//     branch under target is required.
//   - the user's target may live below the current entry's parent (e.g. the
//     entry is an aggregator file at apps/helmfile.yaml and target is
//     apps/x); in that case descending may reach it.
func couldContainDirMatch(entryPath, entryParent, target string) bool {
	if entryPath == target || entryParent == target {
		return true
	}
	return strings.HasPrefix(entryPath, target+"/") ||
		strings.HasPrefix(target+"/", entryParent+"/")
}
