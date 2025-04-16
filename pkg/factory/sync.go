package factory

import (
	"github.com/helmfile/helmfile/pkg/config"
	"github.com/helmfile/helmfile/pkg/flags"
)

// SyncOptionsFactory creates SyncOptions and their flag registry
type SyncOptionsFactory struct{}

func NewSyncOptionsFactory() *SyncOptionsFactory {
	return &SyncOptionsFactory{}
}

func (f *SyncOptionsFactory) CreateOptions() config.Options {
	return config.NewSyncOptions()
}

func (f *SyncOptionsFactory) GetFlagRegistry() flags.FlagRegistry {
	return flags.NewSyncFlagRegistry()
}
