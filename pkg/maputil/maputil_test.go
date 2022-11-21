package maputil

import (
	"reflect"
	"strings"
	"testing"
)

func TestMapUtil_StrKeys(t *testing.T) {
	m := map[string]interface{}{
		"a": []interface{}{
			map[string]interface{}{
				"b": []interface{}{
					map[string]interface{}{
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

	a := r["a"].([]interface{})
	a0 := a[0].(map[string]interface{})
	b := a0["b"].([]interface{})
	b0 := b[0].(map[string]interface{})
	c := b0["c"]

	if c != "C" {
		t.Errorf("unexpected c: expected=C, got=%s", c)
	}
}

func TestMapUtil_IFKeys(t *testing.T) {
	m := map[interface{}]interface{}{
		"a": []interface{}{
			map[interface{}]interface{}{
				"b": []interface{}{
					map[interface{}]interface{}{
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

	a := r["a"].([]interface{})
	a0 := a[0].(map[string]interface{})
	b := a0["b"].([]interface{})
	b0 := b[0].(map[string]interface{})
	c := b0["c"]

	if c != "C" {
		t.Errorf("unexpected c: expected=C, got=%s", c)
	}
}

func TestMapUtil_KeyArg(t *testing.T) {
	m := map[string]interface{}{}

	key := []string{"a", "b", "c"}

	Set(m, key, "C")

	c := (((m["a"].(map[string]interface{}))["b"]).(map[string]interface{}))["c"]

	if c != "C" {
		t.Errorf("unexpected c: expected=C, got=%s", c)
	}
}

func TestMapUtil_IndexedKeyArg(t *testing.T) {
	m := map[string]interface{}{}

	key := []string{"a", "b[0]", "c"}

	Set(m, key, "C")

	c := (((m["a"].(map[string]interface{}))["b"].([]interface{}))[0].(map[string]interface{}))["c"]

	if c != "C" {
		t.Errorf("unexpected c: expected=C, got=%s", c)
	}
}

func TestMapUtil_IndexedKeyArg2(t *testing.T) {
	cases := []struct {
		name           string
		stateValuesSet []string
		want           map[string]interface{}
	}{
		{
			name:           "IndexedKeyArg",
			stateValuesSet: []string{"myvalues[0]=HELLO,myvalues[1]=HELMFILE"},
			want:           map[string]interface{}{"myvalues": []interface{}{"HELLO", "HELMFILE"}},
		},
		{
			name:           "two state value",
			stateValuesSet: []string{"myvalues[0]=HELLO,myvalues[1]=HELMFILE", "myvalues[2]=HELLO"},
			want:           map[string]interface{}{"myvalues": []interface{}{"HELLO", "HELMFILE", "HELLO"}},
		},
		{
			name:           "different key",
			stateValuesSet: []string{"myvalues[0]=HELLO,key2[0]=HELMFILE", "myvalues[1]=HELLO2"},
			want:           map[string]interface{}{"myvalues": []interface{}{"HELLO", "HELLO2"}, "key2": []interface{}{"HELMFILE"}},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			set := map[string]interface{}{}
			for i := range c.stateValuesSet {
				ops := strings.Split(c.stateValuesSet[i], ",")
				for j := range ops {
					op := strings.SplitN(ops[j], "=", 2)
					k := ParseKey(op[0])
					v := op[1]

					Set(set, k, v)
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
