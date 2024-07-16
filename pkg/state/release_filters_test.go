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
