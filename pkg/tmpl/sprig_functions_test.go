package tmpl

import (
	"testing"

	"github.com/helmfile/helmfile/pkg/filesystem"
)

func TestMergeOverwrite(t *testing.T) {
	ctx := &Context{
		fs: &filesystem.FileSystem{
			Getwd: func() (string, error) {
				return "", nil
			},
			Glob: func(pattern string) ([]string, error) {
				return nil, nil
			},
		},
	}
	buf, err := ctx.RenderTemplateToBuffer(`
		{{- $v1 := dict "bool" true  "int" 2 "str" "v1" "str2" "v1" -}}
		{{- $v2 := dict "bool" false "int" 0 "str" "v2" "str2" "" -}}
		{{- $mo1 := mergeOverwrite (dict) $v1 $v2 }}
		{{- $mo1 -}}
	`)
	expected := "map[bool:false int:0 str:v2 str2:]"
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	actual := buf.String()
	if actual != expected {
		t.Errorf("unexpected result: expected=%v, actual=%v", expected, actual)
	}
}
