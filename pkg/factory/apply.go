package factory

import (
	"github.com/helmfile/helmfile/pkg/config"
	"github.com/helmfile/helmfile/pkg/flags"
)

// ApplyOptionsFactory creates ApplyOptions and their flag registry
type ApplyOptionsFactory struct{}

func NewApplyOptionsFactory() *ApplyOptionsFactory {
	return &ApplyOptionsFactory{}
}

func (f *ApplyOptionsFactory) CreateOptions() config.Options {
	return config.NewApplyOptions()
}

func (f *ApplyOptionsFactory) GetFlagRegistry() flags.FlagRegistry {
	return flags.NewApplyFlagRegistry()
}
