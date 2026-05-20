package state

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/go-test/deep"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"

	"github.com/helmfile/helmfile/pkg/environment"
	"github.com/helmfile/helmfile/pkg/filesystem"
)

func boolPtrToString(ptr *bool) string {
	if ptr == nil {
		return "<nil>"
	}
	return fmt.Sprintf("&%t", *ptr)
}

func TestHelmState_executeTemplates(t *testing.T) {
	tests := []struct {
		name  string
		input ReleaseSpec
		want  ReleaseSpec
	}{
		{
			name: "Has template expressions in chart, values, secrets, version, labels",
			input: ReleaseSpec{
				Chart:          "test-charts/{{ .Release.Name }}",
				Version:        "{{ .Release.Name }}-0.1",
				Name:           "test-app",
				Namespace:      "test-namespace-{{ .Release.Name }}",
				ValuesTemplate: []any{"config/{{ .Environment.Name }}/{{ .Release.Name }}/{{ .Release.ChartVersion }}/values.yaml"},
				Secrets:        []any{"config/{{ .Environment.Name }}/{{ .Release.Name }}/{{ .Release.ChartVersion }}/secrets.yaml"},
				Labels:         map[string]string{"id": "{{ .Release.Name }}"},
			},
			want: ReleaseSpec{
				Chart:     "test-charts/test-app",
				Version:   "test-app-0.1",
				Name:      "test-app",
				Namespace: "test-namespace-test-app",
				Values:    []any{"config/test_env/test-app/test-app-0.1/values.yaml"},
				Secrets:   []any{"config/test_env/test-app/test-app-0.1/secrets.yaml"},
				Labels:    map[string]string{"id": "test-app"},
			},
		},
		{
			name: "Has template expressions in name with recursive refs",
			input: ReleaseSpec{
				Chart:     "test-chart",
				Name:      "{{ .Release.Labels.id }}-{{ .Release.Namespace }}",
				Namespace: "dev",
				Labels:    map[string]string{"id": "{{ .Release.Chart }}"},
			},
			want: ReleaseSpec{
				Chart:     "test-chart",
				Name:      "test-chart-dev",
				Namespace: "dev",
				Labels:    map[string]string{"id": "test-chart"},
			},
		},
		{
			name: "Has template expressions in boolean values",
			input: ReleaseSpec{
				Chart:             "test-chart",
				Name:              "app-dev",
				Namespace:         "dev",
				Labels:            map[string]string{"id": "app"},
				InstalledTemplate: func(i string) *string { return &i }(`{{ eq .Release.Labels.id "app" | ternary "true" "false" }}`),
				VerifyTemplate:    func(i string) *string { return &i }(`{{ true }}`),
				Verify:            func(i bool) *bool { return &i }(false),
				WaitTemplate:      func(i string) *string { return &i }(`{{ false }}`),
			},
			want: ReleaseSpec{
				Chart:     "test-chart",
				Name:      "app-dev",
				Namespace: "dev",
				Labels:    map[string]string{"id": "app"},
				Installed: func(i bool) *bool { return &i }(true),
				Verify:    func(i bool) *bool { return &i }(true),
				Wait:      func(i bool) *bool { return &i }(false),
			},
		},
		{
			name: "Has template in set-values",
			input: ReleaseSpec{
				Chart:     "test-charts/chart",
				Name:      "test-app",
				Namespace: "dev",
				Version:   "1.5",
				SetValuesTemplate: []SetValue{
					{Name: "val1", Value: "{{ .Release.Name }}-val1"},
					{Name: "val2", File: "{{ .Release.Name }}.yml"},
					{Name: "val3", Values: []string{"{{ .Release.Name }}-val2", "{{ .Release.Name }}-val3"}},
					{Name: "val4", Value: "{{ .Release.Chart }}-{{ .Release.ChartVersion}}"},
				},
			},
			want: ReleaseSpec{
				Chart:     "test-charts/chart",
				Name:      "test-app",
				Namespace: "dev",
				Version:   "1.5",
				SetValues: []SetValue{
					{Name: "val1", Value: "test-app-val1"},
					{Name: "val2", File: "test-app.yml"},
					{Name: "val3", Values: []string{"test-app-val2", "test-app-val3"}},
					{Name: "val4", Value: "test-charts/chart-1.5"},
				},
			},
		},
		{
			name: "Has template in values (map)",
			input: ReleaseSpec{
				Chart:          "test-charts/chart",
				Verify:         nil,
				Name:           "app",
				Namespace:      "dev",
				ValuesTemplate: []any{map[string]string{"key": "{{ .Release.Name }}-val0"}},
			},
			want: ReleaseSpec{
				Chart:     "test-charts/chart",
				Verify:    nil,
				Name:      "app",
				Namespace: "dev",
				Values:    []any{map[string]any{"key": "app-val0"}},
			},
		},
		{
			name: "Has template expressions in post renderer args",
			input: ReleaseSpec{
				Chart: "test-chart",
				PostRendererArgs: []string{
					"--release",
					"{{ .Release.Name }}",
					"--chart",
					"{{ .Release.Chart }}",
				},
				Name: "test-release",
			},
			want: ReleaseSpec{
				Chart: "test-chart",
				Name:  "test-release",
				PostRendererArgs: []string{
					"--release",
					"test-chart-dev",
					"--chart",
					"test-chart",
				},
			},
		},
		{
			name: "Version is empty but used in templates (render as empty string)",
			input: ReleaseSpec{
				Name:           "test-app",
				Chart:          "test-charts/{{ .Release.Name }}",
				ValuesTemplate: []any{"config/values-{{ .Release.ChartVersion }}.yaml"},
			},
			want: ReleaseSpec{
				Name:   "test-app",
				Chart:  "test-charts/test-app",
				Values: []any{"config/values-.yaml"},
			},
		},
	}

	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			state := &HelmState{
				fs: &filesystem.FileSystem{
					Glob: func(s string) ([]string, error) { return nil, nil }},
				basePath: ".",
				ReleaseSetSpec: ReleaseSetSpec{
					HelmDefaults: HelmSpec{
						KubeContext: "test_context",
					},
					Env:               environment.Environment{Name: "test_env"},
					OverrideNamespace: "test-namespace_",
					Repositories:      nil,
					Releases: []ReleaseSpec{
						tt.input,
					},
				},
				RenderedValues: map[string]any{},
			}

			r, err := state.ExecuteTemplates()
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				t.FailNow()
			}

			actual := r.Releases[0]

			if !reflect.DeepEqual(actual.Name, tt.want.Name) {
				t.Errorf("expected Name %+v, got %+v", tt.want.Name, actual.Name)
			}
			if !reflect.DeepEqual(actual.Chart, tt.want.Chart) {
				t.Errorf("expected Chart %+v, got %+v", tt.want.Chart, actual.Chart)
			}
			if !reflect.DeepEqual(actual.Namespace, tt.want.Namespace) {
				t.Errorf("expected Namespace %+v, got %+v", tt.want.Namespace, actual.Namespace)
			}
			if diff := deep.Equal(actual.Values, tt.want.Values); diff != nil && len(actual.Values) > 0 {
				t.Errorf("Values differs \n%+v", strings.Join(diff, "\n"))
			}
			if diff := deep.Equal(actual.Secrets, tt.want.Secrets); diff != nil && len(actual.Secrets) > 0 {
				t.Errorf("Secrets differs \n%+v", strings.Join(diff, "\n"))
			}
			if diff := deep.Equal(actual.SetValues, tt.want.SetValues); diff != nil && len(actual.SetValues) > 0 {
				t.Errorf("SetValues differs \n%+v", strings.Join(diff, "\n"))
			}
			if diff := deep.Equal(actual.Labels, tt.want.Labels); diff != nil && len(actual.Labels) > 0 {
				t.Errorf("Labels differs \n%+v", strings.Join(diff, "\n"))
			}
			if !reflect.DeepEqual(actual.Version, tt.want.Version) {
				t.Errorf("expected Version %+v, got %+v", tt.want.Version, actual.Version)
			}
			if !reflect.DeepEqual(actual.Installed, tt.want.Installed) {
				t.Errorf("expected actual.Installed %+v, got %+v",
					boolPtrToString(tt.want.Installed), boolPtrToString(actual.Installed),
				)
			}
			if !reflect.DeepEqual(actual.Verify, tt.want.Verify) {
				t.Errorf("expected actual.Verify %+v, got %+v",
					boolPtrToString(tt.want.Verify), boolPtrToString(actual.Verify),
				)
			}
			if !reflect.DeepEqual(actual.Wait, tt.want.Wait) {
				t.Errorf("expected actual.Wait %+v, got %+v",
					boolPtrToString(tt.want.Wait), boolPtrToString(actual.Wait),
				)
			}
		})
	}
}

