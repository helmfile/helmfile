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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := processor.MergeWithAppend(tt.dest, tt.src)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, tt.dest)
		})
	}
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
