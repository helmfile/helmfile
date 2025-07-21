package tmpl

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	goruntime "runtime"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/helmfile/helmfile/pkg/filesystem"
	"github.com/helmfile/helmfile/pkg/runtime"
)

func TestCreateFuncMap(t *testing.T) {
	currentVal := disableInsecureFeatures

	{
		disableInsecureFeatures = false
		ctx := &Context{basePath: "."}
		funcMaps := ctx.createFuncMap()
		args := make([]any, 0)
		outputExec, _ := funcMaps["exec"].(func(command string, args []any, inputs ...string) (string, error))("ls", args)
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
		args := make([]any, 0)
		_, err1 := funcMaps["exec"].(func(command string, args []any, inputs ...string) (string, error))("ls", args)
		require.ErrorIs(t, err1, DisableInsecureFeaturesErr)
		_, err2 := funcMaps["readFile"].(func(filename string) (string, error))("context_funcs_test.go")
		require.ErrorIs(t, err2, DisableInsecureFeaturesErr)
	}

	disableInsecureFeatures = currentVal
}

func newFSExpecting(expectedFilename string, expected string) *filesystem.FileSystem {
	return filesystem.FromFileSystem(filesystem.FileSystem{
		ReadFile: func(filename string) ([]byte, error) {
			if filename != expectedFilename {
				return nil, fmt.Errorf("unexpected filename: expected=%v, actual=%s", expectedFilename, filename)
			}
			return []byte(expected), nil
		},
	})
}

func TestReadFile(t *testing.T) {
	expected := `foo:
  bar: BAR
`
	expectedFilename := "values.yaml"
	ctx := &Context{basePath: ".", fs: newFSExpecting(expectedFilename, expected)}
	actual, err := ctx.ReadFile(expectedFilename)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}

type entry struct {
	name  string
	fType fs.FileMode
	isDir bool
}

func (e *entry) Name() string {
	return e.name
}

func (e *entry) IsDir() bool {
	return e.isDir
}

func (e *entry) Type() fs.FileMode {
	return e.fType
}

func (e *entry) Info() (fs.FileInfo, error) {
	return nil, fmt.Errorf("You should not call this method")
}

func TestReadDir(t *testing.T) {
	result := []fs.DirEntry{
		&entry{name: "file1.yaml"},
		&entry{name: "file2.yaml"},
		&entry{name: "file3.yaml"},
		&entry{name: "folder1", isDir: true},
	}
	expectedArrayWindows := []string{
		"sampleDirectory\\file1.yaml",
		"sampleDirectory\\file2.yaml",
		"sampleDirectory\\file3.yaml",
	}
	expectedArrayUnix := []string{
		"sampleDirectory/file1.yaml",
		"sampleDirectory/file2.yaml",
		"sampleDirectory/file3.yaml",
	}
	var expectedArray []string
	if goruntime.GOOS == "windows" {
		expectedArray = expectedArrayWindows
	} else {
		expectedArray = expectedArrayUnix
	}

	expectedDirname := "sampleDirectory"
	ctx := &Context{basePath: ".", fs: filesystem.FromFileSystem(filesystem.FileSystem{ReadDir: func(dirname string) ([]fs.DirEntry, error) {
		if dirname != expectedDirname {
			return nil, fmt.Errorf("unexpected filename: expected=%v, actual=%s", expectedDirname, dirname)
		}
		return result, nil
	}})}

	actual, err := ctx.ReadDir(expectedDirname)
	require.NoError(t, err)
	require.ElementsMatch(t, expectedArray, actual)
}

func TestReadDirEntries(t *testing.T) {
	result := []fs.DirEntry{
		&entry{name: "file1.yaml"},
		&entry{name: "file2.yaml"},
		&entry{name: "file3.yaml"},
		&entry{name: "folder1", isDir: true},
	}

	expectedDirname := "sampleDirectory"
	ctx := &Context{basePath: ".", fs: filesystem.FromFileSystem(filesystem.FileSystem{ReadDir: func(dirname string) ([]fs.DirEntry, error) {
		if dirname != expectedDirname {
			return nil, fmt.Errorf("unexpected filename: expected=%v, actual=%s", expectedDirname, dirname)
		}
		return result, nil
	}})}

	actual, err := ctx.ReadDirEntries(expectedDirname)
	require.NoError(t, err)
	require.ElementsMatch(t, result, actual)
}

func TestReadFile_PassAbsPath(t *testing.T) {
	expected := `foo:
  bar: BAR
`
	expectedFilename, _ := filepath.Abs("values.yaml")
	ctx := &Context{basePath: ".", fs: newFSExpecting(expectedFilename, expected)}
	actual, err := ctx.ReadFile(expectedFilename)
	require.NoError(t, err)
	require.Equal(t, actual, expected)
}