func TestHelmState_recursiveRefsTemplates(t *testing.T) {
	tests := []struct {
		name  string
		input ReleaseSpec
	}{
		{
			name: "Has reqursive references",
			input: ReleaseSpec{
				Chart:     "test-charts/{{ .Release.Name }}",
				Verify:    nil,
				Name:      "{{ .Release.Labels.id }}",
				Namespace: "dev",
				Labels:    map[string]string{"id": "app-{{ .Release.Name }}"},
			},
		},
		{
			name: "Has unresolvable boolean templates",
			input: ReleaseSpec{
				Name:         "app-dev",
				Chart:        "test-charts/app",
				Verify:       nil,
				Namespace:    "dev",
				WaitTemplate: func(i string) *string { return &i }("hi"),
			},
		},
	}

	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			state := &HelmState{
				basePath: ".",
				fs: &filesystem.FileSystem{
					Glob: func(s string) ([]string, error) { return nil, nil },
				},
				ReleaseSetSpec: ReleaseSetSpec{
					HelmDefaults: HelmSpec{
						KubeContext: "test_context",
					},
					Env:               environment.Environment{Name: "test_env"},
					OverrideNamespace: "test-namespace_",
					Repositories:      nil,
					Releases: []ReleaseSpec{
						tt.input,
					},
				},
				RenderedValues: map[string]any{},
			}

			r, err := state.ExecuteTemplates()
			if err == nil {
				t.Errorf("Expected error, got valid response: %v", r)
				t.FailNow()
			}
		})
	}
}

