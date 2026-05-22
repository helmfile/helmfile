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
		wantErr    bool
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
		{
			name:       "malformed selector returns conservative true with error",
			selectorsA: []string{"name=b"},
			selectorsB: []string{"invalid_label"},
			compatible: true,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SelectorsAreCompatible(tt.selectorsA, tt.selectorsB)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.compatible, got)
		})
	}
}

func TestLabelFilterMatchDir(t *testing.T) {
	tests := []struct {
		name     string
		selector string
		dir      string
		want     bool
	}{
		{name: "exact dir match", selector: "dir=apps/opencloud", dir: "apps/opencloud", want: true},
		{name: "prefix dir match", selector: "dir=apps/opencloud", dir: "apps/opencloud/sub", want: true},
		{name: "sibling does not match", selector: "dir=apps/opencloud", dir: "apps/xwiki", want: false},
		{name: "shallower target does not match deeper", selector: "dir=apps/opencloud/sub", dir: "apps/opencloud", want: false},
		{name: "ancestor prefix match", selector: "dir=apps", dir: "apps/opencloud/sub", want: true},
		{name: "missing dir label fails positive", selector: "dir=apps/x", dir: "", want: false},
		{name: "negative dir on different branch matches", selector: "dir!=apps/x", dir: "apps/y", want: true},
		{name: "negative dir on same branch excludes", selector: "dir!=apps/x", dir: "apps/x/foo", want: false},
		{name: "missing dir label still matches negative", selector: "dir!=apps/x", dir: "", want: true},
		{name: "partial-name prefix is not a dir match", selector: "dir=apps/open", dir: "apps/opencloud", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lf, err := ParseLabels(tt.selector)
			assert.NoError(t, err)
			release := ReleaseSpec{Labels: map[string]string{}}
			if tt.dir != "" {
				release.Labels[DirLabel] = tt.dir
			}
			assert.Equal(t, tt.want, lf.Match(release))
		})
	}
}

func TestLabelFilterMatchDirComposesWithOtherKeys(t *testing.T) {
	lf, err := ParseLabels("dir=apps/opencloud,name=foo")
	assert.NoError(t, err)

	tests := []struct {
		name    string
		release ReleaseSpec
		want    bool
	}{
		{
			name:    "both dir and name match",
			release: ReleaseSpec{Labels: map[string]string{DirLabel: "apps/opencloud", "name": "foo"}},
			want:    true,
		},
		{
			name:    "dir matches but name does not",
			release: ReleaseSpec{Labels: map[string]string{DirLabel: "apps/opencloud", "name": "bar"}},
			want:    false,
		},
		{
			name:    "name matches but dir does not",
			release: ReleaseSpec{Labels: map[string]string{DirLabel: "apps/other", "name": "foo"}},
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, lf.Match(tt.release))
		})
	}
}

func TestMatchDirPrefix(t *testing.T) {
	tests := []struct {
		name     string
		dirLabel string
		value    string
		want     bool
	}{
		{"empty label never matches", "", "any", false},
		{"exact equality", "apps/x", "apps/x", true},
		{"deeper is prefix", "apps/x/sub", "apps/x", true},
		{"sibling not matched", "apps/y", "apps/x", false},
		{"target deeper than label", "apps/x", "apps/x/sub", false},
		{"name-collision not a match", "apps/xenial", "apps/x", false},
		{"value with trailing slash normalizes", "apps/x", "apps/x/", true},
		{"label with trailing slash normalizes", "apps/x/", "apps/x", true},
		{"value with ./ prefix normalizes", "apps/x", "./apps/x", true},
		{"root label does not match nested target", ".", "apps/x", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, matchDirPrefix(tt.dirLabel, tt.value))
		})
	}
}

func TestLabelFilterMatchDirCompoundWithNegative(t *testing.T) {
	lf, err := ParseLabels("name=foo,dir!=apps/excluded")
	assert.NoError(t, err)

	tests := []struct {
		name    string
		release ReleaseSpec
		want    bool
	}{
		{
			name:    "name matches and dir is not excluded",
			release: ReleaseSpec{Labels: map[string]string{DirLabel: "apps/wanted", "name": "foo"}},
			want:    true,
		},
		{
			name:    "name matches but dir falls under excluded subtree",
			release: ReleaseSpec{Labels: map[string]string{DirLabel: "apps/excluded/sub", "name": "foo"}},
			want:    false,
		},
		{
			name:    "name does not match and dir is fine",
			release: ReleaseSpec{Labels: map[string]string{DirLabel: "apps/wanted", "name": "bar"}},
			want:    false,
		},
		{
			name:    "no dir label still matches negative dir",
			release: ReleaseSpec{Labels: map[string]string{"name": "foo"}},
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, lf.Match(tt.release))
		})
	}
}

func TestNormalizeDirValue(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"", ""},
		{".", "."},
		{"apps/x", "apps/x"},
		{"apps/x/", "apps/x"},
		{"./apps/x", "apps/x"},
		{"apps//x", "apps/x"},
		{"apps/./x", "apps/x"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			assert.Equal(t, tt.want, NormalizeDirValue(tt.in))
		})
	}
}

func TestDirsCompatible(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want bool
	}{
		{"identical paths", "apps/x", "apps/x", true},
		{"a is ancestor of b", "apps", "apps/x", true},
		{"b is ancestor of a", "apps/x/sub", "apps", true},
		{"siblings are incompatible", "apps/x", "apps/y", false},
		{"name-collision incompatible", "apps/x", "apps/xenial", false},
		{"identical after normalization", "apps/x/", "./apps/x", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, dirsCompatible(tt.a, tt.b))
		})
	}
}

func TestParseLabelsRejectsInvalidDirValues(t *testing.T) {
	cases := []string{
		"dir=.",
		"dir=./",
		"dir=apps/..",
		"dir=/absolute/path",
		"dir=..",
		"dir=../escape",
		"dir!=.",
		"dir!=/absolute",
	}
	for _, sel := range cases {
		t.Run(sel, func(t *testing.T) {
			_, err := ParseLabels(sel)
			assert.Error(t, err, "expected ParseLabels to reject %q", sel)
		})
	}
}

func TestParseLabelsAcceptsValidDirValues(t *testing.T) {
	cases := []string{
		"dir=apps/x",
		"dir=apps/x/sub",
		"dir=./apps/x",
		"dir=apps/x/",
		"dir!=apps/excluded",
		"dir=apps/x,name=foo",
	}
	for _, sel := range cases {
		t.Run(sel, func(t *testing.T) {
			_, err := ParseLabels(sel)
			assert.NoError(t, err, "expected ParseLabels to accept %q", sel)
		})
	}
}
