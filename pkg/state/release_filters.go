package state

import (
	"fmt"
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
			if rVal, ok := r.Labels[k]; !ok {

			} else if rVal == v {
				return false
			}
		}
	}
	return true
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
// do not conflict (i.e., no shared key with a different value).
func (l LabelFilter) positiveLabelsCompatibleWith(other LabelFilter) bool {
	for _, a := range l.positiveLabels {
		for _, b := range other.positiveLabels {
			if a[0] == b[0] && a[1] != b[1] {
				return false
			}
		}
	}
	return true
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
			lf.negativeLabels = append(lf.negativeLabels, kv)
		} else if match := reLabelMatch.MatchString(label); match {
			kv := strings.Split(label, "=")
			lf.positiveLabels = append(lf.positiveLabels, kv)
		} else {
			return lf, fmt.Errorf("malformed label: %s. Expected label in form k=v or k!=v", label)
		}
	}
	return lf, err
}
