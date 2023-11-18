package state

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/helmfile/vals"
	"github.com/stretchr/testify/require"

	"github.com/helmfile/helmfile/pkg/environment"
	"github.com/helmfile/helmfile/pkg/exectest"
	"github.com/helmfile/helmfile/pkg/filesystem"
	"github.com/helmfile/helmfile/pkg/helmexec"
	"github.com/helmfile/helmfile/pkg/testhelper"
	"github.com/helmfile/helmfile/pkg/testutil"
)

var logger = helmexec.NewLogger(io.Discard, "warn")
var valsRuntime, _ = vals.New(vals.Options{CacheSize: 32})

func injectFs(st *HelmState, fs *testhelper.TestFs) *HelmState {
	st.fs = fs.ToFileSystem()
	return st
}

func TestLabelParsing(t *testing.T) {
	cases := []struct {
		labelString    string
		expectedFilter LabelFilter
		errorExected   bool
	}{
		{"foo=bar", LabelFilter{positiveLabels: [][]string{{"foo", "bar"}}, negativeLabels: [][]string{}}, false},
		{"foo!=bar", LabelFilter{positiveLabels: [][]string{}, negativeLabels: [][]string{{"foo", "bar"}}}, false},
		{"foo!=bar,baz=bat", LabelFilter{positiveLabels: [][]string{{"baz", "bat"}}, negativeLabels: [][]string{{"foo", "bar"}}}, false},
		{"foo", LabelFilter{positiveLabels: [][]string{}, negativeLabels: [][]string{}}, true},
		{"foo!=bar=baz", LabelFilter{positiveLabels: [][]string{}, negativeLabels: [][]string{}}, true},
		{"=bar", LabelFilter{positiveLabels: [][]string{}, negativeLabels: [][]string{}}, true},
	}
	for idx, c := range cases {
		filter, err := ParseLabels(c.labelString)
		if err != nil && !c.errorExected {
			t.Errorf("[%d] Didn't expect an error parsing labels: %s", idx, err)
		} else if err == nil && c.errorExected {
			t.Errorf("[%d] Expected %s to result in an error but got none", idx, c.labelString)
		} else if !reflect.DeepEqual(filter, c.expectedFilter) {
			t.Errorf("[%d] parsed label did not result in expected filter: %v, expected: %v", idx, filter, c.expectedFilter)
		}
	}
}

func TestHelmState_applyDefaultsTo(t *testing.T) {
	type fields struct {
		BaseChartPath string
		Context       string

		// TODO: Remove this function once Helmfile v0.x
		DeprecatedReleases []ReleaseSpec

		Namespace    string
		Repositories []RepositorySpec
		Releases     []ReleaseSpec
	}
	type args struct {
		spec ReleaseSpec
	}
	verify := false
	specWithNamespace := ReleaseSpec{
		Chart:     "test/chart",
		Version:   "0.1",
		Verify:    &verify,
		Name:      "test-charts",
		Namespace: "test-namespace",
		Values:    nil,
		SetValues: nil,
		EnvValues: nil,
	}

	specWithoutNamespace := specWithNamespace
	specWithoutNamespace.Namespace = ""
	specWithNamespaceFromFields := specWithNamespace
	specWithNamespaceFromFields.Namespace = "test-namespace-field"

	fieldsWithNamespace := fields{
		BaseChartPath: ".",
		Context:       "test_context",

		// TODO: Remove this function once Helmfile v0.x
		DeprecatedReleases: nil,

		Namespace:    specWithNamespaceFromFields.Namespace,
		Repositories: nil,
		Releases: []ReleaseSpec{
			specWithNamespace,
		},
	}

	fieldsWithoutNamespace := fieldsWithNamespace
	fieldsWithoutNamespace.Namespace = ""

	tests := []struct {
		name   string
		fields fields
		args   args
		want   ReleaseSpec
	}{
		{
			name:   "Has a namespace from spec",
			fields: fieldsWithoutNamespace,
			args: args{
				spec: specWithNamespace,
			},
			want: specWithNamespace,
		},
		{
			name:   "Has a namespace from flags and from spec",
			fields: fieldsWithNamespace,
			args: args{
				spec: specWithNamespace,
			},
			want: specWithNamespaceFromFields,
		},
		{
			name:   "Spec and flag Has no a namespace",
			fields: fieldsWithoutNamespace,
			args: args{
				spec: specWithoutNamespace,
			},
			want: specWithoutNamespace,
		},
		{
			name:   "Spec has no a namespace but from flag",
			fields: fieldsWithNamespace,
			args: args{
				spec: specWithoutNamespace,
			},
			want: specWithNamespaceFromFields,
		},
	}
	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			state := &HelmState{
				basePath: tt.fields.BaseChartPath,
				ReleaseSetSpec: ReleaseSetSpec{
					// TODO: Remove this function once Helmfile v0.x
					DeprecatedContext:  tt.fields.Context,
					DeprecatedReleases: tt.fields.DeprecatedReleases,

					OverrideNamespace: tt.fields.Namespace,
					Repositories:      tt.fields.Repositories,
					Releases:          tt.fields.Releases,
				},
			}
			if state.ApplyOverrides(&tt.args.spec); !reflect.DeepEqual(tt.args.spec, tt.want) {
				t.Errorf("HelmState.ApplyOverrides() = %v, want %v", tt.args.spec, tt.want)
			}
		})
	}
}

func boolValue(v bool) *bool {
	return &v
}

