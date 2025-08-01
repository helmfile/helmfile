package yaml

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppendProcessor_MergeWithAppend(t *testing.T) {
	processor := NewAppendProcessor()

	tests := []struct {
		name     string
		dest     map[string]any
		src      map[string]any
		expected map[string]any
	}{
		{
			name: "append to existing slice",
			dest: map[string]any{
				"values": []any{
					map[string]any{
						"key": "a",
					},
					map[string]any{
						"key": "b",
					},
				},
			},
			src: map[string]any{
				"values+": []any{
					map[string]any{
						"key": "c",
					},
				},
			},
			expected: map[string]any{
				"values": []any{
					map[string]any{
						"key": "a",
					},
					map[string]any{
						"key": "b",
					},
					map[string]any{
						"key": "c",
					},
				},
			},
		},
		{
			name: "nested append",
			dest: map[string]any{
				"config": map[string]any{
					"values": []any{
						map[string]any{
							"key": "a",
						},
					},
				},
			},
			src: map[string]any{
				"config": map[string]any{
					"values+": []any{
						map[string]any{
							"key": "b",
						},
					},
				},
			},
			expected: map[string]any{
				"config": map[string]any{
					"values": []any{
						map[string]any{
							"key": "a",
						},
						map[string]any{
							"key": "b",
						},
					},
				},
			},
		},
		{
			name: "create new slice if not exists",
			dest: map[string]any{},
			src: map[string]any{
				"values+": []any{
					map[string]any{
						"key": "a",
					},
				},
			},
			expected: map[string]any{
				"values": []any{
					map[string]any{
						"key": "a",
					},
				},
			},
		},
		{
			name: "nested append with non-existent parent",
			dest: map[string]any{},
			src: map[string]any{
				"kube-state-metrics": map[string]any{
					"prometheus": map[string]any{
						"monitor": map[string]any{
							"metricRelabelings+": []any{
								map[string]any{
									"action": "labeldrop",
									"regex":  "info_.*",
								},
							},
						},
					},
				},
			},
			expected: map[string]any{
				"kube-state-metrics": map[string]any{
					"prometheus": map[string]any{
						"monitor": map[string]any{
							"metricRelabelings": []any{
								map[string]any{
									"action": "labeldrop",
									"regex":  "info_.*",
								},
							},
						},
					},
				},
			},
		},
		{
			name: "overwrite non-slice with append",
			dest: map[string]any{
				"values": "not a slice",
			},
			src: map[string]any{
				"values+": []any{
					map[string]any{
						"key": "a",
					},
				},
			},
			expected: map[string]any{
				"values": []any{
					map[string]any{
						"key": "a",
					},
				},
			},
		},
		{
			name: "type collision - []string vs []any",
			dest: map[string]any{
				"values": []string{"a", "b"},
			},
			src: map[string]any{
				"values+": []any{"c", "d"},
			},
			expected: map[string]any{
				"values": []any{"c", "d"},
			},
		},
		{
			name: "type collision - []int vs []any",
			dest: map[string]any{
				"values": []int{1, 2},
			},
			src: map[string]any{
				"values+": []any{3, 4},
			},
			expected: map[string]any{
				"values": []any{3, 4},
			},
		},
		{
			name: "append on map - should overwrite",
			dest: map[string]any{
				"config": map[string]any{"key": "value"},
			},
			src: map[string]any{
				"config+": []any{"new", "values"},
			},
			expected: map[string]any{
				"config": []any{"new", "values"},
			},
		},
		{
			name: "append on scalar - should overwrite",
			dest: map[string]any{
				"version": "1.0.0",
			},
			src: map[string]any{
				"version+": []any{"2.0.0", "3.0.0"},
			},
			expected: map[string]any{
				"version": []any{"2.0.0", "3.0.0"},
			},
		},
		{
			name: "nil slice in destination",
			dest: map[string]any{
				"values": nil,
			},
			src: map[string]any{
				"values+": []any{"a", "b"},
			},
			expected: map[string]any{
				"values": []any{"a", "b"},
			},
		},
		{
			name: "empty slice in destination",
			dest: map[string]any{
				"values": []any{},
			},
			src: map[string]any{
				"values+": []any{"a", "b"},
			},
			expected: map[string]any{
				"values": []any{"a", "b"},
			},
		},
		{
			name: "nil slice in source",
			dest: map[string]any{
				"values": []any{"existing"},
			},
			src: map[string]any{
				"values+": nil,
			},
			expected: map[string]any{
				"values": nil,
			},
		},
		{
			name: "mixed types in slices",
			dest: map[string]any{
				"data": []any{"string", 42, true},
			},
			src: map[string]any{
				"data+": []any{"new", 100, false},
			},
			expected: map[string]any{
				"data": []any{"string", 42, true, "new", 100, false},
			},
		},
		{
			name: "multiple append keys",
			dest: map[string]any{
				"list1": []any{"a"},
				"list2": []any{"x"},
			},
			src: map[string]any{
				"list1+": []any{"b"},
				"list2+": []any{"y"},
			},
			expected: map[string]any{
				"list1": []any{"a", "b"},
				"list2": []any{"x", "y"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srcCopy := make(map[string]any)
			for k, v := range tt.src {
				srcCopy[k] = v
			}

			err := processor.MergeWithAppend(tt.dest, tt.src)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, tt.dest)

			assert.Equal(t, srcCopy, tt.src, "source map should not be mutated")
		})
	}
}

