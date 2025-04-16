package factory

import (
	"github.com/helmfile/helmfile/pkg/config"
	"github.com/helmfile/helmfile/pkg/flags"
)

// TemplateOptionsFactory creates TemplateOptions and their flag registry
type TemplateOptionsFactory struct{}

func NewTemplateOptionsFactory() *TemplateOptionsFactory {
	return &TemplateOptionsFactory{}
}

func (f *TemplateOptionsFactory) CreateOptions() config.Options {
	return config.NewTemplateOptions()
}

func (f *TemplateOptionsFactory) GetFlagRegistry() flags.FlagRegistry {
	return flags.NewTemplateFlagRegistry()
}
