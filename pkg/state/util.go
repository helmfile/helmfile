package state

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	// current working directory
	currentDirSymbol = "."
	// parent directory
	parentDirSymbol = ".."
)

func isLocalChart(chart string) bool {
	if strings.HasPrefix(chart, fmt.Sprintf("%s%c", currentDirSymbol, os.PathSeparator)) || strings.HasPrefix(chart, fmt.Sprintf("%s%c", parentDirSymbol, os.PathSeparator)) {
		return true
	}

	uriLike := strings.Contains(chart, "://")
	if uriLike {
		return false
	}

	return chart == "" ||
		filepath.IsAbs(chart) ||
		!strings.Contains(chart, "/") ||
		(len(strings.Split(chart, "/")) != 2 &&
			len(strings.Split(chart, "/")) != 3)
}

func resolveRemoteChart(repoAndChart string) (string, string, bool) {
	if isLocalChart(repoAndChart) {
		return "", "", false
	}

	uriLike := strings.Contains(repoAndChart, "://")
	if uriLike {
		return "", "", false
	}

	parts := strings.SplitN(repoAndChart, "/", 2)
	if len(parts) < 2 {
		return "", "", false
	}

	repo := parts[0]
	chart := parts[1]

	return repo, chart, true
}

// normalizeChart allows for the distinction between a file path reference and repository references.
// - Any single (or double character) followed by a `/` will be considered a local file reference and
// be constructed relative to the `base path`.
// - Everything else is assumed to be an absolute path or an actual <repository>/<chart> reference.
func normalizeChart(basePath, chart string) string {
	if !isLocalChart(chart) || filepath.IsAbs(chart) {
		return chart
	}
	return filepath.Join(basePath, chart)
}

func getBuildDepsFlags(cpr *chartPrepareResult) []string {
	flags := []string{}
	if cpr.skipRefresh {
		flags = append(flags, "--skip-refresh")
	}

	return flags
}

// safePath returns a clean path
func safeVersionPath(version string) string {
	c := regexp.MustCompile(`=|>|<|!|\||~|\^| |,|\*`)
	sp := c.ReplaceAll([]byte(version), []byte("_"))
	return string(sp)
}

// versionConstraintChars are the characters that make a `version:` value a
// semver constraint rather than an exact pinned version. Kept in sync with
// safeVersionPath's substitution set so that any string touched by
// safeVersionPath is by definition a constraint. Whitespace is included
// because e.g. ">=1.0 <2.0" is a valid Masterminds/semver constraint.
const versionConstraintChars = "=><!|~^ ,*"

// isVersionConstraint reports whether v looks like a semver constraint (e.g.
// "~1", "^2.0", ">=1.0.0 <2.0.0", "*") rather than an exact pinned version
// (e.g. "1.0.1"). The check is a fast character scan; the caller can pair it
// with semver.NewConstraint if it needs to validate the constraint's syntax.
func isVersionConstraint(v string) bool {
	return strings.ContainsAny(v, versionConstraintChars)
}
