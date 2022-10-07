package util

import "testing"

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
