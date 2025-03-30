package state

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/helmfile/helmfile/pkg/helmexec"
	"github.com/helmfile/helmfile/pkg/testutil"
)

func TestAppendWaitForJobsFlags(t *testing.T) {
	tests := []struct {
		name     string
		release  *ReleaseSpec
		syncOpts *SyncOpts
		helmSpec HelmSpec
		expected []string
	}{
		{
			name:     "release wait for jobs",
			release:  &ReleaseSpec{WaitForJobs: &[]bool{true}[0]},
			syncOpts: nil,
			helmSpec: HelmSpec{},
			expected: []string{"--wait-for-jobs"},
		},
		{
			name:     "cli flags wait for jobs",
			release:  &ReleaseSpec{},
			syncOpts: &SyncOpts{WaitForJobs: true},
			helmSpec: HelmSpec{},
			expected: []string{"--wait-for-jobs"},
		},
		{
			name:     "helm defaults wait for jobs",
			release:  &ReleaseSpec{},
			syncOpts: nil,
			helmSpec: HelmSpec{WaitForJobs: true},
			expected: []string{"--wait-for-jobs"},
		},
		{
			name:     "release wait for jobs false",
			release:  &ReleaseSpec{WaitForJobs: &[]bool{false}[0]},
			syncOpts: nil,
			helmSpec: HelmSpec{WaitForJobs: true},
			expected: []string{},
		},
		{
			name:     "cli flags wait for jobs false",
			release:  &ReleaseSpec{},
			syncOpts: &SyncOpts{},
			helmSpec: HelmSpec{WaitForJobs: true},
			expected: []string{"--wait-for-jobs"},
		},
		{
			name:     "helm defaults wait for jobs false",
			release:  &ReleaseSpec{},
			syncOpts: nil,
			helmSpec: HelmSpec{WaitForJobs: false},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := &HelmState{}
			st.HelmDefaults = tt.helmSpec
			got := st.appendWaitForJobsFlags([]string{}, tt.release, tt.syncOpts)
			require.Equalf(t, tt.expected, got, "appendWaitForJobsFlags() = %v, want %v", got, tt.expected)
		})
	}
}

func TestAppendWaitFlags(t *testing.T) {
	tests := []struct {
		name     string
		release  *ReleaseSpec
		syncOpts *SyncOpts
		helm     helmexec.Interface
		helmSpec HelmSpec
		expected []string
	}{
		// --wait
		{
			name:     "release wait",
			release:  &ReleaseSpec{Wait: &[]bool{true}[0]},
			syncOpts: nil,
			helm:     testutil.NewVersionHelmExec("3.11.0"),
			helmSpec: HelmSpec{},
			expected: []string{"--wait"},
		},
		{
			name:     "cli flags wait",
			release:  &ReleaseSpec{},
			syncOpts: &SyncOpts{Wait: true},
			helm:     testutil.NewVersionHelmExec("3.11.0"),
			helmSpec: HelmSpec{},
			expected: []string{"--wait"},
		},
		{
			name:     "helm defaults wait",
			release:  &ReleaseSpec{},
			syncOpts: nil,
			helm:     testutil.NewVersionHelmExec("3.11.0"),
			helmSpec: HelmSpec{Wait: true},
			expected: []string{"--wait"},
		},
		{
			name:     "release wait false",
			release:  &ReleaseSpec{Wait: &[]bool{false}[0]},
			syncOpts: nil,
			helm:     testutil.NewVersionHelmExec("3.11.0"),
			helmSpec: HelmSpec{Wait: true},
			expected: []string{},
		},
		{
			name:     "cli flags wait false",
			release:  &ReleaseSpec{},
			syncOpts: &SyncOpts{},
			helm:     testutil.NewVersionHelmExec("3.11.0"),
			helmSpec: HelmSpec{Wait: true},
			expected: []string{"--wait"},
		},
		{
			name:     "helm defaults wait false",
			release:  &ReleaseSpec{},
			syncOpts: nil,
			helm:     testutil.NewVersionHelmExec("3.11.0"),
			helmSpec: HelmSpec{Wait: false},
			expected: []string{},
		},
		// --wait-retries
		{
			name:     "release wait and retry unsupported",
			release:  &ReleaseSpec{Wait: &[]bool{true}[0], WaitRetries: &[]int{1}[0]},
			syncOpts: nil,
			helm:     testutil.NewVersionHelmExec("3.11.0"),
			helmSpec: HelmSpec{},
			expected: []string{"--wait"},
		},
		{
			name:     "release wait and retry supported",
			release:  &ReleaseSpec{Wait: &[]bool{true}[0], WaitRetries: &[]int{1}[0]},
			syncOpts: nil,
			helm:     testutil.NewVersionHelmExec("3.15.0"),
			helmSpec: HelmSpec{},
			expected: []string{"--wait", "--wait-retries", "1"},
		},
		{
			name:     "no wait retry",
			release:  &ReleaseSpec{WaitRetries: &[]int{1}[0]},
			syncOpts: nil,
			helm:     testutil.NewVersionHelmExec("3.15.0"),
			helmSpec: HelmSpec{},
			expected: []string{},
		},
		{
			name:     "cli flags wait and retry",
			release:  &ReleaseSpec{},
			syncOpts: &SyncOpts{Wait: true, WaitRetries: 2},
			helm:     testutil.NewVersionHelmExec("3.15.0"),
			helmSpec: HelmSpec{},
			expected: []string{"--wait", "--wait-retries", "2"},
		},
		{
			name:     "helm defaults wait retry",
			release:  &ReleaseSpec{},
			syncOpts: nil,
			helm:     testutil.NewVersionHelmExec("3.15.0"),
			helmSpec: HelmSpec{Wait: true, WaitRetries: 3},
			expected: []string{"--wait", "--wait-retries", "3"},
		},
		{
			name:     "release wait default retries",
			release:  &ReleaseSpec{Wait: &[]bool{true}[0]},
			syncOpts: nil,
			helm:     testutil.NewVersionHelmExec("3.15.0"),
			helmSpec: HelmSpec{WaitRetries: 4},
			expected: []string{"--wait", "--wait-retries", "4"},
		},
		{
			name:     "release retries default wait",
			release:  &ReleaseSpec{WaitRetries: &[]int{5}[0]},
			syncOpts: nil,
			helm:     testutil.NewVersionHelmExec("3.15.0"),
			helmSpec: HelmSpec{Wait: true},
			expected: []string{"--wait", "--wait-retries", "5"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := &HelmState{}
			st.HelmDefaults = tt.helmSpec
			got := st.appendWaitFlags([]string{}, tt.helm, tt.release, tt.syncOpts)
			require.Equalf(t, tt.expected, got, "appendWaitFlags() = %v, want %v", got, tt.expected)
		})
	}
}

