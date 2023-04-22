package state

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/helmfile/helmfile/pkg/filesystem"
	"github.com/helmfile/helmfile/pkg/runtime"
	"github.com/helmfile/helmfile/pkg/tmpl"
)

func TestExecuteTemplateExpressions(t *testing.T) {
	render := tmpl.NewFileRenderer(filesystem.DefaultFileSystem(), "", map[string]interface{}{
		"Values": map[string]interface{}{
			"foo": map[string]interface{}{
				"releaseName": "foo",
			},
		},
		"Release": map[string]interface{}{
			"Name": "foo",
		},
	})

	v := runtime.GoccyGoYaml
	runtime.GoccyGoYaml = true
	t.Cleanup(func() {
		runtime.GoccyGoYaml = v
	})

	rs := ReleaseSpec{
		Name:      "foo",
		Chart:     "bar",
		Namespace: "baz",
		ValuesTemplate: []interface{}{
			map[string]interface{}{
				"fullnameOverride": "{{ .Values | get (printf \"%s.releaseName\" .Release.Name) .Release.Name }}",
			},
		},
	}
	result, err := rs.ExecuteTemplateExpressions(render)

	require.NoErrorf(t, err, "failed to execute template expressions: %v", err)
	require.Equalf(t, result.ValuesTemplate[0].(map[string]interface{})["fullnameOverride"], "foo", "failed to execute template expressions")
}

func TestCacheDir(t *testing.T) {
	tests := []struct {
		name string
		rs   ReleaseSpec
		want string
	}{
		{
			name: "empty",
			rs:   ReleaseSpec{},
			want: "",
		},
		{
			name: "namespace",
			rs:   ReleaseSpec{Namespace: "baz"},
			want: "baz",
		},
		{
			name: "kubeContext",
			rs:   ReleaseSpec{KubeContext: "qux"},
			want: "qux",
		},
		{
			name: "name",
			rs:   ReleaseSpec{Name: "foo"},
			want: "foo",
		},
		{
			name: "namespace and kubeContext",
			rs:   ReleaseSpec{Namespace: "baz", KubeContext: "qux"},
			want: "baz/qux",
		},
		{
			name: "namespace and name",
			rs:   ReleaseSpec{Namespace: "baz", Name: "foo"},
			want: "baz/foo",
		},
		{
			name: "kubeContext and name",
			rs:   ReleaseSpec{KubeContext: "qux", Name: "foo"},
			want: "qux/foo",
		},
		{
			name: "namespace and kubeContext and name",
			rs:   ReleaseSpec{Namespace: "baz", KubeContext: "qux", Name: "foo"},
			want: "baz/qux/foo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.rs.CacheDir(); got != tt.want {
				t.Errorf("ReleaseSpec.CacheDir() = %v, want %v", got, tt.want)
			}
		})
	}
}