func TestHelmState_flagsForUpgrade(t *testing.T) {
	enable := true
	disable := false
	postRendererDefault := "foo-default.sh"
	postRendererRelease := "foo-release.sh"
	some := func(v int) *int {
		return &v
	}

	tests := []struct {
		name     string
		version  *semver.Version
		defaults HelmSpec
		release  *ReleaseSpec
		want     []string
		wantErr  string
	}{
		{
			name: "no-options",
			defaults: HelmSpec{
				Verify: false,
			},
			release: &ReleaseSpec{
				Chart:     "test/chart",
				Version:   "0.1",
				Verify:    &disable,
				Name:      "test-charts",
				Namespace: "test-namespace",
			},
			want: []string{
				"--version", "0.1",
				"--namespace", "test-namespace",
			},
		},
		{
			name: "verify",
			defaults: HelmSpec{
				Verify: false,
			},
			release: &ReleaseSpec{
				Chart:     "test/chart",
				Version:   "0.1",
				Verify:    &enable,
				Name:      "test-charts",
				Namespace: "test-namespace",
			},
			want: []string{
				"--version", "0.1",
				"--verify",
				"--namespace", "test-namespace",
			},
		},
		{
			name: "verify-from-default",
			defaults: HelmSpec{
				Verify: true,
			},
			release: &ReleaseSpec{
				Chart:     "test/chart",
				Version:   "0.1",
				Verify:    &disable,
				Name:      "test-charts",
				Namespace: "test-namespace",
			},
			want: []string{
				"--version", "0.1",
				"--namespace", "test-namespace",
			},
		},
		{
			name: "enable-dns",
			defaults: HelmSpec{
				EnableDNS: false,
			},
			release: &ReleaseSpec{
				Chart:     "test/chart",
				Version:   "0.1",
				EnableDNS: &enable,
				Name:      "test-charts",
				Namespace: "test-namespace",
			},
			want: []string{
				"--version", "0.1",
				"--enable-dns",
				"--namespace", "test-namespace",
			},
		},
		{
			name: "enable-dns-from-default",
			defaults: HelmSpec{
				EnableDNS: true,
			},
			release: &ReleaseSpec{
				Chart:     "test/chart",
				Version:   "0.1",
				EnableDNS: &disable,
				Name:      "test-charts",
				Namespace: "test-namespace",
			},
			want: []string{
				"--version", "0.1",
				"--namespace", "test-namespace",
			},
		},
		{
			name: "force",
			defaults: HelmSpec{
				Force: false,
			},
			release: &ReleaseSpec{
				Chart:     "test/chart",
				Version:   "0.1",
				Force:     &enable,
				Name:      "test-charts",
				Namespace: "test-namespace",
			},
			want: []string{
				"--version", "0.1",
				"--force",
				"--namespace", "test-namespace",
			},
		},
		{
			name: "force-from-default",
			defaults: HelmSpec{
				Force: true,
			},
			release: &ReleaseSpec{
				Chart:     "test/chart",
				Version:   "0.1",
				Force:     &disable,
				Name:      "test-charts",
				Namespace: "test-namespace",
			},
			want: []string{
				"--version", "0.1",
				"--namespace", "test-namespace",
			},
		},
		{
			name: "recreate-pods",
			defaults: HelmSpec{
				RecreatePods: false,
			},
			release: &ReleaseSpec{
				Chart:        "test/chart",
				Version:      "0.1",
				RecreatePods: &enable,
				Name:         "test-charts",
				Namespace:    "test-namespace",
			},
			want: []string{
				"--version", "0.1",
				"--recreate-pods",
				"--namespace", "test-namespace",
			},
		},
		{
			name: "recreate-pods-from-default",
			defaults: HelmSpec{
				RecreatePods: true,
			},
			release: &ReleaseSpec{
				Chart:        "test/chart",
				Version:      "0.1",
				RecreatePods: &disable,
				Name:         "test-charts",
				Namespace:    "test-namespace",
			},
			want: []string{
				"--version", "0.1",
				"--namespace", "test-namespace",
			},
		},
		{
			name: "wait",
			defaults: HelmSpec{
				Wait: false,
			},
			release: &ReleaseSpec{
				Chart:     "test/chart",
				Version:   "0.1",
				Wait:      &enable,
				Name:      "test-charts",
				Namespace: "test-namespace",
			},
			want: []string{
				"--version", "0.1",
				"--wait",
				"--namespace", "test-namespace",
			},
		},
		{
			name: "wait-for-jobs",
			defaults: HelmSpec{
				WaitForJobs: false,
			},
			release: &ReleaseSpec{
				Chart:       "test/chart",
				Version:     "0.1",
				WaitForJobs: &enable,
				Name:        "test-charts",
				Namespace:   "test-namespace",
			},
			want: []string{
				"--version", "0.1",
				"--wait-for-jobs",
				"--namespace", "test-namespace",
			},
		},
		{
			name: "devel",
			defaults: HelmSpec{
				Devel: true,
			},
			release: &ReleaseSpec{
				Chart:     "test/chart",
				Version:   "0.1",
				Wait:      &enable,
				Name:      "test-charts",
				Namespace: "test-namespace",
			},
			want: []string{
				"--version", "0.1",
				"--devel",
				"--wait",
				"--namespace", "test-namespace",
			},
		},
		{
			name: "devel-release",
			defaults: HelmSpec{
				Devel: true,
			},
			release: &ReleaseSpec{
				Chart:     "test/chart",
				Version:   "0.1",
				Devel:     &disable,
				Name:      "test-charts",
				Namespace: "test-namespace",
			},
			want: []string{
				"--version", "0.1",
				"--namespace", "test-namespace",
			},
		},
		{
			name: "wait-from-default",
			defaults: HelmSpec{
				Wait: true,
			},
			release: &ReleaseSpec{
				Chart:     "test/chart",
				Version:   "0.1",
				Wait:      &disable,
				Name:      "test-charts",
				Namespace: "test-namespace",
			},
			want: []string{
				"--version", "0.1",
				"--namespace", "test-namespace",
			},
		},
		{
			name: "timeout",
			defaults: HelmSpec{
				Timeout: 0,
			},
			release: &ReleaseSpec{
				Chart:     "test/chart",
				Version:   "0.1",
				Timeout:   some(123),
				Name:      "test-charts",
				Namespace: "test-namespace",
			},
			want: []string{
				"--version", "0.1",
				"--timeout", "123s",
				"--namespace", "test-namespace",
			},
		},
		{
			name: "timeout-from-default",
			defaults: HelmSpec{
				Timeout: 123,
			},
			release: &ReleaseSpec{
				Chart:     "test/chart",
				Version:   "0.1",
				Timeout:   nil,
				Name:      "test-charts",
				Namespace: "test-namespace",
			},
			want: []string{
				"--version", "0.1",
				"--timeout", "123s",
				"--namespace", "test-namespace",
			},
		},
		{
			name: "atomic",
			defaults: HelmSpec{
				Atomic: false,
			},
			release: &ReleaseSpec{
				Chart:     "test/chart",
				Version:   "0.1",
				Atomic:    &enable,
				Name:      "test-charts",
				Namespace: "test-namespace",
			},
			want: []string{
				"--version", "0.1",
				"--atomic",
				"--namespace", "test-namespace",
			},
		},
		{
			name: "atomic-override-default",
			defaults: HelmSpec{
				Atomic: true,
			},
			release: &ReleaseSpec{
				Chart:     "test/chart",
				Version:   "0.1",
				Atomic:    &disable,
				Name:      "test-charts",
				Namespace: "test-namespace",
			},
			want: []string{
				"--version", "0.1",
				"--namespace", "test-namespace",
			},
		},
		{
			name: "atomic-from-default",
			defaults: HelmSpec{
				Atomic: true,
			},
			release: &ReleaseSpec{
				Chart:     "test/chart",
				Version:   "0.1",
				Name:      "test-charts",
				Namespace: "test-namespace",
			},
			want: []string{
				"--version", "0.1",
				"--atomic",
				"--namespace", "test-namespace",
			},
		},
		{
			name: "cleanup-on-fail",
			defaults: HelmSpec{
				CleanupOnFail: false,
			},
			release: &ReleaseSpec{
				Chart:         "test/chart",
				Version:       "0.1",
				CleanupOnFail: &enable,
				Name:          "test-charts",
				Namespace:     "test-namespace",
			},
			want: []string{
				"--version", "0.1",
				"--cleanup-on-fail",
				"--namespace", "test-namespace",
			},
		},
		{
			name: "cleanup-on-fail-override-default",
			defaults: HelmSpec{
				CleanupOnFail: true,
			},
			release: &ReleaseSpec{
				Chart:         "test/chart",
				Version:       "0.1",
				CleanupOnFail: &disable,
				Name:          "test-charts",
				Namespace:     "test-namespace",
			},
			want: []string{
				"--version", "0.1",
				"--namespace", "test-namespace",
			},
		},
		{
			name: "cleanup-on-fail-from-default",
			defaults: HelmSpec{
				CleanupOnFail: true,
			},
			release: &ReleaseSpec{
				Chart:     "test/chart",
				Version:   "0.1",
				Name:      "test-charts",
				Namespace: "test-namespace",
			},
			want: []string{
				"--version", "0.1",
				"--cleanup-on-fail",
				"--namespace", "test-namespace",
			},
		},
		{
			name: "create-namespace-default-helm3.2",
			defaults: HelmSpec{
				Verify: false,
			},
			version: semver.MustParse("3.2.0"),
			release: &ReleaseSpec{
				Chart:     "test/chart",
				Version:   "0.1",
				Verify:    &disable,
				Name:      "test-charts",
				Namespace: "test-namespace",
			},
			want: []string{
				"--version", "0.1",
				"--create-namespace",
				"--namespace", "test-namespace",
			},
		},
		{
			name: "create-namespace-disabled-helm3.2",
			defaults: HelmSpec{
				Verify:          false,
				CreateNamespace: &disable,
			},
			version: semver.MustParse("3.2.0"),
			release: &ReleaseSpec{
				Chart:     "test/chart",
				Version:   "0.1",
				Verify:    &disable,
				Name:      "test-charts",
				Namespace: "test-namespace",
			},
			want: []string{
				"--version", "0.1",
				"--namespace", "test-namespace",
			},
		},
		{
			name: "create-namespace-release-override-enabled-helm3.2",
			defaults: HelmSpec{
				Verify:          false,
				CreateNamespace: &disable,
			},
			version: semver.MustParse("3.2.0"),
			release: &ReleaseSpec{
				Chart:           "test/chart",
				Version:         "0.1",
				Verify:          &disable,
				Name:            "test-charts",
				Namespace:       "test-namespace",
				CreateNamespace: &enable,
			},
			want: []string{
				"--version", "0.1",
				"--create-namespace",
				"--namespace", "test-namespace",
			},
		},
		{
			name: "create-namespace-release-override-disabled-helm3.2",
			defaults: HelmSpec{
				Verify:          false,
				CreateNamespace: &enable,
			},
			version: semver.MustParse("3.2.0"),
			release: &ReleaseSpec{
				Chart:           "test/chart",
				Version:         "0.1",
				Verify:          &disable,
				Name:            "test-charts",
				Namespace:       "test-namespace",
				CreateNamespace: &disable,
			},
			want: []string{
				"--version", "0.1",
				"--namespace", "test-namespace",
			},
		},
		{
			name: "create-namespace-unsupported",
			defaults: HelmSpec{
				Verify:          false,
				CreateNamespace: &enable,
			},
			version: semver.MustParse("2.16.0"),
			release: &ReleaseSpec{
				Chart:     "test/chart",
				Version:   "0.1",
				Verify:    &disable,
				Name:      "test-charts",
				Namespace: "test-namespace",
			},
			wantErr: "releases[].createNamespace requires Helm 3.2.0 or greater",
		},
		{
			name: "post-renderer-flags-use-helmdefault",
			defaults: HelmSpec{
				Verify:          false,
				CreateNamespace: &enable,
				PostRenderer:    &postRendererDefault,
			},
			version: semver.MustParse("3.10.0"),
			release: &ReleaseSpec{
				Chart:           "test/chart",
				Version:         "0.1",
				Verify:          &disable,
				Name:            "test-charts",
				Namespace:       "test-namespace",
				CreateNamespace: &disable,
			},
			want: []string{
				"--version", "0.1",
				"--post-renderer", postRendererDefault,
				"--namespace", "test-namespace",
			},
		},
		{
			name: "post-renderer-flags-use-release",
			defaults: HelmSpec{
				Verify:          false,
				CreateNamespace: &enable,
			},
			version: semver.MustParse("3.10.0"),
			release: &ReleaseSpec{
				Chart:           "test/chart",
				Version:         "0.1",
				Verify:          &disable,
				Name:            "test-charts",
				Namespace:       "test-namespace",
				CreateNamespace: &disable,
				PostRenderer:    &postRendererRelease,
			},
			want: []string{
				"--version", "0.1",
				"--post-renderer", postRendererRelease,
				"--namespace", "test-namespace",
			},
		},
		{
			name: "post-renderer-flags-use-release-prior-helmdefault",
			defaults: HelmSpec{
				Verify:          false,
				CreateNamespace: &enable,
				PostRenderer:    &postRendererDefault,
			},
			version: semver.MustParse("3.10.0"),
			release: &ReleaseSpec{
				Chart:           "test/chart",
				Version:         "0.1",
				Verify:          &disable,
				Name:            "test-charts",
				Namespace:       "test-namespace",
				CreateNamespace: &disable,
				PostRenderer:    &postRendererRelease,
			},
			want: []string{
				"--version", "0.1",
				"--post-renderer", postRendererRelease,
				"--namespace", "test-namespace",
			},
		},
	}
	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			state := &HelmState{
				basePath: "./",
				ReleaseSetSpec: ReleaseSetSpec{
					// TODO: Remove this function once Helmfile v0.x
					DeprecatedContext: "default",

					Releases:     []ReleaseSpec{*tt.release},
					HelmDefaults: tt.defaults,
				},
				valsRuntime: valsRuntime,
			}
			helm := &exectest.Helm{
				Version: tt.version,
			}

			args, _, err := state.flagsForUpgrade(helm, tt.release, 0, nil)
			if err != nil && tt.wantErr == "" {
				t.Errorf("unexpected error flagsForUpgrade: %v", err)
			}
			if tt.wantErr != "" && (err == nil || err.Error() != tt.wantErr) {
				t.Errorf("expected error '%v'; got '%v'", err, tt.wantErr)
			}
			if !reflect.DeepEqual(args, tt.want) {
				t.Errorf("flagsForUpgrade returned = %v, want %v", args, tt.want)
			}
		})
	}
}

func TestHelmState_flagsForTemplate(t *testing.T) {
	enable := true
	disable := false
	postRendererDefault := "foo-default.sh"
	postRendererRelease := "foo-release.sh"

	tests := []struct {
		name         string
		version      *semver.Version
		defaults     HelmSpec
		release      *ReleaseSpec
		templateOpts TemplateOpts
		want         []string
		wantErr      string
	}{
		{
			name: "post-renderer-flags-use-helmdefault",
			defaults: HelmSpec{
				Verify:          false,
				CreateNamespace: &enable,
				PostRenderer:    &postRendererDefault,
			},
			version: semver.MustParse("3.10.0"),
			release: &ReleaseSpec{
				Chart:           "test/chart",
				Version:         "0.1",
				Verify:          &disable,
				Name:            "test-charts",
				Namespace:       "test-namespace",
				CreateNamespace: &disable,
			},
			want: []string{
				"--version", "0.1",
				"--post-renderer", postRendererDefault,
				"--namespace", "test-namespace",
			},
		},
		{
			name: "post-renderer-flags-use-release",
			defaults: HelmSpec{
				Verify:          false,
				CreateNamespace: &enable,
			},
			version: semver.MustParse("3.10.0"),
			release: &ReleaseSpec{
				Chart:           "test/chart",
				Version:         "0.1",
				Verify:          &disable,
				Name:            "test-charts",
				Namespace:       "test-namespace",
				CreateNamespace: &disable,
				PostRenderer:    &postRendererRelease,
			},
			want: []string{
				"--version", "0.1",
				"--post-renderer", postRendererRelease,
				"--namespace", "test-namespace",
			},
		},
		{
			name: "post-renderer-flags-use-release-prior-helmdefault",
			defaults: HelmSpec{
				Verify:          false,
				CreateNamespace: &enable,
				PostRenderer:    &postRendererDefault,
			},
			version: semver.MustParse("3.10.0"),
			release: &ReleaseSpec{
				Chart:           "test/chart",
				Version:         "0.1",
				Verify:          &disable,
				Name:            "test-charts",
				Namespace:       "test-namespace",
				CreateNamespace: &disable,
				PostRenderer:    &postRendererRelease,
			},
			want: []string{
				"--version", "0.1",
				"--post-renderer", postRendererRelease,
				"--namespace", "test-namespace",
			},
		},
		{
			name: "kube-version-flag-should-be-used",
			defaults: HelmSpec{
				Verify:          false,
				CreateNamespace: &enable,
			},
			version: semver.MustParse("3.10.0"),
			release: &ReleaseSpec{
				Chart:           "test/chart",
				Version:         "0.1",
				Verify:          &disable,
				Name:            "test-charts",
				Namespace:       "test-namespace",
				CreateNamespace: &disable,
			},
			templateOpts: TemplateOpts{
				KubeVersion: "1.100",
			},
			want: []string{
				"--version", "0.1",
				"--kube-version", "1.100",
				"--namespace", "test-namespace",
			},
		},
		{
			name: "kube-version-flag-should-be-respected",
			defaults: HelmSpec{
				Verify:          false,
				CreateNamespace: &enable,
			},
			version: semver.MustParse("3.10.0"),
			release: &ReleaseSpec{
				Chart:           "test/chart",
				Version:         "0.1",
				Verify:          &disable,
				Name:            "test-charts",
				Namespace:       "test-namespace",
				CreateNamespace: &disable,
				KubeVersion:     "1.25",
			},
			templateOpts: TemplateOpts{
				KubeVersion: "1.100",
			},
			want: []string{
				"--version", "0.1",
				"--kube-version", "1.100",
				"--namespace", "test-namespace",
			},
		},
	}
	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			state := &HelmState{
				basePath: "./",
				ReleaseSetSpec: ReleaseSetSpec{
					DeprecatedContext: "default",
					Releases:          []ReleaseSpec{*tt.release},
					HelmDefaults:      tt.defaults,
				},
				valsRuntime: valsRuntime,
			}
			helm := &exectest.Helm{
				Version: tt.version,
			}

			args, _, err := state.flagsForTemplate(helm, tt.release, 0, &(tt.templateOpts))
			if err != nil && tt.wantErr == "" {
				t.Errorf("unexpected error flagsForUpgrade: %v", err)
			}
			if tt.wantErr != "" && (err == nil || err.Error() != tt.wantErr) {
				t.Errorf("expected error '%v'; got '%v'", err, tt.wantErr)
			}
			if !reflect.DeepEqual(args, tt.want) {
				t.Errorf("flagsForUpgrade returned = %v, want %v", args, tt.want)
			}
		})
	}
}

