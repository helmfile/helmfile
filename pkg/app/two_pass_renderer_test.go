package app

import (
	"bytes"
	"strings"
	"testing"

	"github.com/helmfile/helmfile/pkg/helmexec"
	"github.com/helmfile/helmfile/pkg/remote"
	"github.com/helmfile/helmfile/pkg/runtime"
	"github.com/helmfile/helmfile/pkg/state"
	"github.com/helmfile/helmfile/pkg/testhelper"
	"github.com/helmfile/helmfile/pkg/yaml"
)

// nolint: unparam
func makeLoader(files map[string]string, env string) (*desiredStateLoader, *testhelper.TestFs, *bytes.Buffer) {
	testfs := testhelper.NewTestFs(files)
	logger := newAppTestLogger()
	r := remote.NewRemote(logger, testfs.Cwd, testfs.ToFileSystem())
	var buf bytes.Buffer
	loaderLogger := helmexec.NewLogger(&buf, "debug")
	return &desiredStateLoader{
		env:       env,
		namespace: "namespace",
		logger:    loaderLogger,
		fs:        testfs.ToFileSystem(),
		remote:    r,
	}, testfs, &buf
}

func TestReadFromYaml_MakeEnvironmentHasNoSideEffects(t *testing.T) {
	yamlContent := []byte(`
environments:
  staging:
    values:
    - default/values.yaml
  production:

releases:
- name: {{ readFile "other/default/values.yaml" }}
  chart: mychart1
`)

	files := map[string]string{
		"/path/to/default/values.yaml":       ``,
		"/path/to/other/default/values.yaml": `SecondPass`,
	}

	r, testfs, _ := makeLoader(files, "staging")
	yamlBuf, err := r.renderTemplatesToYaml("", "", yamlContent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var state state.HelmState
	err = yaml.Unmarshal(yamlBuf.Bytes(), &state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if testfs.FileReaderCalls() > 2 {
		t.Error("reader should be called only twice")
	}

	if state.Releases[0].Name != "SecondPass" {
		t.Errorf("release name should have ben set as SecondPass")
	}
}

func TestReadFromYaml_RenderTemplate(t *testing.T) {
	defaultValuesYaml := `
releaseName: "hello"
conditionalReleaseTag: "yes"
`

	yamlContent := []byte(`
environments:
  staging:
    values:
    - default/values.yaml
  production:

releases:
- name: {{ .Environment.Values.releaseName }}
  chart: mychart1

{{ if (eq .Environment.Values.conditionalReleaseTag "yes") }}
- name: conditionalRelease
{{ end }}

`)

	files := map[string]string{
		"/path/to/default/values.yaml": defaultValuesYaml,
	}

	r, _, _ := makeLoader(files, "staging")
	// test the double rendering
	yamlBuf, err := r.renderTemplatesToYaml("", "", yamlContent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var state state.HelmState
	err = yaml.Unmarshal(yamlBuf.Bytes(), &state)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(state.Releases) != 2 {
		t.Fatal("there should be 2 releases")
	}

	if state.Releases[0].Name != "hello" {
		t.Errorf("release name should be hello")
	}

	if state.Releases[1].Name != "conditionalRelease" {
		t.Error("conditional release should have been present")
	}
}

func testReadFromYaml_RenderTemplateLog(t *testing.T) {
	t.Helper()

	yamlContent := []byte(`
releases:
- name: foo
  chart: mychart1
- name: bar

`)

	files := map[string]string{}

	r, _, logs := makeLoader(files, "default")
	// test the double rendering
	yamlBuf, err := r.renderTemplatesToYaml("", "", yamlContent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var state state.HelmState
	err = yaml.Unmarshal(yamlBuf.Bytes(), &state)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(state.Releases) != 2 {
		t.Fatal("there should be 2 releases")
	}

	if state.Releases[0].Name != "foo" {
		t.Errorf("release name should be hello")
	}

	if state.Releases[1].Name != "bar" {
		t.Error("conditional release should have been present")
	}

	assertLogEqualsToSnapshot(t, logs.String())
}

func TestReadFromYaml_RenderTemplateLog(t *testing.T) {
	v := runtime.V1Mode
	t.Cleanup(func() {
		runtime.V1Mode = v
	})

	t.Run("v0mode", func(t *testing.T) {
		runtime.V1Mode = false
		testReadFromYaml_RenderTemplateLog(t)
	})

	t.Run("v1mode", func(t *testing.T) {
		runtime.V1Mode = true
		testReadFromYaml_RenderTemplateLog(t)
	})
}

func TestReadFromYaml_RenderTemplateWithValuesReferenceError(t *testing.T) {
	defaultValuesYaml := ``

	yamlContent := []byte(`
environments:
  staging:
    values:
    - default/values.yaml
  production:

{{ if (eq .Environment.Values.releaseName "a") }} # line 8
releases:
- name: a
	chart: mychart1
{{ end }}
`)

	files := map[string]string{
		"/path/to/default/values.yaml": defaultValuesYaml,
	}

	r, _, _ := makeLoader(files, "staging")
	// test the double rendering
	_, err := r.renderTemplatesToYaml("", "", yamlContent)

	if !strings.Contains(err.Error(), "stringTemplate:8") {
		t.Fatalf("error should contain a stringTemplate error (reference to unknow key) %v", err)
	}
}

// This test shows that a gotmpl reference will get rendered correctly
// even if the pre-render disables the readFile and exec functions.
// This does not apply to .gotmpl files, which is a nice side-effect.
func TestReadFromYaml_RenderTemplateWithGotmpl(t *testing.T) {
	defaultValuesYamlGotmpl := `
releaseName: {{ readFile "nonIgnoredFile" }}
`

	yamlContent := []byte(`
environments:
  staging:
    values:
    - values.yaml.gotmpl
  production:

{{ if (eq .Environment.Values.releaseName "release-a") }} # line 8
releases:
- name: a
  chart: mychart1
{{ end }}
`)

	files := map[string]string{
		"/path/to/nonIgnoredFile":     `release-a`,
		"/path/to/values.yaml.gotmpl": defaultValuesYamlGotmpl,
	}

	r, _, _ := makeLoader(files, "staging")
	rendered, _ := r.renderTemplatesToYaml("", "", yamlContent)

	var state state.HelmState
	err := yaml.Unmarshal(rendered.Bytes(), &state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(state.Releases) != 1 {
		t.Fatal("there should be 1 release")
	}

	if state.Releases[0].Name != "a" {
		t.Fatal("release should have been declared")
	}
}

func TestReadFromYaml_RenderTemplateWithNamespace(t *testing.T) {
	yamlContent := []byte(`releases:
- name: {{ .Namespace }}-myrelease
  chart: mychart
`)

	files := map[string]string{}

	r, _, _ := makeLoader(files, "staging")
	yamlBuf, err := r.renderTemplatesToYaml("", "", yamlContent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var state state.HelmState
	err = yaml.Unmarshal(yamlBuf.Bytes(), &state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if state.Releases[0].Name != "namespace-myrelease" {
		t.Errorf("release name should be namespace-myrelease")
	}
}

func TestReadFromYaml_HelmfileShouldBeResilentToTemplateErrors(t *testing.T) {
	yamlContent := []byte(`environments:
  staging:
	production:

releases:
{{ if (eq .Environment.Name "production" }}  # notice syntax error: unclosed left paren
- name: prod-myrelease
{{ else }}
- name: myapp
{{ end }}
  chart: mychart
`)

	r, _, _ := makeLoader(map[string]string{}, "staging")
	_, err := r.renderTemplatesToYaml("", "", yamlContent)
	if err == nil {
		t.Fatalf("wanted error, none returned")
	}
}
