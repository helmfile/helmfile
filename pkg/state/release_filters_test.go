package state

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLabelFilterMatchSuccessfulMatch(t *testing.T) {
	lf := LabelFilter{
		positiveLabels: [][]string{{"foo", "bar"}},
		negativeLabels: [][]string{{"baz", "bat"}},
	}

	release := ReleaseSpec{
		Labels: map[string]string{
			"foo": "bar",
			"baz": "notbat",
		},
	}

	assert.True(t, lf.Match(release), "expected match but got no match")
}

func TestLabelFilterMatchWithNegativeLabels(t *testing.T) {
	lf := LabelFilter{
		positiveLabels: [][]string{{"foo", "bar"}},
		negativeLabels: [][]string{{"baz", "bat"}},
	}

	release := ReleaseSpec{
		Labels: map[string]string{
			"foo": "bar",
			"baz": "bat",
		},
	}

	assert.False(t, lf.Match(release), "expected no match but got match")
}

func TestParseLabelsValidInput(t *testing.T) {
	labelStr := "foo=bar,baz!=bat"
	expectedPositive := [][]string{{"foo", "bar"}}
	expectedNegative := [][]string{{"baz", "bat"}}

	lf, err := ParseLabels(labelStr)
	assert.NoError(t, err, "unexpected error")

	assert.Equal(t, expectedPositive, lf.positiveLabels, "unexpected positive labels")
	assert.Equal(t, expectedNegative, lf.negativeLabels, "unexpected negative labels")

	release := ReleaseSpec{
		Labels: map[string]string{
			"foo": "bar",
			"baz": "notbat",
		},
	}

	assert.True(t, lf.Match(release), "expected match but got no match")
}

func TestParseLabelsInvalidFormat(t *testing.T) {
	labelStr := "foo=bar,invalid_label"
	_, err := ParseLabels(labelStr)
	assert.Error(t, err, "expected error but got none")

	expectedErrorMsg := "malformed label: invalid_label. Expected label in form k=v or k!=v"
	assert.EqualError(t, err, expectedErrorMsg, "unexpected error message")
}

func TestSelectorsAreCompatible(t *testing.T) {
	tests := []struct {
		name       string
		selectorsA []string
		selectorsB []string
		compatible bool
	}{
		{
			name:       "same key different value",
			selectorsA: []string{"name=b"},
			selectorsB: []string{"name=a"},
			compatible: false,
		},
		{
			name:       "same key same value",
			selectorsA: []string{"name=b"},
			selectorsB: []string{"name=b"},
			compatible: true,
		},
		{
			name:       "different keys no conflict",
			selectorsA: []string{"name=b"},
			selectorsB: []string{"env=prod"},
			compatible: true,
		},
		{
			name:       "one compatible pair among multiple selectors",
			selectorsA: []string{"name=b"},
			selectorsB: []string{"name=a", "name=b"},
			compatible: true,
		},
		{
			name:       "all pairs conflict",
			selectorsA: []string{"name=b"},
			selectorsB: []string{"name=a", "name=c"},
			compatible: false,
		},
		{
			name:       "one compatible pair with same key same value",
			selectorsA: []string{"name=b", "env=prod"},
			selectorsB: []string{"name=b"},
			compatible: true,
		},
		{
			name:       "empty selectorsA always compatible",
			selectorsA: []string{},
			selectorsB: []string{"name=a"},
			compatible: true,
		},
		{
			name:       "empty selectorsB always compatible",
			selectorsA: []string{"name=a"},
			selectorsB: []string{},
			compatible: true,
		},
		{
			name:       "negative labels not compared treated as compatible",
			selectorsA: []string{"name!=a"},
			selectorsB: []string{"name=a"},
			compatible: true,
		},
		{
			name:       "compound selector with conflicting key",
			selectorsA: []string{"name=a,env=prod"},
			selectorsB: []string{"name=a,env=staging"},
			compatible: false,
		},
		{
			name:       "compound selector with matching keys",
			selectorsA: []string{"name=a,env=prod"},
			selectorsB: []string{"name=a,env=prod"},
			compatible: true,
		},
		{
			name:       "compound selector partial overlap different key",
			selectorsA: []string{"name=a,env=prod"},
			selectorsB: []string{"name=a"},
			compatible: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SelectorsAreCompatible(tt.selectorsA, tt.selectorsB)
			assert.NoError(t, err)
			assert.Equal(t, tt.compatible, got)
		})
	}
}