func Test_isLocalChart(t *testing.T) {
	type args struct {
		chart string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "local chart",
			args: args{
				chart: "./",
			},
			want: true,
		},
		{
			name: "repo chart",
			args: args{
				chart: "stable/genius",
			},
			want: false,
		},
		{
			name: "empty",
			args: args{
				chart: "",
			},
			want: true,
		},
		{
			name: "parent local path",
			args: args{
				chart: "../examples",
			},
			want: true,
		},
		{
			name: "parent-parent local path",
			args: args{
				chart: "../../",
			},
			want: true,
		},
		{
			name: "absolute path",
			args: args{
				chart: "/foo/bar/baz",
			},
			want: true,
		},
		{
			name: "remote chart in 3-level deep dir (e.g. ChartCenter)",
			args: args{
				chart: "center/bar/baz",
			},
			want: false,
		},
	}
	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			if got := isLocalChart(tt.args.chart); got != tt.want {
				t.Errorf("%s(\"%s\") isLocalChart(): got %v, want %v", tt.name, tt.args.chart, got, tt.want)
			}
		})
	}
}

func Test_normalizeChart(t *testing.T) {
	type args struct {
		basePath string
		chart    string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "construct local chart path",
			args: args{
				basePath: "/src",
				chart:    "./app",
			},
			want: "/src/app",
		},
		{
			name: "construct local chart path, without leading dot",
			args: args{
				basePath: "/src",
				chart:    "published",
			},
			want: "/src/published",
		},
		{
			name: "repo path",
			args: args{
				basePath: "/src",
				chart:    "remote/app",
			},
			want: "remote/app",
		},
		{
			name: "chartcenter repo path",
			args: args{
				basePath: "/src",
				chart:    "center/stable/myapp",
			},
			want: "center/stable/myapp",
		},
		{
			name: "construct local chart path, sibling dir",
			args: args{
				basePath: "/src",
				chart:    "../app",
			},
			want: "/app",
		},
		{
			name: "construct local chart path, parent dir",
			args: args{
				basePath: "/src",
				chart:    "./..",
			},
			want: "/",
		},
		{
			name: "too much parent levels",
			args: args{
				basePath: "/src",
				chart:    "../../app",
			},
			want: "/app",
		},
	}
	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeChart(tt.args.basePath, tt.args.chart); got != tt.want {
				t.Errorf("normalizeChart() = %v, want %v", got, tt.want)
			}
		})
	}
}

// mocking helmexec.Interface
func TestHelmState_SyncRepos(t *testing.T) {
	tests := []struct {
		name  string
		repos []RepositorySpec
		helm  *exectest.Helm
		envs  map[string]string
		want  []string
	}{
		{
			name: "normal repository",
			repos: []RepositorySpec{
				{
					Name:     "name",
					URL:      "http://example.com/",
					CertFile: "",
					KeyFile:  "",
					Username: "",
					Password: "",
				},
			},
			helm: &exectest.Helm{},
			want: []string{"name", "http://example.com/", "", "", "", "", "", "", "false", "false"},
		},
		{
			name: "ACR hosted repository",
			repos: []RepositorySpec{
				{
					Name:    "name",
					Managed: "acr",
				},
			},
			helm: &exectest.Helm{},
			want: []string{"name", "", "", "", "", "", "", "acr", "false", "false"},
		},
		{
			name: "repository with cert and key",
			repos: []RepositorySpec{
				{
					Name:     "name",
					URL:      "http://example.com/",
					CertFile: "certfile",
					KeyFile:  "keyfile",
					Username: "",
					Password: "",
				},
			},
			helm: &exectest.Helm{},
			want: []string{"name", "http://example.com/", "", "certfile", "keyfile", "", "", "", "false", "false"},
		},
		{
			name: "repository with ca file",
			repos: []RepositorySpec{
				{
					Name:     "name",
					URL:      "http://example.com/",
					CaFile:   "cafile",
					Username: "",
					Password: "",
				},
			},
			helm: &exectest.Helm{},
			want: []string{"name", "http://example.com/", "cafile", "", "", "", "", "", "false", "false"},
		},
		{
			name: "repository with username and password",
			repos: []RepositorySpec{
				{
					Name:     "name",
					URL:      "http://example.com/",
					CertFile: "",
					KeyFile:  "",
					Username: "example_user",
					Password: "example_password",
				},
			},
			helm: &exectest.Helm{},
			want: []string{"name", "http://example.com/", "", "", "", "example_user", "example_password", "", "false", "false"},
		},
		{
			name: "repository with username and password and pass-credentials",
			repos: []RepositorySpec{
				{
					Name:            "name",
					URL:             "http://example.com/",
					CertFile:        "",
					KeyFile:         "",
					Username:        "example_user",
					Password:        "example_password",
					PassCredentials: true,
				},
			},
			helm: &exectest.Helm{},
			want: []string{"name", "http://example.com/", "", "", "", "example_user", "example_password", "", "true", "false"},
		},
		{
			name: "repository without username and password and environment with username and password",
			repos: []RepositorySpec{
				{
					Name:     "name",
					URL:      "http://example.com/",
					CertFile: "",
					KeyFile:  "",
					Username: "",
					Password: "",
				},
			},
			envs: map[string]string{
				"NAME_USERNAME": "example_user",
				"NAME_PASSWORD": "example_password",
			},
			helm: &exectest.Helm{},
			want: []string{"name", "http://example.com/", "", "", "", "example_user", "example_password", "", "false", "false"},
		},
		{
			name: "repository with username and password and environment with username and password",
			repos: []RepositorySpec{
				{
					Name:     "name",
					URL:      "http://example.com/",
					CertFile: "",
					KeyFile:  "",
					Username: "example_user1",
					Password: "example_password1",
				},
			},
			envs: map[string]string{
				"NAME_USERNAME": "example_user2",
				"NAME_PASSWORD": "example_password2",
			},
			helm: &exectest.Helm{},
			want: []string{"name", "http://example.com/", "", "", "", "example_user1", "example_password1", "", "false", "false"},
		},
		{
			name: "repository with skip-tls-verify",
			repos: []RepositorySpec{
				{
					Name:          "name",
					URL:           "http://example.com/",
					CertFile:      "",
					KeyFile:       "",
					Username:      "",
					Password:      "",
					SkipTLSVerify: true,
				},
			},
			helm: &exectest.Helm{},
			want: []string{"name", "http://example.com/", "", "", "", "", "", "", "false", "true"},
		},
	}
	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.envs {
				t.Setenv(k, v)
			}
			state := &HelmState{
				ReleaseSetSpec: ReleaseSetSpec{
					Repositories: tt.repos,
				},
			}
			if _, _ = state.SyncRepos(tt.helm, map[string]bool{}); !reflect.DeepEqual(tt.helm.Repo, tt.want) {
				t.Errorf("HelmState.SyncRepos() for [%s] = %v, want %v", tt.name, tt.helm.Repo, tt.want)
			}
		})
	}
}

func TestHelmState_SyncReleases(t *testing.T) {
	postRenderer := "foo.sh"
	tests := []struct {
		name          string
		releases      []ReleaseSpec
		helm          *exectest.Helm
		wantReleases  []exectest.Release
		wantErrorMsgs []string
	}{
		{
			name: "normal release",
			releases: []ReleaseSpec{
				{
					Name:  "releaseName",
					Chart: "foo",
				},
			},
			helm:         &exectest.Helm{},
			wantReleases: []exectest.Release{{Name: "releaseName", Flags: []string{"--reset-values"}}},
		},
		{
			name: "escaped values",
			releases: []ReleaseSpec{
				{
					Name:  "releaseName",
					Chart: "foo",
					SetValues: []SetValue{
						{
							Name:  "someList",
							Value: "a,b,c",
						},
						{
							Name:  "json",
							Value: "{\"name\": \"john\"}",
						},
					},
				},
			},
			helm:         &exectest.Helm{},
			wantReleases: []exectest.Release{{Name: "releaseName", Flags: []string{"--set", "someList=a\\,b\\,c", "--set", "json=\\{\"name\": \"john\"\\}", "--reset-values"}}},
		},
		{
			name: "set single value from file",
			releases: []ReleaseSpec{
				{
					Name:  "releaseName",
					Chart: "foo",
					SetValues: []SetValue{
						{
							Name:  "foo",
							Value: "FOO",
						},
						{
							Name: "bar",
							File: "path/to/bar",
						},
						{
							Name:  "baz",
							Value: "BAZ",
						},
					},
				},
			},
			helm:         &exectest.Helm{},
			wantReleases: []exectest.Release{{Name: "releaseName", Flags: []string{"--set", "foo=FOO", "--set-file", "bar=path/to/bar", "--set", "baz=BAZ", "--reset-values"}}},
		},
		{
			name: "set single array value in an array",
			releases: []ReleaseSpec{
				{
					Name:  "releaseName",
					Chart: "foo",
					SetValues: []SetValue{
						{
							Name: "foo.bar[0]",
							Values: []string{
								"A",
								"B",
							},
						},
					},
				},
			},
			helm:         &exectest.Helm{},
			wantReleases: []exectest.Release{{Name: "releaseName", Flags: []string{"--set", "foo.bar[0]={A,B}", "--reset-values"}}},
		},
		{
			name: "post renderer",
			releases: []ReleaseSpec{
				{
					Name:         "releaseName",
					Chart:        "foo",
					PostRenderer: &postRenderer,
				},
			},
			helm: &exectest.Helm{
				Helm3: true,
			},
			wantReleases: []exectest.Release{{Name: "releaseName", Flags: []string{"--post-renderer", postRenderer, "--reset-values"}}},
		},
	}
	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			state := &HelmState{
				ReleaseSetSpec: ReleaseSetSpec{
					Releases: tt.releases,
				},
				logger:         logger,
				valsRuntime:    valsRuntime,
				RenderedValues: map[string]any{},
			}
			if errs := state.SyncReleases(&AffectedReleases{}, tt.helm, []string{}, 1); len(errs) > 0 {
				if len(errs) != len(tt.wantErrorMsgs) {
					t.Fatalf("Unexpected errors: %v\nExpected: %v", errs, tt.wantErrorMsgs)
				}
				var mismatch int
				for i := range tt.wantErrorMsgs {
					expected := tt.wantErrorMsgs[i]
					actual := errs[i].Error()
					if !reflect.DeepEqual(actual, expected) {
						t.Errorf("Unexpected error: expected=%v, got=%v", expected, actual)
					}
				}
				if mismatch > 0 {
					t.Fatalf("%d unexpected errors detected", mismatch)
				}
			}
			if !reflect.DeepEqual(tt.helm.Releases, tt.wantReleases) {
				t.Errorf("HelmState.SyncReleases() for [%s] = %v, want %v", tt.name, tt.helm.Releases, tt.wantReleases)
			}
		})
	}
}

