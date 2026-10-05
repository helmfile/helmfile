package state

import (
	"fmt"
	"maps"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// ReleaseFilter is used to determine if a given release should be used during helmfile execution
type ReleaseFilter interface {
	// Match returns true if the ReleaseSpec matches the Filter
	Match(r ReleaseSpec) bool
}

// LabelFilter matches a release with the given positive lables. Negative labels
// invert the match for cases such as tier!=backend
type LabelFilter struct {
	positiveLabels [][]string
	negativeLabels [][]string
}

// Match will match a release that has the same labels as the filter
func (l LabelFilter) Match(r ReleaseSpec) bool {
	for _, element := range l.positiveLabels {
		if !labelMatches(r.Labels, element[0], element[1]) {
			return false
		}
	}
	for _, element := range l.negativeLabels {
		if labelMatches(r.Labels, element[0], element[1]) {
			return false
		}
	}
	return true
}

// labelMatches reports whether the release's labels satisfy the single k=v
// constraint: positive callers require a match, negative callers require no
// match. The reserved "dir" key matches by directory prefix instead of
// strict equality.
func labelMatches(labels map[string]string, k, v string) bool {
	if k == DirLabel {
		return matchDirPrefix(labels[DirLabel], v)
	}
	val, ok := labels[k]
	return ok && val == v
}

// DirLabel is the reserved selector key for path-based release filtering.
// Releases get this label injected at match time with the directory of their
// defining helmfile, relative to the root helmfile. It uses directory-prefix
// semantics in LabelFilter rather than the strict equality of other keys.
const DirLabel = "dir"

// withDirLabel returns r with the "dir" label set to the auto-populated dir
// for matching, without mutating the source release or surfacing the value in
// user-facing label output. A user-defined "dir" label is always shadowed:
// when dir is empty (remote or outside-root helmfiles) it is dropped, so such
// releases never take part in dir-based filtering.
func withDirLabel(r ReleaseSpec, dir string) ReleaseSpec {
	if _, userDefined := r.Labels[DirLabel]; dir == "" && !userDefined {
		return r
	}
	cloned := r
	cloned.Labels = make(map[string]string, len(r.Labels)+1)
	maps.Copy(cloned.Labels, r.Labels)
	if dir == "" {
		delete(cloned.Labels, DirLabel)
	} else {
		cloned.Labels[DirLabel] = dir
	}
	return cloned
}

// matchDirPrefix reports whether a release's dirLabel falls under the user's
// target value, using directory-prefix semantics: the label must equal the
// target or live under target/. Inputs are normalized so trailing slashes and
// `./` prefixes do not matter. Empty dirLabel never matches (releases from
// remote helmfiles or outside-root branches have no anchor).
func matchDirPrefix(dirLabel, value string) bool {
	if dirLabel == "" {
		return false
	}
	return dirAtOrBelow(NormalizeDirValue(dirLabel), NormalizeDirValue(value))
}

func dirAtOrBelow(dir, ancestor string) bool {
	return dir == ancestor || strings.HasPrefix(dir, ancestor+"/")
}

func escapesRoot(normalizedDir string) bool {
	return normalizedDir == ".." || strings.HasPrefix(normalizedDir, "../")
}

// DirRelativeToRoot returns absDir relative to rootDir in normalized slash
// form, or false when absDir lies outside rootDir.
func DirRelativeToRoot(rootDir, absDir string) (string, bool) {
	rel, err := filepath.Rel(rootDir, absDir)
	if err != nil {
		return "", false
	}
	rel = NormalizeDirValue(rel)
	if escapesRoot(rel) {
		return "", false
	}
	return rel, true
}

// NormalizeDirValue canonicalizes a dir= selector value or a "dir" auto-label
// to slash form with no trailing slash and no redundant elements. Empty input
// stays empty. Examples: `apps/x/` → `apps/x`, `./apps/x` → `apps/x`,
// `apps//x` → `apps/x`, `.` → `.`.
func NormalizeDirValue(v string) string {
	if v == "" {
		return ""
	}
	return path.Clean(filepath.ToSlash(v))
}

// PositiveDirTargets parses one selector group string and returns the
// normalized positive dir= values declared in it. Returns the same error as
// ParseLabels on malformed input so callers can choose to skip the group.
// Used by traversal-skip logic to peek at dir constraints without exposing
// LabelFilter internals.
func PositiveDirTargets(selector string) ([]string, error) {
	lf, err := ParseLabels(selector)
	if err != nil {
		return nil, err
	}
	var dirs []string
	for _, kv := range lf.positiveLabels {
		if kv[0] == DirLabel {
			dirs = append(dirs, NormalizeDirValue(kv[1]))
		}
	}
	return dirs, nil
}

// SelectorsAreCompatible checks whether any pair of selectors from two sets could
// potentially match the same release. It compares only positive labels (key=value):
// two selectors conflict if they require the same key to have different values.
// Returns true if at least one pair is compatible, false only if all pairs conflict.
// On parse error, returns (true, err): the conservative true ensures subhelmfiles
// are never incorrectly skipped due to malformed input, while the non-nil error
// allows callers to log or handle the parse failure if desired.
func SelectorsAreCompatible(selectorsA, selectorsB []string) (bool, error) {
	if len(selectorsA) == 0 || len(selectorsB) == 0 {
		return true, nil
	}

	filtersA, err := parseLabelFilters(selectorsA)
	if err != nil {
		return true, err
	}

	filtersB, err := parseLabelFilters(selectorsB)
	if err != nil {
		return true, err
	}

	for _, a := range filtersA {
		if slices.ContainsFunc(filtersB, a.positiveLabelsCompatibleWith) {
			return true, nil
		}
	}

	return false, nil
}

func parseLabelFilters(selectors []string) ([]LabelFilter, error) {
	filters := make([]LabelFilter, 0, len(selectors))
	for _, s := range selectors {
		f, err := ParseLabels(s)
		if err != nil {
			return nil, err
		}
		filters = append(filters, f)
	}
	return filters, nil
}

// positiveLabelsCompatibleWith returns true if the positive labels of two filters
// do not conflict (i.e., no shared key with a different value). The "dir" key
// uses directory-prefix semantics, so two `dir=` constraints are compatible
// when one path is at or below the other; only siblings genuinely conflict.
func (l LabelFilter) positiveLabelsCompatibleWith(other LabelFilter) bool {
	for _, a := range l.positiveLabels {
		for _, b := range other.positiveLabels {
			if !positiveLabelPairCompatible(a, b) {
				return false
			}
		}
	}
	return true
}

// positiveLabelPairCompatible reports whether two positive k=v constraints
// could be satisfied by the same release. The "dir" key compares with
// directory-prefix semantics, so overlapping subtrees are compatible.
func positiveLabelPairCompatible(a, b []string) bool {
	if a[0] != b[0] {
		return true
	}
	if a[0] == DirLabel {
		return DirsCompatible(a[1], b[1])
	}
	return a[1] == b[1]
}

// DirsCompatible reports whether two dir= values could both be satisfied by
// the same release. Compatible when one is at-or-below the other in the
// directory hierarchy; siblings or unrelated subtrees conflict.
func DirsCompatible(a, b string) bool {
	a = NormalizeDirValue(a)
	b = NormalizeDirValue(b)
	return dirAtOrBelow(a, b) || dirAtOrBelow(b, a)
}

var (
	reLabelMismatch = regexp.MustCompile(`^[a-zA-Z0-9_\.\/\+-]+!=[a-zA-Z0-9_\.\/\+-]+$`)
	reLabelMatch    = regexp.MustCompile(`^[a-zA-Z0-9_\.\/\+-]+=[a-zA-Z0-9_\.\/\+-]+$`)
)

// ParseLabels takes a label in the form foo=bar,baz!=bat and returns a LabelFilter that will match the labels
func ParseLabels(l string) (LabelFilter, error) {
	lf := LabelFilter{
		positiveLabels: [][]string{},
		negativeLabels: [][]string{},
	}
	for label := range strings.SplitSeq(l, ",") {
		kv, negative, err := parseOneLabel(label)
		if err != nil {
			return lf, err
		}
		if negative {
			lf.negativeLabels = append(lf.negativeLabels, kv)
		} else {
			lf.positiveLabels = append(lf.positiveLabels, kv)
		}
	}
	return lf, nil
}

// parseOneLabel parses a single k=v or k!=v label. Values of the reserved
// "dir" key are validated so unusable targets fail fast.
func parseOneLabel(label string) (kv []string, negative bool, err error) {
	sep := ""
	if reLabelMismatch.MatchString(label) {
		sep = "!="
	} else if reLabelMatch.MatchString(label) {
		sep = "="
	}
	if sep == "" {
		return nil, false, fmt.Errorf("malformed label: %s. Expected label in form k=v or k!=v", label)
	}
	kv = strings.Split(label, sep)
	if kv[0] != DirLabel {
		return kv, sep == "!=", nil
	}
	if err := validateDirSelectorValue(kv[1]); err != nil {
		return nil, false, err
	}
	return kv, sep == "!=", nil
}

// validateDirSelectorValue rejects dir= and dir!= values that cannot be
// matched against the auto-populated dir label: absolute paths (labels are
// always root-relative), paths that escape the root via "..", and the bare
// "." which would either be a no-op or carry surprising "root-only" semantics
// depending on interpretation. Callers should omit dir= entirely to select
// every release.
func validateDirSelectorValue(v string) error {
	n := NormalizeDirValue(v)
	if n == "." {
		return fmt.Errorf("dir= selector value %q is not allowed; omit -l dir= entirely to match every release", v)
	}
	if strings.HasPrefix(n, "/") {
		return fmt.Errorf("dir= selector value %q must be a path relative to the root helmfile, not absolute", v)
	}
	if escapesRoot(n) {
		return fmt.Errorf("dir= selector value %q escapes the root helmfile directory", v)
	}
	return nil
}
