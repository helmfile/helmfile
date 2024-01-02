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
		helmSpec HelmSpec
		expected []string
	}{
		{
			name:     "release wait",
			release:  &ReleaseSpec{Wait: &[]bool{true}[0]},
			syncOpts: nil,
			helmSpec: HelmSpec{},
			expected: []string{"--wait"},
		},
		{
			name:     "cli flags wait",
			release:  &ReleaseSpec{},
			syncOpts: &SyncOpts{Wait: true},
			helmSpec: HelmSpec{},
			expected: []string{"--wait"},
		},
		{
			name:     "helm defaults wait",
			release:  &ReleaseSpec{},
			syncOpts: nil,
			helmSpec: HelmSpec{Wait: true},
			expected: []string{"--wait"},
		},
		{
			name:     "release wait false",
			release:  &ReleaseSpec{Wait: &[]bool{false}[0]},
			syncOpts: nil,
			helmSpec: HelmSpec{Wait: true},
			expected: []string{},
		},
		{
			name:     "cli flags wait false",
			release:  &ReleaseSpec{},
			syncOpts: &SyncOpts{},
			helmSpec: HelmSpec{Wait: true},
			expected: []string{"--wait"},
		},
		{
			name:     "helm defaults wait false",
			release:  &ReleaseSpec{},
			syncOpts: nil,
			helmSpec: HelmSpec{Wait: false},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := &HelmState{}
			st.HelmDefaults = tt.helmSpec
			got := st.appendWaitFlags([]string{}, tt.release, tt.syncOpts)
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

func TestAppendDryRunFlags(t *testing.T) {
	type args struct {
		flags    []string
		dry-run  string
		helm     helmexec.Interface
		expected []string
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "do dry-run on client",
			args: args{
				flags:    []string{},
				dry-run:  "client",
				helm:     testutil.NewVersionHelmExec("3.13.0"),
				expected: []string{"--dry-run", "client"},
			},
		},
		{
			name: "empty dry-run means client",
			args: args{
				flags:    []string{},
				dry-run:  "",
				helm:     testutil.NewVersionHelmExec("3.13.0"),
				expected: []string{"--dry-run", "client"},
			},
		},
		{
			name: "do dry-run on server",
			args: args{
				flags:    []string{},
				dry-run:  "server",
				helm:     testutil.NewVersionHelmExec("3.13.0"),
				expected: []string{"--dry-run", "server"},
			},
			{
				name: "no version below 3.13.0",
				args: args{
					flags:    []string{},
					dry-run:  "server",
					helm:     testutil.NewVersionHelmExec("3.12.1"),
					expected: []string{},
				},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := &HelmState{}
			got := st.appendDryRunFlags(tt.args.flags, tt.args.helm, tt.args.release, tt.args.dry-run)
			require.Equalf(t, tt.args.expected, got, "appendDryRunFlags() = %v, want %v", got, tt.args.expected)
		})
	}
}
