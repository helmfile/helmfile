package maputil

import (
	"reflect"
	"strings"
	"testing"
)

func TestMapUtil_StrKeys(t *testing.T) {
	m := map[string]any{
		"a": []any{
			map[string]any{
				"b": []any{
					map[string]any{
						"c": "C",
					},
				},
			},
		},
	}

	r, err := CastKeysToStrings(m)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	a := r["a"].([]any)
	a0 := a[0].(map[string]any)
	b := a0["b"].([]any)
	b0 := b[0].(map[string]any)
	c := b0["c"]

	if c != "C" {
		t.Errorf("unexpected c: expected=C, got=%s", c)
	}
}

func TestMapUtil_IFKeys(t *testing.T) {
	m := map[any]any{
		"a": []any{
			map[any]any{
				"b": []any{
					map[any]any{
						"c": "C",
					},
				},
			},
		},
	}

	r, err := CastKeysToStrings(m)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	a := r["a"].([]any)
	a0 := a[0].(map[string]any)
	b := a0["b"].([]any)
	b0 := b[0].(map[string]any)
	c := b0["c"]

	if c != "C" {
		t.Errorf("unexpected c: expected=C, got=%s", c)
	}
}

func TestMapUtil_KeyArg(t *testing.T) {
	m := map[string]any{}

	key := []string{"a", "b", "c"}

	Set(m, key, "C", false)

	c := (((m["a"].(map[string]any))["b"]).(map[string]any))["c"]

	if c != "C" {
		t.Errorf("unexpected c: expected=C, got=%s", c)
	}
}

func TestMapUtil_IndexedKeyArg(t *testing.T) {
	m := map[string]any{}

	key := []string{"a", "b[0]", "c"}

	Set(m, key, "C", false)

	c := (((m["a"].(map[string]any))["b"].([]any))[0].(map[string]any))["c"]

	if c != "C" {
		t.Errorf("unexpected c: expected=C, got=%s", c)
	}
}

func TestMapUtil_IndexedKeyArg2(t *testing.T) {
	cases := []struct {
		name           string
		stateValuesSet []string
		want           map[string]any
	}{
		{
			name:           "IndexedKeyArg",
			stateValuesSet: []string{"myvalues[0]=HELLO,myvalues[1]=HELMFILE"},
			want:           map[string]any{"myvalues": []any{"HELLO", "HELMFILE"}},
		},
		{
			name:           "two state value",
			stateValuesSet: []string{"myvalues[0]=HELLO,myvalues[1]=HELMFILE", "myvalues[2]=HELLO"},
			want:           map[string]any{"myvalues": []any{"HELLO", "HELMFILE", "HELLO"}},
		},
		{
			name:           "different key",
			stateValuesSet: []string{"myvalues[0]=HELLO,key2[0]=HELMFILE", "myvalues[1]=HELLO2"},
			want:           map[string]any{"myvalues": []any{"HELLO", "HELLO2"}, "key2": []any{"HELMFILE"}},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			set := map[string]any{}
			for i := range c.stateValuesSet {
				ops := strings.Split(c.stateValuesSet[i], ",")
				for j := range ops {
					op := strings.SplitN(ops[j], "=", 2)
					k := ParseKey(op[0])
					v := op[1]

					Set(set, k, v, false)
				}
			}
			if !reflect.DeepEqual(set, c.want) {
				t.Errorf("expected set %v, got %v", c.want, set)
			}
		})
	}
}

type parseKeyTc struct {
	key    string
	result map[int]string
}

func TestMapUtil_ParseKey(t *testing.T) {
	tcs := []parseKeyTc{
		{
			key: `a.b.c`,
			result: map[int]string{
				0: "a",
				1: "b",
				2: "c",
			},
		},
		{
			key: `a\.b.c`,
			result: map[int]string{
				0: "a.b",
				1: "c",
			},
		},
		{
			key: `a\.b\.c`,
			result: map[int]string{
				0: "a.b.c",
			},
		},
	}

	for _, tc := range tcs {
		parts := ParseKey(tc.key)

		for index, value := range tc.result {
			if parts[index] != value {
				t.Errorf("unexpected key part[%d]: expected=%s, got=%s", index, value, parts[index])
			}
		}
	}
}

func TestMapUtil_typedVal(t *testing.T) {
	typedValueTest(t, "true", true)
	typedValueTest(t, "null", nil)
	typedValueTest(t, "0", int64(0))
	typedValueTest(t, "5", int64(5))
	typedValueTest(t, "05", "05")
}

func typedValueTest(t *testing.T, input string, expectedWhenNoStr any) {
	returnValue := typedVal(input, true)
	if returnValue != input {
		t.Errorf("unexpected typed value: expected=%s, got=%s", input, returnValue)
	}
	returnValue = typedVal(input, false)
	if returnValue != expectedWhenNoStr {
		t.Errorf("unexpected typed value: expected=%s, got=%s", input, returnValue)
	}
}

func TestMapUtil_MergeMaps(t *testing.T) {
	map1 := map[string]interface{}{
		"debug": true,
	}
	map2 := map[string]interface{}{
		"logLevel":     "info",
		"replicaCount": 3,
	}
	map3 := map[string]interface{}{
		"logLevel": "info",
		"replicaCount": map[string]any{
			"app1":    3,
			"awesome": 4,
		},
	}
	map4 := map[string]interface{}{
		"logLevel": "info",
		"replicaCount": map[string]any{
			"app1": 3,
		},
	}
	map5 := map[string]interface{}{
		"logLevel":     "error",
		"replicaCount": nil,
	}

	testMap := MergeMaps(map2, map4)
	equal := reflect.DeepEqual(testMap, map4)
	if !equal {
		t.Errorf("Expected a nested map to overwrite a flat value. Expected: %v, got %v", map4, testMap)
	}

	testMap = MergeMaps(map4, map2)
	equal = reflect.DeepEqual(testMap, map2)
	if !equal {
		t.Errorf("Expected a flat value to overwrite a map. Expected: %v, got %v", map2, testMap)
	}

	testMap = MergeMaps(map4, map3)
	equal = reflect.DeepEqual(testMap, map3)
	if !equal {
		t.Errorf("Expected a nested map to overwrite another nested map. Expected: %v, got %v", map3, testMap)
	}

	testMap = MergeMaps(map1, map3)
	expectedMap := map[string]interface{}{
		"debug":    true,
		"logLevel": "info",
		"replicaCount": map[string]any{
			"app1":    3,
			"awesome": 4,
		},
	}
	equal = reflect.DeepEqual(testMap, expectedMap)
	if !equal {
		t.Errorf("Expected a map with different keys to merge properly with another map. Expected: %v, got %v", expectedMap, testMap)
	}

	testMap = MergeMaps(map3, map5)
	expectedMap = map[string]interface{}{
		"logLevel": "error",
		"replicaCount": map[string]any{
			"app1":    3,
			"awesome": 4,
		},
	}
	equal = reflect.DeepEqual(testMap, expectedMap)
	if !equal {
		t.Errorf("Expected a map with empty value not to overwrite another map's value. Expected: %v, got %v", expectedMap, testMap)
	}
}
