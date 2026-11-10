package maputil

import (
	"fmt"
	"maps"
	"strconv"
	"strings"
)

func CastKeysToStrings(s any) (map[string]any, error) {
	new := map[string]any{}
	switch src := s.(type) {
	case map[any]any:
		for k, v := range src {
			var strK string
			switch typedK := k.(type) {
			case string:
				strK = typedK
			default:
				return nil, fmt.Errorf("unexpected type of key in map: expected string, got %T: value=%v, map=%v", typedK, typedK, src)
			}

			castedV, err := RecursivelyStringifyMapKey(v)
			if err != nil {
				return nil, err
			}

			new[strK] = castedV
		}
	case map[string]any:
		for k, v := range src {
			castedV, err := RecursivelyStringifyMapKey(v)
			if err != nil {
				return nil, err
			}

			new[k] = castedV
		}
	}
	return new, nil
}

// DeepCopyMap creates a deep copy of a map[string]any, recursively copying all
// nested maps and slices. Unlike YAML marshal/unmarshal round-trips, it preserves
// the original Go types of all values (e.g. the string "true" stays a string,
// not a bool) and never loses data due to special characters in values.
// It also normalises map[any]any keys to strings, matching CastKeysToStrings.
func DeepCopyMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	result := make(map[string]any, len(m))
	for k, v := range m {
		result[k] = deepCopyValue(v)
	}
	return result
}

func deepCopyValue(v any) any {
	switch val := v.(type) {
	case map[string]any:
		return DeepCopyMap(val)
	case map[any]any:
		copied := make(map[string]any, len(val))
		for k, item := range val {
			copied[fmt.Sprintf("%v", k)] = deepCopyValue(item)
		}
		return copied
	case []any:
		copied := make([]any, len(val))
		for i, item := range val {
			copied[i] = deepCopyValue(item)
		}
		return copied
	default:
		// Primitives (string, int, bool, nil, float, etc.) are copied by value.
		return val
	}
}

func RecursivelyStringifyMapKey(v any) (any, error) {
	var castedV any
	switch typedV := v.(type) {
	case map[any]any, map[string]any:
		tmp, err := CastKeysToStrings(typedV)
		if err != nil {
			return nil, err
		}
		castedV = tmp
	case []any:
		a := []any{}
		for i := range typedV {
			res, err := RecursivelyStringifyMapKey(typedV[i])
			if err != nil {
				return nil, err
			}
			a = append(a, res)
		}
		castedV = a
	default:
		castedV = typedV
	}
	return castedV, nil
}

type arg interface {
	getMap(map[string]any) map[string]any
	set(map[string]any, any)
}

type keyArg struct {
	key string
}

// getMap returns the map stored at a.key, replacing any value of another
// type so that a later assignment such as foo.bar=2 wins over an earlier
// foo=1 instead of panicking.
func (a keyArg) getMap(m map[string]any) map[string]any {
	t, ok := m[a.key].(map[string]any)
	if !ok {
		t = map[string]any{}
		m[a.key] = t
	}
	return t
}

func (a keyArg) set(m map[string]any, value any) {
	m[a.key] = value
}

type indexedKeyArg struct {
	key   string
	index int
}

// getArray returns the array stored at a.key, grown to hold a.index. A value
// of another type is replaced by a new array, so the later assignment wins.
func (a indexedKeyArg) getArray(m map[string]any) []any {
	t, ok := m[a.key].([]any)
	if !ok {
		t = make([]any, a.index+1)
		m[a.key] = t
	}
	if len(t) <= a.index {
		t2 := make([]any, a.index+1)
		copy(t2, t)
		t = t2
		m[a.key] = t
	}
	return t
}

// getMap returns the map stored at a.key[a.index], replacing a missing value
// or a value of another type with a new map.
func (a indexedKeyArg) getMap(m map[string]any) map[string]any {
	t := a.getArray(m)
	elem, ok := t[a.index].(map[string]any)
	if !ok {
		elem = map[string]any{}
		t[a.index] = elem
	}
	return elem
}

func (a indexedKeyArg) set(m map[string]any, value any) {
	t := a.getArray(m)
	t[a.index] = value
	m[a.key] = t
}

func getCursor(key string) arg {
	key = strings.ReplaceAll(key, "[", " ")
	key = strings.ReplaceAll(key, "]", " ")
	k := key
	idx := 0

	n, err := fmt.Sscanf(key, "%s %d", &k, &idx)

	if n == 2 && err == nil {
		return indexedKeyArg{
			key:   k,
			index: idx,
		}
	}

	return keyArg{
		key: key,
	}
}

func ParseKey(key string) []string {
	r := []string{}
	part := ""
	escaped := false
	for _, rune := range key {
		split := false
		switch {
		case !escaped && rune == '\\':
			escaped = true
			continue
		case rune == '.':
			split = !escaped
		}
		escaped = false
		if split {
			r = append(r, part)
			part = ""
		} else {
			part += string(rune)
		}
	}
	if len(part) > 0 {
		r = append(r, part)
	}
	return r
}

// MaxIndex is the largest list index accepted by Set, the same limit Helm
// applies to --set.
const MaxIndex = 65536