func TestToYaml_NestedMapInterfaceKey(t *testing.T) {
	v := runtime.GoYamlV3
	t.Cleanup(func() {
		runtime.GoYamlV3 = v
	})

	// nolint: unconvert
	vals := Values(map[string]any{
		"foo": map[any]any{
			"bar": "BAR",
		},
	})

	runtime.GoYamlV3 = true

	actual, err := ToYaml(vals)
	require.Equal(t, "foo:\n  bar: BAR\n", actual)
	require.NoError(t, err, "expected nil, but got: %v, when type: map[interface {}]interface {}", err)

	runtime.GoYamlV3 = false

	actual, err = ToYaml(vals)
	require.Equal(t, "foo:\n  bar: BAR\n", actual)
	require.NoError(t, err, "expected nil, but got: %v, when type: map[interface {}]interface {}", err)
}

func TestToYaml(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected string
		wantErr  bool
	}{
		{
			// https://github.com/helmfile/helmfile/issues/2024
			name:  "test unmarshalling issue 2024",
			input: map[string]any{"thisShouldBeString": "01234567890123456789"},
			expected: `thisShouldBeString: "01234567890123456789"
`,
		},
		{
			name:  "test unmarshalling issue 2024 with int64",
			input: map[string]any{"thisShouldBeString": int64(1234567890123456789)},
			expected: `thisShouldBeString: 1234567890123456789
`,
		},
		{
			name:  "test unmarshalling object",
			input: map[string]any{"foo": map[string]any{"bar": "BAR"}},
			expected: `foo:
  bar: BAR
`,
		},
		{
			name:  "test unmarshalling array",
			input: []any{"foo", map[string]any{"bar": "BAR"}},
			expected: `- foo
- bar: BAR
`,
		},
		{
			name:     "test unmarshalling string",
			input:    "foo",
			expected: "foo\n",
		},
		{
			name:     "test unmarshalling number",
			input:    1234,
			expected: "1234\n",
		},
		{
			name:     "test unmarshalling boolean",
			input:    true,
			expected: "true\n",
		},
		{
			name:     "test unmarshalling null",
			input:    nil,
			expected: "null\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := ToYaml(tt.input)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.expected, actual)
		})
	}
}

func testFromYamlObject(t *testing.T) {
	raw := `foo:
  bar: BAR
`

	actual, err := FromYaml(raw)
	require.NoError(t, err)
	require.Equal(
		t,
		map[string]any{
			"foo": map[string]any{
				"bar": "BAR",
			},
		},
		actual,
	)
}

func testFromYamlArray(t *testing.T) {
	raw := `- foo
- bar: BAR
`

	actual, err := FromYaml(raw)
	require.NoError(t, err)
	require.Equal(
		t,
		[]any{
			"foo",
			map[string]any{
				"bar": "BAR",
			},
		},
		actual,
	)
}

func testFromYamlString(t *testing.T) {
	raw := `foo
`

	actual, err := FromYaml(raw)
	require.NoError(t, err)
	require.Equal(t, "foo", actual)
}

func testFromYamlNumber(t *testing.T) {
	raw := `1234
`

	actual, err := FromYaml(raw)
	require.NoError(t, err)

	switch a := actual.(type) {
	case int:
		require.Equal(t, 1234, a)
	case uint64:
		require.Equal(t, uint64(1234), a)
	default:
		t.Errorf("unexpected type: %T", a)
	}
}

func testFromYamlBoolean(t *testing.T) {
	raw := `true
`

	actual, err := FromYaml(raw)
	require.NoError(t, err)
	require.Equal(t, true, actual)
}

func testFromYamlNull(t *testing.T) {
	raw := `null
`

	actual, err := FromYaml(raw)
	require.NoError(t, err)
	require.Equal(t, nil, actual)
}

func testFromYaml(t *testing.T, GoYamlV3 bool) {
	t.Helper()

	v := runtime.GoYamlV3
	runtime.GoYamlV3 = GoYamlV3
	t.Cleanup(func() {
		runtime.GoYamlV3 = v
	})

	t.Run("test unmarshalling object", testFromYamlObject)

	t.Run("test unmarshalling array", testFromYamlArray)

	t.Run("test unmarshalling string", testFromYamlString)

	t.Run("test unmarshalling number", testFromYamlNumber)

	t.Run("test unmarshalling boolean", testFromYamlBoolean)

	t.Run("test unmarshalling null", testFromYamlNull)
}

func TestFromYaml(t *testing.T) {
	t.Run("with gopkg.in/yaml.v2", func(t *testing.T) {
		testFromYaml(t, true)
	})

	t.Run("with gopkg.in/yaml.v3", func(t *testing.T) {
		testFromYaml(t, false)
	})
}

func TestFromYamlToJson(t *testing.T) {
	input := `foo:
  bar: BAR
`
	want := `{"foo":{"bar":"BAR"}}`

	m, err := FromYaml(input)
	require.NoError(t, err)

	got, err := json.Marshal(m)
	require.NoError(t, err)
	require.Equal(t, string(got), want)
}