func TestAppendCascadeFlags(t *testing.T) {
	type args struct {
		flags    []string
		release  *ReleaseSpec
		cascade  string
		helm     helmexec.Interface
		helmSpec HelmSpec
		expected []string
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "no cascade when helm less than 3.11.0",
			args: args{
				flags:    []string{},
				release:  &ReleaseSpec{},
				cascade:  "background",
				helm:     testutil.NewVersionHelmExec("3.11.0"),
				expected: []string{},
			},
		},
		{
			name: "cascade from release",
			args: args{
				flags:    []string{},
				release:  &ReleaseSpec{Cascade: &[]string{"background", "background"}[0]},
				cascade:  "",
				helm:     testutil.NewVersionHelmExec("3.12.1"),
				expected: []string{"--cascade", "background"},
			},
		},
		{
			name: "cascade from cmd flag",
			args: args{
				flags:    []string{},
				release:  &ReleaseSpec{},
				cascade:  "background",
				helm:     testutil.NewVersionHelmExec("3.12.1"),
				expected: []string{"--cascade", "background"},
			},
		},
		{
			name: "cascade from helm defaults",
			args: args{
				flags:    []string{},
				release:  &ReleaseSpec{},
				helmSpec: HelmSpec{Cascade: &[]string{"background", "background"}[0]},
				cascade:  "",
				helm:     testutil.NewVersionHelmExec("3.12.1"),
				expected: []string{"--cascade", "background"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := &HelmState{}
			st.HelmDefaults = tt.args.helmSpec
			got := st.appendCascadeFlags(tt.args.flags, tt.args.helm, tt.args.release, tt.args.cascade)
			require.Equalf(t, tt.args.expected, got, "appendCascadeFlags() = %v, want %v", got, tt.args.expected)
		})
	}
}
func TestAppendSuppressOutputLineRegexFlags(t *testing.T) {
	tests := []struct {
		name                    string
		flags                   []string
		release                 *ReleaseSpec
		suppressOutputLineRegex []string
		helmDefaults            HelmSpec
		expected                []string
	}{
		{
			name:                    "release suppress output line regex",
			flags:                   []string{},
			release:                 &ReleaseSpec{SuppressOutputLineRegex: []string{"regex1", "regex2"}},
			suppressOutputLineRegex: nil,
			helmDefaults:            HelmSpec{},
			expected:                []string{"--suppress-output-line-regex", "regex1", "--suppress-output-line-regex", "regex2"},
		},
		{
			name:                    "cmd suppress output line regex",
			flags:                   []string{},
			release:                 &ReleaseSpec{},
			suppressOutputLineRegex: []string{"regex1", "regex2"},
			helmDefaults:            HelmSpec{},
			expected:                []string{"--suppress-output-line-regex", "regex1", "--suppress-output-line-regex", "regex2"},
		},
		{
			name:                    "helm defaults suppress output line regex",
			flags:                   []string{},
			release:                 &ReleaseSpec{},
			suppressOutputLineRegex: nil,
			helmDefaults:            HelmSpec{SuppressOutputLineRegex: []string{"regex1", "regex2"}},
			expected:                []string{"--suppress-output-line-regex", "regex1", "--suppress-output-line-regex", "regex2"},
		},
		{
			name:                    "empty suppress output line regex",
			flags:                   []string{},
			release:                 &ReleaseSpec{},
			suppressOutputLineRegex: nil,
			helmDefaults:            HelmSpec{},
			expected:                []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := &HelmState{}
			st.HelmDefaults = tt.helmDefaults
			got := st.appendSuppressOutputLineRegexFlags(tt.flags, tt.release, tt.suppressOutputLineRegex)
			require.Equalf(t, tt.expected, got, "appendSuppressOutputLineRegexFlags() = %v, want %v", got, tt.expected)
		})
	}
}