// Set assigns value at key in m. It returns an error, leaving m unchanged,
// when a list index in key is negative or greater than MaxIndex.
func Set(m map[string]any, key []string, value string, stringBool bool) error {
	if len(key) == 0 {
		panic(fmt.Errorf("bug: unexpected length of key: %d", len(key)))
	}

	cursors := make([]arg, len(key))
	for i, k := range key {
		cursor := getCursor(k)
		if indexed, ok := cursor.(indexedKeyArg); ok {
			if indexed.index < 0 {
				return fmt.Errorf("negative index %d not allowed in %q", indexed.index, k)
			}
			if indexed.index > MaxIndex {
				return fmt.Errorf("index %d in %q is greater than the maximum supported index of %d", indexed.index, k, MaxIndex)
			}
		}
		cursors[i] = cursor
	}

	for _, cursor := range cursors[:len(cursors)-1] {
		m = cursor.getMap(m)
	}

	cursors[len(cursors)-1].set(m, typedVal(value, stringBool))
	return nil
}

func typedVal(val string, st bool) any {
	// if st is true, directly return it without casting it
	if st {
		return val
	}

	if strings.EqualFold(val, "true") {
		return true
	}

	if strings.EqualFold(val, "false") {
		return false
	}

	if strings.EqualFold(val, "null") {
		return nil
	}

	// handling of only zero, if val has zero prefix, it will be considered as string
	if strings.EqualFold(val, "0") {
		return int64(0)
	}

	// If this value does not start with zero, try parsing it to an int
	if len(val) != 0 && val[0] != '0' {
		if iv, err := strconv.ParseInt(val, 10, 64); err == nil {
			return iv
		}
	}

	return val
}

// MergeMaps merges two maps with special handling for nested maps and arrays.
func MergeMaps(a, b map[string]any, opts ...MergeOptions) map[string]any {
	arrayStrategy := ArrayMergeStrategySparse
	if len(opts) > 0 {
		arrayStrategy = opts[0].ArrayStrategy
	}

	out := make(map[string]any, len(a))
	maps.Copy(out, a)
	for k, v := range b {
		if v == nil {
			// If key doesn't exist in base, add nil (issue #1154).
			// If key exists in base, don't overwrite with nil.
			if _, exists := out[k]; !exists {
				out[k] = nil
			}
			continue
		}
		if v, ok := v.(map[string]any); ok {
			if bv, ok := out[k]; ok {
				if bv, ok := bv.(map[string]any); ok {
					out[k] = MergeMaps(bv, v, opts...)
					continue
				}
			}
		}
		vSlice := toInterfaceSlice(v)
		if vSlice != nil {
			if outVal, exists := out[k]; exists {
				outSlice := toInterfaceSlice(outVal)
				if outSlice != nil {
					out[k] = mergeSlices(outSlice, vSlice, arrayStrategy)
					continue
				}
			}
		}
		out[k] = v
	}
	return out
}

func toInterfaceSlice(v any) []any {
	if slice, ok := v.([]any); ok {
		return slice
	}
	return nil
}

type ArrayMergeStrategy int

const (
	// ArrayMergeStrategySparse uses auto-detection: sparse arrays (with nils) merge
	// element-by-element, complete arrays (no nils) replace entirely.
	ArrayMergeStrategySparse ArrayMergeStrategy = iota
	// ArrayMergeStrategyReplace always replaces arrays entirely.
	ArrayMergeStrategyReplace
	// ArrayMergeStrategyMerge always merges arrays element-by-element (for CLI overrides).
	ArrayMergeStrategyMerge
)

type MergeOptions struct {
	ArrayStrategy ArrayMergeStrategy
}

// mergeSlices merges two slices based on the strategy.
//
// Behavior by strategy:
//   - ArrayMergeStrategyReplace: always returns override as-is
//   - ArrayMergeStrategySparse: auto-detects based on nil values (see below)
//   - ArrayMergeStrategyMerge: always merges element-by-element
//
// For Sparse strategy, auto-detection logic:
//   - If override contains ANY nil values → treated as sparse → merge element-by-element
//   - If override contains NO nil values → treated as complete → replace entirely
//
// Edge cases:
//   - Empty array `[]` has no nils, so it replaces entirely. This is intentional:
//     explicitly setting an empty array should clear the base array.
//   - Explicit `[null, value]` in YAML is treated as sparse (rare but correct).
//
// Recursive behavior: When merging maps within array elements, the same strategy
// is propagated to nested MergeMaps calls, maintaining consistent merge semantics.
func mergeSlices(base, override []any, strategy ArrayMergeStrategy) []any {
	if strategy == ArrayMergeStrategyReplace {
		return override
	}

	if strategy == ArrayMergeStrategySparse {
		isSparse := false
		for _, v := range override {
			if v == nil {
				isSparse = true
				break
			}
		}
		if !isSparse {
			return override
		}
	}

	// Merge element-by-element (for ArrayMergeStrategyMerge or sparse arrays)
	maxLen := max(len(override), len(base))

	result := make([]any, maxLen)
	copy(result, base)

	for i := range override {
		overrideVal := override[i]
		if overrideVal == nil {
			continue
		}

		if overrideMap, ok := overrideVal.(map[string]any); ok {
			if i < len(base) {
				if baseMap, ok := base[i].(map[string]any); ok {
					result[i] = MergeMaps(baseMap, overrideMap, MergeOptions{ArrayStrategy: strategy})
					continue
				}
			}
		}

		result[i] = overrideVal
	}

	return result
}