func TestSetValueAtPath_OneComponent(t *testing.T) {
	input := map[string]any{
		"foo": "",
	}
	expected := map[string]any{
		"foo": "FOO",
	}
	actual, err := SetValueAtPath("foo", "FOO", input)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}

func TestSetValueAtPath_TwoComponents(t *testing.T) {
	input := map[string]any{
		"foo": map[any]any{
			"bar": "",
		},
	}
	expected := map[string]any{
		"foo": map[any]any{
			"bar": "FOO_BAR",
		},
	}
	actual, err := SetValueAtPath("foo.bar", "FOO_BAR", input)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}

func TestTpl(t *testing.T) {
	ctx := &Context{
		basePath: ".",
		fs: &filesystem.FileSystem{
			Glob: func(s string) ([]string, error) {
				return nil, nil
			}},
	}
	tests := []struct {
		name     string
		input    string
		expected string
		hasErr   bool
		data     map[string]any
	}{
		{
			name:     "simple",
			input:    `foo: {{ .foo }}`,
			expected: `foo: Foo`,
			data: map[string]any{
				"foo": "Foo",
			},
		},
		{
			name: "multiline_input",
			input: `{{ .name }}
end`,
			expected: "multiline\nend",
			data: map[string]any{
				"name": "multiline",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := ctx.Tpl(tt.input, tt.data)
			if tt.hasErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.expected, actual)
		})
	}
}

func TestRequired(t *testing.T) {
	type args struct {
		warn string
		val  any
	}
	tests := []struct {
		name    string
		args    args
		want    any
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
			if testCase.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, testCase.want, got)
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
	t.Setenv(envKey, "")
	envVal, err = RequiredEnv(envKey)

	require.NotNilf(t, err, "Expected error to be returned when environment variable %s is set to an empty string", envKey)
	require.Emptyf(t, envVal, "Expected empty string to be returned when environment variable %s is set to an empty string", envKey)

	// test that the environment variable is set to a non-empty string
	expected := "helmfile"
	t.Setenv(envKey, expected)

	envVal, err = RequiredEnv(envKey)
	require.Nilf(t, err, "Expected no error to be returned when environment variable %s is set to a non-empty string", envKey)
	require.Equalf(t, expected, envVal, "Expected %s to be returned when environment variable %s is set to a non-empty string", expected, envKey)
}

// TestExec tests that Exec returns the expected output.
func TestExec(t *testing.T) {
	ctx := &Context{basePath: "."}

	// test that the command is executed
	expected := "foo\n"
	output, err := ctx.Exec("echo", []any{"foo"}, "")
	require.Nilf(t, err, "Expected no error to be returned when executing command")
	require.Equalf(t, expected, output, "Expected %s to be returned when executing command", expected)

	// test that the command is executed with no-zero exit code
	_, err = ctx.Exec("bash", []any{"-c", "exit 1"}, "")
	require.Error(t, err, "Expected error to be returned when executing command with non-zero exit code")
}

// TestEnvExec tests that EnvExec returns the expected output.
// TODO: in the next major version, this test should be removed.
func TestEnvExec(t *testing.T) {
	ctx := &Context{basePath: "."}

	expected := "foo"

	testKey := "testkey"

	// test that the command is executed with environment variables
	output, err := ctx.EnvExec(map[string]any{testKey: "foo"}, "bash", []any{"-c", fmt.Sprintf("echo -n $%s", testKey)}, "")

	require.Nilf(t, err, "Expected no error to be returned when executing command with environment variables")

	require.Equalf(t, expected, output, "Expected %s to be returned when executing command with environment variables", expected)

	// test that the command is executed with invalid environment variables
	output, err = ctx.EnvExec(map[string]any{testKey: 123}, "bash", []any{"-c", fmt.Sprintf("echo -n $%s", testKey)}, "")

	require.Errorf(t, err, "Expected error to be returned when executing command with invalid environment variables")
	require.Emptyf(t, output, "Expected empty string to be returned when executing command with invalid environment variables")

	// test that the command is executed with no environment variables
	output, err = ctx.EnvExec(nil, "bash", []any{"-c", fmt.Sprintf("echo -n $%s", testKey)}, "")
	require.Nilf(t, err, "Expected no error to be returned when executing command with no environment variables")

	require.Emptyf(t, output, "Expected empty string to be returned when executing command with no environment variables")

	// test that the command is executed with os environment variables
	t.Setenv(testKey, "foo")
	output, err = ctx.EnvExec(nil, "bash", []any{"-c", fmt.Sprintf("echo -n $%s", testKey)}, "")

	require.Nilf(t, err, "Expected no error to be returned when executing command with environment variables")

	require.Equalf(t, expected, output, "Expected %s to be returned when executing command with environment variables", expected)
}
