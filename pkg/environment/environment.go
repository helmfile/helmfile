package environment

import (
	"github.com/helmfile/helmfile/pkg/maputil"
)

type Environment struct {
	Name         string
	KubeContext  string
	Values       map[string]any
	Defaults     map[string]any
	CLIOverrides map[string]any // CLI --state-values-set values, merged element-by-element
}

var EmptyEnvironment = Environment{
	Name:         "",
	KubeContext:  "",
	Values:       map[string]any{},
	Defaults:     map[string]any{},
	CLIOverrides: map[string]any{},
}

// New return Environment with default name and values
func New(name string) *Environment {
	return &Environment{
		Name:         name,
		KubeContext:  "",
		Values:       map[string]any{},
		Defaults:     map[string]any{},
		CLIOverrides: map[string]any{},
	}
}

// DeepCopy returns a deep copy of the environment.
// It uses maputil.DeepCopyMap rather than a YAML marshal/unmarshal round-trip
// so that secret values containing special characters (colons, quotes, braces,
// etc.) are never mangled or lost. This fixes issue #973 where large SOPS-encrypted
// secret files caused environment values like "myValue" to silently disappear.
func (e Environment) DeepCopy() Environment {
	return Environment{
		Name:         e.Name,
		KubeContext:  e.KubeContext,
		Values:       maputil.DeepCopyMap(e.Values),
		Defaults:     maputil.DeepCopyMap(e.Defaults),
		CLIOverrides: maputil.DeepCopyMap(e.CLIOverrides),
	}
}

func (e *Environment) Merge(other *Environment) (*Environment, error) {
	if e == nil {
		if other != nil {
			copy := other.DeepCopy()
			// Don't merge CLIOverrides into Values here - keep them separate.
			// The proper merge happens in GetMergedValues() with correct layering.
			return &copy, nil
		}
		return nil, nil
	}
	copy := e.DeepCopy()
	if other != nil {
		// Merge scalar fields
		if other.Name != "" {
			copy.Name = other.Name
		}
		if other.KubeContext != "" {
			copy.KubeContext = other.KubeContext
		}
		// Merge Values - layer values replace arrays (using default Sparse strategy)
		copy.Values = maputil.MergeMaps(copy.Values, other.Values)
		copy.Defaults = maputil.MergeMaps(copy.Defaults, other.Defaults)
		// Merge CLIOverrides using element-by-element array merging
		copy.CLIOverrides = maputil.MergeMaps(copy.CLIOverrides, other.CLIOverrides,
			maputil.MergeOptions{ArrayStrategy: maputil.ArrayMergeStrategyMerge})
		// Don't merge CLIOverrides into Values here - keep them separate.
		// The proper merge happens in GetMergedValues() with correct layering.
	}
	return &copy, nil
}

func (e *Environment) GetMergedValues() (map[string]any, error) {
	vals := map[string]any{}
	vals = maputil.MergeMaps(vals, e.Defaults)
	vals = maputil.MergeMaps(vals, e.Values)
	// CLI overrides are merged last using element-by-element array merging.
	// This ensures --state-values-set array[0]=x only changes that index.
	vals = maputil.MergeMaps(vals, e.CLIOverrides,
		maputil.MergeOptions{ArrayStrategy: maputil.ArrayMergeStrategyMerge})

	vals, err := maputil.CastKeysToStrings(vals)
	if err != nil {
		return nil, err
	}

	return vals, nil
}