func TestHelmState_SyncReleases_MissingValuesFileForUndesiredRelease(t *testing.T) {
	no := false
	tests := []struct {
		name          string
		release       ReleaseSpec
		listResult    string
		expectedError string
	}{
		{
			name: "should install",
			release: ReleaseSpec{
				Name:  "foo",
				Chart: "../../foo-bar",
			},
			listResult:    ``,
			expectedError: ``,
		},
		{
			name: "should upgrade",
			release: ReleaseSpec{
				Name:  "foo",
				Chart: "../../foo-bar",
			},
			listResult: `NAME 	REVISION	UPDATED                 	STATUS  	CHART                      	APP VERSION	NAMESPACE
										foo	1       	Wed Apr 17 17:39:04 2019	DEPLOYED	foo-bar-2.0.4	0.1.0      	default`,
			expectedError: ``,
		},
		{
			name: "should uninstall",
			release: ReleaseSpec{
				Name:      "foo",
				Chart:     "../../foo-bar",
				Installed: &no,
			},
			listResult: `NAME 	REVISION	UPDATED                 	STATUS  	CHART                      	APP VERSION	NAMESPACE
										foo	1       	Wed Apr 17 17:39:04 2019	DEPLOYED	foo-bar-2.0.4	0.1.0      	default`,
			expectedError: ``,
		},
		{
			name: "should fail installing due to missing values file",
			release: ReleaseSpec{
				Name:   "foo",
				Chart:  "../../foo-bar",
				Values: []any{"noexistent.values.yaml"},
			},
			listResult:    ``,
			expectedError: `failed processing release foo: values file matching "noexistent.values.yaml" does not exist in "."`,
		},
		{
			name: "should fail upgrading due to missing values file",
			release: ReleaseSpec{
				Name:   "foo",
				Chart:  "../../foo-bar",
				Values: []any{"noexistent.values.yaml"},
			},
			listResult: `NAME 	REVISION	UPDATED                 	STATUS  	CHART                      	APP VERSION	NAMESPACE
										foo	1       	Wed Apr 17 17:39:04 2019	DEPLOYED	foo-bar-2.0.4	0.1.0      	default`,
			expectedError: `failed processing release foo: values file matching "noexistent.values.yaml" does not exist in "."`,
		},
		{
			name: "should uninstall even when there is a missing values file",
			release: ReleaseSpec{
				Name:      "foo",
				Chart:     "../../foo-bar",
				Values:    []any{"noexistent.values.yaml"},
				Installed: &no,
			},
			listResult: `NAME 	REVISION	UPDATED                 	STATUS  	CHART                      	APP VERSION	NAMESPACE
										foo	1       	Wed Apr 17 17:39:04 2019	DEPLOYED	foo-bar-2.0.4	0.1.0      	default`,
			expectedError: ``,
		},
	}
	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			state := &HelmState{
				basePath: ".",
				ReleaseSetSpec: ReleaseSetSpec{
					Releases: []ReleaseSpec{tt.release},
				},
				logger:         logger,
				valsRuntime:    valsRuntime,
				RenderedValues: map[string]any{},
			}
			fs := testhelper.NewTestFs(map[string]string{})
			state = injectFs(state, fs)
			helm := &exectest.Helm{
				Lists: map[exectest.ListKey]string{},
				Helm3: true,
			}
			//simulate the helm.list call result
			helm.Lists[exectest.ListKey{Filter: "^" + tt.release.Name + "$"}] = tt.listResult

			affectedReleases := AffectedReleases{}
			errs := state.SyncReleases(&affectedReleases, helm, []string{}, 1)

			if tt.expectedError != "" {
				if len(errs) == 0 {
					t.Fatalf("expected error not occurred: expected=%s, got none", tt.expectedError)
				}
				if len(errs) != 1 {
					t.Fatalf("too many errors: expected %d, got %d: %v", 1, len(errs), errs)
				}
				err := errs[0]
				if err.Error() != tt.expectedError {
					t.Fatalf("unexpected error: expected=%s, got=%v", tt.expectedError, err)
				}
			} else {
				if len(errs) > 0 {
					t.Fatalf("unexpected error(s): expected=0, got=%d: %v", len(errs), errs)
				}
			}
		})
	}
}

func TestHelmState_SyncReleasesAffectedRealeases(t *testing.T) {
	no := false
	tests := []struct {
		name         string
		releases     []ReleaseSpec
		installed    []bool
		wantAffected exectest.Affected
	}{
		{
			name: "2 release",
			releases: []ReleaseSpec{
				{
					Name:  "releaseNameFoo",
					Chart: "foo",
				},
				{
					Name:  "releaseNameBar",
					Chart: "bar",
				},
			},
			wantAffected: exectest.Affected{
				Upgraded: []*exectest.Release{
					{Name: "releaseNameFoo", Flags: []string{}},
					{Name: "releaseNameBar", Flags: []string{}},
				},
				Deleted: nil,
				Failed:  nil,
			},
		},
		{
			name: "2 removed",
			releases: []ReleaseSpec{
				{
					Name:      "releaseNameFoo",
					Chart:     "foo",
					Installed: &no,
				},
				{
					Name:      "releaseNameBar",
					Chart:     "foo",
					Installed: &no,
				},
			},
			installed: []bool{true, true},
			wantAffected: exectest.Affected{
				Upgraded: nil,
				Deleted: []*exectest.Release{
					{Name: "releaseNameFoo", Flags: []string{}},
					{Name: "releaseNameBar", Flags: []string{}},
				},
				Failed: nil,
			},
		},
		{
			name: "2 errors",
			releases: []ReleaseSpec{
				{
					Name:  "releaseNameFoo-error",
					Chart: "foo",
				},
				{
					Name:  "releaseNameBar-error",
					Chart: "foo",
				},
			},
			wantAffected: exectest.Affected{
				Upgraded: nil,
				Deleted:  nil,
				Failed: []*exectest.Release{
					{Name: "releaseNameFoo-error", Flags: []string{}},
					{Name: "releaseNameBar-error", Flags: []string{}},
				},
			},
		},
		{
			name: "1 removed, 1 new, 1 error",
			releases: []ReleaseSpec{
				{
					Name:  "releaseNameFoo",
					Chart: "foo",
				},
				{
					Name:      "releaseNameBar",
					Chart:     "foo",
					Installed: &no,
				},
				{
					Name:  "releaseNameFoo-error",
					Chart: "foo",
				},
			},
			installed: []bool{true, true, true},
			wantAffected: exectest.Affected{
				Upgraded: []*exectest.Release{
					{Name: "releaseNameFoo", Flags: []string{}},
				},
				Deleted: []*exectest.Release{
					{Name: "releaseNameBar", Flags: []string{}},
				},
				Failed: []*exectest.Release{
					{Name: "releaseNameFoo-error", Flags: []string{}},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &HelmState{
				ReleaseSetSpec: ReleaseSetSpec{
					Releases: tt.releases,
				},
				logger:         logger,
				valsRuntime:    valsRuntime,
				RenderedValues: map[string]any{},
			}
			helm := &exectest.Helm{
				Lists: map[exectest.ListKey]string{},
			}
			//simulate the release is already installed
			for i, release := range tt.releases {
				if tt.installed != nil && tt.installed[i] {
					helm.Lists[exectest.ListKey{Filter: "^" + release.Name + "$", Flags: "--uninstalling --deployed --failed --pending"}] = release.Name
				}
			}

			affectedReleases := AffectedReleases{}
			if err := state.SyncReleases(&affectedReleases, helm, []string{}, 1); err != nil {
				if !testEq(affectedReleases.Failed, tt.wantAffected.Failed) {
					t.Errorf("HelmState.SynchAffectedRelease() error failed for [%s] = %v, want %v", tt.name, affectedReleases.Failed, tt.wantAffected.Failed)
				} //else expected error
			}
			if !testEq(affectedReleases.Upgraded, tt.wantAffected.Upgraded) {
				t.Errorf("HelmState.SynchAffectedRelease() upgrade failed for [%s] = %v, want %v", tt.name, affectedReleases.Upgraded, tt.wantAffected.Upgraded)
			}
			if !testEq(affectedReleases.Deleted, tt.wantAffected.Deleted) {
				t.Errorf("HelmState.SynchAffectedRelease() deleted failed for [%s] = %v, want %v", tt.name, affectedReleases.Deleted, tt.wantAffected.Deleted)
			}
		})
	}
}

func testEq(a []*ReleaseSpec, b []*exectest.Release) bool {
	// If one is nil, the other must also be nil.
	if (a == nil) != (b == nil) {
		return false
	}

	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i].Name != b[i].Name {
			return false
		}
	}

	return true
}

func TestGetDeployedVersion(t *testing.T) {
	tests := []struct {
		name             string
		release          ReleaseSpec
		listResult       string
		installedVersion string
	}{
		{
			name: "chart version",
			release: ReleaseSpec{
				Name:  "foo",
				Chart: "../../foo-bar",
			},
			listResult: `NAME 	REVISION	UPDATED                 	STATUS  	CHART                      	APP VERSION	NAMESPACE
										foo	1       	Wed Apr 17 17:39:04 2019	DEPLOYED	foo-bar-2.0.4	0.1.0      	default`,
			installedVersion: "2.0.4",
		},
		{
			name: "chart version with a dash",
			release: ReleaseSpec{
				Name:  "foo-bar",
				Chart: "registry/foo-bar",
			},
			listResult: `NAME 	REVISION	UPDATED                 	STATUS  	CHART                      	APP VERSION	NAMESPACE
										foo	1       	Wed Apr 17 17:39:04 2019	DEPLOYED	foo-bar-1.0.0-alpha.1	0.1.0      	default`,
			installedVersion: "1.0.0-alpha.1",
		},
		{
			name: "chart version with dash and plus",
			release: ReleaseSpec{
				Name:  "foo-bar",
				Chart: "registry/foo-bar",
			},
			listResult: `NAME 	REVISION	UPDATED                 	STATUS  	CHART                      	APP VERSION	NAMESPACE
										foo	1       	Wed Apr 17 17:39:04 2019	DEPLOYED	foo-bar-1.0.0-alpha+001	0.1.0      	default`,
			installedVersion: "1.0.0-alpha+001",
		},
		{
			name: "chart version with dash and release with dash",
			release: ReleaseSpec{
				Name:  "foo-bar",
				Chart: "registry/foo-bar",
			},
			listResult: `NAME 	REVISION	UPDATED                 	STATUS  	CHART                      	APP VERSION	NAMESPACE
										foo-bar-release	1       	Wed Apr 17 17:39:04 2019	DEPLOYED	foo-bar-1.0.0-alpha+001	0.1.0      	default`,
			installedVersion: "1.0.0-alpha+001",
		},
		{
			name: "chart version from helm show chart",
			release: ReleaseSpec{
				Name:  "foo",
				Chart: "../../foo-bar",
			},
			listResult: `NAME 	REVISION	UPDATED                 	STATUS  	CHART                      	APP VERSION	NAMESPACE
										foo	1       	Wed Apr 17 17:39:04 2019	DEPLOYED	foo-bar      	0.1.0      	default`,
			installedVersion: "3.2.0",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &HelmState{
				ReleaseSetSpec: ReleaseSetSpec{
					Releases: []ReleaseSpec{tt.release},
				},
				logger:         logger,
				valsRuntime:    valsRuntime,
				RenderedValues: map[string]any{},
			}

			helm := &exectest.Helm{
				Lists: map[exectest.ListKey]string{},
			}
			// simulate the helm.list call result
			helm.Lists[exectest.ListKey{Filter: "^" + tt.release.Name + "$", Flags: "--uninstalling --deployed --failed --pending"}] = tt.listResult

			affectedReleases := AffectedReleases{}
			state.SyncReleases(&affectedReleases, helm, []string{}, 1)

			if state.Releases[0].installedVersion != tt.installedVersion {
				t.Errorf("HelmState.TestGetDeployedVersion() failed for [%s] = %v, want %v", tt.name, state.Releases[0].installedVersion, tt.installedVersion)
			}
		})
	}
}

