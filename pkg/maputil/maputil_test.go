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

// TestMapUtil_Issue2281_ArrayMerging tests the bug reported in issue #2281
// where setting nested values in arrays replaces the entire object
func TestMapUtil_Issue2281_ArrayMerging(t *testing.T) {
	tests := []struct {
		name       string
		initialMap map[string]any
		operations []struct {
			key   []string
			value string
		}
		expected map[string]any
	}{
		{
			name: "simple array element replacement should preserve other elements",
			initialMap: map[string]any{
				"top": map[string]any{
					"array": []any{"thing1", "thing2"},
				},
			},
			operations: []struct {
				key   []string
				value string
			}{
				{key: []string{"top", "array[0]"}, value: "cmdlinething1"},
			},
			expected: map[string]any{
				"top": map[string]any{
					"array": []any{"cmdlinething1", "thing2"},
				},
			},
		},
		{
			name: "nested field in array object should merge not replace",
			initialMap: map[string]any{
				"top": map[string]any{
					"complexArray": []any{
						map[string]any{
							"thing":        "a thing",
							"anotherThing": "another thing",
						},
						map[string]any{
							"thing":        "second thing",
							"anotherThing": "a second other thing",
						},
					},
				},
			},
			operations: []struct {
				key   []string
				value string
			}{
				{key: []string{"top", "complexArray[1]", "anotherThing"}, value: "cmdline"},
			},
			expected: map[string]any{
				"top": map[string]any{
					"complexArray": []any{
						map[string]any{
							"thing":        "a thing",
							"anotherThing": "another thing",
						},
						map[string]any{
							"thing":        "second thing",
							"anotherThing": "cmdline",
						},
					},
				},
			},
		},
		{
			name: "complete issue #2281 scenario",
			initialMap: map[string]any{
				"top": map[string]any{
					"array": []any{"thing1", "thing2"},
					"complexArray": []any{
						map[string]any{
							"thing":        "a thing",
							"anotherThing": "another thing",
						},
						map[string]any{
							"thing":        "second thing",
							"anotherThing": "a second other thing",
						},
					},
				},
			},
			operations: []struct {
				key   []string
				value string
			}{
				{key: []string{"top", "array[0]"}, value: "cmdlinething1"},
				{key: []string{"top", "complexArray[1]", "anotherThing"}, value: "cmdline"},
			},
			expected: map[string]any{
				"top": map[string]any{
					"array": []any{"cmdlinething1", "thing2"},
					"complexArray": []any{
						map[string]any{
							"thing":        "a thing",
							"anotherThing": "another thing",
						},
						map[string]any{
							"thing":        "second thing",
							"anotherThing": "cmdline",
						},
					},
				},
			},
		},
		{
			name: "setting nested value in first array element should preserve fields",
			initialMap: map[string]any{
				"top": map[string]any{
					"complexArray": []any{
						map[string]any{
							"thing":        "a thing",
							"anotherThing": "another thing",
						},
					},
				},
			},
			operations: []struct {
				key   []string
				value string
			}{
				{key: []string{"top", "complexArray[0]", "anotherThing"}, value: "modified"},
			},
			expected: map[string]any{
				"top": map[string]any{
					"complexArray": []any{
						map[string]any{
							"thing":        "a thing",
							"anotherThing": "modified",
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.initialMap
			for _, op := range tt.operations {
				Set(result, op.key, op.value, false)
			}

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("Result mismatch:\nExpected: %+v\nGot:      %+v", tt.expected, result)
			}
		})
	}
}

// TestMapUtil_Issue2281_EmptyMapScenario demonstrates the actual bug
// when starting from an empty map (like --state-values-set does)
func TestMapUtil_Issue2281_EmptyMapScenario(t *testing.T) {
	// This test demonstrates what currently happens vs what should happen
	// when using --state-values-set with array indices

	// What currently happens: setting multiple values creates sparse arrays with nulls
	t.Run("current buggy behavior - demonstrates the issue", func(t *testing.T) {
		set := map[string]any{}

		// Simulating: --state-values-set top.array[0]=cmdlinething1
		Set(set, []string{"top", "array[0]"}, "cmdlinething1", false)

		// Check what we got
		topArray := set["top"].(map[string]any)["array"].([]any)

		// Currently this creates: ["cmdlinething1"]
		// which is actually correct for a single set operation
		if len(topArray) != 1 {
			t.Errorf("Expected array length 1, got %d", len(topArray))
		}
		if topArray[0] != "cmdlinething1" {
			t.Errorf("Expected array[0] to be 'cmdlinething1', got %v", topArray[0])
		}
	})

	t.Run("actual bug - setting array index 1 without index 0 creates null at 0", func(t *testing.T) {
		set := map[string]any{}

		// Simulating: --state-values-set top.complexArray[1].anotherThing=cmdline
		// WITHOUT first defining complexArray[0]
		Set(set, []string{"top", "complexArray[1]", "anotherThing"}, "cmdline", false)

		// Check what we got
		topComplexArray := set["top"].(map[string]any)["complexArray"].([]any)

		// BUG: This creates [nil, {anotherThing: "cmdline"}]
		// The issue description says array entries not referenced are being deleted or set to null
		if len(topComplexArray) != 2 {
			t.Errorf("Expected array length 2, got %d", len(topComplexArray))
		}

		// Index 0 should be nil (this is the bug!)
		if topComplexArray[0] != nil {
			t.Logf("Note: topComplexArray[0] = %v (expected nil for this test showing the bug)", topComplexArray[0])
		}

		// Index 1 should have the value
		obj1 := topComplexArray[1].(map[string]any)
		if obj1["anotherThing"] != "cmdline" {
			t.Errorf("Expected complexArray[1].anotherThing to be 'cmdline', got %v", obj1["anotherThing"])
		}
	})
}

// TestMapUtil_Issue2281_MergeArrays tests that MergeMaps should merge arrays element-by-element
func TestMapUtil_Issue2281_MergeArrays(t *testing.T) {
	t.Run("merging sparse arrays should preserve elements from base that aren't in override", func(t *testing.T) {
		// Base values from helmfile
		base := map[string]interface{}{
			"top": map[string]any{
				"array": []any{"thing1", "thing2"},
			},
		}

		// Override values from --state-values-set top.array[1]=cmdlinething1
		// This creates a sparse array with nil at index 0
		override := map[string]interface{}{
			"top": map[string]any{
				"array": []any{nil, "cmdlinething1"},
			},
		}

		result := MergeMaps(base, override)

		// Expected: array should be ["thing1", "cmdlinething1"]
		// array[0] is preserved from base (nil in override), array[1] is overridden
		resultArray := result["top"].(map[string]any)["array"].([]any)

		expected := []any{"thing1", "cmdlinething1"}
		if !reflect.DeepEqual(resultArray, expected) {
			t.Errorf("Array merge failed:\nExpected: %+v\nGot:      %+v", expected, resultArray)
		}
	})

	t.Run("complete arrays without nils should replace entirely (layer behavior)", func(t *testing.T) {
		// Base values from helmfile
		base := map[string]interface{}{
			"top": map[string]any{
				"array": []any{"thing1", "thing2", "thing3"},
			},
		}

		// Override values from environment YAML (complete array, no nils)
		// This should REPLACE the base array entirely
		override := map[string]interface{}{
			"top": map[string]any{
				"array": []any{"override1"},
			},
		}

		result := MergeMaps(base, override)

		// Expected: array should be ["override1"] - complete replacement
		resultArray := result["top"].(map[string]any)["array"].([]any)

		expected := []any{"override1"}
		if !reflect.DeepEqual(resultArray, expected) {
			t.Errorf("Array replace failed:\nExpected: %+v\nGot:      %+v", expected, resultArray)
		}
	})

	t.Run("merging complex arrays should preserve non-overridden elements and fields", func(t *testing.T) {
		// Base values from helmfile
		base := map[string]interface{}{
			"top": map[string]any{
				"complexArray": []any{
					map[string]any{
						"thing":        "a thing",
						"anotherThing": "another thing",
					},
					map[string]any{
						"thing":        "second thing",
						"anotherThing": "a second other thing",
					},
				},
			},
		}

		// Override values from --state-values-set top.complexArray[1].anotherThing=cmdline
		override := map[string]interface{}{
			"top": map[string]any{
				"complexArray": []any{
					nil,
					map[string]any{
						"anotherThing": "cmdline",
					},
				},
			},
		}

		result := MergeMaps(base, override)

		// Expected: complexArray[0] should be unchanged, complexArray[1] should have merged fields
		resultArray := result["top"].(map[string]any)["complexArray"].([]any)

		// Check array length
		if len(resultArray) != 2 {
			t.Fatalf("Expected array length 2, got %d", len(resultArray))
		}

		// Check complexArray[0] is unchanged
		elem0 := resultArray[0].(map[string]any)
		if elem0["thing"] != "a thing" || elem0["anotherThing"] != "another thing" {
			t.Errorf("complexArray[0] was modified:\nGot: %+v", elem0)
		}

		// Check complexArray[1] has merged fields
		elem1 := resultArray[1].(map[string]any)
		if elem1["thing"] != "second thing" {
			t.Errorf("complexArray[1].thing should be preserved, got %v", elem1["thing"])
		}
		if elem1["anotherThing"] != "cmdline" {
			t.Errorf("complexArray[1].anotherThing should be 'cmdline', got %v", elem1["anotherThing"])
		}
	})

	t.Run("complete issue #2281 scenario with MergeMaps - sparse arrays", func(t *testing.T) {
		// Base values from helmfile
		base := map[string]interface{}{
			"top": map[string]any{
				"array": []any{"thing1", "thing2"},
				"complexArray": []any{
					map[string]any{
						"thing":        "a thing",
						"anotherThing": "another thing",
					},
					map[string]any{
						"thing":        "second thing",
						"anotherThing": "a second other thing",
					},
				},
			},
		}

		// Override values from:
		// --state-values-set top.array[1]=cmdlinething1 (creates sparse array with nil at 0)
		// --state-values-set top.complexArray[1].anotherThing=cmdline
		override := map[string]interface{}{
			"top": map[string]any{
				"array": []any{nil, "cmdlinething1"}, // Sparse array - nil at index 0
				"complexArray": []any{
					nil,
					map[string]any{
						"anotherThing": "cmdline",
					},
				},
			},
		}

		result := MergeMaps(base, override)

		// Check array - sparse merge preserves base[0], overrides base[1]
		resultArray := result["top"].(map[string]any)["array"].([]any)
		expectedArray := []any{"thing1", "cmdlinething1"}
		if !reflect.DeepEqual(resultArray, expectedArray) {
			t.Errorf("Array merge failed:\nExpected: %+v\nGot:      %+v", expectedArray, resultArray)
		}

		// Check complexArray
		resultComplexArray := result["top"].(map[string]any)["complexArray"].([]any)
		if len(resultComplexArray) != 2 {
			t.Fatalf("Expected complexArray length 2, got %d", len(resultComplexArray))
		}

		elem0 := resultComplexArray[0].(map[string]any)
		if elem0["thing"] != "a thing" || elem0["anotherThing"] != "another thing" {
			t.Errorf("complexArray[0] was modified:\nGot: %+v", elem0)
		}

		elem1 := resultComplexArray[1].(map[string]any)
		if elem1["thing"] != "second thing" || elem1["anotherThing"] != "cmdline" {
			t.Errorf("complexArray[1] merge failed:\nExpected: {thing: second thing, anotherThing: cmdline}\nGot: %+v", elem1)
		}
	})
}

// TestMergeMaps_ArrayStrategies tests both ArrayMergeStrategySparse and ArrayMergeStrategyReplace
// This tests the fix for issue #2353 while preserving issue #2281 fix
func TestMergeMaps_ArrayStrategies(t *testing.T) {
	t.Run("ArrayMergeStrategyReplace should replace arrays entirely - fixes #2353", func(t *testing.T) {
		// This simulates layer value overriding where outer layer array should replace inner
		base := map[string]interface{}{
			"array": []any{"inner1", "inner2", "inner3"},
		}
		override := map[string]interface{}{
			"array": []any{"outer1", "outer2"},
		}

		opts := MergeOptions{ArrayStrategy: ArrayMergeStrategyReplace}
		result := MergeMaps(base, override, opts)
		resultArray := result["array"].([]any)

		// Expected: complete replacement
		expected := []any{"outer1", "outer2"}
		if !reflect.DeepEqual(resultArray, expected) {
			t.Errorf("Replace strategy should replace entirely.\nExpected: %v\nGot: %v", expected, resultArray)
		}
	})

	t.Run("ArrayMergeStrategySparse (default) should merge element-by-element - preserves #2281 fix", func(t *testing.T) {
		// This simulates --state-values-set which creates sparse arrays
		base := map[string]interface{}{
			"array": []any{"base1", "base2", "base3"},
		}
		override := map[string]interface{}{
			"array": []any{nil, "override2"}, // Has nil = sparse array from CLI
		}

		// Default strategy is Sparse
		result := MergeMaps(base, override)
		resultArray := result["array"].([]any)

		// Expected: element-by-element merge, preserving base[0] and base[2]
		expected := []any{"base1", "override2", "base3"}
		if !reflect.DeepEqual(resultArray, expected) {
			t.Errorf("Sparse strategy should merge element-by-element.\nExpected: %v\nGot: %v", expected, resultArray)
		}
	})

	t.Run("Auto-detect: complete array (no nils) replaces base entirely", func(t *testing.T) {
		// Array without nils is detected as "complete" (layer value) and replaces entirely
		base := map[string]interface{}{
			"array": []any{"base1", "base2", "base3"},
		}
		override := map[string]interface{}{
			"array": []any{"override1"}, // Single element, no nils
		}

		// Default strategy uses auto-detection: no nils = complete array = replace
		result := MergeMaps(base, override)
		resultArray := result["array"].([]any)

		// Expected: complete replacement (auto-detected as complete array)
		expected := []any{"override1"}
		if !reflect.DeepEqual(resultArray, expected) {
			t.Errorf("Complete array (no nils) should replace base entirely.\nExpected: %v\nGot: %v", expected, resultArray)
		}
	})

	t.Run("Auto-detect: sparse array (with nils) preserves base at nil indices", func(t *testing.T) {
		// Array with nils is detected as "sparse" (CLI value) and merges element-by-element
		base := map[string]interface{}{
			"array": []any{"base1", "base2", "base3"},
		}
		override := map[string]interface{}{
			"array": []any{nil, nil, "override3"}, // Has nils at indices 0, 1
		}

		// Default strategy uses auto-detection: has nils = sparse array = merge
		result := MergeMaps(base, override)
		resultArray := result["array"].([]any)

		// Expected: element-by-element merge, preserving base[0] and base[1]
		expected := []any{"base1", "base2", "override3"}
		if !reflect.DeepEqual(resultArray, expected) {
			t.Errorf("Sparse array (with nils) should preserve base at nil indices.\nExpected: %v\nGot: %v", expected, resultArray)
		}
	})

	t.Run("nested maps in sparse arrays should merge recursively", func(t *testing.T) {
		base := map[string]interface{}{
			"complexArray": []any{
				map[string]any{"field1": "a", "field2": "b"},
				map[string]any{"field1": "c", "field2": "d"},
			},
		}
		override := map[string]interface{}{
			"complexArray": []any{
				nil,                                  // Skip index 0
				map[string]any{"field2": "override"}, // Only override field2 at index 1
			},
		}

		result := MergeMaps(base, override)
		resultArray := result["complexArray"].([]any)

		// Index 0 should be unchanged (nil skipped)
		elem0 := resultArray[0].(map[string]any)
		if elem0["field1"] != "a" || elem0["field2"] != "b" {
			t.Errorf("Index 0 should be unchanged: %v", elem0)
		}

		// Index 1 should have merged fields (field1 preserved, field2 overridden)
		elem1 := resultArray[1].(map[string]any)
		if elem1["field1"] != "c" || elem1["field2"] != "override" {
			t.Errorf("Index 1 should have merged fields.\nExpected: {field1: c, field2: override}\nGot: %v", elem1)
		}
	})

	t.Run("Replace strategy: array of maps replaced entirely - layer scenario #2353", func(t *testing.T) {
		// This is the key scenario from #2353: outer layer defining complete array of objects
		base := map[string]interface{}{
			"releases": []any{
				map[string]any{"name": "inner-release-1", "chart": "inner-chart-1"},
				map[string]any{"name": "inner-release-2", "chart": "inner-chart-2"},
				map[string]any{"name": "inner-release-3", "chart": "inner-chart-3"},
			},
		}
		override := map[string]interface{}{
			"releases": []any{
				map[string]any{"name": "outer-release-1", "chart": "outer-chart-1"},
				map[string]any{"name": "outer-release-2", "chart": "outer-chart-2"},
			},
		}

		opts := MergeOptions{ArrayStrategy: ArrayMergeStrategyReplace}
		result := MergeMaps(base, override, opts)
		resultArray := result["releases"].([]any)

		// Expected: complete replacement - only 2 releases from outer layer
		if len(resultArray) != 2 {
			t.Fatalf("Expected 2 releases (complete replacement), got %d", len(resultArray))
		}

		release1 := resultArray[0].(map[string]any)
		if release1["name"] != "outer-release-1" {
			t.Errorf("Release 1 should be from outer layer, got: %v", release1)
		}

		release2 := resultArray[1].(map[string]any)
		if release2["name"] != "outer-release-2" {
			t.Errorf("Release 2 should be from outer layer, got: %v", release2)
		}
	})

	t.Run("Replace strategy: empty override array replaces base", func(t *testing.T) {
		base := map[string]interface{}{
			"array": []any{"a", "b", "c"},
		}
		override := map[string]interface{}{
			"array": []any{}, // Empty array
		}

		opts := MergeOptions{ArrayStrategy: ArrayMergeStrategyReplace}
		result := MergeMaps(base, override, opts)
		resultArray := result["array"].([]any)

		// Expected: empty array (complete replacement)
		if len(resultArray) != 0 {
			t.Errorf("Replace strategy with empty array should replace base. Got: %v", resultArray)
		}
	})

	t.Run("Auto-detect: empty override replaces base (no nils means complete)", func(t *testing.T) {
		base := map[string]interface{}{
			"array": []any{"a", "b", "c"},
		}
		override := map[string]interface{}{
			"array": []any{}, // Empty array - has no nils, detected as complete
		}

		// Default strategy uses auto-detection: empty array has no nils = complete = replace
		result := MergeMaps(base, override)
		resultArray := result["array"].([]any)

		// Empty array with no nils is detected as "complete" and replaces entirely
		// This is consistent: layer specifying array: [] means "I want an empty array"
		if len(resultArray) != 0 {
			t.Errorf("Empty complete array should replace base with empty array. Got: %v", resultArray)
		}
	})

	t.Run("strategies propagate to nested maps", func(t *testing.T) {
		base := map[string]interface{}{
			"outer": map[string]any{
				"inner": []any{"a", "b", "c"},
			},
		}

		// With Replace strategy - explicit replacement
		overrideComplete := map[string]interface{}{
			"outer": map[string]any{
				"inner": []any{"x", "y"}, // Complete array (no nils)
			},
		}

		optsReplace := MergeOptions{ArrayStrategy: ArrayMergeStrategyReplace}
		resultReplace := MergeMaps(base, overrideComplete, optsReplace)
		resultArrayReplace := resultReplace["outer"].(map[string]any)["inner"].([]any)

		expectedReplace := []any{"x", "y"}
		if !reflect.DeepEqual(resultArrayReplace, expectedReplace) {
			t.Errorf("Replace strategy should propagate to nested arrays.\nExpected: %v\nGot: %v", expectedReplace, resultArrayReplace)
		}

		// With auto-detection and sparse array (has nils)
		overrideSparse := map[string]interface{}{
			"outer": map[string]any{
				"inner": []any{"x", "y", nil}, // Sparse array - has nil at index 2
			},
		}

		resultSparse := MergeMaps(base, overrideSparse)
		resultArraySparse := resultSparse["outer"].(map[string]any)["inner"].([]any)

		// nil at index 2 preserves base[2]="c"
		expectedSparse := []any{"x", "y", "c"}
		if !reflect.DeepEqual(resultArraySparse, expectedSparse) {
			t.Errorf("Sparse strategy should propagate to nested arrays.\nExpected: %v\nGot: %v", expectedSparse, resultArraySparse)
		}
	})

	t.Run("ArrayMergeStrategyMerge always merges element-by-element (CLI index 0 case)", func(t *testing.T) {
		// This tests the CLI scenario: --state-values-set array[0]=value
		// Creates array ["value"] with NO nils, but should still merge
		base := map[string]interface{}{
			"array": []any{"base0", "base1", "base2"},
		}
		override := map[string]interface{}{
			"array": []any{"override0"}, // Single element, no nils - from CLI index 0
		}

		opts := MergeOptions{ArrayStrategy: ArrayMergeStrategyMerge}
		result := MergeMaps(base, override, opts)
		resultArray := result["array"].([]any)

		// With Merge strategy, always merge element-by-element regardless of nils
		expected := []any{"override0", "base1", "base2"}
		if !reflect.DeepEqual(resultArray, expected) {
			t.Errorf("Merge strategy should always merge element-by-element.\nExpected: %v\nGot: %v", expected, resultArray)
		}
	})
}
