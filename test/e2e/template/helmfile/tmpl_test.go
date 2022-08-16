package helmfile

import (
	"bytes"
	"fmt"
	"os"
	"testing"
	"text/template"

	"github.com/stretchr/testify/require"

	"github.com/helmfile/helmfile/pkg/tmpl"
)

type tmplTestCase struct {
	//  envs are set in the test environment
	envs map[string]string
	// name of the test
	name string
	// tmplString is the template string to be parsed
	tmplString string
	// data is the data to be passed to the template
	data interface{}
	// wantErr is true if the template should fail to parse
	wantErr bool
	// output is the expected output of the template
	output string
}

// setEnvs sets the environment variables for the test case
func (t *tmplTestCase) setEnvs(tt *testing.T) {
	for k, v := range t.envs {
		tt.Setenv(k, v)
	}
}

var requireEnvTestCases = []tmplTestCase{
	{
		envs: map[string]string{
			"TEST_VAR1": "test1",
		},
		name:       "requiredEnvWithEnvs",
		tmplString: `{{ requiredEnv "TEST_VAR1" }}`,
		output:     "test1",
	},
	{
		name:       "requiredEnv",
		tmplString: `{{ requiredEnv "TEST_VAR" }}`,
		wantErr:    true,
	},
}

var requiredTestCases = []tmplTestCase{
	{
		name:       "requiredWithEmptyString",
		tmplString: `{{ "" | required "required" }}`,
		wantErr:    true,
	},
	{
		name:       "required",
		tmplString: `{{ "test" | required "required" }}`,
		output:     "test",
	},
}

var envExecTestCases = []tmplTestCase{
	{
		name:       "envExecWithEnvs",
		tmplString: `{{ envExec (dict "testkey" "test2") "bash" (list "-c" "echo -n $testkey" ) }}`,
		output:     "test2",
	},
	{
		name:       "envExec",
		tmplString: `{{ envExec (dict) "bash" (list "-c" "echo -n $testkey" ) }}`,
	},
	{
		name:       "envExecInvalidEnvType",
		wantErr:    true,
		tmplString: `{{ envExec "dict" "bash" (list "-c" "echo -n $testkey" ) }}`,
	},
}

var execTestCases = []tmplTestCase{
	{
		name:       "exec",
		tmplString: `{{ exec "bash" (list "-c" "echo -n $testkey" ) }}`,
	},
	{
		name:       "execWithError",
		wantErr:    true,
		tmplString: `{{ exec "bash" (list "-c" "exit 1" ) }}`,
	},
}

var readFileTestCases = []tmplTestCase{
	{
		name:       "readFile",
		tmplString: `{{ readFile "./testdata/tmpl/readfile.txt" }}`,
		output:     "test",
	},
	{
		name:       "readFileWithError",
		tmplString: `{{ readFile "./testdata/tmpl/readfile_error.txt" }}`,
		wantErr:    true,
	},
}

var readDirTestCases = []tmplTestCase{
	{
		name: "readDir",
		tmplString: `{{ range $index,$item := readDir "./testdata/tmpl/sample_folder/" }}
             {{- $itemSplit := splitList  "/" $item -}}
			 {{- if contains "\\" $item -}}
			 {{- $itemSplit = splitList "\\" $item -}}
			 {{- end -}}
			 {{- $itemValue := $itemSplit | last -}}
			 {{- $itemValue -}}
			 {{- end -}}`,
		output: "file1.txtfile2.txt",
	},
	{
		name:       "readDirWithError",
		tmplString: `{{ readFile "./testdata/tmpl/sample_folder_error/" }}`,
		wantErr:    true,
	},
}

var readDirEntriesTestCases = []tmplTestCase{
	{
		name: "readDirEntries",
		tmplString: `{{ range $index,$item := readDirEntries "./testdata/tmpl/sample_folder/" }}
		{{- $item.Name -}}
		{{- end -}}`,
		output: "file1.txtfile2.txtsub_folder",
	},
	{
		name: "readDirEntriesOnlyFolders",
		tmplString: `{{ range $index,$item := readDirEntries "./testdata/tmpl/sample_folder/" }}
		{{- if $item.IsDir -}}
		{{- $item.Name -}}
		{{- end -}}
		{{- end -}}`,
		output: "sub_folder",
	},
	{
		name:       "readDirEntriesWithError",
		tmplString: `{{ readDirEntries "./testdata/tmpl/sample_folder_error/" }}`,
		wantErr:    true,
	},
}

var toYamlTestCases = []tmplTestCase{
	{
		data: map[string]string{
			"test": "test",
		},
		name:       "toYaml",
		tmplString: `{{ . | toYaml }}`,
		output:     "test: test\n",
	},
}

