package tmpl

import (
	"fmt"
	"reflect"
	"testing"

	ffs "github.com/helmfile/helmfile/pkg/filesystem"
)

func TestRenderTemplate_Values(t *testing.T) {
	valuesYamlContent := `foo:
  bar: BAR
`
	expected := `foo:
  bar: FOO_BAR
`
	expectedFilename := "values.yaml"
	ctx := &Context{fs: &ffs.FileSystem{
		Glob: func(s string) ([]string, error) {
			return nil, nil
		},
		ReadFile: func(filename string) ([]byte, error) {
			if filename != expectedFilename {
				return nil, fmt.Errorf("unexpected filename: expected=%v, actual=%s", expectedFilename, filename)
			}
			return []byte(valuesYamlContent), nil
		}}}
	buf, err := ctx.RenderTemplateToBuffer(`{{ readFile "values.yaml" | fromYaml | setValueAtPath "foo.bar" "FOO_BAR" | toYaml }}`)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	actual := buf.String()
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("unexpected result: expected=%v, actual=%v", expected, actual)
	}
}

func TestRenderTemplate_WithData(t *testing.T) {
	valuesYamlContent := `foo:
  bar: {{ .foo.bar }}
`
	expected := `foo:
  bar: FOO_BAR
`
	expectedFilename := "values.yaml"
	data := map[string]any{
		"foo": map[string]any{
			"bar": "FOO_BAR",
		},
	}
	ctx := &Context{fs: &ffs.FileSystem{
		Glob: func(s string) ([]string, error) {
			return nil, nil
		},
		ReadFile: func(filename string) ([]byte, error) {
			if filename != expectedFilename {
				return nil, fmt.Errorf("unexpected filename: expected=%v, actual=%s", expectedFilename, filename)
			}
			return []byte(valuesYamlContent), nil
		}}}
	buf, err := ctx.RenderTemplateToBuffer(valuesYamlContent, data)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	actual := buf.String()
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("unexpected result: expected=%v, actual=%v", expected, actual)
	}
}

func TestRenderTemplate_AccessingMissingKeyWithGetOrNil(t *testing.T) {
	valuesYamlContent := `foo:
  bar: {{ . | getOrNil "foo.bar" }}
`
	expected := `foo:
  bar: <no value>
`
	expectedFilename := "values.yaml"
	data := map[string]any{}
	ctx := &Context{fs: &ffs.FileSystem{
		Glob: func(s string) ([]string, error) {
			return nil, nil
		},
		ReadFile: func(filename string) ([]byte, error) {
			if filename != expectedFilename {
				return nil, fmt.Errorf("unexpected filename: expected=%v, actual=%s", expectedFilename, filename)
			}
			return []byte(valuesYamlContent), nil
		}}}
	buf, err := ctx.RenderTemplateToBuffer(valuesYamlContent, data)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	actual := buf.String()
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("unexpected result: expected=%v, actual=%v", expected, actual)
	}
}

func TestRenderTemplate_Defaulting(t *testing.T) {
	valuesYamlContent := `foo:
  bar: {{ . | getOrNil "foo.bar" | default "DEFAULT" }}
`
	expected := `foo:
  bar: DEFAULT
`
	expectedFilename := "values.yaml"
	data := map[string]any{}
	ctx := &Context{fs: &ffs.FileSystem{
		Glob: func(s string) ([]string, error) {
			return nil, nil
		},
		ReadFile: func(filename string) ([]byte, error) {
			if filename != expectedFilename {
				return nil, fmt.Errorf("unexpected filename: expected=%v, actual=%s", expectedFilename, filename)
			}
			return []byte(valuesYamlContent), nil
		}}}
	buf, err := ctx.RenderTemplateToBuffer(valuesYamlContent, data)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	actual := buf.String()
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("unexpected result: expected=%v, actual=%v", expected, actual)
	}
}

func renderTemplateToString(s string, data ...any) (string, error) {
	ctx := &Context{fs: &ffs.FileSystem{
		Glob: func(s string) ([]string, error) {
			return nil, nil
		},
		ReadFile: func(filename string) ([]byte, error) {
			return nil, fmt.Errorf("unexpected call to readFile: filename=%s", filename)
		}}}
	tplString, err := ctx.RenderTemplateToBuffer(s, data...)
	if err != nil {
		return "", err
	}
	return tplString.String(), nil
}

