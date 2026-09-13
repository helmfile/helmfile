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

func TestEnvironment_GetMergedValues_Issue2353_LayerArrayReplace(t *testing.T) {
	env := &Environment{
		Name: "test",
		Defaults: map[string]any{
			"top": map[string]any{
				"array": []any{"default1", "default2", "default3"},
			},
		},
		Values: map[string]any{
			"top": map[string]any{
				"array": []any{"override1", "override2"},
			},
		},
	}

	mergedValues, err := env.GetMergedValues()
	require.NoError(t, err)

	resultArray := mergedValues["top"].(map[string]any)["array"].([]any)
	expected := []any{"override1", "override2"}
	assert.Equal(t, expected, resultArray, "Layer arrays should replace defaults entirely")
}

func TestEnvironment_GetMergedValues_Issue2281_SparseArrayMerge(t *testing.T) {
	env := &Environment{
		Name: "test",
		Defaults: map[string]any{
			"top": map[string]any{
				"array": []any{"thing1", "thing2"},
				"complexArray": []any{
					map[string]any{"thing": "a thing", "anotherThing": "another thing"},
					map[string]any{"thing": "second thing", "anotherThing": "a second other thing"},
				},
			},
		},
		Values: map[string]any{
			"top": map[string]any{
				"array":        []any{nil, "cmdlinething1"},
				"complexArray": []any{nil, map[string]any{"anotherThing": "cmdline"}},
			},
		},
	}

	mergedValues, err := env.GetMergedValues()
	require.NoError(t, err)

	top := mergedValues["top"].(map[string]any)

	resultArray := top["array"].([]any)
	expectedArray := []any{"thing1", "cmdlinething1"}
	assert.Equal(t, expectedArray, resultArray, "CLI sparse arrays should merge element-by-element")

	resultComplex := top["complexArray"].([]any)
	assert.Len(t, resultComplex, 2)

	elem0 := resultComplex[0].(map[string]any)
	assert.Equal(t, "a thing", elem0["thing"])
	assert.Equal(t, "another thing", elem0["anotherThing"])

	elem1 := resultComplex[1].(map[string]any)
	assert.Equal(t, "second thing", elem1["thing"])
	assert.Equal(t, "cmdline", elem1["anotherThing"])
}

func TestEnvironment_GetMergedValues_Issue2527_ValuesOverrideDefaults(t *testing.T) {
	// Regression test for https://github.com/helmfile/helmfile/issues/2527:
	// A boolean false in Values must not be overridden by a true in Defaults.
	env := &Environment{
		Name: "test",
		Defaults: map[string]any{
			"helmDefaults": map[string]any{
				"atomic":  true,
				"wait":    true,
				"timeout": 300,
			},
		},
		Values: map[string]any{
			"appName": "my-app",
			"helmDefaults": map[string]any{
				"atomic":  false, // explicit false override must survive
				"wait":    true,
				"timeout": 300,
			},
		},
		CLIOverrides: map[string]any{},
	}

	mergedValues, err := env.GetMergedValues()
	require.NoError(t, err)

	hd := mergedValues["helmDefaults"].(map[string]any)
	assert.Equal(t, false, hd["atomic"], "Values false should override Defaults true for atomic")
	assert.Equal(t, true, hd["wait"])
	assert.Equal(t, 300, hd["timeout"])
}

// TestEnvironment_DeepCopy_Issue973_SecretSpecialChars verifies that DeepCopy
// preserves all keys when values contain special characters (colons, quotes,
// braces, etc.) typical of SOPS/KMS-encrypted secrets.
// Regression test for https://github.com/helmfile/helmfile/issues/973.
func TestEnvironment_DeepCopy_Issue973_SecretSpecialChars(t *testing.T) {
	env := &Environment{
		Name: "myEnv",
		Values: map[string]any{
			"masked1": map[string]any{
				"masked2": "xxxxxxxxxx",
				"masked3": "xxxxxxxxxx",
			},
			"masked85": map[string]any{
				"masked86": map[string]any{"masked87": "xxxxxxxxxx"},
				"masked88": map[string]any{"masked89": "~masked:ab#7i7!;{'\"."},
			},
			// myValue must survive the deep copy alongside the secret values.
			"myValue": "valueOfMyValue",
		},
		Defaults: map[string]any{
			"aDependentValue": "{{.Values.myValue}}",
		},
	}

	copied := env.DeepCopy()

	// myValue must be preserved.
	assert.Equal(t, "valueOfMyValue", copied.Values["myValue"], "myValue must survive DeepCopy")
	// The secret value must be preserved exactly, not mangled.
	masked85 := copied.Values["masked85"].(map[string]any)
	masked88 := masked85["masked88"].(map[string]any)
	assert.Equal(t, "~masked:ab#7i7!;{'\".", masked88["masked89"],
		"secret value with special chars must survive DeepCopy unchanged")
	// Defaults must be preserved too.
	assert.Equal(t, "{{.Values.myValue}}", copied.Defaults["aDependentValue"])

	// Mutating the copy must not affect the original.
	copied.Values["myValue"] = "changed"
	assert.Equal(t, "valueOfMyValue", env.Values["myValue"], "DeepCopy must be independent")
}

// TestEnvironment_Merge_Issue973_NoDataLoss verifies that Merge (which calls
// DeepCopy internally) does not drop any keys when secret values with special
// characters are present.
func TestEnvironment_Merge_Issue973_NoDataLoss(t *testing.T) {
	base := &Environment{
		Name:   "myEnv",
		Values: map[string]any{},
	}

	loaded := &Environment{
		Name: "myEnv",
		Values: map[string]any{
			"masked1":  map[string]any{"masked2": "xxxxxxxxxx"},
			"masked89": "~masked:ab#7i7!;{'\".",
			"myValue":  "valueOfMyValue",
		},
	}

	merged, err := base.Merge(loaded)
	require.NoError(t, err)

	assert.Equal(t, "valueOfMyValue", merged.Values["myValue"],
		"myValue must survive Merge with secret values present")
	assert.Equal(t, "~masked:ab#7i7!;{'\".", merged.Values["masked89"],
		"secret value must survive Merge unchanged")
}
