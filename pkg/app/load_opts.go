package app

import (
	"github.com/helmfile/helmfile/pkg/state"
	"github.com/helmfile/helmfile/pkg/yaml"
)

type LoadOpts struct {
	Selectors   []string
	Environment state.SubhelmfileEnvironmentSpec

	RetainValuesFiles bool

	// CalleePath is the absolute path to the file being loaded
	CalleePath string

	Reverse bool

	Filter bool

	// Inherited carries parent-helmfile config that this sub-helmfile opts into
	// via `inherits:`. See state.InheritedConfig and state.MergeInherited.
	Inherited *state.InheritedConfig

	// ParentRepoNames is the list of repository names declared in the parent
	// helmfile. It is used by the "did you mean inherits: [repositories]?"
	// footgun warning (state.WarnUninheritedRepos).
	ParentRepoNames []string `yaml:"parentRepoNames,omitempty"`
}

func (o LoadOpts) DeepCopy() LoadOpts {
	bytes, err := yaml.Marshal(o)
	if err != nil {
		panic(err)
	}

	new := LoadOpts{}
	if err := yaml.Unmarshal(bytes, &new); err != nil {
		panic(err)
	}

	if src := o.Environment.OverrideCLISetValues; src != nil {
		b, err := yaml.Marshal(src)
		if err != nil {
			panic(err)
		}
		var dst []any
		if err := yaml.Unmarshal(b, &dst); err != nil {
			panic(err)
		}
		new.Environment.OverrideCLISetValues = dst
	}

	// InheritedConfig.Env is tagged yaml:"-" so it does not survive the
	// marshal/unmarshal round-trip above. Deep-copy it explicitly (mirroring
	// the OverrideCLISetValues handling) so sub-helmfile processing never
	// shares the parent's environment maps by reference.
	if o.Inherited != nil && o.Inherited.Env != nil {
		e := o.Inherited.Env.DeepCopy()
		if new.Inherited == nil {
			new.Inherited = &state.InheritedConfig{}
		}
		new.Inherited.Env = &e
	}

	return new
}
