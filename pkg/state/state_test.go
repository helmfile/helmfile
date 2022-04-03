package state

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/roboll/helmfile/pkg/exectest"
	"github.com/roboll/helmfile/pkg/helmexec"
	"github.com/roboll/helmfile/pkg/testhelper"
	"github.com/variantdev/vals"
)

var logger = helmexec.NewLogger(os.Stdout, "warn")
var valsRuntime, _ = vals.New(vals.Options{CacheSize: 32})

func injectFs(st *HelmState, fs *testhelper.TestFs) *HelmState {
	st.glob = fs.Glob
	st.readFile = fs.ReadFile
	st.fileExists = fs.FileExists
	st.directoryExistsAt = fs.DirectoryExistsAt
	return st
}

func TestLabelParsing(t *testing.T) {
	cases := []struct {
		labelString    string
		expectedFilter LabelFilter
		errorExected   bool
	}{
		{"foo=bar", LabelFilter{positiveLabels: [][]string{[]string{"foo", "bar"}}, negativeLabels: [][]string{}}, false},
		{"foo!=bar", LabelFilter{positiveLabels: [][]string{}, negativeLabels: [][]string{[]string{"foo", "bar"}}}, false},
		{"foo!=bar,baz=bat", LabelFilter{positiveLabels: [][]string{[]string{"baz", "bat"}}, negativeLabels: [][]string{[]string{"foo", "bar"}}}, false},
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
		BaseChartPath      string
		Context            string
		DeprecatedReleases []ReleaseSpec
		Namespace          string
		Repositories       []RepositorySpec
		Releases           []ReleaseSpec
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
		BaseChartPath:      ".",
		Context:            "test_context",
		DeprecatedReleases: nil,
		Namespace:          specWithNamespaceFromFields.Namespace,
		Repositories:       nil,
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
			name:   "Has a namespace from flags",
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
	}
	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			state := &HelmState{
				basePath: tt.fields.BaseChartPath,
				ReleaseSetSpec: ReleaseSetSpec{
					DeprecatedContext:  tt.fields.Context,
					DeprecatedReleases: tt.fields.DeprecatedReleases,
					OverrideNamespace:  tt.fields.Namespace,
					Repositories:       tt.fields.Repositories,
					Releases:           tt.fields.Releases,
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
				"--timeout", "123",
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
				"--timeout", "123",
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
			name:     "tiller",
			defaults: HelmSpec{},
			release: &ReleaseSpec{
				Chart:           "test/chart",
				Version:         "0.1",
				Name:            "test-charts",
				TLS:             boolValue(true),
				TillerNamespace: "tiller-system",
				TLSKey:          "key.pem",
				TLSCert:         "cert.pem",
				TLSCACert:       "ca.pem",
			},
			want: []string{
				"--version", "0.1",
				"--tiller-namespace", "tiller-system",
				"--tls",
				"--tls-key", "key.pem",
				"--tls-cert", "cert.pem",
				"--tls-ca-cert", "ca.pem",
			},
		},
		{
			name: "tiller-override-defaults",
			defaults: HelmSpec{
				TLS:             false,
				TillerNamespace: "a",
				TLSKey:          "b.pem",
				TLSCert:         "c.pem",
				TLSCACert:       "d.pem",
			},
			release: &ReleaseSpec{
				Chart:           "test/chart",
				Version:         "0.1",
				Name:            "test-charts",
				TLS:             boolValue(true),
				TillerNamespace: "tiller-system",
				TLSKey:          "key.pem",
				TLSCert:         "cert.pem",
				TLSCACert:       "ca.pem",
			},
			want: []string{
				"--version", "0.1",
				"--tiller-namespace", "tiller-system",
				"--tls",
				"--tls-key", "key.pem",
				"--tls-cert", "cert.pem",
				"--tls-ca-cert", "ca.pem",
			},
		},
		{
			name: "tiller-from-defaults",
			defaults: HelmSpec{
				TLS:             true,
				TillerNamespace: "tiller-system",
				TLSKey:          "key.pem",
				TLSCert:         "cert.pem",
				TLSCACert:       "ca.pem",
			},
			release: &ReleaseSpec{
				Chart:   "test/chart",
				Version: "0.1",
				Name:    "test-charts",
			},
			want: []string{
				"--version", "0.1",
				"--tiller-namespace", "tiller-system",
				"--tls",
				"--tls-key", "key.pem",
				"--tls-cert", "cert.pem",
				"--tls-ca-cert", "ca.pem",
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

			args, _, err := state.flagsForUpgrade(helm, tt.release, 0)
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
					Name:            "name",
					URL:             "http://example.com/",
					CertFile:        "",
					KeyFile:         "",
					Username:        "",
					Password:        "",
					PassCredentials: "",
					SkipTLSVerify:   "",
				},
			},
			helm: &exectest.Helm{},
			want: []string{"name", "http://example.com/", "", "", "", "", "", "", "", ""},
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
			want: []string{"name", "", "", "", "", "", "", "acr", "", ""},
		},
		{
			name: "repository with cert and key",
			repos: []RepositorySpec{
				{
					Name:            "name",
					URL:             "http://example.com/",
					CertFile:        "certfile",
					KeyFile:         "keyfile",
					Username:        "",
					Password:        "",
					PassCredentials: "",
					SkipTLSVerify:   "",
				},
			},
			helm: &exectest.Helm{},
			want: []string{"name", "http://example.com/", "", "certfile", "keyfile", "", "", "", "", ""},
		},
		{
			name: "repository with ca file",
			repos: []RepositorySpec{
				{
					Name:            "name",
					URL:             "http://example.com/",
					CaFile:          "cafile",
					Username:        "",
					Password:        "",
					PassCredentials: "",
					SkipTLSVerify:   "",
				},
			},
			helm: &exectest.Helm{},
			want: []string{"name", "http://example.com/", "cafile", "", "", "", "", "", "", ""},
		},
		{
			name: "repository with username and password",
			repos: []RepositorySpec{
				{
					Name:            "name",
					URL:             "http://example.com/",
					CertFile:        "",
					KeyFile:         "",
					Username:        "example_user",
					Password:        "example_password",
					PassCredentials: "",
					SkipTLSVerify:   "",
				},
			},
			helm: &exectest.Helm{},
			want: []string{"name", "http://example.com/", "", "", "", "example_user", "example_password", "", "", ""},
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
					PassCredentials: "true",
					SkipTLSVerify:   "",
				},
			},
			helm: &exectest.Helm{},
			want: []string{"name", "http://example.com/", "", "", "", "example_user", "example_password", "", "true", ""},
		},
		{
			name: "repository with skip-tls-verify",
			repos: []RepositorySpec{
				{
					Name:            "name",
					URL:             "http://example.com/",
					CertFile:        "",
					KeyFile:         "",
					Username:        "",
					Password:        "",
					PassCredentials: "",
					SkipTLSVerify:   "true",
				},
			},
			helm: &exectest.Helm{},
			want: []string{"name", "http://example.com/", "", "", "", "", "", "", "", "true"},
		},
	}
	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.envs {
				err := os.Setenv(k, v)
				if err != nil {
					t.Error("HelmState.SyncRepos() could not set env var for testing")
				}
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
			wantReleases: []exectest.Release{{Name: "releaseName", Flags: []string{}}},
		},
		{
			name: "with tiller args",
			releases: []ReleaseSpec{
				{
					Name:            "releaseName",
					Chart:           "foo",
					TillerNamespace: "tillerns",
				},
			},
			helm:         &exectest.Helm{},
			wantReleases: []exectest.Release{{Name: "releaseName", Flags: []string{"--tiller-namespace", "tillerns"}}},
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
			wantReleases: []exectest.Release{{Name: "releaseName", Flags: []string{"--set", "someList=a\\,b\\,c", "--set", "json=\\{\"name\": \"john\"\\}"}}},
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
			wantReleases: []exectest.Release{{Name: "releaseName", Flags: []string{"--set", "foo=FOO", "--set-file", "bar=path/to/bar", "--set", "baz=BAZ"}}},
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
			wantReleases: []exectest.Release{{Name: "releaseName", Flags: []string{"--set", "foo.bar[0]={A,B}"}}},
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
				RenderedValues: map[string]interface{}{},
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
				Values: []interface{}{"noexistent.values.yaml"},
			},
			listResult:    ``,
			expectedError: `failed processing release foo: values file matching "noexistent.values.yaml" does not exist in "."`,
		},
		{
			name: "should fail upgrading due to missing values file",
			release: ReleaseSpec{
				Name:   "foo",
				Chart:  "../../foo-bar",
				Values: []interface{}{"noexistent.values.yaml"},
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
				Values:    []interface{}{"noexistent.values.yaml"},
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
				RenderedValues: map[string]interface{}{},
			}
			fs := testhelper.NewTestFs(map[string]string{})
			state = injectFs(state, fs)
			helm := &exectest.Helm{
				Lists: map[exectest.ListKey]string{},
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
	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			state := &HelmState{
				ReleaseSetSpec: ReleaseSetSpec{
					Releases: tt.releases,
				},
				logger:         logger,
				valsRuntime:    valsRuntime,
				RenderedValues: map[string]interface{}{},
			}
			helm := &exectest.Helm{
				Lists: map[exectest.ListKey]string{},
			}
			//simulate the release is already installed
			for i, release := range tt.releases {
				if tt.installed != nil && tt.installed[i] {
					helm.Lists[exectest.ListKey{Filter: "^" + release.Name + "$", Flags: "--deleting--deployed--failed--pending"}] = release.Name
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
	}
	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			state := &HelmState{
				ReleaseSetSpec: ReleaseSetSpec{
					Releases: []ReleaseSpec{tt.release},
				},
				logger:         logger,
				valsRuntime:    valsRuntime,
				RenderedValues: map[string]interface{}{},
			}
			helm := &exectest.Helm{
				Lists: map[exectest.ListKey]string{},
			}
			//simulate the helm.list call result
			helm.Lists[exectest.ListKey{Filter: "^" + tt.release.Name + "$", Flags: "--deleting--deployed--failed--pending"}] = tt.listResult

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
			wantReleases: []exectest.Release{{Name: "releaseName", Flags: []string{}}},
		},
		{
			name: "with tiller args",
			releases: []ReleaseSpec{
				{
					Name:            "releaseName",
					Chart:           "foo",
					TillerNamespace: "tillerns",
				},
			},
			helm:         &exectest.Helm{},
			wantReleases: []exectest.Release{{Name: "releaseName", Flags: []string{"--tiller-namespace", "tillerns"}}},
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
				{Name: "releaseName", Flags: []string{"--set", "someList=a\\,b\\,c", "--set", "json=\\{\"name\": \"john\"\\}"}},
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
				{Name: "releaseName", Flags: []string{"--set", "foo=FOO", "--set-file", "bar=path/to/bar", "--set", "baz=BAZ"}},
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
				{Name: "releaseName", Flags: []string{"--set", "foo.bar[0]={A,B}"}},
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
				RenderedValues: map[string]interface{}{},
			}
			_, errs := state.DiffReleases(tt.helm, []string{}, 1, false, false, []string{}, false, false, false, false)
			if len(errs) > 0 {
				t.Errorf("unexpected error: %v", errs)
			}
			if !reflect.DeepEqual(tt.helm.Diffed, tt.wantReleases) {
				t.Errorf("HelmState.DiffReleases() for [%s] = %v, want %v", tt.name, tt.helm.Releases, tt.wantReleases)
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
					Values: []interface{}{
						map[interface{}]interface{}{
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
					Values: []interface{}{
						map[interface{}]interface{}{
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
				logger:      logger,
				valsRuntime: valsRuntime,
				removeFile: func(f string) error {
					numRemovedFiles += 1
					return nil
				},
				RenderedValues: map[string]interface{}{},
			}
			testfs := testhelper.NewTestFs(map[string]string{
				"/path/to/someFile": `foo: FOO`,
			})
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
					Values: []interface{}{
						map[interface{}]interface{}{
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
					Values: []interface{}{
						map[interface{}]interface{}{
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
				logger:      logger,
				valsRuntime: valsRuntime,
				removeFile: func(f string) error {
					numRemovedFiles += 1
					return nil
				},
				RenderedValues: map[string]interface{}{},
			}
			testfs := testhelper.NewTestFs(map[string]string{
				"/path/to/someFile": `foo: bar
`,
			})
			state = injectFs(state, testfs)
			if _, errs := state.DiffReleases(tt.helm, []string{}, 1, false, false, []string{}, false, false, false, false); len(errs) > 0 {
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
	}

	var generatedDir string
	tempDir := func(dir, prefix string) (string, error) {
		var err error
		generatedDir, err = os.MkdirTemp(dir, prefix)
		if err != nil {
			return "", err
		}
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
			filename := filepath.Join(generatedDir, "requirements.lock")
			logger.Debugf("test: writing %s: %s", filename, content)
			return os.WriteFile(filename, content, 0644)
		}
		return generatedDir, nil
	}

	logger := helmexec.NewLogger(os.Stderr, "debug")
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
	logger := helmexec.NewLogger(os.Stderr, "debug")
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
		readFile: func(f string) ([]byte, error) {
			if f != "helmfile.lock" {
				return nil, fmt.Errorf("stub: unexpected file: %s", f)
			}
			return nil, os.ErrNotExist
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
					Values: []interface{}{
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
					Values: []interface{}{
						"foo.yaml",
					},
					Installed: boolValue(false),
				},
			},
			helm:    &exectest.Helm{},
			wantErr: false,
		},
		{
			name: "with tiller args",
			releases: []ReleaseSpec{
				{
					Name:            "releaseA",
					TillerNamespace: "tillerns",
				},
			},
			helm: &exectest.Helm{},
			want: []exectest.Release{
				{Name: "releaseA", Flags: []string{"--tiller-namespace", "tillerns"}},
			},
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
				fileExists: func(f string) (bool, error) {
					if f != "foo.yaml" {
						return false, fmt.Errorf("unexpected file: %s", f)
					}
					return true, nil
				},
				readFile: func(f string) ([]byte, error) {
					if f != "foo.yaml" {
						return nil, fmt.Errorf("unexpected file: %s", f)
					}
					return []byte{}, nil
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
		name            string
		cleanup         bool
		releases        []ReleaseSpec
		helm            *exectest.Helm
		want            []exectest.Release
		wantErr         bool
		tillerNamespace string
	}{
		{
			name: "happy path",
			releases: []ReleaseSpec{
				{
					Name: "releaseA",
				},
			},
			helm: &exectest.Helm{},
			want: []exectest.Release{{Name: "releaseA", Flags: []string{"--timeout", "1"}}},
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
			want: []exectest.Release{{Name: "releaseB", Flags: []string{"--cleanup", "--timeout", "1"}}},
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
			name: "with tiller args",
			releases: []ReleaseSpec{
				{
					Name:            "releaseA",
					TillerNamespace: "tillerns",
				},
			},
			helm: &exectest.Helm{},
			want: []exectest.Release{{Name: "releaseA", Flags: []string{"--timeout", "1", "--tiller-namespace", "tillerns"}}},
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
			}
			errs := state.TestReleases(tt.helm, tt.cleanup, 1, 1)
			if (errs != nil) != tt.wantErr {
				t.Errorf("TestReleases() for %s error = %v, wantErr %v", tt.name, errs, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(tt.helm.Releases, tt.want) {
				t.Errorf("HelmState.TestReleases() for [%s] = %v, want %v", tt.name, tt.helm.Releases, tt.want)
			}
		}
		t.Run(tt.name, f)
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
				RenderedValues: map[string]interface{}{},
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
		name            string
		deleted         []exectest.Release
		wantErr         bool
		desired         *bool
		installed       bool
		purge           bool
		flags           string
		tillerNamespace string
		kubeContext     string
		defKubeContext  string
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
			deleted:   []exectest.Release{{Name: "releaseA", Flags: []string{"--purge"}}},
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
			deleted:   []exectest.Release{{Name: "releaseA", Flags: []string{"--purge"}}},
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
			deleted:   []exectest.Release{{Name: "releaseA", Flags: []string{"--purge"}}},
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
			deleted:   []exectest.Release{{Name: "releaseA", Flags: []string{"--purge"}}},
		},
		{
			name:            "with tiller args",
			wantErr:         false,
			desired:         nil,
			installed:       true,
			purge:           true,
			tillerNamespace: "tillerns",
			flags:           "--tiller-namespacetillerns",
			deleted:         []exectest.Release{{Name: "releaseA", Flags: []string{"--purge", "--tiller-namespace", "tillerns"}}},
		},
		{
			name:        "with kubecontext",
			wantErr:     false,
			desired:     nil,
			installed:   true,
			purge:       true,
			kubeContext: "ctx",
			flags:       "--kube-contextctx",
			deleted:     []exectest.Release{{Name: "releaseA", Flags: []string{"--purge", "--kube-context", "ctx"}}},
		},
		{
			name:           "with default kubecontext",
			wantErr:        false,
			desired:        nil,
			installed:      true,
			purge:          true,
			defKubeContext: "defctx",
			flags:          "--kube-contextdefctx",
			deleted:        []exectest.Release{{Name: "releaseA", Flags: []string{"--purge", "--kube-context", "defctx"}}},
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
			deleted:        []exectest.Release{{Name: "releaseA", Flags: []string{"--purge", "--kube-context", "ctx"}}},
		},
	}
	for i := range tests {
		tt := tests[i]
		f := func(t *testing.T) {
			name := "releaseA"
			if tt.wantErr {
				name = "releaseA-error"
			}
			release := ReleaseSpec{
				Name:            name,
				Installed:       tt.desired,
				TillerNamespace: tt.tillerNamespace,
				KubeContext:     tt.kubeContext,
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
				RenderedValues: map[string]interface{}{},
			}
			helm := &exectest.Helm{
				Lists:   map[exectest.ListKey]string{},
				Deleted: []exectest.Release{},
			}
			if tt.installed {
				helm.Lists[exectest.ListKey{Filter: "^" + name + "$", Flags: tt.flags}] = name
			}
			affectedReleases := AffectedReleases{}
			errs := state.DeleteReleases(&affectedReleases, helm, 1, tt.purge)
			if errs != nil {
				if !tt.wantErr || len(affectedReleases.Failed) != 1 || affectedReleases.Failed[0].Name != release.Name {
					t.Errorf("DeleteReleases() for %s error = %v, wantErr %v", tt.name, errs, tt.wantErr)
					return
				}
			} else if !(reflect.DeepEqual(tt.deleted, helm.Deleted) && (len(affectedReleases.Deleted) == len(tt.deleted))) {
				t.Errorf("unexpected deletions happened: expected %v, got %v", tt.deleted, helm.Deleted)
			}
		}
		t.Run(tt.name, f)
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
