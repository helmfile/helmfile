package app

import (
	"path/filepath"
	"slices"

	"github.com/helmfile/helmfile/pkg/filesystem"
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
	a.rootHelmfileDirOnce.Do(func() {
		a.rootHelmfileDir = a.computeRootHelmfileDir()
	})
}

func (a *App) computeRootHelmfileDir() string {
	dir, err := DirSelectorRoot(a.fs, a.FileOrDir)
	if err != nil {
		a.Logger.Debugf("dir selector unavailable: cannot resolve root helmfile directory: %v", err)
	}
	return dir
}

// DirSelectorRoot returns the absolute directory that dir= selector values
// are relative to for the given -f value: the working directory when it is
// empty, the directory itself when it points to one, the file's directory
// otherwise. Returns "" for a remote helmfile.
func DirSelectorRoot(fs *filesystem.FileSystem, fileOrDir string) (string, error) {
	if fileOrDir == "" {
		return fs.Getwd()
	}
	if remote.IsRemote(fileOrDir) {
		return "", nil
	}
	absPath, err := fs.Abs(fileOrDir)
	if err != nil {
		return "", err
	}
	if fs.DirectoryExistsAt(absPath) {
		return absPath, nil
	}
	return filepath.Dir(absPath), nil
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

// skipForDirFilter reports whether a `helmfiles:` entry can be skipped for
// the given selectors. Remote entries and unresolvable roots always descend.
func (a *App) skipForDirFilter(stateDir, entryPath string, selectors []string) bool {
	rootDir := a.RootHelmfileDir()
	if rootDir == "" || remote.IsRemote(entryPath) {
		return false
	}
	groups := extractDirSelectorTargets(selectors)
	if len(groups) == 0 {
		return false
	}
	entryDir, ok := a.entryDirRelativeToRoot(rootDir, stateDir, entryPath)
	if !ok {
		return false
	}
	return !shouldDescendForDirFilter(entryDir, groups)
}

// entryDirRelativeToRoot returns the directory holding the helmfile(s) of a
// `helmfiles:` entry, relative to rootDir: the entry itself when it is a
// directory, its parent otherwise. Returns false when the entry lies outside
// rootDir, signaling the caller to descend conservatively.
func (a *App) entryDirRelativeToRoot(rootDir, stateDir, entryPath string) (string, bool) {
	abs := entryPath
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(stateDir, entryPath)
	}
	if !a.fs.DirectoryExistsAt(abs) {
		abs = filepath.Dir(abs)
	}
	return state.DirRelativeToRoot(rootDir, abs)
}

// shouldDescendForDirFilter returns true when a sub-helmfile located in
// entryDir (relative to the root) could contain a release matching at least
// one of the dir-selector groups. A branch is relevant for a dir=target when
// it lives at or below target, or when target lives below it (e.g. an
// aggregator at apps/helmfile.yaml for target apps/x). The decision only
// looks at the entry's own location, so it assumes nested entries stay inside
// their parent's directory.
func shouldDescendForDirFilter(entryDir string, groups []dirSelectorGroup) bool {
	if len(groups) == 0 || entryDir == "." {
		return true
	}
	for _, g := range groups {
		conflicts := slices.ContainsFunc(g.targets, func(target string) bool {
			return !state.DirsCompatible(entryDir, target)
		})
		if !conflicts {
			return true
		}
	}
	return false
}