func TestAppendProcessor_EdgeCases(t *testing.T) {
	processor := NewAppendProcessor()

	t.Run("empty maps", func(t *testing.T) {
		dest := make(map[string]any)
		src := make(map[string]any)

		err := processor.MergeWithAppend(dest, src)
		require.NoError(t, err)
		assert.Empty(t, dest)
	})

	t.Run("nil maps", func(t *testing.T) {
		var dest map[string]any
		var src map[string]any

		err := processor.MergeWithAppend(dest, src)
		require.NoError(t, err)
		assert.Nil(t, dest)
	})

	t.Run("deep nested with append", func(t *testing.T) {
		dest := map[string]any{
			"level1": map[string]any{
				"level2": map[string]any{
					"level3": map[string]any{
						"values": []any{"deep"},
					},
				},
			},
		}
		src := map[string]any{
			"level1": map[string]any{
				"level2": map[string]any{
					"level3": map[string]any{
						"values+": []any{"nested"},
					},
				},
			},
		}
		expected := map[string]any{
			"level1": map[string]any{
				"level2": map[string]any{
					"level3": map[string]any{
						"values": []any{"deep", "nested"},
					},
				},
			},
		}

		err := processor.MergeWithAppend(dest, src)
		require.NoError(t, err)
		assert.Equal(t, expected, dest)
	})

	t.Run("complex nested structure", func(t *testing.T) {
		dest := map[string]any{
			"config": map[string]any{
				"services": []any{
					map[string]any{"name": "service1"},
				},
				"settings": map[string]any{
					"timeout": 30,
				},
			},
		}
		src := map[string]any{
			"config": map[string]any{
				"services+": []any{
					map[string]any{"name": "service2"},
				},
				"settings": map[string]any{
					"retries": 3,
				},
			},
		}
		expected := map[string]any{
			"config": map[string]any{
				"services": []any{
					map[string]any{"name": "service1"},
					map[string]any{"name": "service2"},
				},
				"settings": map[string]any{
					"timeout": 30,
					"retries": 3,
				},
			},
		}

		err := processor.MergeWithAppend(dest, src)
		require.NoError(t, err)
		assert.Equal(t, expected, dest)
	})
}

func TestAppendProcessor_TypeConversions(t *testing.T) {
	processor := NewAppendProcessor()

	t.Run("map[any]any to map[string]any conversion", func(t *testing.T) {
		dest := map[string]any{
			"values": []any{"a"},
		}
		src := map[any]any{
			"values+": []any{"b"},
		}

		srcConverted := make(map[string]any)
		for k, v := range src {
			if ks, ok := k.(string); ok {
				srcConverted[ks] = v
			}
		}

		err := processor.MergeWithAppend(dest, srcConverted)
		require.NoError(t, err)
		assert.Equal(t, []any{"a", "b"}, dest["values"])
	})

	t.Run("mixed key types", func(t *testing.T) {
		dest := map[string]any{
			"values": []any{"a"},
		}
		src := map[any]any{
			"values+": []any{"b"},
			"other":   "value",
			42:        "number_key",
		}

		srcConverted := make(map[string]any)
		for k, v := range src {
			if ks, ok := k.(string); ok {
				srcConverted[ks] = v
			}
		}

		err := processor.MergeWithAppend(dest, srcConverted)
		require.NoError(t, err)
		assert.Equal(t, []any{"a", "b"}, dest["values"])
		assert.Equal(t, "value", dest["other"])
		assert.NotContains(t, dest, "42")
	})
}

func TestAppendProcessor_PropertyBased(t *testing.T) {
	processor := NewAppendProcessor()

	t.Run("idempotent regular merge", func(t *testing.T) {
		dest := map[string]any{
			"key1": "value1",
			"key2": []any{"a", "b"},
		}
		src := map[string]any{
			"key1": "value1",
			"key3": "value3",
		}

		err := processor.MergeWithAppend(dest, src)
		require.NoError(t, err)

		err = processor.MergeWithAppend(dest, src)
		require.NoError(t, err)

		expected := map[string]any{
			"key1": "value1",
			"key2": []any{"a", "b"},
			"key3": "value3",
		}
		assert.Equal(t, expected, dest)
	})

	t.Run("append is not idempotent", func(t *testing.T) {
		dest := map[string]any{
			"values": []any{"a"},
		}
		src := map[string]any{
			"values+": []any{"b"},
		}

		err := processor.MergeWithAppend(dest, src)
		require.NoError(t, err)
		assert.Equal(t, []any{"a", "b"}, dest["values"])

		err = processor.MergeWithAppend(dest, src)
		require.NoError(t, err)
		assert.Equal(t, []any{"a", "b", "b"}, dest["values"])
	})

	t.Run("merge is not commutative", func(t *testing.T) {
		map1 := map[string]any{"a": 1, "b": 2}
		map2 := map[string]any{"b": 3, "c": 4}

		result1 := make(map[string]any)
		for k, v := range map1 {
			result1[k] = v
		}
		err := processor.MergeWithAppend(result1, map2)
		require.NoError(t, err)

		result2 := make(map[string]any)
		for k, v := range map2 {
			result2[k] = v
		}
		err = processor.MergeWithAppend(result2, map1)
		require.NoError(t, err)

		assert.NotEqual(t, result1, result2)

		expected1 := map[string]any{"a": 1, "b": 3, "c": 4}
		expected2 := map[string]any{"a": 1, "b": 2, "c": 4}
		assert.Equal(t, expected1, result1)
		assert.Equal(t, expected2, result2)
	})
}

func TestIsAppendKey(t *testing.T) {
	tests := []struct {
		key      string
		expected bool
	}{
		{"key+", true},
		{"key", false},
		{"key++", true},
		{"+key", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			result := IsAppendKey(tt.key)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetBaseKey(t *testing.T) {
	tests := []struct {
		key      string
		expected string
	}{
		{"key+", "key"},
		{"key", "key"},
		{"key++", "key+"},
		{"+key", "+key"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			result := GetBaseKey(tt.key)
			assert.Equal(t, tt.expected, result)
		})
	}
}
