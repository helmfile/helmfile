package state

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Masterminds/semver/v3"
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

// isVersionConstraint reports whether v is a Masterminds/semver constraint
// (e.g. "~1", "^2.0", ">=1.0.0 <2.0.0", "*", "1.x", "1.X.x") rather than an
// exact pinned version (e.g. "1.0.1", "v1.0.0-rc.1", "1.0.0+build.1"). Uses
// the semver parser instead of a character scan so that wildcard-segment
// constraints ("1.x", "1.X", "1.x.x") — which contain no operator characters
// — are correctly classified as constraints and reach the OCI resolver.
//
// Only fully-qualified semvers (see isFullSemver) count as exact pins. Helm
// resolves partial versions ("1", "1.2") as floating ranges against OCI
// registries — see helm's registry.GetTagMatchingVersionOrConstraint, which
// honors a version string as an exact pin only when a registry tag literally
// equals it — so partial versions must be resolved before deriving a cache
// path, too, or they reproduce the stale-cache bug of issue #2766.
//
// Values that are neither a valid semver nor a valid constraint (empty
// string, "latest", junk) return false: helm handles those separately (empty
// means "let helm pick latest"; "latest" is rejected earlier by
// getOCIQualifiedChartName).
func isVersionConstraint(v string) bool {
	if v == "" {
		return false
	}
	// Fully-qualified semver — with or without a "v" prefix, with prerelease
	// and build metadata (which may legitimately contain "x") — is an exact
	// pin, not a constraint.
	if isFullSemver(v) {
		return false
	}
	// Everything Masterminds accepts as a constraint is a constraint. This
	// covers operator forms (~, ^, >=, ...), wildcard segment forms (1.x,
	// 1.X, 1.x.x), and partial versions (1, 1.2) that float as ranges in
	// helm's OCI tag matching.
	_, err := semver.NewConstraint(v)
	return err == nil
}

// isFullSemver reports whether v is a fully-qualified semantic version that
// spells out the whole major.minor.patch triple (e.g. "1.2.3", "v1.2.3-rc.1",
// "1.2.3+build.5"). Masterminds' lenient parser also accepts partial versions
// ("1", "1.2", "v1.2"), but helm's OCI resolution treats those as floating
// ranges rather than exact pins, so they cannot be cached under their raw
// spelling either.
func isFullSemver(v string) bool {
	if _, err := semver.NewVersion(v); err != nil {
		return false
	}
	// Drop build metadata first, then the prerelease segment, leaving the
	// numeric release core ("v1.2.3-rc.1+b" -> "v1.2.3").
	if i := strings.IndexByte(v, '+'); i >= 0 {
		v = v[:i]
	}
	if i := strings.IndexByte(v, '-'); i >= 0 {
		v = v[:i]
	}
	v = strings.TrimPrefix(v, "v")
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return false
	}
	for _, p := range parts {
		if p == "" {
			return false
		}
		for _, r := range p {
			if r < '0' || r > '9' {
				return false
			}
		}
	}
	return true
}
