package state

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/go-test/deep"

	"github.com/helmfile/helmfile/pkg/environment"
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
				ValuesTemplate: []interface{}{"config/{{ .Environment.Name }}/{{ .Release.Name }}/values.yaml"},
				Secrets:        []interface{}{"config/{{ .Environment.Name }}/{{ .Release.Name }}/secrets.yaml"},
				Labels:         map[string]string{"id": "{{ .Release.Name }}"},
			},
			want: ReleaseSpec{
				Chart:     "test-charts/test-app",
				Version:   "test-app-0.1",
				Name:      "test-app",
				Namespace: "test-namespace-test-app",
				Values:    []interface{}{"config/test_env/test-app/values.yaml"},
				Secrets:   []interface{}{"config/test_env/test-app/secrets.yaml"},
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
				Chart:              "test-chart",
				Name:               "app-dev",
				Namespace:          "dev",
				Labels:             map[string]string{"id": "app"},
				InstalledTemplate:  func(i string) *string { return &i }(`{{ eq .Release.Labels.id "app" | ternary "true" "false" }}`),
				VerifyTemplate:     func(i string) *string { return &i }(`{{ true }}`),
				Verify:             func(i bool) *bool { return &i }(false),
				WaitTemplate:       func(i string) *string { return &i }(`{{ false }}`),
				TillerlessTemplate: func(i string) *string { return &i }(`true`),
			},
			want: ReleaseSpec{
				Chart:      "test-chart",
				Name:       "app-dev",
				Namespace:  "dev",
				Labels:     map[string]string{"id": "app"},
				Installed:  func(i bool) *bool { return &i }(true),
				Verify:     func(i bool) *bool { return &i }(true),
				Wait:       func(i bool) *bool { return &i }(false),
				Tillerless: func(i bool) *bool { return &i }(true),
			},
		},
		{
			name: "Has template in set-values",
			input: ReleaseSpec{
				Chart:     "test-charts/chart",
				Name:      "test-app",
				Namespace: "dev",
				SetValuesTemplate: []SetValue{
					{Name: "val1", Value: "{{ .Release.Name }}-val1"},
					{Name: "val2", File: "{{ .Release.Name }}.yml"},
					{Name: "val3", Values: []string{"{{ .Release.Name }}-val2", "{{ .Release.Name }}-val3"}},
				},
			},
			want: ReleaseSpec{
				Chart:     "test-charts/chart",
				Name:      "test-app",
				Namespace: "dev",
				SetValues: []SetValue{
					{Name: "val1", Value: "test-app-val1"},
					{Name: "val2", File: "test-app.yml"},
					{Name: "val3", Values: []string{"test-app-val2", "test-app-val3"}},
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
				ValuesTemplate: []interface{}{map[string]string{"key": "{{ .Release.Name }}-val0"}},
			},
			want: ReleaseSpec{
				Chart:     "test-charts/chart",
				Verify:    nil,
				Name:      "app",
				Namespace: "dev",
				Values:    []interface{}{map[interface{}]interface{}{"key": "app-val0"}},
			},
		},
	}

	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			state := &HelmState{
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
				RenderedValues: map[string]interface{}{},
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
			if !reflect.DeepEqual(actual.Tillerless, tt.want.Tillerless) {
				t.Errorf("expected actual.Tillerless %+v, got %+v",
					boolPtrToString(tt.want.Tillerless), boolPtrToString(actual.Tillerless),
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
				RenderedValues: map[string]interface{}{},
			}

			r, err := state.ExecuteTemplates()
			if err == nil {
				t.Errorf("Expected error, got valid response: %v", r)
				t.FailNow()
			}
		})
	}
}