func TestApplyDefaultInherit(t *testing.T) {
	tests := []struct {
		name           string
		defaultInherit DefaultInherits
		releaseInherit Inherits
		want           Inherits
	}{
		{
			name:           "no default inherit",
			defaultInherit: nil,
			releaseInherit: Inherits{{Template: "foo"}},
			want:           Inherits{{Template: "foo"}},
		},
		{
			name:           "default inherit prepended",
			defaultInherit: DefaultInherits{"default"},
			releaseInherit: Inherits{{Template: "foo"}},
			want:           Inherits{{Template: "default"}, {Template: "foo"}},
		},
		{
			name:           "default inherit already in release inherit is not duplicated",
			defaultInherit: DefaultInherits{"default"},
			releaseInherit: Inherits{{Template: "default"}, {Template: "foo"}},
			want:           Inherits{{Template: "default"}, {Template: "foo"}},
		},
		{
			name:           "multiple default inherits",
			defaultInherit: DefaultInherits{"a", "b"},
			releaseInherit: Inherits{{Template: "c"}},
			want:           Inherits{{Template: "a"}, {Template: "b"}, {Template: "c"}},
		},
		{
			name:           "release inherit empty with defaults",
			defaultInherit: DefaultInherits{"default"},
			releaseInherit: nil,
			want:           Inherits{{Template: "default"}},
		},
		{
			name:           "default inherit deduplicates and skips empty values",
			defaultInherit: DefaultInherits{"default", " ", "default", "ops"},
			releaseInherit: Inherits{{Template: "foo"}},
			want:           Inherits{{Template: "default"}, {Template: "ops"}, {Template: "foo"}},
		},
	}

	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			st := &HelmState{
				ReleaseSetSpec: ReleaseSetSpec{
					DefaultInherit: tt.defaultInherit,
				},
			}
			got := st.applyDefaultInherit(tt.releaseInherit)
			if len(got) != len(tt.want) {
				t.Fatalf("expected %d inherits, got %d", len(tt.want), len(got))
			}
			for j := range got {
				if got[j].Template != tt.want[j].Template {
					t.Errorf("inherit[%d]: expected template %q, got %q", j, tt.want[j].Template, got[j].Template)
				}
				if len(got[j].Except) != len(tt.want[j].Except) {
					t.Errorf("inherit[%d]: expected %d except, got %d", j, len(tt.want[j].Except), len(got[j].Except))
				}
			}
		})
	}
}