func TestHelmState_DiffReleases(t *testing.T) {
	tests := []struct {
		name         string
		releases     []ReleaseSpec
		helm         *exectest.Helm
		wantReleases []exectest.Release
	}{
		{
			name: "normal release",
			releases: []ReleaseSpec{
				{
					Name:  "releaseName",
					Chart: "foo",
				},
			},
			helm:         &exectest.Helm{},
			wantReleases: []exectest.Release{{Name: "releaseName", Flags: []string{"--reset-values"}}},
		},
		{
			name: "escaped values",
			releases: []ReleaseSpec{
				{
					Name:  "releaseName",
					Chart: "foo",
					SetValues: []SetValue{
						{
							Name:  "someList",
							Value: "a,b,c",
						},
						{
							Name:  "json",
							Value: "{\"name\": \"john\"}",
						},
					},
				},
			},
			helm: &exectest.Helm{},
			wantReleases: []exectest.Release{
				{Name: "releaseName", Flags: []string{"--set", "someList=a\\,b\\,c", "--set", "json=\\{\"name\": \"john\"\\}", "--reset-values"}},
			},
		},
		{
			name: "set single value from file",
			releases: []ReleaseSpec{
				{
					Name:  "releaseName",
					Chart: "foo",
					SetValues: []SetValue{
						{
							Name:  "foo",
							Value: "FOO",
						},
						{
							Name: "bar",
							File: "path/to/bar",
						},
						{
							Name:  "baz",
							Value: "BAZ",
						},
					},
				},
			},
			helm: &exectest.Helm{},
			wantReleases: []exectest.Release{
				{Name: "releaseName", Flags: []string{"--set", "foo=FOO", "--set-file", "bar=path/to/bar", "--set", "baz=BAZ", "--reset-values"}},
			},
		},
		{
			name: "set single array value in an array",
			releases: []ReleaseSpec{
				{
					Name:  "releaseName",
					Chart: "foo",
					SetValues: []SetValue{
						{
							Name: "foo.bar[0]",
							Values: []string{
								"A",
								"B",
							},
						},
					},
				},
			},
			helm: &exectest.Helm{},
			wantReleases: []exectest.Release{
				{Name: "releaseName", Flags: []string{"--set", "foo.bar[0]={A,B}", "--reset-values"}},
			},
		},
	}
	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			state := &HelmState{
				ReleaseSetSpec: ReleaseSetSpec{
					Releases: tt.releases,
				},
				logger:         logger,
				valsRuntime:    valsRuntime,
				RenderedValues: map[string]any{},
			}
			_, errs := state.DiffReleases(tt.helm, []string{}, 1, false, false, false, []string{}, false, false, false, false, false)
			if len(errs) > 0 {
				t.Errorf("unexpected error: %v", errs)
			}
			if !reflect.DeepEqual(tt.helm.Diffed, tt.wantReleases) {
				t.Errorf("HelmState.DiffReleases() for [%s] = %v, want %v", tt.name, tt.helm.Releases, tt.wantReleases)
			}
		})
	}
}

func TestHelmState_DiffFlags(t *testing.T) {
	tests := []struct {
		name          string
		releases      []ReleaseSpec
		helm          *exectest.Helm
		wantDiffFlags []string
	}{
		{
			name: "release with api version and kubeversion",
			releases: []ReleaseSpec{
				{
					Name:        "releaseName",
					Chart:       "foo",
					KubeVersion: "1.21",
					ApiVersions: []string{"helmfile.test/v1", "helmfile.test/v2"},
				},
			},
			helm:          &exectest.Helm{},
			wantDiffFlags: []string{"--api-versions", "helmfile.test/v1", "--api-versions", "helmfile.test/v2", "--kube-version", "1.21"},
		},
	}
	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			state := &HelmState{
				ReleaseSetSpec: ReleaseSetSpec{
					Releases: tt.releases,
				},
				logger:         logger,
				valsRuntime:    valsRuntime,
				RenderedValues: map[string]any{},
			}
			for j := range tt.releases {
				flags, _, errs := state.flagsForDiff(tt.helm, &tt.releases[j], false, 1, nil)
				if errs != nil {
					t.Errorf("unexpected error: %v", errs)
				}
				if !reflect.DeepEqual(flags, tt.wantDiffFlags) {
					t.Errorf("HelmState.flagsForDiff() for [%s][%s] = %v, want %v", tt.name, tt.releases[j].Name, flags, tt.wantDiffFlags)
				}
			}
		})
	}
}

func TestHelmState_SyncReleasesCleanup(t *testing.T) {
	tests := []struct {
		name                    string
		releases                []ReleaseSpec
		helm                    *exectest.Helm
		expectedNumRemovedFiles int
	}{
		{
			name: "normal release",
			releases: []ReleaseSpec{
				{
					Name:  "releaseName",
					Chart: "foo",
				},
			},
			helm:                    &exectest.Helm{},
			expectedNumRemovedFiles: 0,
		},
		{
			name: "inline values",
			releases: []ReleaseSpec{
				{
					Name:  "releaseName",
					Chart: "foo",
					Values: []any{
						map[any]any{
							"someList": "a,b,c",
						},
					},
				},
			},
			helm:                    &exectest.Helm{},
			expectedNumRemovedFiles: 1,
		},
		{
			name: "inline values and values file",
			releases: []ReleaseSpec{
				{
					Name:  "releaseName",
					Chart: "foo",
					Values: []any{
						map[any]any{
							"someList": "a,b,c",
						},
						"someFile",
					},
				},
			},
			helm:                    &exectest.Helm{},
			expectedNumRemovedFiles: 2,
		},
	}
	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			numRemovedFiles := 0
			state := &HelmState{
				ReleaseSetSpec: ReleaseSetSpec{
					Releases: tt.releases,
				},
				logger:         logger,
				valsRuntime:    valsRuntime,
				RenderedValues: map[string]any{},
			}
			testfs := testhelper.NewTestFs(map[string]string{
				"/path/to/someFile": `foo: FOO`,
			})
			testfs.DeleteFile = func(f string) error {
				numRemovedFiles += 1
				return nil
			}
			state = injectFs(state, testfs)
			if errs := state.SyncReleases(&AffectedReleases{}, tt.helm, []string{}, 1); len(errs) > 0 {
				t.Errorf("unexpected errors: %v", errs)
			}

			if errs := state.Clean(); len(errs) > 0 {
				t.Errorf("unexpected errors: %v", errs)
			}

			if numRemovedFiles != tt.expectedNumRemovedFiles {
				t.Errorf("unexpected number of removed files: expected %d, got %d", tt.expectedNumRemovedFiles, numRemovedFiles)
			}
		})
	}
}

func TestHelmState_DiffReleasesCleanup(t *testing.T) {
	tests := []struct {
		name                    string
		releases                []ReleaseSpec
		helm                    *exectest.Helm
		expectedNumRemovedFiles int
	}{
		{
			name: "normal release",
			releases: []ReleaseSpec{
				{
					Name:  "releaseName",
					Chart: "foo",
				},
			},
			helm:                    &exectest.Helm{},
			expectedNumRemovedFiles: 0,
		},
		{
			name: "inline values",
			releases: []ReleaseSpec{
				{
					Name:  "releaseName",
					Chart: "foo",
					Values: []any{
						map[any]any{
							"someList": "a,b,c",
						},
					},
				},
			},
			helm:                    &exectest.Helm{},
			expectedNumRemovedFiles: 1,
		},
		{
			name: "inline values and values file",
			releases: []ReleaseSpec{
				{
					Name:  "releaseName",
					Chart: "foo",
					Values: []any{
						map[any]any{
							"someList": "a,b,c",
						},
						"someFile",
					},
				},
			},
			helm:                    &exectest.Helm{},
			expectedNumRemovedFiles: 2,
		},
	}
	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			numRemovedFiles := 0
			state := &HelmState{
				ReleaseSetSpec: ReleaseSetSpec{
					Releases: tt.releases,
				},
				logger:         logger,
				valsRuntime:    valsRuntime,
				RenderedValues: map[string]any{},
			}
			testfs := testhelper.NewTestFs(map[string]string{
				"/path/to/someFile": `foo: bar
`,
			})
			testfs.DeleteFile = func(f string) error {
				numRemovedFiles += 1
				return nil
			}
			state = injectFs(state, testfs)
			if _, errs := state.DiffReleases(tt.helm, []string{}, 1, false, false, false, []string{}, false, false, false, false, false); len(errs) > 0 {
				t.Errorf("unexpected errors: %v", errs)
			}

			if errs := state.Clean(); len(errs) > 0 {
				t.Errorf("unexpected errors: %v", errs)
			}

			if numRemovedFiles != tt.expectedNumRemovedFiles {
				t.Errorf("unexpected number of removed files: expected %d, got %d", tt.expectedNumRemovedFiles, numRemovedFiles)
			}
		})
	}
}

func TestHelmState_UpdateDeps(t *testing.T) {
	helm := &exectest.Helm{
		UpdateDepsCallbacks: map[string]func(string) error{},
		Helm3:               true,
	}

	var generatedDir string
	tempDir := func(dir, prefix string) (string, error) {
		var err error
		generatedDir, err = os.MkdirTemp(dir, prefix)
		if err != nil {
			return "", err
		}
		// nolint: unparam
		helm.UpdateDepsCallbacks[generatedDir] = func(chart string) error {
			content := []byte(`dependencies:
- name: envoy
  repository: https://kubernetes-charts.storage.googleapis.com
  version: 1.5.0
- name: envoy
  repository: https://kubernetes-charts.storage.googleapis.com
  version: 1.4.0
digest: sha256:8194b597c85bb3d1fee8476d4a486e952681d5c65f185ad5809f2118bc4079b5
generated: 2019-05-16T15:42:45.50486+09:00
`)
			filename := filepath.Join(generatedDir, "Chart.lock")
			logger.Debugf("test: writing %s: %s", filename, content)
			return os.WriteFile(filename, content, 0644)
		}
		return generatedDir, nil
	}

	logger := helmexec.NewLogger(io.Discard, "debug")
	basePath := "/src"
	state := &HelmState{
		basePath: basePath,
		FilePath: "/src/helmfile.yaml",
		ReleaseSetSpec: ReleaseSetSpec{
			Releases: []ReleaseSpec{
				{
					Chart: "/example",
				},
				{
					Chart: "./example",
				},
				{
					Chart: "published/deeper",
				},
				{
					Chart:   "stable/envoy",
					Version: "1.5.0",
				},
				{
					Chart:   "stable/envoy",
					Version: "1.4.0",
				},
			},
			Repositories: []RepositorySpec{
				{
					Name: "stable",
					URL:  "https://kubernetes-charts.storage.googleapis.com",
				},
			},
		},
		tempDir: tempDir,
		logger:  logger,
	}

	fs := testhelper.NewTestFs(map[string]string{
		"/example/Chart.yaml":     `foo: FOO`,
		"/src/example/Chart.yaml": `foo: FOO`,
	})
	fs.Cwd = basePath
	state = injectFs(state, fs)
	errs := state.UpdateDeps(helm, false)

	want := []string{"/example", "./example", generatedDir}
	if !reflect.DeepEqual(helm.Charts, want) {
		t.Errorf("HelmState.UpdateDeps() = %v, want %v", helm.Charts, want)
	}
	if len(errs) != 0 {
		t.Errorf("HelmState.UpdateDeps() - unexpected %d errors: %v", len(errs), errs)
	}

	resolved, err := state.ResolveDeps()
	if err != nil {
		t.Errorf("HelmState.ResolveDeps() - unexpected error: %v", err)
	}

	if resolved.Releases[3].Version != "1.5.0" {
		t.Errorf("HelmState.ResolveDeps() - unexpected version number: expected=1.5.0, got=%s", resolved.Releases[5].Version)
	}
	if resolved.Releases[4].Version != "1.4.0" {
		t.Errorf("HelmState.ResolveDeps() - unexpected version number: expected=1.4.0, got=%s", resolved.Releases[6].Version)
	}
}

