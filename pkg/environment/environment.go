package environment

import (
	"github.com/imdario/mergo"
	"gopkg.in/yaml.v3"

	"github.com/helmfile/helmfile/pkg/maputil"
)

type Environment struct {
	Name     string
	Values   map[string]interface{}
	Defaults map[string]interface{}
}

var EmptyEnvironment Environment

func (e Environment) DeepCopy() Environment {
	valuesBytes, err := maputil.YamlMarshal(e.Values)
	if err != nil {
		panic(err)
	}
	var values map[string]interface{}
	if err := yaml.Unmarshal(valuesBytes, &values); err != nil {
		panic(err)
	}
	values, err = maputil.CastKeysToStrings(values)
	if err != nil {
		panic(err)
	}

	defaultsBytes, err := maputil.YamlMarshal(e.Defaults)
	if err != nil {
		panic(err)
	}
	var defaults map[string]interface{}
	if err := yaml.Unmarshal(defaultsBytes, &defaults); err != nil {
		panic(err)
	}
	defaults, err = maputil.CastKeysToStrings(defaults)
	if err != nil {
		panic(err)
	}

	return Environment{
		Name:     e.Name,
		Values:   values,
		Defaults: defaults,
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
		if err := mergo.Merge(&copy, other, mergo.WithOverride, mergo.WithOverwriteWithEmptyValue); err != nil {
			return nil, err
		}
	}
	return &copy, nil
}

func (e *Environment) GetMergedValues() (map[string]interface{}, error) {
	vals := map[string]interface{}{}

	if err := mergo.Merge(&vals, e.Defaults, mergo.WithOverride, mergo.WithOverwriteWithEmptyValue); err != nil {
		return nil, err
	}

	if err := mergo.Merge(&vals, e.Values, mergo.WithOverride, mergo.WithOverwriteWithEmptyValue); err != nil {
		return nil, err
	}

	vals, err := maputil.CastKeysToStrings(vals)
	if err != nil {
		return nil, err
	}

	return vals, nil
}
