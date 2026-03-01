package resource

import (
	"strings"

	"go.uber.org/zap"
)

type ResourceFilter struct {
	logger     *zap.SugaredLogger
	config     *FilterConfig
	skipKinds  map[string]bool
	trackKinds map[string]bool
}

func NewResourceFilter(config *FilterConfig, logger *zap.SugaredLogger) *ResourceFilter {
	f := &ResourceFilter{
		config: config,
		logger: logger,
	}

	if config != nil {
		f.skipKinds = make(map[string]bool)
		for _, kind := range config.SkipKinds {
			f.skipKinds[strings.ToLower(kind)] = true
		}

		f.trackKinds = make(map[string]bool)
		for _, kind := range config.TrackKinds {
			f.trackKinds[strings.ToLower(kind)] = true
		}
	}

	return f
}

func (f *ResourceFilter) Filter(resources []Resource) []Resource {
	if f.config == nil {
		return resources
	}

	var filtered []Resource
	for i := range resources {
		if f.ShouldTrack(&resources[i]) {
			filtered = append(filtered, resources[i])
		} else if f.logger != nil {
			res := resources[i]
			f.logger.Debugf("Skipping resource %s/%s (kind: %s) based on configuration", res.Namespace, res.Name, res.Kind)
		}
	}
	return filtered
}

func (f *ResourceFilter) ShouldTrack(r *Resource) bool {
	if f.config == nil {
		return true
	}

	if len(f.config.TrackResources) > 0 {
		return f.matchWhitelist(r)
	}

	kindLower := strings.ToLower(r.Kind)
	if f.skipKinds[kindLower] {
		return false
	}

	if len(f.trackKinds) > 0 {
		return f.trackKinds[kindLower]
	}

	return true
}

func (f *ResourceFilter) matchWhitelist(r *Resource) bool {
	for _, tr := range f.config.TrackResources {
		// At least one field must be specified for a match
		if tr.Kind == "" && tr.Name == "" && tr.Namespace == "" {
			continue
		}

		if tr.Kind != "" && !strings.EqualFold(tr.Kind, r.Kind) {
			continue
		}
		if tr.Name != "" && tr.Name != r.Name {
			continue
		}
		if tr.Namespace != "" && tr.Namespace != r.Namespace {
			continue
		}
		return true
	}
	return false
}