func TestHelmState_ResolveDeps_NoLockFile(t *testing.T) {
	logger := helmexec.NewLogger(io.Discard, "debug")
	state := &HelmState{
		basePath: "/src",
		FilePath: "/src/helmfile.yaml",
		ReleaseSetSpec: ReleaseSetSpec{
			Releases: []ReleaseSpec{
				{
					Chart: "./..",
				},
				{
					Chart: "../examples",
				},
				{
					Chart: "../../helmfile",
				},
				{
					Chart: "published",
				},
				{
					Chart: "published/deeper",
				},
				{
					Chart: "stable/envoy",
				},
			},
			Repositories: []RepositorySpec{
				{
					Name: "stable",
					URL:  "https://kubernetes-charts.storage.googleapis.com",
				},
			},
		},
		logger: logger,
		fs: &filesystem.FileSystem{
			ReadFile: func(f string) ([]byte, error) {
				if f != "helmfile.lock" {
					return nil, fmt.Errorf("stub: unexpected file: %s", f)
				}
				return nil, os.ErrNotExist
			},
		},
	}

	_, err := state.ResolveDeps()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHelmState_ResolveDeps_NoLockFile_WithCustomLockFile(t *testing.T) {
	logger := helmexec.NewLogger(io.Discard, "debug")
	state := &HelmState{
		basePath: "/src",
		FilePath: "/src/helmfile.yaml",
		ReleaseSetSpec: ReleaseSetSpec{
			LockFile: "custom-lock-file",
			Releases: []ReleaseSpec{
				{
					Chart: "./..",
				},
				{
					Chart: "../examples",
				},
				{
					Chart: "../../helmfile",
				},
				{
					Chart: "published",
				},
				{
					Chart: "published/deeper",
				},
				{
					Chart: "stable/envoy",
				},
			},
			Repositories: []RepositorySpec{
				{
					Name: "stable",
					URL:  "https://kubernetes-charts.storage.googleapis.com",
				},
			},
		},
		logger: logger,
		fs: &filesystem.FileSystem{
			ReadFile: func(f string) ([]byte, error) {
				if f != "custom-lock-file" {
					return nil, fmt.Errorf("stub: unexpected file: %s", f)
				}
				return nil, os.ErrNotExist
			},
		},
	}

	_, err := state.ResolveDeps()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
func TestHelmState_ReleaseStatuses(t *testing.T) {
	tests := []struct {
		name     string
		releases []ReleaseSpec
		helm     *exectest.Helm
		want     []exectest.Release
		wantErr  bool
	}{
		{
			name: "happy path",
			releases: []ReleaseSpec{
				{
					Name: "releaseA",
				},
			},
			helm: &exectest.Helm{},
			want: []exectest.Release{
				{Name: "releaseA", Flags: []string{}},
			},
		},
		{
			name: "happy path",
			releases: []ReleaseSpec{
				{
					Name: "error",
				},
			},
			helm:    &exectest.Helm{},
			wantErr: true,
		},
		{
			name: "complain missing values file for desired release",
			releases: []ReleaseSpec{
				{
					Name: "error",
					Values: []any{
						"foo.yaml",
					},
				},
			},
			helm:    &exectest.Helm{},
			wantErr: true,
		},
		{
			name: "should not complain missing values file for undesired release",
			releases: []ReleaseSpec{
				{
					Name: "error",
					Values: []any{
						"foo.yaml",
					},
					Installed: boolValue(false),
				},
			},
			helm:    &exectest.Helm{},
			wantErr: false,
		},
	}
	for i := range tests {
		tt := tests[i]
		f := func(t *testing.T) {
			state := &HelmState{
				ReleaseSetSpec: ReleaseSetSpec{
					Releases: tt.releases,
				},
				logger: logger,
				fs: &filesystem.FileSystem{
					FileExists: func(f string) (bool, error) {
						if f != "foo.yaml" {
							return false, fmt.Errorf("unexpected file: %s", f)
						}
						return true, nil
					},
					ReadFile: func(f string) ([]byte, error) {
						if f != "foo.yaml" {
							return nil, fmt.Errorf("unexpected file: %s", f)
						}
						return []byte{}, nil
					},
				},
			}
			errs := state.ReleaseStatuses(tt.helm, 1)
			if (errs != nil) != tt.wantErr {
				t.Errorf("ReleaseStatuses() for %s error = %v, wantErr %v", tt.name, errs, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(tt.helm.Releases, tt.want) {
				t.Errorf("HelmState.ReleaseStatuses() for [%s] = %v, want %v", tt.name, tt.helm.Releases, tt.want)
			}
		}
		t.Run(tt.name, f)
	}
}

func TestHelmState_TestReleasesNoCleanUp(t *testing.T) {
	tests := []struct {
		name     string
		cleanup  bool
		releases []ReleaseSpec
		helm     *exectest.Helm
		want     []exectest.Release
		wantErr  bool
	}{
		{
			name: "happy path",
			releases: []ReleaseSpec{
				{
					Name: "releaseA",
				},
			},
			helm: &exectest.Helm{},
			want: []exectest.Release{{Name: "releaseA", Flags: []string{"--timeout", "1s"}}},
		},
		{
			name:    "do cleanup",
			cleanup: true,
			releases: []ReleaseSpec{
				{
					Name: "releaseB",
				},
			},
			helm: &exectest.Helm{},
			want: []exectest.Release{{Name: "releaseB", Flags: []string{"--timeout", "1s"}}},
		},
		{
			name: "happy path",
			releases: []ReleaseSpec{
				{
					Name: "error",
				},
			},
			helm:    &exectest.Helm{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &HelmState{
				ReleaseSetSpec: ReleaseSetSpec{
					Releases: tt.releases,
				},
				logger: logger,
			}
			errs := state.TestReleases(tt.helm, tt.cleanup, 1, 1)
			if (errs != nil) != tt.wantErr {
				t.Errorf("TestReleases() for %s error = %v, wantErr %v", tt.name, errs, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(tt.helm.Releases, tt.want) {
				t.Errorf("HelmState.TestReleases() for [%s] = %v, want %v", tt.name, tt.helm.Releases, tt.want)
			}
		})
	}
}

func TestConditionEnabled(t *testing.T) {
	tests := []struct {
		name      string
		condition string
		values    map[string]any
		want      bool
		wantErr   bool
		wantPanic bool
	}{
		{
			name:      "enabled",
			condition: "foo.enabled",
			values: map[string]any{
				"foo": map[string]any{
					"enabled": true,
				},
			},
			want: true,
		},
		{
			name:      "disabled",
			condition: "foo.enabled",
			values: map[string]any{
				"foo": map[string]any{
					"enabled": false,
				},
			},
			want: false,
		},
		{
			name:      "missing enabled",
			condition: "foo.enabled",
			values: map[string]any{
				"foo": map[string]any{
					"something else": false,
				},
			},
			want: false,
		},
		{
			name:      "foo nil",
			condition: "foo.enabled",
			values: map[string]any{
				"foo": nil,
			},
			wantPanic: true,
		},
		{
			name:      "foo missing",
			condition: "foo.enabled",
			values:    map[string]any{},
			wantPanic: true,
		},
		{
			name:      "wrong suffix",
			condition: "services.foo_enabled",
			want:      false,
			wantErr:   true,
		},
		{
			name:      "too short condition",
			condition: "rnd42",
			want:      false,
			wantErr:   true,
		},
		{
			name:      "too long condition",
			condition: "rnd42.really.enabled",
			want:      false,
			wantErr:   true,
		},
		{
			name:      "empty",
			condition: "",
			want:      true,
		},
	}
	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantPanic {
				defer func() {
					if r := recover(); r == nil {
						t.Errorf("ConditionEnabled() for %s expected panic", tt.name)
					}
				}()
			}
			res, err := ConditionEnabled(ReleaseSpec{Condition: tt.condition}, tt.values)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ConditionEnabled() for %s expected err response", tt.name)
				}
				return
			}
			if err != nil {
				t.Errorf("ConditionEnabled() for %s unexpected err %v", tt.name, err)
			}
			if res != tt.want {
				t.Errorf("ConditionEnabled() for %s = %v, want %v", tt.name, res, tt.want)
			}
		})
	}
}

func TestHelmState_NoReleaseMatched(t *testing.T) {
	releases := []ReleaseSpec{
		{
			Name: "releaseA",
			Labels: map[string]string{
				"foo": "bar",
			},
		},
	}
	tests := []struct {
		name    string
		labels  string
		wantErr bool
	}{
		{
			name: "happy path",

			labels:  "foo=bar",
			wantErr: false,
		},
		{
			name:    "name does not exist",
			labels:  "name=releaseB",
			wantErr: false,
		},
		{
			name:    "label does not match anything",
			labels:  "foo=notbar",
			wantErr: false,
		},
	}
	for i := range tests {
		tt := tests[i]
		f := func(t *testing.T) {
			state := &HelmState{
				ReleaseSetSpec: ReleaseSetSpec{
					Releases: releases,
				},
				logger:         logger,
				RenderedValues: map[string]any{},
			}
			state.Selectors = []string{tt.labels}
			errs := state.FilterReleases(false)
			if (errs != nil) != tt.wantErr {
				t.Errorf("ReleaseStatuses() for %s error = %v, wantErr %v", tt.name, errs, tt.wantErr)
				return
			}
		}
		t.Run(tt.name, f)
	}
}

func TestHelmState_Delete(t *testing.T) {
	tests := []struct {
		name           string
		deleted        []exectest.Release
		wantErr        bool
		desired        *bool
		installed      bool
		purge          bool
		flags          string
		namespace      string
		kubeContext    string
		defKubeContext string
	}{
		{
			name:      "desired and installed (purge=false)",
			wantErr:   false,
			desired:   boolValue(true),
			installed: true,
			purge:     false,
			deleted:   []exectest.Release{{Name: "releaseA", Flags: []string{}}},
		},
		{
			name:      "desired(default) and installed (purge=false)",
			wantErr:   false,
			desired:   nil,
			installed: true,
			purge:     false,
			deleted:   []exectest.Release{{Name: "releaseA", Flags: []string{}}},
		},
		{
			name:      "desired(default) and installed (purge=false) but error",
			wantErr:   true,
			desired:   nil,
			installed: true,
			purge:     false,
			deleted:   []exectest.Release{{Name: "releaseA", Flags: []string{}}},
		},
		{
			name:      "desired and installed (purge=true)",
			wantErr:   false,
			desired:   boolValue(true),
			installed: true,
			purge:     true,
			deleted:   []exectest.Release{{Name: "releaseA", Flags: []string{}}},
		},
		{
			name:      "desired but not installed (purge=false)",
			wantErr:   false,
			desired:   boolValue(true),
			installed: false,
			purge:     false,
			deleted:   []exectest.Release{{Name: "releaseA", Flags: []string{}}},
		},
		{
			name:      "desired but not installed (purge=true)",
			wantErr:   false,
			desired:   boolValue(true),
			installed: false,
			purge:     true,
			deleted:   []exectest.Release{{Name: "releaseA", Flags: []string{}}},
		},
		{
			name:      "installed but filtered (purge=false)",
			wantErr:   false,
			desired:   boolValue(false),
			installed: true,
			purge:     false,
			deleted:   []exectest.Release{{Name: "releaseA", Flags: []string{}}},
		},
		{
			name:      "installed but filtered (purge=true)",
			wantErr:   false,
			desired:   boolValue(false),
			installed: true,
			purge:     true,
			deleted:   []exectest.Release{{Name: "releaseA", Flags: []string{}}},
		},
		{
			name:      "not installed, and filtered (purge=false)",
			wantErr:   false,
			desired:   boolValue(false),
			installed: false,
			purge:     false,
			deleted:   []exectest.Release{{Name: "releaseA", Flags: []string{}}},
		},
		{
			name:      "not installed, and filtered (purge=true)",
			wantErr:   false,
			desired:   boolValue(false),
			installed: false,
			purge:     true,
			deleted:   []exectest.Release{{Name: "releaseA", Flags: []string{}}},
		},
		{
			name:        "with kubecontext",
			wantErr:     false,
			desired:     nil,
			installed:   true,
			purge:       true,
			kubeContext: "ctx",
			flags:       "--kube-contextctx",
			deleted:     []exectest.Release{{Name: "releaseA", Flags: []string{"--kube-context", "ctx"}}},
		},
		{
			name:           "with default kubecontext",
			wantErr:        false,
			desired:        nil,
			installed:      true,
			purge:          true,
			defKubeContext: "defctx",
			flags:          "--kube-contextdefctx",
			deleted:        []exectest.Release{{Name: "releaseA", Flags: []string{"--kube-context", "defctx"}}},
		},
		{
			name:           "with non-default and default kubecontexts",
			wantErr:        false,
			desired:        nil,
			installed:      true,
			purge:          true,
			kubeContext:    "ctx",
			defKubeContext: "defctx",
			flags:          "--kube-contextctx",
			deleted:        []exectest.Release{{Name: "releaseA", Flags: []string{"--kube-context", "ctx"}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name := "releaseA"
			if tt.wantErr {
				name = "releaseA-error"
			}
			release := ReleaseSpec{
				Name:        name,
				Installed:   tt.desired,
				Namespace:   tt.namespace,
				KubeContext: tt.kubeContext,
			}
			releases := []ReleaseSpec{
				release,
			}
			state := &HelmState{
				ReleaseSetSpec: ReleaseSetSpec{
					HelmDefaults: HelmSpec{
						KubeContext: tt.defKubeContext,
					},
					Releases: releases,
				},
				logger:         logger,
				RenderedValues: map[string]any{},
			}
			helm := &exectest.Helm{
				Lists:   map[exectest.ListKey]string{},
				Deleted: []exectest.Release{},
				Helm3:   true,
			}
			if tt.installed {
				helm.Lists[exectest.ListKey{Filter: "^" + name + "$", Flags: tt.flags}] = name
			}
			affectedReleases := AffectedReleases{}
			errs := state.DeleteReleases(&affectedReleases, helm, 1, tt.purge, "")
			if errs != nil {
				if !tt.wantErr || len(affectedReleases.Failed) != 1 || affectedReleases.Failed[0].Name != release.Name {
					t.Errorf("DeleteReleases() for %s error = %v, wantErr %v", tt.name, errs, tt.wantErr)
					return
				}
			} else if !(reflect.DeepEqual(tt.deleted, helm.Deleted) && (len(affectedReleases.Deleted) == len(tt.deleted))) {
				t.Errorf("unexpected deletions happened: expected %v, got %v", tt.deleted, helm.Deleted)
			}
		})
	}
}

func TestDiffpareSyncReleases(t *testing.T) {
	tests := []struct {
		name         string
		flags        []string
		diffOptions  *DiffOpts
		helmDefaults *HelmSpec
	}{
		{
			name:  "reuse-values",
			flags: []string{"--reuse-values"},
			diffOptions: &DiffOpts{
				ReuseValues: true,
			},
			helmDefaults: &HelmSpec{},
		},
		{
			name:         "reset-values",
			flags:        []string{"--reset-values"},
			diffOptions:  &DiffOpts{},
			helmDefaults: &HelmSpec{},
		},
		{
			name:        "default-reuse-values",
			flags:       []string{"--reuse-values"},
			diffOptions: &DiffOpts{},
			helmDefaults: &HelmSpec{
				ReuseValues: true,
			},
		},
		{
			name:  "force-reset-values",
			flags: []string{"--reset-values"},
			diffOptions: &DiffOpts{
				ResetValues: true,
			},
			helmDefaults: &HelmSpec{
				ReuseValues: true,
			},
		},
		{
			name:  "both-reset-reuse-values",
			flags: []string{"--reset-values"},
			diffOptions: &DiffOpts{
				ReuseValues: true,
				ResetValues: true,
			},
			helmDefaults: &HelmSpec{},
		},
		{
			name:  "both-reset-reuse-default-reuse-values",
			flags: []string{"--reset-values"},
			diffOptions: &DiffOpts{
				ReuseValues: true,
				ResetValues: true,
			},
			helmDefaults: &HelmSpec{
				ReuseValues: true,
			},
		},
	}

	for _, tt := range tests {
		release := ReleaseSpec{
			Name: tt.name,
		}
		releases := []ReleaseSpec{
			release,
		}
		state := &HelmState{
			ReleaseSetSpec: ReleaseSetSpec{
				Releases:     releases,
				HelmDefaults: *tt.helmDefaults,
			},
			logger:      logger,
			valsRuntime: valsRuntime,
		}
		helm := &exectest.Helm{
			Lists: map[exectest.ListKey]string{},
			Helm3: true,
		}
		results, es := state.prepareDiffReleases(helm, []string{}, 1, false, false, false, []string{}, false, false, false, tt.diffOptions)

		require.Len(t, es, 0)
		require.Len(t, results, 1)

		r := results[0]

		require.Equal(t, tt.flags, r.flags)
	}
}

func TestPrepareSyncReleases(t *testing.T) {
	tests := []struct {
		name         string
		flags        []string
		syncOptions  *SyncOpts
		helmDefaults *HelmSpec
	}{
		{
			name:  "reuse-values",
			flags: []string{"--reuse-values"},
			syncOptions: &SyncOpts{
				ReuseValues: true,
			},
			helmDefaults: &HelmSpec{},
		},
		{
			name:         "reset-values",
			flags:        []string{"--reset-values"},
			syncOptions:  &SyncOpts{},
			helmDefaults: &HelmSpec{},
		},
		{
			name:        "reuse-default-values",
			flags:       []string{"--reuse-values"},
			syncOptions: &SyncOpts{},
			helmDefaults: &HelmSpec{
				ReuseValues: true,
			},
		},
		{
			name:  "force-reset-values",
			flags: []string{"--reset-values"},
			syncOptions: &SyncOpts{
				ResetValues: true,
			},
			helmDefaults: &HelmSpec{
				ReuseValues: true,
			},
		},
		{
			name:  "both-reset-reuse-values",
			flags: []string{"--reset-values"},
			syncOptions: &SyncOpts{
				ReuseValues: true,
				ResetValues: true,
			},
			helmDefaults: &HelmSpec{},
		},
		{
			name:  "both-reset-reuse-default-reuse-values",
			flags: []string{"--reset-values"},
			syncOptions: &SyncOpts{
				ReuseValues: true,
				ResetValues: true,
			},
			helmDefaults: &HelmSpec{
				ReuseValues: true,
			},
		},
	}

	for _, tt := range tests {
		release := ReleaseSpec{
			Name: tt.name,
		}
		releases := []ReleaseSpec{
			release,
		}
		state := &HelmState{
			ReleaseSetSpec: ReleaseSetSpec{
				Releases:     releases,
				HelmDefaults: *tt.helmDefaults,
			},
			logger:      logger,
			valsRuntime: valsRuntime,
		}
		helm := &exectest.Helm{
			Lists: map[exectest.ListKey]string{},
			Helm3: true,
		}
		results, es := state.prepareSyncReleases(helm, []string{}, 1, tt.syncOptions)

		require.Len(t, es, 0)
		require.Len(t, results, 1)

		r := results[0]

		require.Equal(t, tt.flags, r.flags)
	}
}

func TestReverse(t *testing.T) {
	num := 8
	st := &HelmState{}

	for i := 0; i < num; i++ {
		name := fmt.Sprintf("%d", i)
		st.Helmfiles = append(st.Helmfiles, SubHelmfileSpec{
			Path: name,
		})
		st.Releases = append(st.Releases, ReleaseSpec{
			Name: name,
		})
	}

	st.Reverse()

	for i := 0; i < num; i++ {
		j := num - 1 - i
		want := fmt.Sprintf("%d", j)

		if got := st.Helmfiles[i].Path; got != want {
			t.Errorf("sub-helmfile at %d has incorrect path: want %q, got %q", i, want, got)
		}

		if got := st.Releases[i].Name; got != want {
			t.Errorf("release at %d has incorrect name: want %q, got %q", i, want, got)
		}
	}
}

func Test_gatherUsernamePassword(t *testing.T) {
	type args struct {
		repoName string
		username string
		password string
	}
	tests := []struct {
		name             string
		args             args
		envUsernameKey   string
		envUsernameValue string
		envPasswordKey   string
		envPasswordValue string
		wantUsername     string
		wantPassword     string
	}{
		{
			name: "pass username/password from args",
			args: args{
				repoName: "myRegistry",
				username: "username1",
				password: "password1",
			},
			wantUsername: "username1",
			wantPassword: "password1",
		},
		{
			name: "repoName does not contain hyphen, read username/password from environment variables",
			args: args{
				repoName: "myRegistry",
			},
			envUsernameKey:   "MYREGISTRY_USERNAME",
			envUsernameValue: "username2",
			envPasswordKey:   "MYREGISTRY_PASSWORD",
			envPasswordValue: "password2",
			wantUsername:     "username2",
			wantPassword:     "password2",
		},
		{
			name: "repoName contain hyphen, read username/password from environment variables",
			args: args{
				repoName: "my-registry",
			},
			envUsernameKey:   "MY_REGISTRY_USERNAME",
			envUsernameValue: "username3",
			envPasswordKey:   "MY_REGISTRY_PASSWORD",
			envPasswordValue: "password3",
			wantUsername:     "username3",
			wantPassword:     "password3",
		},
	}
	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			if tt.envUsernameKey != "" && tt.envUsernameValue != "" {
				t.Setenv(tt.envUsernameKey, tt.envUsernameValue)
			}
			if tt.envPasswordKey != "" && tt.envPasswordValue != "" {
				t.Setenv(tt.envPasswordKey, tt.envPasswordValue)
			}

			gotUsername, gotPassword := gatherUsernamePassword(tt.args.repoName, tt.args.username, tt.args.password)
			if gotUsername != tt.wantUsername || gotPassword != tt.wantPassword {
				t.Errorf("gatherUsernamePassword() = got username/password %v/%v, want username/password %v/%v", gotUsername, gotPassword, tt.wantUsername, tt.wantPassword)
			}
		})
	}
}

