package app

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsRequiredRender(t *testing.T) {
	tests := []struct {
		name     string
		helmfile string
		want     bool
	}{
		{
			name:     "helmfile with tpl extension",
			helmfile: "test.yaml.tpl",
			want:     false,
		},
		{
			name:     "helmfile with gotmpl extension",
			helmfile: "test.yaml.gotmpl",
			want:     true,
		},
		{
			name:     "helmfile with yaml extension",
			helmfile: "test.yaml",
			want:     false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isRequiredRender(tt.helmfile)
			require.Equalf(t, tt.want, got, "isRequiredRender() = %v, want %v", got, tt.want)
		})
	}
}
