package state

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/helmfile/helmfile/pkg/filesystem"
	"github.com/helmfile/helmfile/pkg/runtime"
	"github.com/helmfile/helmfile/pkg/tmpl"
)

func TestExecuteTemplateExpressions(t *testing.T) {
	render := tmpl.NewFileRenderer(filesystem.DefaultFileSystem(), "", map[string]any{
		"Values": map[string]any{
			"foo": map[string]any{
				"releaseName": "foo",
			},
		},
		"Release": map[string]any{
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
		ValuesTemplate: []any{
			map[string]any{
				"fullnameOverride": "{{ .Values | get (printf \"%s.releaseName\" .Release.Name) .Release.Name }}",
			},
		},
	}
	result, err := rs.ExecuteTemplateExpressions(render)

	require.NoErrorf(t, err, "failed to execute template expressions: %v", err)
	require.Equalf(t, result.ValuesTemplate[0].(map[string]any)["fullnameOverride"], "foo", "failed to execute template expressions")
}

func TestDesired(t *testing.T) {
	cases := []struct {
		name     string
		expected bool
		release  ReleaseSpec
	}{
		{
			name:     "desired is true",
			expected: true,
			release: ReleaseSpec{
				Installed: &[]bool{true}[0],
			},
		},
		{
			name:     "desired is false",
			expected: false,
			release: ReleaseSpec{
				Installed: &[]bool{false}[0],
			},
		},
		{
			name:     "desired is nil",
			expected: true,
			release:  ReleaseSpec{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			actual := tc.release.Desired()
			require.Equal(t, tc.expected, actual)

			// test compatibility with older versions of helmfile
			// r.Installed != nil && !*r.Installed compare to !r.Desired()
			require.Equal(t, tc.release.Installed != nil && !*tc.release.Installed, !actual)
		})
	}
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