func TestGenerateOutputFilePath(t *testing.T) {
	tests := []struct {
		envName            string
		filePath           string
		releaseName        string
		outputFileTemplate string
		wantErr            bool
		expected           string
	}{
		{
			envName:            "dev",
			releaseName:        "release1",
			filePath:           "/path/to/helmfile.yaml",
			outputFileTemplate: "helmfile-{{ .Environment.Name }}.yaml",
			expected:           "helmfile-dev.yaml",
		},
		{
			envName:            "error",
			releaseName:        "release2",
			filePath:           "helmfile.yaml",
			outputFileTemplate: "helmfile-{{ .Environment.Name",
			wantErr:            true,
			expected:           "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.envName, func(t *testing.T) {
			st := &HelmState{
				FilePath: tt.envName,
				ReleaseSetSpec: ReleaseSetSpec{
					Env: environment.Environment{
						Name: tt.envName,
					},
				},
			}
			ra := &ReleaseSpec{
				Name: tt.releaseName,
			}
			got, err := st.GenerateOutputFilePath(ra, tt.outputFileTemplate)

			if tt.wantErr {
				require.Errorf(t, err, "GenerateOutputFilePath() error = %v, want error", err)
			} else {
				require.NoError(t, err, "GenerateOutputFilePath() error = %v, want nil", err)
			}
			require.Equalf(t, got, tt.expected, "GenerateOutputFilePath() got = %v, want %v", got, tt.expected)
		})
	}
}

