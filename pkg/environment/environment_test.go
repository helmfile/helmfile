package environment

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMerge_Issues(t *testing.T) {
	tests := []struct {
		name     string
		dst      *Environment
		src      *Environment
		expected map[string]any
	}{
		{
			name: "OverwriteNilValue_Issue1150",
			dst: &Environment{
				Name: "dst",
				Values: map[string]any{
					"components": map[string]any{
						"etcd-operator": nil,
					},
				},
				Defaults: nil,
			},
			src: &Environment{
				Name: "src",
				Values: map[string]any{
					"components": map[string]any{
						"etcd-operator": map[string]any{
							"version": "0.10.3",
						},
					},
				},
				Defaults: nil,
			},
			expected: map[string]any{
				"components": map[string]any{
					"etcd-operator": map[string]any{
						"version": "0.10.3",
					},
				},
			},
		},
		{
			name: "OverwriteWithNilValue_Issue1154",
			dst: &Environment{
				Name: "dst",
				Values: map[string]any{
					"components": map[string]any{
						"etcd-operator": map[string]any{
							"version": "0.10.0",
						},
					},
				},
				Defaults: nil,
			},
			src: &Environment{
				Name: "src",
				Values: map[string]any{
					"components": map[string]any{
						"etcd-operator": map[string]any{
							"version": "0.10.3",
						},
						"prometheus": nil,
					},
				},
				Defaults: nil,
			},
			expected: map[string]any{
				"components": map[string]any{
					"etcd-operator": map[string]any{
						"version": "0.10.3",
					},
					"prometheus": nil,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			merged, err := tt.dst.Merge(tt.src)
			require.NoError(t, err)

			actual := merged.Values
			assert.Empty(t, cmp.Diff(tt.expected, actual), "unexpected diff")
		})
	}
}

func TestNew(t *testing.T) {
	envName := "test"
	env := New(envName)

	require.Equal(t, envName, env.Name, "environment name should be %s, but got %s", envName, env.Name)
}

func TestEnvironment_DeepCopy(t *testing.T) {
	env := &Environment{
		Name: "test",
		Values: map[string]any{
			"foo": "bar",
		},
		Defaults: map[string]any{
			"baz": "qux",
		},
	}

	copy := env.DeepCopy()

	assert.Equal(t, env.Name, copy.Name)
	assert.Equal(t, env.Values, copy.Values)
	assert.Equal(t, env.Defaults, copy.Defaults)

	copy.Values["foo"] = "modified"
	assert.NotEqual(t, env.Values["foo"], copy.Values["foo"])
}

func TestEnvironment_GetMergedValues(t *testing.T) {
	env := &Environment{
		Name: "test",
		Values: map[string]any{
			"foo": "bar",
		},
		Defaults: map[string]any{
			"baz": "qux",
		},
	}

	mergedValues, err := env.GetMergedValues()
	require.NoError(t, err)

	expected := map[string]any{
		"foo": "bar",
		"baz": "qux",
	}

	assert.Equal(t, expected, mergedValues)
}
