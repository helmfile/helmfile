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

// TestExec tests that Exec returns the expected output.
func TestExec(t *testing.T) {
	ctx := &Context{basePath: "."}

	// test that the command is executed
	expected := "foo\n"
	output, err := ctx.Exec("echo", []interface{}{"foo"}, "")
	require.Nilf(t, err, "Expected no error to be returned when executing command")
	require.Equalf(t, expected, output, "Expected %s to be returned when executing command", expected)

	// test that the command is executed with no-zero exit code
	_, err = ctx.Exec("bash", []interface{}{"-c", "exit 1"}, "")
	require.Error(t, err, "Expected error to be returned when executing command with non-zero exit code")
}

func TestSetEnv(t *testing.T) {
	testEnvName := "testkey"
	_, isExists := os.LookupEnv(testEnvName)
	require.Falsef(t, isExists, "Expected environment variable %s to not exist", testEnvName)

	SetEnv(testEnvName, "testvalue")

	require.Equalf(t, "testvalue", os.Getenv(testEnvName), "Expected environment variable %s to be set to %s", testEnvName, "testvalue")

}
