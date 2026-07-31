package state

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestIsChartifyEmptyRenderOutputError verifies that isChartifyEmptyRenderOutputError
// only matches chartify's specific "empty rendered output dir" assertion failure, not
// other, unrelated chartify errors. This is a regression test for issue #1757: a chart
// that renders zero resources (e.g. everything gated behind a falsy `if`) combined with
// transformers/jsonPatches/strategicMergePatches used to crash helmfile with
//
//	assertion failed: unexpected dir entry "" it must be the abs path to the output directory
//
// because chartify expects exactly one rendered output directory and found none.
func TestIsChartifyEmptyRenderOutputError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "chartify empty render assertion error",
			err:      errors.New(`assertion failed: unexpected dir entry "" it must be the abs path to the output directory`),
			expected: true,
		},
		{
			name:     "unrelated chartify error",
			err:      errors.New("exec: \"kustomize\": executable file not found in %PATH%"),
			expected: false,
		},
		{
			name:     "unrelated multiple-dir-entries assertion",
			err:      errors.New(`assertion failed: there should be only one dir entry under the helm output dir /tmp/foo`),
			expected: false,
		},
		{
			name:     "helm template failure",
			err:      errors.New("Error: template: mychart/templates/broken.yaml:3:5: executing..."),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, isChartifyEmptyRenderOutputError(tt.err))
		})
	}
}
