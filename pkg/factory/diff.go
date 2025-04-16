package factory

import (
	"github.com/helmfile/helmfile/pkg/config"
	"github.com/helmfile/helmfile/pkg/flags"
)

// DiffOptionsFactory creates DiffOptions and their flag registry
type DiffOptionsFactory struct{}

func NewDiffOptionsFactory() *DiffOptionsFactory {
	return &DiffOptionsFactory{}
}

func (f *DiffOptionsFactory) CreateOptions() config.Options {
	return config.NewDiffOptions()
}

func (f *DiffOptionsFactory) GetFlagRegistry() flags.FlagRegistry {
	return flags.NewDiffFlagRegistry()
}
