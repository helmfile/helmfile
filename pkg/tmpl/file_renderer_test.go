package tmpl

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/helmfile/helmfile/pkg/environment"
	"github.com/helmfile/helmfile/pkg/filesystem"
)

var emptyEnvTmplData = map[string]interface{}{
	"Environment": environment.EmptyEnvironment,
	"Namespace":   "",
}

func TestRenderToBytes_Gotmpl(t *testing.T) {
	valuesYamlTmplContent := `foo:
  bar: '{{ readFile "data.txt" }}'
`
	dataFileContent := "FOO_BAR"
	expected := `foo:
  bar: 'FOO_BAR'
`
	dataFile := "data.txt"
	valuesTmplFile := "values.yaml.gotmpl"
	r := NewFileRenderer(&filesystem.FileSystem{ReadFile: func(filename string) ([]byte, error) {
		switch filename {
		case valuesTmplFile:
			return []byte(valuesYamlTmplContent), nil
		case dataFile:
			return []byte(dataFileContent), nil
		}
		return nil, fmt.Errorf("unexpected filename: expected=%v or %v, actual=%s", dataFile, valuesTmplFile, filename)
	}}, "", emptyEnvTmplData)
	buf, err := r.RenderToBytes(valuesTmplFile)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	actual := string(buf)
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("unexpected result: expected=%v, actual=%v", expected, actual)
	}
}

func TestRenderToBytes_Yaml(t *testing.T) {
	valuesYamlContent := `foo:
  bar: '{{ readFile "data.txt" }}'
`
	expected := `foo:
  bar: '{{ readFile "data.txt" }}'
`
	valuesFile := "values.yaml"
	r := NewFileRenderer(&filesystem.FileSystem{ReadFile: func(filename string) ([]byte, error) {
		if filename == valuesFile {
			return []byte(valuesYamlContent), nil
		}
		return nil, fmt.Errorf("unexpected filename: expected=%v, actual=%s", valuesFile, filename)
	}}, "", emptyEnvTmplData)
	buf, err := r.RenderToBytes(valuesFile)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	actual := string(buf)
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("unexpected result: expected=%v, actual=%v", expected, actual)
	}
}