func TestAppendShowOnlyFlags(t *testing.T) {
	tests := []struct {
		name         string
		templateOpts []string
		expected     []string
	}{
		{
			name:         "cli template show only with 1 file",
			templateOpts: []string{"templates/config.yaml"},
			expected:     []string{"--show-only", "templates/config.yaml"},
		},
		{
			name:         "cli template show only with 2 files",
			templateOpts: []string{"templates/config.yaml", "templates/resources.yaml"},
			expected:     []string{"--show-only", "templates/config.yaml", "--show-only", "templates/resources.yaml"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := &HelmState{}
			got := st.appendShowOnlyFlags([]string{}, tt.templateOpts)
			require.Equalf(t, tt.expected, got, "appendShowOnlyFlags() = %v, want %v", got, tt.expected)
		})
	}
}

func TestAppendHideNotesFlags(t *testing.T) {
	type args struct {
		flags    []string
		helm     helmexec.Interface
		helmSpec HelmSpec
		opt      *SyncOpts
		expected []string
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "no hide-notes when helm less than 3.16.0",
			args: args{
				flags: []string{},
				helm:  testutil.NewVersionHelmExec("3.15.0"),
				opt: &SyncOpts{
					HideNotes: true,
				},
				expected: []string{},
			},
		},
		{
			name: "hide-notes from cmd flag",
			args: args{
				flags: []string{},
				helm:  testutil.NewVersionHelmExec("3.16.0"),
				opt: &SyncOpts{
					HideNotes: true,
				},
				expected: []string{"--hide-notes"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := &HelmState{}
			st.HelmDefaults = tt.args.helmSpec
			got := st.appendHideNotesFlags(tt.args.flags, tt.args.helm, tt.args.opt)
			require.Equalf(t, tt.args.expected, got, "appendHideNotesFlags() = %v, want %v", got, tt.args.expected)
		})
	}
}

func TestAppendTakeOwnershipFlags(t *testing.T) {
	type args struct {
		flags    []string
		helm     helmexec.Interface
		helmSpec HelmSpec
		opt      *SyncOpts
		expected []string
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "no take-ownership when helm less than 3.17.0",
			args: args{
				flags: []string{},
				helm:  testutil.NewVersionHelmExec("3.16.0"),
				opt: &SyncOpts{
					TakeOwnership: true,
				},
				expected: []string{},
			},
		},
		{
			name: "take-ownership from cmd flag",
			args: args{
				flags: []string{},
				helm:  testutil.NewVersionHelmExec("3.17.0"),
				opt: &SyncOpts{
					TakeOwnership: true,
				},
				expected: []string{"--take-ownership"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := &HelmState{}
			st.HelmDefaults = tt.args.helmSpec
			got := st.appendTakeOwnershipFlags(tt.args.flags, tt.args.helm, tt.args.opt)
			require.Equalf(t, tt.args.expected, got, "appendTakeOwnershipFlags() = %v, want %v", got, tt.args.expected)
		})
	}
}

func TestFormatLabels(t *testing.T) {
	tests := []struct {
		name   string
		labels map[string]string
		want   string
	}{
		{
			name:   "empty labels",
			labels: map[string]string{},
			want:   "",
		},
		{
			name:   "single label",
			labels: map[string]string{"foo": "bar"},
			want:   "foo=bar",
		},
		{
			name:   "multiple labels",
			labels: map[string]string{"foo": "bar", "baz": "qux"},
			want:   "baz=qux,foo=bar",
		},
		{
			name:   "multiple labels with empty value",
			labels: map[string]string{"foo": "bar", "baz": "qux", "quux": ""},
			want:   "baz=qux,foo=bar,quux=",
		},
		{
			name:   "multiple labels with empty key",
			labels: map[string]string{"foo": "bar", "baz": "qux", "": "quux"},
			want:   "baz=qux,foo=bar",
		},
		{
			name:   "empty label value",
			labels: map[string]string{"foo": ""},
			want:   "foo=",
		},
		{
			name:   "empty label key",
			labels: map[string]string{"": "bar"},
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatLabels(tt.labels)
			require.Equal(t, tt.want, got, "formatLabels() = %v, want %v", got, tt.want)
		})
	}
}
