package environment

import (
	"dario.cat/mergo"

	"github.com/helmfile/helmfile/pkg/maputil"
	"github.com/helmfile/helmfile/pkg/yaml"
)

type Environment struct {
	Name        string
	KubeContext string
	Values      map[string]any
	Defaults    map[string]any
}

var EmptyEnvironment Environment

// New return Environment with default name and values
func New(name string) *Environment {
	return &Environment{
		Name:        name,
		KubeContext: "",
		Values:      map[string]any{},
		Defaults:    map[string]any{},
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

	return Environment{
		Name:        e.Name,
		KubeContext: e.KubeContext,
		Values:      values,
		Defaults:    defaults,
	}
}

func (e *Environment) Merge(other *Environment) (*Environment, error) {
	if e == nil {
		if other != nil {
			copy := other.DeepCopy()
			return &copy, nil
		}
		return nil, nil
	}
	copy := e.DeepCopy()
	if other != nil {
		if err := mergo.Merge(&copy, other, mergo.WithOverride); err != nil {
			return nil, err
		}
	}
	return &copy, nil
}

func (e *Environment) GetMergedValues() (map[string]any, error) {
	vals := map[string]any{}

	if err := mergo.Merge(&vals, e.Defaults, mergo.WithOverride); err != nil {
		return nil, err
	}

	processor := yaml.NewAppendProcessor()
	if err := processor.MergeWithAppend(vals, e.Values); err != nil {
		return nil, err
	}

	vals, err := maputil.CastKeysToStrings(vals)
	if err != nil {
		return nil, err
	}

	return vals, nil
}