func Test_renderTemplateToString(t *testing.T) {
	type args struct {
		s    string
		envs map[string]string
		data any
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "simple replacement",
			args: args{
				s: "{{ env \"HF_TEST_VAR\" }}",
				envs: map[string]string{
					"HF_TEST_VAR": "content",
				},
			},
			want:    "content",
			wantErr: false,
		},
		{
			name: "two replacements",
			args: args{
				s: "{{ env \"HF_TEST_ALPHA\" }}{{ env \"HF_TEST_BETA\" }}",
				envs: map[string]string{
					"HF_TEST_ALPHA": "first",
					"HF_TEST_BETA":  "second",
				},
			},
			want:    "firstsecond",
			wantErr: false,
		},
		{
			name: "replacement and comment",
			args: args{
				s: "{{ env \"HF_TEST_ALPHA\" }}{{/* comment */}}",
				envs: map[string]string{
					"HF_TEST_ALPHA": "first",
				},
			},
			want:    "first",
			wantErr: false,
		},
		{
			name: "global template function",
			args: args{
				s: "{{ env \"HF_TEST_ALPHA\" | len }}",
				envs: map[string]string{
					"HF_TEST_ALPHA": "abcdefg",
				},
			},
			want:    "7",
			wantErr: false,
		},
		{
			name: "get",
			args: args{
				s:    `{{ . | get "Foo" }}, {{ . | get "Bar" "2" }}`,
				envs: map[string]string{},
				data: map[string]any{
					"Foo": "1",
				},
			},
			want:    "1, 2",
			wantErr: false,
		},
		{
			name: "env var not set",
			args: args{
				s: "{{ env \"HF_TEST_NONE\" }}",
				envs: map[string]string{
					"HF_TEST_THIS": "first",
				},
			},
			want: "",
		},
		{
			name: "undefined function",
			args: args{
				s: "{{ env foo }}",
				envs: map[string]string{
					"foo": "bar",
				},
			},
			wantErr: true,
		},
		{
			name: "required env var",
			args: args{
				s: "{{ requiredEnv \"HF_TEST\" }}",
				envs: map[string]string{
					"HF_TEST": "value",
				},
			},
			want:    "value",
			wantErr: false,
		},
		{
			name: "required env var not set",
			args: args{
				s:    "{{ requiredEnv \"HF_TEST_NONE\" }}",
				envs: map[string]string{},
			},
			wantErr: true,
		},
	}
	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.args.envs {
				t.Setenv(k, v)
			}
			got, err := renderTemplateToString(tt.args.s, tt.args.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("renderTemplateToString() for %s error = %v, wantErr %v", tt.name, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("renderTemplateToString() for %s = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestRenderTemplate_Required(t *testing.T) {
	tests := []struct {
		name    string
		s       string
		data    map[string]any
		want    string
		wantErr bool
	}{
		{
			name: ".foo is existed",
			s:    `{{ required ".foo.bar is required" .foo }}`,
			data: map[string]any{
				"foo": "bar",
			},
			want:    "bar",
			wantErr: false,
		},
		{
			name: ".foo.bar is existed",
			s:    `{{ required "foo.bar is required" .foo.bar }}`,
			data: map[string]any{
				"foo": map[string]any{
					"bar": "FOO_BAR",
				},
			},
			want:    "FOO_BAR",
			wantErr: false,
		},
		{
			name: ".foo.bar is existed but value is nil",
			s:    `{{ required "foo.bar is required" .foo.bar }}`,
			data: map[string]any{
				"foo": map[string]any{
					"bar": nil,
				},
			},
			wantErr: true,
		},
		{
			name: ".foo.bar is existed but value is empty string",
			s:    `{{ required "foo.bar is required" .foo.bar }}`,
			data: map[string]any{
				"foo": map[string]any{
					"bar": "",
				},
			},
			wantErr: true,
		},
		{
			name: ".foo is nil",
			s:    `{{ required "foo is required" .foo }}`,
			data: map[string]any{
				"foo": nil,
			},
			wantErr: true,
		},
		{
			name: ".foo is a empty string",
			s:    `{{ required "foo is required" .foo }}`,
			data: map[string]any{
				"foo": "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		got, err := renderTemplateToString(tt.s, tt.data)
		if (err != nil) != tt.wantErr {
			t.Errorf("renderTemplateToString() for %s error = %v, wantErr %v", tt.name, err, tt.wantErr)
			return
		}
		if got != tt.want {
			t.Errorf("renderTemplateToString() for %s = %v, want %v", tt.name, got, tt.want)
		}
	}
}
func TestContext_helperTPLs(t *testing.T) {
	c := &Context{
		fs: &ffs.FileSystem{
			Glob: func(s string) ([]string, error) {
				return []string{
					"/helmfiletmpl/_template1.tpl",
					"/helmfiletmpl/_template2.tpl",
				}, nil
			},
			ReadFile: func(filename string) ([]byte, error) {
				switch filename {
				case "/helmfiletmpl/_template1.tpl":
					return []byte("Template 1 content"), nil
				case "/helmfiletmpl/_template2.tpl":
					return []byte("Template 2 content"), nil
				default:
					return nil, fmt.Errorf("unexpected filename: %s", filename)
				}
			},
		},
	}

	want := []tplInfo{
		{
			name:    "/helmfiletmpl/_template1.tpl",
			content: "Template 1 content",
		},
		{
			name:    "/helmfiletmpl/_template2.tpl",
			content: "Template 2 content",
		},
	}
	got, err := c.helperTPLs()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("unexpected result: got=%v, want=%v", got, want)
	}
}
func TestContext_RenderTemplateToBuffer(t *testing.T) {
	c := &Context{
		basePath: "/helmfile",
		fs: &ffs.FileSystem{
			Glob: func(s string) ([]string, error) {
				return []string{
					"/helmfile/_template1.tpl",
				}, nil
			},
			ReadFile: func(filename string) ([]byte, error) {
				if filename == "/helmfile/_template1.tpl" {
					return []byte("{{- define \"name\" -}}\n{{ .Name }}\n{{- end }}"), nil
				}
				return nil, fmt.Errorf("unexpected filename: %s", filename)
			},
		},
	}
	s := "Hello, {{ include \"name\" . }}!"
	data := map[string]interface{}{
		"Name": "Alice",
	}
	expected := "Hello, Alice!"

	buf, err := c.RenderTemplateToBuffer(s, data)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	actual := buf.String()
	if actual != expected {
		t.Errorf("unexpected result: expected=%s, actual=%s", expected, actual)
	}
}