func TestFullFilePath(t *testing.T) {
	fs := testhelper.NewTestFs(map[string]string{})
	tests := []struct {
		basePath string
		filePath string
		fs       *filesystem.FileSystem
		expected string
	}{
		{
			basePath: ".",
			filePath: "helmfile.yaml",
			expected: "helmfile.yaml",
		},
		{
			basePath: "./test-1/",
			filePath: "helmfile.yaml",
			expected: "test-1/helmfile.yaml",
		},
		{
			basePath: "/test-2/",
			filePath: "helmfile.yaml",
			expected: "/test-2/helmfile.yaml",
		},
		{
			basePath: "./test-3/",
			filePath: "helmfile.yaml",
			fs:       fs.ToFileSystem(),
			expected: "/path/to/test-3/helmfile.yaml",
		},
		{
			basePath: "/test-4/",
			filePath: "helmfile.yaml",
			fs:       fs.ToFileSystem(),
			expected: "/path/to/test-4/helmfile.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			st := &HelmState{
				basePath: tt.basePath,
				FilePath: tt.filePath,
				fs:       tt.fs,
			}
			actual, err := st.FullFilePath()
			require.Equalf(t, actual, tt.expected, "FullFilePath() got = %v, want %v", actual, tt.expected)
			require.Equalf(t, err, nil, "error %v", err)
		})
	}
}

func TestGetOCIQualifiedChartName(t *testing.T) {
	tests := []struct {
		state    HelmState
		expected []struct {
			qualifiedChartName string
			chartName          string
			chartVersion       string
		}
		helmVersion string
		wantErr     bool
	}{
		{
			state: HelmState{
				ReleaseSetSpec: ReleaseSetSpec{
					Repositories: []RepositorySpec{},
					Releases: []ReleaseSpec{
						{
							Chart:   "oci://registry/chart-path/chart-name",
							Version: "0.1.2",
						},
					},
				},
			},
			helmVersion: "3.13.2",
			expected: []struct {
				qualifiedChartName string
				chartName          string
				chartVersion       string
			}{
				{"registry/chart-path/chart-name:0.1.2", "chart-name", "0.1.2"},
			},
		},
		{
			state: HelmState{
				ReleaseSetSpec: ReleaseSetSpec{
					Repositories: []RepositorySpec{},
					Releases: []ReleaseSpec{
						{
							Chart:   "oci://registry/chart-path/chart-name",
							Version: "latest",
						},
					},
				},
			},
			helmVersion: "3.13.2",
			wantErr:     true,
		},
		{
			state: HelmState{
				ReleaseSetSpec: ReleaseSetSpec{
					Repositories: []RepositorySpec{},
					Releases: []ReleaseSpec{
						{
							Chart:   "oci://registry/chart-path/chart-name",
							Version: "latest",
						},
					},
				},
			},
			helmVersion: "3.7.0",
		},
		{
			state: HelmState{
				ReleaseSetSpec: ReleaseSetSpec{
					Repositories: []RepositorySpec{
						{
							Name: "oci-repo",
							URL:  "registry/chart-path",
							OCI:  true,
						},
					},
					Releases: []ReleaseSpec{
						{
							Chart:   "oci-repo/chart-name",
							Version: "0.1.2",
						},
					},
				},
			},
			helmVersion: "3.13.2",
			expected: []struct {
				qualifiedChartName string
				chartName          string
				chartVersion       string
			}{
				{"registry/chart-path/chart-name:0.1.2", "chart-name", "0.1.2"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%+v", tt.expected), func(t *testing.T) {
			helm := testutil.NewVersionHelmExec(tt.helmVersion)
			for i, r := range tt.state.Releases {
				qualifiedChartName, chartName, chartVersion, err := tt.state.getOCIQualifiedChartName(&r, helm)
				if tt.wantErr {
					require.Error(t, err, "getOCIQualifiedChartName() error = nil, want error")
					return
				}
				require.NoError(t, err, "getOCIQualifiedChartName() error = %v, want nil", err)
				if len(tt.expected) > 0 {
					require.Equalf(t, qualifiedChartName, tt.expected[i].qualifiedChartName, "qualifiedChartName got = %v, want %v", qualifiedChartName, tt.expected[i].qualifiedChartName)
					require.Equalf(t, chartName, tt.expected[i].chartName, "chartName got = %v, want %v", chartName, tt.expected[i].chartName)
					require.Equalf(t, chartVersion, tt.expected[i].chartVersion, "chartVersion got = %v, want %v", chartVersion, tt.expected[i].chartVersion)
				}
			}
		})
	}
}

func TestGenerateChartPath(t *testing.T) {
	tests := []struct {
		testName          string
		chartName         string
		release           *ReleaseSpec
		outputDir         string
		outputDirTemplate string
		wantErr           bool
		expected          string
	}{
		{
			testName:  "PathGeneratedWithGivenOutputDirAndDefaultReleaseVersion",
			chartName: "chart-name",
			release:   &ReleaseSpec{Name: "release-name"},
			outputDir: "/output-dir",
			wantErr:   false,
			expected:  "/output-dir/release-name/chart-name/latest",
		},
		{
			testName:  "PathGeneratedWithGivenOutputDirAndGivenReleaseVersion",
			chartName: "chart-name",
			release:   &ReleaseSpec{Name: "release-name", Version: "0.0.0"},
			outputDir: "/output-dir",
			wantErr:   false,
			expected:  "/output-dir/release-name/chart-name/0.0.0",
		},
		{
			testName:  "PathGeneratedWithGivenOutputDirAndGivenReleaseNamespace",
			chartName: "chart-name",
			release:   &ReleaseSpec{Name: "release-name", Namespace: "release-namespace"},
			outputDir: "/output-dir",
			wantErr:   false,
			expected:  "/output-dir/release-namespace/release-name/chart-name/latest",
		},
		{
			testName:  "PathGeneratedWithGivenOutputDirAndGivenReleaseKubeContext",
			chartName: "chart-name",
			release:   &ReleaseSpec{Name: "release-name", KubeContext: "kube-context"},
			outputDir: "/output-dir",
			wantErr:   false,
			expected:  "/output-dir/kube-context/release-name/chart-name/latest",
		},
		{
			testName:  "PathGeneratedWithGivenOutputDirAndGivenReleaseNamespaceAndGivenReleaseKubeContext",
			chartName: "chart-name",
			release:   &ReleaseSpec{Name: "release-name", Namespace: "release-namespace", KubeContext: "kube-context"},
			outputDir: "/output-dir",
			wantErr:   false,
			expected:  "/output-dir/release-namespace/kube-context/release-name/chart-name/latest",
		},
		{
			testName:          "PathGeneratedWithGivenOutputDirAndGivenOutputDirTemplateWithFieldNameOutputDir",
			chartName:         "chart-name",
			release:           &ReleaseSpec{Name: "release-name"},
			outputDir:         "/output-dir",
			outputDirTemplate: "{{ .OutputDir }}",
			wantErr:           false,
			expected:          "/output-dir",
		},
		{
			testName:          "PathGeneratedWithGivenOutputDirAndGivenOutputDirTemplateWithFieldNamesOutputDirAndReleaseName",
			chartName:         "chart-name",
			release:           &ReleaseSpec{Name: "release-name"},
			outputDir:         "/output-dir",
			outputDirTemplate: "{{ .OutputDir }}/{{ .Release.Name }}",
			wantErr:           false,
			expected:          "/output-dir/release-name",
		},
		{
			testName:          "PathGeneratedWithGivenOutputDirTemplateWithFieldNamesOutputDir",
			chartName:         "chart-name",
			release:           &ReleaseSpec{Name: "release-name"},
			outputDirTemplate: "{{ .OutputDir }}",
			wantErr:           false,
			expected:          "",
		},
		{
			testName:          "PathGeneratedWithGivenOutputDirTemplateWithFieldNameReleaseName",
			chartName:         "chart-name",
			release:           &ReleaseSpec{Name: "release-name"},
			outputDirTemplate: "{{ .Release.Name }}",
			wantErr:           false,
			expected:          "release-name",
		},
		{
			testName:          "PathGeneratedWithGivenOutputDirTemplateWithStringAndFieldNameReleaseName",
			chartName:         "chart-name",
			release:           &ReleaseSpec{Name: "release-name"},
			outputDirTemplate: "./charts/{{ .Release.Name }}",
			wantErr:           false,
			expected:          "./charts/release-name",
		},
		{
			testName:          "ErrorReturnedWithGivenInvalidOutputDirTemplate",
			chartName:         "chart-name",
			release:           &ReleaseSpec{Name: "release-name"},
			outputDirTemplate: "{{ .OutputDir }",
			wantErr:           true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			got, err := generateChartPath(tt.chartName, tt.outputDir, tt.release, tt.outputDirTemplate)

			if tt.wantErr {
				require.Errorf(t, err, "GenerateChartPath() error \"%v\", want error", err)
			} else {
				require.NoError(t, err, "GenerateChartPath() error \"%v\", want no error", err)
			}
			require.Equalf(t, tt.expected, got, "GenerateChartPath() got \"%v\", want \"%v\"", got, tt.expected)
		})
	}
}

func TestCommonDiffFlags(t *testing.T) {
	tests := []struct {
		name string
		// stripTrailingCR is a flag to strip trailing carriage returns from the output
		stripTrailingCR bool
		expected        []string
	}{
		{
			name:            "stripTrailingCR enabled",
			stripTrailingCR: true,
			expected: []string{
				"--strip-trailing-cr",
				"--reset-values",
			},
		},
		{
			name: "stripTrailingCR disenabled",
			expected: []string{
				"--reset-values",
			},
		},
	}
	for _, tt := range tests {
		st := &HelmState{}
		result := st.commonDiffFlags(false, tt.stripTrailingCR, false, []string{}, false, false, false, &DiffOpts{})

		require.Equal(t, tt.expected, result)
	}
}

func TestAppendChartDownloadTLSFlags(t *testing.T) {
	tests := []struct {
		name                         string
		defaultInsecureSkipTLSVerify bool
		releaseInsecureSkipTLSVerify bool
		expected                     []string
	}{
		{
			name:                         "defaultInsecureSkipTLSVerify is true and releaseInsecureSkipTLSVerify is false",
			defaultInsecureSkipTLSVerify: true,
			releaseInsecureSkipTLSVerify: false,
			expected:                     []string{"--insecure-skip-tls-verify"},
		},
		{
			name:                         "defaultInsecureSkipTLSVerify is false and releaseInsecureSkipTLSVerify is true",
			defaultInsecureSkipTLSVerify: false,
			releaseInsecureSkipTLSVerify: true,
			expected:                     []string{"--insecure-skip-tls-verify"},
		},
		{
			name:                         "defaultInsecureSkipTLSVerify is false and releaseInsecureSkipTLSVerify is false",
			defaultInsecureSkipTLSVerify: false,
			releaseInsecureSkipTLSVerify: false,
			expected:                     []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := &HelmState{}
			release := &ReleaseSpec{}
			st.HelmDefaults.InsecureSkipTLSVerify = tt.defaultInsecureSkipTLSVerify
			release.InsecureSkipTLSVerify = tt.releaseInsecureSkipTLSVerify

			result := st.appendChartDownloadTLSFlags([]string{}, release)

			require.Equal(t, tt.expected, result)
		})
	}
}
