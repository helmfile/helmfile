package tmpl

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/helmfile/helmfile/pkg/filesystem"
)

// TestTextRenderer tests the text renderer.
func TestNewTextRenderer(t *testing.T) {
	tData := map[string]any{
		"foo": "bar",
	}
	tr := NewTextRenderer(filesystem.DefaultFileSystem(), ".", tData)
	require.Equal(t, tData, tr.Data)
	require.Equal(t, ".", tr.Context.basePath)
}

// TestTextRenderer tests the text renderer.
func TestTextRender(t *testing.T) {
	tData := map[string]any{
		"foot": "bart",
	}
	tr := NewTextRenderer(filesystem.DefaultFileSystem(), ".", tData)

	tests := []struct {
		text    string
		expectd string
		wantErr bool
	}{
		{
			text:    `{{ $foo := .foot }}{{ $foo }}`,
			expectd: "bart",
			wantErr: false,
		},
		{
			text:    `{ $foo := .foot }}{{ $foo }}`,
			wantErr: true,
		},
		{
			text:    `{{ $foo := .a }}{{ $foo }}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			got, err := tr.RenderTemplateText(tt.text)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.expectd, got)
		})
	}
}
