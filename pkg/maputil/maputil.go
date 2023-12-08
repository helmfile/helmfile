package maputil

import (
	"fmt"
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

			castedV, err := recursivelyStringifyMapKey(v)
			if err != nil {
				return nil, err
			}

			new[strK] = castedV
		}
	case map[string]any:
		for k, v := range src {
			castedV, err := recursivelyStringifyMapKey(v)
			if err != nil {
				return nil, err
			}

			new[k] = castedV
		}
	}
	return new, nil
}

func recursivelyStringifyMapKey(v any) (any, error) {
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
			res, err := recursivelyStringifyMapKey(typedV[i])
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

func (a keyArg) getMap(m map[string]any) map[string]any {
	_, ok := m[a.key]
	if !ok {
		m[a.key] = map[string]any{}
	}
	switch t := m[a.key].(type) {
	case map[string]any:
		return t
	default:
		panic(fmt.Errorf("unexpected type: %v(%T)", t, t))
	}
}

func (a keyArg) set(m map[string]any, value any) {
	m[a.key] = value
}

type indexedKeyArg struct {
	key   string
	index int
}

func (a indexedKeyArg) getArray(m map[string]any) []any {
	_, ok := m[a.key]
	if !ok {
		m[a.key] = make([]any, a.index+1)
	}
	switch t := m[a.key].(type) {
	case []any:
		if len(t) <= a.index {
			t2 := make([]any, a.index+1)
			copy(t2, t)
			t = t2
		}
		return t
	default:
		panic(fmt.Errorf("unexpected type: %v(%T)", t, t))
	}
}

func (a indexedKeyArg) getMap(m map[string]any) map[string]any {
	t := a.getArray(m)
	if t[a.index] == nil {
		t[a.index] = map[string]any{}
	}
	switch t := t[a.index].(type) {
	case map[string]any:
		return t
	default:
		panic(fmt.Errorf("unexpected type: %v(%T)", t, t))
	}
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

func Set(m map[string]any, key []string, value string, stringBool bool) {
	if len(key) == 0 {
		panic(fmt.Errorf("bug: unexpected length of key: %d", len(key)))
	}

	for len(key) > 1 {
		m, key = getCursor(key[0]).getMap(m), key[1:]
	}

	getCursor(key[0]).set(m, typedVal(value, stringBool))
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

func MergeMaps(a, b map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(a))
	// fill the out map with the first map
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		if v, ok := v.(map[string]interface{}); ok {
			if bv, ok := out[k]; ok {
				if bv, ok := bv.(map[string]interface{}); ok {
					// if b and out map has a map value, merge it too
					out[k] = MergeMaps(bv, v)
					continue
				}
			}
		}
		out[k] = v
	}
	return out
}
