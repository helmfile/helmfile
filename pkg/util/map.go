package util

import (
	"fmt"
	"strings"
)

func CastKeysToStrings(s interface{}) (map[string]interface{}, error) {
	new := map[string]interface{}{}
	switch src := s.(type) {
	case map[interface{}]interface{}:
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
	case map[string]interface{}:
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

func recursivelyStringifyMapKey(v interface{}) (interface{}, error) {
	var castedV interface{}
	switch typedV := v.(type) {
	case map[interface{}]interface{}, map[string]interface{}:
		tmp, err := CastKeysToStrings(typedV)
		if err != nil {
			return nil, err
		}
		castedV = tmp
	case []interface{}:
		a := []interface{}{}
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
	getMap(map[string]interface{}) map[string]interface{}
	set(map[string]interface{}, string)
}

type keyArg struct {
	key string
}

func (a keyArg) getMap(m map[string]interface{}) map[string]interface{} {
	_, ok := m[a.key]
	if !ok {
		m[a.key] = map[string]interface{}{}
	}
	switch t := m[a.key].(type) {
	case map[string]interface{}:
		return t
	default:
		panic(fmt.Errorf("unexpected type: %v(%T)", t, t))
	}
}

func (a keyArg) set(m map[string]interface{}, value string) {
	m[a.key] = value
}

type indexedKeyArg struct {
	key   string
	index int
}

func (a indexedKeyArg) getArray(m map[string]interface{}) []interface{} {
	_, ok := m[a.key]
	if !ok {
		m[a.key] = make([]interface{}, a.index+1)
	}
	switch t := m[a.key].(type) {
	case []interface{}:
		if len(t) <= a.index {
			t2 := make([]interface{}, a.index+1)
			copy(t, t2)
			t = t2
		}
		return t
	default:
		panic(fmt.Errorf("unexpected type: %v(%T)", t, t))
	}
}

func (a indexedKeyArg) getMap(m map[string]interface{}) map[string]interface{} {
	t := a.getArray(m)
	if t[a.index] == nil {
		t[a.index] = map[string]interface{}{}
	}
	switch t := t[a.index].(type) {
	case map[string]interface{}:
		return t
	default:
		panic(fmt.Errorf("unexpected type: %v(%T)", t, t))
	}
}

func (a indexedKeyArg) set(m map[string]interface{}, value string) {
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

func Set(m map[string]interface{}, key []string, value string) {
	if len(key) == 0 {
		panic(fmt.Errorf("bug: unexpected length of key: %d", len(key)))
	}

	for len(key) > 1 {
		m, key = getCursor(key[0]).getMap(m), key[1:]
	}

	getCursor(key[0]).set(m, value)
}
