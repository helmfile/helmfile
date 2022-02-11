package tmpl

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
)

func TestCreateFuncMap(t *testing.T) {
	currentVal := disableInsecureFeatures

	{
		disableInsecureFeatures = false
		ctx := &Context{basePath: "."}
		funcMaps := ctx.createFuncMap()
		args := make([]interface{}, 0)
		outputExec, _ := funcMaps["exec"].(func(command string, args []interface{}, inputs ...string) (string, error))("ls", args)
		require.Contains(t, outputExec, "context.go")
	}

	disableInsecureFeatures = currentVal
}

func TestCreateFuncMap_DisabledInsecureFeatures(t *testing.T) {
	currentVal := disableInsecureFeatures

	{
		disableInsecureFeatures = true
		ctx := &Context{basePath: "."}
		funcMaps := ctx.createFuncMap()
		args := make([]interface{}, 0)
		_, err1 := funcMaps["exec"].(func(command string, args []interface{}, inputs ...string) (string, error))("ls", args)
		require.ErrorIs(t, err1, DisableInsecureFeaturesErr)
		_, err2 := funcMaps["readFile"].(func(filename string) (string, error))("context_funcs_test.go")
		require.ErrorIs(t, err2, DisableInsecureFeaturesErr)
	}

	disableInsecureFeatures = currentVal
}

func TestCreateFuncMap_SkipInsecureTemplateFunctions(t *testing.T) {
	currentVal := skipInsecureTemplateFunctions

	{
		skipInsecureTemplateFunctions = true
		ctx := &Context{basePath: "."}
		funcMaps := ctx.createFuncMap()
		args := make([]interface{}, 0)
		actual1, err1 := funcMaps["exec"].(func(command string, args []interface{}, inputs ...string) (string, error))("ls", args)
		require.Equal(t, "", actual1)
		require.ErrorIs(t, err1, nil)
		actual2, err2 := funcMaps["readFile"].(func(filename string) (string, error))("context_funcs_test.go")
		require.Equal(t, "", actual2)
		require.ErrorIs(t, err2, nil)
	}

	skipInsecureTemplateFunctions = currentVal
}

func TestReadFile(t *testing.T) {
	expected := `foo:
  bar: BAR
`
	expectedFilename := "values.yaml"
	ctx := &Context{basePath: ".", readFile: func(filename string) ([]byte, error) {
		if filename != expectedFilename {
			return nil, fmt.Errorf("unexpected filename: expected=%v, actual=%s", expectedFilename, filename)
		}
		return []byte(expected), nil
	}}
	actual, err := ctx.ReadFile(expectedFilename)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("unexpected result: expected=%v, actual=%v", expected, actual)
	}
}

func TestReadFile_PassAbsPath(t *testing.T) {
	expected := `foo:
  bar: BAR
`
	expectedFilename, _ := filepath.Abs("values.yaml")
	ctx := &Context{basePath: ".", readFile: func(filename string) ([]byte, error) {
		if filename != expectedFilename {
			return nil, fmt.Errorf("unexpected filename: expected=%v, actual=%s", expectedFilename, filename)
		}
		return []byte(expected), nil
	}}
	actual, err := ctx.ReadFile(expectedFilename)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("unexpected result: expected=%v, actual=%v", expected, actual)
	}
}

func TestToYaml_UnsupportedNestedMapKey(t *testing.T) {
	expected := ``
	vals := Values(map[string]interface{}{
		"foo": map[interface{}]interface{}{
			"bar": "BAR",
		},
	})
	actual, err := ToYaml(vals)
	if err == nil {
		t.Fatalf("expected error but got none")
	} else if err.Error() != "error marshaling into JSON: json: unsupported type: map[interface {}]interface {}" {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("unexpected result: expected=%v, actual=%v", expected, actual)
	}
}

func TestToYaml(t *testing.T) {
	expected := `foo:
  bar: BAR
`
	vals := Values(map[string]interface{}{
		"foo": map[string]interface{}{
			"bar": "BAR",
		},
	})
	actual, err := ToYaml(vals)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("unexpected result: expected=%v, actual=%v", expected, actual)
	}
}