func TestHelmState_executeTemplatesWithDefaultTemplates(t *testing.T) {
	logger := zap.NewNop().Sugar()
	state := &HelmState{
		logger: logger,
		fs: &filesystem.FileSystem{
			Glob: func(s string) ([]string, error) { return nil, nil },
		},
		basePath: ".",
		ReleaseSetSpec: ReleaseSetSpec{
			HelmDefaults: HelmSpec{
				KubeContext: "test_context",
			},
			Env: environment.Environment{Name: "test_env"},
			Templates: map[string]TemplateSpec{
				"default": {
					ReleaseSpec: ReleaseSpec{
						Namespace: "default-ns",
						Labels:    map[string]string{"managed": "true"},
					},
				},
			},
			DefaultInherit: DefaultInherits{"default"},
			Releases: []ReleaseSpec{
				{
					Name:  "app1",
					Chart: "test-chart",
				},
				{
					Name:  "app2",
					Chart: "test-chart-2",
					Inherit: Inherits{
						{Template: "default", Except: []string{"labels"}},
					},
				},
			},
		},
		RenderedValues: map[string]any{},
	}

	r, err := state.ExecuteTemplates()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	app1 := r.Releases[0]
	if app1.Namespace != "default-ns" {
		t.Errorf("app1: expected namespace %q, got %q", "default-ns", app1.Namespace)
	}
	if app1.Labels["managed"] != "true" {
		t.Errorf("app1: expected label managed=true, got %v", app1.Labels)
	}

	app2 := r.Releases[1]
	if app2.Namespace != "default-ns" {
		t.Errorf("app2: expected namespace %q, got %q", "default-ns", app2.Namespace)
	}
	if _, ok := app2.Labels["managed"]; ok {
		t.Errorf("app2: expected labels to be excluded, but got %v", app2.Labels)
	}
}

func TestDefaultInherits_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  DefaultInherits
	}{
		{
			name:  "single string",
			input: `default`,
			want:  DefaultInherits{"default"},
		},
		{
			name:  "list of strings",
			input: `["a", "b"]`,
			want:  DefaultInherits{"a", "b"},
		},
		{
			name:  "null value",
			input: `null`,
			want:  nil,
		},
		{
			name:  "empty string value",
			input: `""`,
			want:  nil,
		},
		{
			name:  "list trims and drops empty names",
			input: `[" a ", "", " ", "b"]`,
			want:  DefaultInherits{"a", "b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got DefaultInherits
			err := yaml.Unmarshal([]byte(tt.input), &got)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("expected %d items, got %d", len(tt.want), len(got))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("item[%d]: expected %q, got %q", i, tt.want[i], got[i])
				}
			}
		})
	}
}
