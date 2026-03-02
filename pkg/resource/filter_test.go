package resource

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestResourceFilter_Filter(t *testing.T) {
	resources := []Resource{
		{Kind: "Deployment", Name: "app1", Namespace: "default"},
		{Kind: "StatefulSet", Name: "db1", Namespace: "default"},
		{Kind: "ConfigMap", Name: "cm1", Namespace: "default"},
		{Kind: "Secret", Name: "sec1", Namespace: "kube-system"},
	}

	tests := []struct {
		name     string
		config   *FilterConfig
		expected int
	}{
		{
			name:     "nil filter returns all",
			config:   nil,
			expected: 4,
		},
		{
			name: "TrackKinds whitelist",
			config: &FilterConfig{
				TrackKinds: []string{"Deployment", "StatefulSet"},
			},
			expected: 2,
		},
		{
			name: "SkipKinds blacklist",
			config: &FilterConfig{
				SkipKinds: []string{"ConfigMap", "Secret"},
			},
			expected: 2,
		},
		{
			name: "TrackResources whitelist by kind and namespace",
			config: &FilterConfig{
				TrackResources: []Resource{
					{Kind: "Deployment", Namespace: "default"},
				},
			},
			expected: 1,
		},
		{
			name: "TrackResources whitelist by name",
			config: &FilterConfig{
				TrackResources: []Resource{
					{Name: "app1"},
					{Name: "db1"},
				},
			},
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := NewResourceFilter(tt.config, zap.NewNop().Sugar())
			filtered := filter.Filter(resources)
			assert.Equal(t, tt.expected, len(filtered))
		})
	}
}

func TestResourceFilter_ShouldTrack(t *testing.T) {
	logger := zap.NewNop().Sugar()

	tests := []struct {
		name     string
		config   *FilterConfig
		resource *Resource
		expected bool
	}{
		{
			name:     "nil config tracks all",
			config:   nil,
			resource: &Resource{Kind: "Deployment", Name: "app1"},
			expected: true,
		},
		{
			name: "TrackKinds matches",
			config: &FilterConfig{
				TrackKinds: []string{"Deployment"},
			},
			resource: &Resource{Kind: "Deployment", Name: "app1"},
			expected: true,
		},
		{
			name: "TrackKinds no match",
			config: &FilterConfig{
				TrackKinds: []string{"StatefulSet"},
			},
			resource: &Resource{Kind: "Deployment", Name: "app1"},
			expected: false,
		},
		{
			name: "SkipKinds matches",
			config: &FilterConfig{
				SkipKinds: []string{"ConfigMap"},
			},
			resource: &Resource{Kind: "ConfigMap", Name: "cm1"},
			expected: false,
		},
		{
			name: "SkipKinds no match",
			config: &FilterConfig{
				SkipKinds: []string{"ConfigMap"},
			},
			resource: &Resource{Kind: "Deployment", Name: "app1"},
			expected: true,
		},
		{
			name: "TrackResources whitelist matches all criteria",
			config: &FilterConfig{
				TrackResources: []Resource{
					{Kind: "Deployment", Name: "app1", Namespace: "default"},
				},
			},
			resource: &Resource{Kind: "Deployment", Name: "app1", Namespace: "default"},
			expected: true,
		},
		{
			name: "TrackResources whitelist partial match",
			config: &FilterConfig{
				TrackResources: []Resource{
					{Kind: "Deployment", Name: "app1"},
				},
			},
			resource: &Resource{Kind: "Deployment", Name: "app1", Namespace: "other"},
			expected: true,
		},
		{
			name: "TrackResources whitelist no match",
			config: &FilterConfig{
				TrackResources: []Resource{
					{Kind: "Deployment", Name: "app2"},
				},
			},
			resource: &Resource{Kind: "Deployment", Name: "app1"},
			expected: false,
		},
		{
			name: "TrackResources takes precedence over TrackKinds",
			config: &FilterConfig{
				TrackResources: []Resource{
					{Kind: "Deployment", Name: "app1"},
				},
				TrackKinds: []string{"StatefulSet"},
			},
			resource: &Resource{Kind: "Deployment", Name: "app1"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := NewResourceFilter(tt.config, logger)
			result := filter.ShouldTrack(tt.resource)
			assert.Equal(t, tt.expected, result)
		})
	}
}
