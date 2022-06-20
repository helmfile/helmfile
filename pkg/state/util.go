package state

import (
	"path/filepath"
	"regexp"
	"strings"
)

func isLocalChart(chart string) bool {
	regex := regexp.MustCompile("^[.]?./")
	matched := regex.MatchString(chart)
	if matched {
		return true
	}

	uriLike := strings.Contains(chart, "://")
	if uriLike {
		return false
	}

	return chart == "" ||
		chart[0] == '/' ||
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
// 	 be constructed relative to the `base path`.
// - Everything else is assumed to be an absolute path or an actual <repository>/<chart> reference.
func normalizeChart(basePath, chart string) string {
	if !isLocalChart(chart) || chart[0] == '/' {
		return chart
	}
	return filepath.Join(basePath, chart)
}
