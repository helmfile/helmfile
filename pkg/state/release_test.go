package state

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/helmfile/helmfile/pkg/filesystem"
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
