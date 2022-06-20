package tmpl

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestTextRenderer tests the text renderer.
func TestNewTextRenderer(t *testing.T) {
	tData := map[string]interface{}{
		"foo": "bar",
	}
	tr := NewTextRenderer(os.ReadFile, ".", tData)
	require.Equal(t, tData, tr.Data)
	require.Equal(t, ".", tr.Context.basePath)
}

// TestTextRenderer tests the text renderer.
func TestTextRender(t *testing.T) {
	tData := map[string]interface{}{
		"foot": "bart",
	}
	tr := NewTextRenderer(os.ReadFile, ".", tData)

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