func TestFromYaml(t *testing.T) {
	raw := `foo:
  bar: BAR
`
	expected := Values(map[string]interface{}{
		"foo": map[string]interface{}{
			"bar": "BAR",
		},
	})
	actual, err := FromYaml(raw)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("unexpected result: expected=%v, actual=%v", expected, actual)
	}
}

func TestFromYamlToJson(t *testing.T) {
	input := `foo:
  bar: BAR
`
	want := `{"foo":{"bar":"BAR"}}`

	m, err := FromYaml(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := json.Marshal(m)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if d := cmp.Diff(want, string(got)); d != "" {
		t.Errorf("unexpected result: want (-), got (+):\n%s", d)
	}
}

func TestSetValueAtPath_OneComponent(t *testing.T) {
	input := map[string]interface{}{
		"foo": "",
	}
	expected := map[string]interface{}{
		"foo": "FOO",
	}
	actual, err := SetValueAtPath("foo", "FOO", input)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("unexpected result: expected=%v, actual=%v", expected, actual)
	}
}

func TestSetValueAtPath_TwoComponents(t *testing.T) {
	input := map[string]interface{}{
		"foo": map[interface{}]interface{}{
			"bar": "",
		},
	}
	expected := map[string]interface{}{
		"foo": map[interface{}]interface{}{
			"bar": "FOO_BAR",
		},
	}
	actual, err := SetValueAtPath("foo.bar", "FOO_BAR", input)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("unexpected result: expected=%v, actual=%v", expected, actual)
	}
}

func TestTpl(t *testing.T) {
	text := `foo: {{ .foo }}
`
	expected := `foo: FOO
`
	ctx := &Context{basePath: "."}
	actual, err := ctx.Tpl(text, map[string]interface{}{"foo": "FOO"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("unexpected result: expected=%v, actual=%v", expected, actual)
	}
}

func TestRequired(t *testing.T) {
	type args struct {
		warn string
		val  interface{}
	}
	tests := []struct {
		name    string
		args    args
		want    interface{}
		wantErr bool
	}{
		{
			name:    "required val is nil",
			args:    args{warn: "This value is required", val: nil},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "required val is empty string",
			args:    args{warn: "This value is required", val: ""},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "required val is existed",
			args:    args{warn: "This value is required", val: "foo"},
			want:    "foo",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		testCase := tt
		t.Run(testCase.name, func(t *testing.T) {
			got, err := Required(testCase.args.warn, testCase.args.val)
			if (err != nil) != testCase.wantErr {
				t.Errorf("Required() error = %v, wantErr %v", err, testCase.wantErr)
				return
			}
			if !reflect.DeepEqual(got, testCase.want) {
				t.Errorf("Required() got = %v, want %v", got, testCase.want)
			}
		})
	}
}

// TestRequiredEnv tests that RequiredEnv returns an error if the environment variable is not set.
func TestRequiredEnv(t *testing.T) {

	// test that the environment variable is not set
	envKey := "HelmFile"
	envVal, err := RequiredEnv(envKey)

	require.NotNilf(t, err, "Expected error to be returned when environment variable %s is not set", envKey)
	require.Emptyf(t, envVal, "Expected empty string to be returned when environment variable %s is not set", envKey)

	// test that the environment variable is set to an empty string
	os.Setenv(envKey, "")
	envVal, err = RequiredEnv(envKey)

	require.NotNilf(t, err, "Expected error to be returned when environment variable %s is set to an empty string", envKey)
	require.Emptyf(t, envVal, "Expected empty string to be returned when environment variable %s is set to an empty string", envKey)

	// test that the environment variable is set to a non-empty string
	expected := "helmfile"
	os.Setenv(envKey, expected)

	// Unset the environment variable
	defer os.Unsetenv(envKey)
	envVal, err = RequiredEnv(envKey)
	require.Nilf(t, err, "Expected no error to be returned when environment variable %s is set to a non-empty string", envKey)
	require.Equalf(t, expected, envVal, "Expected %s to be returned when environment variable %s is set to a non-empty string", expected, envKey)

}
