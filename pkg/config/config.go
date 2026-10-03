package config

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/helmfile/helmfile/pkg/maputil"
)

// stateValuesSetStringRE matches a single key=value assignment within one
// --state-values-set-string flag value. The value is either a quoted string,
// which may contain commas, or a run of plain or backslash-escaped characters
// ending at the next unescaped comma.
var stateValuesSetStringRE = regexp.MustCompile(`(?s)(?:,|^)([^\s=]+)=(['"][^'"]*['"]|(?:\\.|[^,\\])*\\?)`)

// NewCLIConfigImpl parses the raw --state-values-set-string and
// --state-values-set flag values into state value overrides on g.
func NewCLIConfigImpl(g *GlobalImpl) error {
	if err := parseStateValuesSetString(g); err != nil {
		return err
	}
	return parseStateValuesSet(g)
}

// parseStateValuesSetString applies the --state-values-set-string flag values
// to g. Unlike --state-values-set, values are kept as strings without type
// conversion.
func parseStateValuesSetString(g *GlobalImpl) error {
	optsSet := g.RawStateValuesSetString()
	if len(optsSet) == 0 {
		return nil
	}

	set := map[string]any{}
	for _, raw := range optsSet {
		for _, match := range stateValuesSetStringRE.FindAllStringSubmatch(raw, -1) {
			key, value := match[1], match[2]
			maputil.Set(set, maputil.ParseKey(key), unescapeCommas(value), true)
		}
	}
	g.SetSet(set)
	return nil
}

// parseStateValuesSet applies the --state-values-set flag values to g. Values
// are type-converted like Helm's --set.
func parseStateValuesSet(g *GlobalImpl) error {
	optsSet := g.RawStateValuesSet()
	if len(optsSet) == 0 {
		return nil
	}

	set := map[string]any{}
	for _, raw := range optsSet {
		for _, assignment := range splitOnUnescapedCommas(raw) {
			if err := setAssignment(set, assignment); err != nil {
				return err
			}
		}
	}
	g.SetSet(set)
	return nil
}

// setAssignment parses a single key=value assignment into set, returning an
// error instead of panicking when the assignment is malformed.
func setAssignment(set map[string]any, assignment string) error {
	key, value, found := strings.Cut(unescapeCommas(assignment), "=")
	if !found || key == "" {
		return fmt.Errorf("--state-values-set: invalid assignment %q: expected <key>=<value>", assignment)
	}

	maputil.Set(set, maputil.ParseKey(key), value, false)
	return nil
}

// splitOnUnescapedCommas splits the input on commas that are not escaped by a
// preceding backslash, keeping every backslash intact so that escapes remain
// available to key and value parsing.
func splitOnUnescapedCommas(input string) []string {
	segments := make([]string, 0, 1+strings.Count(input, ","))
	var current strings.Builder
	escaped := false

	for i := range len(input) {
		char := input[i]

		switch {
		case escaped:
			current.WriteByte('\\')
			current.WriteByte(char)
			escaped = false
		case char == '\\':
			escaped = true
		case char == ',':
			segments = append(segments, current.String())
			current.Reset()
		default:
			current.WriteByte(char)
		}
	}
	if escaped {
		current.WriteByte('\\')
	}
	return append(segments, current.String())
}

// unescapeCommas turns every backslash-escaped comma into a literal comma,
// preserving all other backslashes, including a trailing one.
func unescapeCommas(input string) string {
	if !strings.Contains(input, `\,`) {
		return input
	}

	var out strings.Builder
	out.Grow(len(input))
	escaped := false

	for i := range len(input) {
		char := input[i]

		switch {
		case escaped && char == ',':
			out.WriteByte(',')
			escaped = false
		case escaped:
			out.WriteByte('\\')
			out.WriteByte(char)
			escaped = false
		case char == '\\':
			escaped = true
		default:
			out.WriteByte(char)
		}
	}
	if escaped {
		out.WriteByte('\\')
	}
	return out.String()
}
