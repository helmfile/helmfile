package app

import (
	"gopkg.in/yaml.v3"

	"github.com/helmfile/helmfile/pkg/maputil"
	"github.com/helmfile/helmfile/pkg/state"
)

type LoadOpts struct {
	Selectors   []string
	Environment state.SubhelmfileEnvironmentSpec

	RetainValuesFiles bool

	// CalleePath is the absolute path to the file being loaded
	CalleePath string

	Reverse bool

	Filter bool
}

func (o LoadOpts) DeepCopy() LoadOpts {
	bytes, err := maputil.YamlMarshal(o)
	if err != nil {
		panic(err)
	}

	new := LoadOpts{}
	if err := yaml.Unmarshal(bytes, &new); err != nil {
		panic(err)
	}

	return new
}