var fromYamlTestCases = []tmplTestCase{
	{
		name: "fromYaml",
		tmplString: `{{ $value :=  "test: test" | fromYaml }}
		             {{- $value.test }}`,
		output: "test",
	},
}

var setValueAtPathTestCases = []tmplTestCase{
	{
		data: map[string]interface{}{
			"root": map[string]interface{}{
				"testKey": map[string]interface{}{
					"testKey2": "test",
				},
			},
		},
		name: "setValueAtPath",
		tmplString: `{{- $newValues := . | setValueAtPath "root.testKey.testKey2" "testNewValue" }}
		             {{- $newValues.root.testKey.testKey2 }}`,
		output: "testNewValue",
	},
	{
		data: map[string]interface{}{
			"root": "test",
		},
		wantErr:    true,
		name:       "setValueAtPathWithInvalidPath",
		tmplString: `{{ . | setValueAtPath "root.nokey" "testNewValue" }}`,
	},
}

var getTestCases = []tmplTestCase{
	{
		data: map[string]interface{}{
			"root": map[string]interface{}{
				"testGetKey": map[string]interface{}{
					"testGetKey2": "test",
				},
			},
		},
		name:       "get",
		tmplString: `{{- . | get "root.testGetKey.testGetKey2" "notfound" }}`,
		output:     "test",
	},
	{
		data: map[string]interface{}{
			"root": map[string]interface{}{
				"testGetKey": map[string]interface{}{
					"testGetKey2": "test",
				},
			},
		},
		name:       "getNotExistWithDefault",
		tmplString: `{{- . | get "root.testGetKey.testGetKey3" "notfound" }}`,
		output:     "notfound",
	},
}

var tplTestCases = []tmplTestCase{
	{
		data: map[string]interface{}{
			"root": "tplvalue",
		},
		name:       "tpl",
		tmplString: `{{ . | tpl "{{ .root }}" }}`,
		output:     "tplvalue",
	},
	{
		data:       map[string]interface{}{},
		name:       "tplInvalidTemplate",
		wantErr:    true,
		tmplString: `{{ . | tpl "{{ .root }}" }}`,
	},
}

// tmplTestCases are the test cases for the template tests
type tmplE2e struct {
	tcs []tmplTestCase
}

// append for append testcase into tmplTestCases
func (t *tmplE2e) append(ts ...tmplTestCase) {
	t.tcs = append(t.tcs, ts...)
}

// load for  load testcase into tmplTestCases
func (t *tmplE2e) load() {
	t.append(requireEnvTestCases...)
	t.append(requiredTestCases...)
	t.append(envExecTestCases...)
	t.append(execTestCases...)
	t.append(readFileTestCases...)
	t.append(readDirTestCases...)
	t.append(readDirEntriesTestCases...)
	t.append(toYamlTestCases...)
	t.append(fromYamlTestCases...)
	t.append(setValueAtPathTestCases...)
	t.append(getTestCases...)
	t.append(tplTestCases...)
}

var tmplE2eTest = tmplE2e{}

func TestFileRendering(t *testing.T) {

	tmplE2eTest.load()

	for _, tc := range tmplE2eTest.tcs {

		t.Run(tc.name, func(t *testing.T) {
			tc.setEnvs(t)
			tempDir, _ := os.MkdirTemp("./testdata", "test")
			defer os.RemoveAll(tempDir)

			filename := fmt.Sprintf("%s/%s.gotmpl", tempDir, tc.name)
			os.WriteFile(filename, []byte(tc.tmplString), 0644)
			fileRenderer := tmpl.NewFileRenderer(os.ReadFile, ".", tc.data)
			tmpl_bytes, err := fileRenderer.RenderToBytes(filename)

			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			tmpl := string(tmpl_bytes)
			require.Equal(t, tc.output, tmpl)
		})
	}
}

// TestTmplStrings tests the template string
func TestTmplStrings(t *testing.T) {
	c := &tmpl.Context{}
	c.SetBasePath(".")
	c.SetReadFile(os.ReadFile)
	c.SetReadDir(os.ReadDir)
	tmpl := template.New("stringTemplateTest").Funcs(c.CreateFuncMap())

	tmplE2eTest.load()

	for _, tc := range tmplE2eTest.tcs {
		t.Run(tc.name, func(t *testing.T) {
			tc.setEnvs(t)
			tmpl, err := tmpl.Parse(tc.tmplString)
			require.Nilf(t, err, "error parsing template: %v", err)

			var tplResult bytes.Buffer
			err = tmpl.Execute(&tplResult, tc.data)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tc.output, tplResult.String())
		})
	}
}
