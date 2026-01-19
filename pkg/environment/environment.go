package environment

import (
	"github.com/helmfile/helmfile/pkg/maputil"
	"github.com/helmfile/helmfile/pkg/yaml"
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

func (e Environment) DeepCopy() Environment {
	valuesBytes, err := yaml.Marshal(e.Values)
	if err != nil {
		panic(err)
	}
	var values map[string]any
	if err := yaml.Unmarshal(valuesBytes, &values); err != nil {
		panic(err)
	}
	values, err = maputil.CastKeysToStrings(values)
	if err != nil {
		panic(err)
	}

	defaultsBytes, err := yaml.Marshal(e.Defaults)
	if err != nil {
		panic(err)
	}
	var defaults map[string]any
	if err := yaml.Unmarshal(defaultsBytes, &defaults); err != nil {
		panic(err)
	}
	defaults, err = maputil.CastKeysToStrings(defaults)
	if err != nil {
		panic(err)
	}

	cliOverridesBytes, err := yaml.Marshal(e.CLIOverrides)
	if err != nil {
		panic(err)
	}
	var cliOverrides map[string]any
	if err := yaml.Unmarshal(cliOverridesBytes, &cliOverrides); err != nil {
		panic(err)
	}
	cliOverrides, err = maputil.CastKeysToStrings(cliOverrides)
	if err != nil {
		panic(err)
	}

	return Environment{
		Name:         e.Name,
		KubeContext:  e.KubeContext,
		Values:       values,
		Defaults:     defaults,
		CLIOverrides: cliOverrides,
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
