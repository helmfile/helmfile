package state

import (
	"fmt"
	"path"
	"path/filepath"
	"regexp"
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
	if len(l.positiveLabels) > 0 {
		for _, element := range l.positiveLabels {
			k := element[0]
			v := element[1]
			if k == DirLabel {
				if !matchDirPrefix(r.Labels[DirLabel], v) {
					return false
				}
				continue
			}
			if rVal, ok := r.Labels[k]; !ok {
				return false
			} else if rVal != v {
				return false
			}
		}
	}

	if len(l.negativeLabels) > 0 {
		for _, element := range l.negativeLabels {
			k := element[0]
			v := element[1]
			if k == DirLabel {
				if matchDirPrefix(r.Labels[DirLabel], v) {
					return false
				}
				continue
			}
			if rVal, ok := r.Labels[k]; !ok {

			} else if rVal == v {
				return false
			}
		}
	}
	return true
}

// DirLabel is the reserved selector key for path-based release filtering.
// Releases get this label injected at match time with the directory of their
// defining helmfile, relative to the root helmfile. It uses directory-prefix
// semantics in LabelFilter rather than the strict equality of other keys.
const DirLabel = "dir"

// injectLabel returns a copy of r with the Labels map extended by a single
// key/value. Used to attach state-derived labels (currently only "dir") for
// matching, without mutating the source release or surfacing the value in
// user-facing label output.
func injectLabel(r ReleaseSpec, key, value string) ReleaseSpec {
	cloned := r
	cloned.Labels = make(map[string]string, len(r.Labels)+1)
	for k, v := range r.Labels {
		cloned.Labels[k] = v
	}
	cloned.Labels[key] = value
	return cloned
}

// matchDirPrefix reports whether a release's dirLabel falls under the user's
// target value, using directory-prefix semantics: the label must equal the
// target or live under target/. Inputs are normalized so trailing slashes and
// `./` prefixes do not matter; `dir=.` selects only releases defined at the
// root helmfile itself. Empty dirLabel never matches (releases from remote
// helmfiles or outside-root branches have no anchor).
func matchDirPrefix(dirLabel, value string) bool {
	if dirLabel == "" {
		return false
	}
	dirLabel = NormalizeDirValue(dirLabel)
	value = NormalizeDirValue(value)
	if dirLabel == value {
		return true
	}
	return strings.HasPrefix(dirLabel, value+"/")
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
		for _, b := range filtersB {
			if a.positiveLabelsCompatibleWith(b) {
				return true, nil
			}
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
			if a[0] != b[0] {
				continue
			}
			if a[0] == DirLabel {
				if !dirsCompatible(a[1], b[1]) {
					return false
				}
				continue
			}
			if a[1] != b[1] {
				return false
			}
		}
	}
	return true
}

// dirsCompatible reports whether two dir= values could both be satisfied by
// the same release. Compatible when one is at-or-below the other in the
// directory hierarchy; siblings or unrelated subtrees conflict.
func dirsCompatible(a, b string) bool {
	a = NormalizeDirValue(a)
	b = NormalizeDirValue(b)
	if a == b {
		return true
	}
	return strings.HasPrefix(a, b+"/") || strings.HasPrefix(b, a+"/")
}

var (
	reLabelMismatch = regexp.MustCompile(`^[a-zA-Z0-9_\.\/\+-]+!=[a-zA-Z0-9_\.\/\+-]+$`)
	reLabelMatch    = regexp.MustCompile(`^[a-zA-Z0-9_\.\/\+-]+=[a-zA-Z0-9_\.\/\+-]+$`)
)

// ParseLabels takes a label in the form foo=bar,baz!=bat and returns a LabelFilter that will match the labels
func ParseLabels(l string) (LabelFilter, error) {
	lf := LabelFilter{}
	lf.positiveLabels = [][]string{}
	lf.negativeLabels = [][]string{}
	var err error
	labels := strings.Split(l, ",")
	for _, label := range labels {
		if match := reLabelMismatch.MatchString(label); match {
			kv := strings.Split(label, "!=")
			if kv[0] == DirLabel {
				if err := validateDirSelectorValue(kv[1]); err != nil {
					return lf, err
				}
			}
			lf.negativeLabels = append(lf.negativeLabels, kv)
		} else if match := reLabelMatch.MatchString(label); match {
			kv := strings.Split(label, "=")
			if kv[0] == DirLabel {
				if err := validateDirSelectorValue(kv[1]); err != nil {
					return lf, err
				}
			}
			lf.positiveLabels = append(lf.positiveLabels, kv)
		} else {
			return lf, fmt.Errorf("malformed label: %s. Expected label in form k=v or k!=v", label)
		}
	}
	return lf, err
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
	if n == ".." || strings.HasPrefix(n, "../") {
		return fmt.Errorf("dir= selector value %q escapes the root helmfile directory", v)
	}
	return nil
}
