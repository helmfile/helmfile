package factory

import (
	"github.com/helmfile/helmfile/pkg/config"
	"github.com/helmfile/helmfile/pkg/flags"
)

// OptionsFactory is the interface for factories that create options and flag registries
type OptionsFactory interface {
	// CreateOptions creates and initializes options
	CreateOptions() config.Options

	// GetFlagRegisty returns the appropriate flag registry
	GetFlagRegistry() flags.FlagRegistry
}
