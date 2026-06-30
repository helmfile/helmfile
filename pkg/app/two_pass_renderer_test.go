package app

import (
	"bytes"
	"strings"
	"testing"

	"github.com/helmfile/helmfile/pkg/helmexec"
	"github.com/helmfile/helmfile/pkg/remote"
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
	testReadFromYaml_RenderTemplateLog(t)
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

// TestReadFromYaml_RequiredEnvFailsWithoutSelectors verifies that requiredEnv
// still fails when no selectors are active (preserving existing behavior).
// See https://github.com/helmfile/helmfile/issues/1172
func TestReadFromYaml_RequiredEnvFailsWithoutSelectors(t *testing.T) {
	yamlContent := []byte(`
releases:
- name: rel1
  chart: mychart1
  values:
  - buzz: {{ requiredEnv "HF_TEST_SELECTOR_BUZZ" }}
- name: rel2
  chart: mychart2
`)

	r, _, _ := makeLoader(map[string]string{}, "default")
	_, err := r.renderTemplatesToYaml("", "", yamlContent)
	if err == nil {
		t.Fatalf("expected requiredEnv error when no selectors are active")
	}
	if !strings.Contains(err.Error(), "required env var") {
		t.Fatalf("expected requiredEnv error, got: %v", err)
	}
}

// TestReadFromYaml_RequiredEnvLenientWithSelectors verifies that when selectors
// are active, a requiredEnv failure does not block rendering — the loader
// retries with lenient requiredEnv so the document can be parsed and then
// filtered by selectors.
// See https://github.com/helmfile/helmfile/issues/1172
func TestReadFromYaml_RequiredEnvLenientWithSelectors(t *testing.T) {
	yamlContent := []byte(`
releases:
- name: rel1
  chart: mychart1
  labels:
    tier: label1
  values:
  - buzz: {{ requiredEnv "HF_TEST_SELECTOR_BUZZ" }}
- name: rel2
  chart: mychart2
  labels:
    tier: label2
`)

	r, _, _ := makeLoader(map[string]string{}, "default")
	r.selectors = []string{"tier=label2"}

	yamlBuf, err := r.renderTemplatesToYaml("", "", yamlContent)
	if err != nil {
		t.Fatalf("expected lenient rendering to succeed with selectors, got: %v", err)
	}

	var st state.HelmState
	if err := yaml.Unmarshal(yamlBuf.Bytes(), &st); err != nil {
		t.Fatalf("failed to unmarshal rendered yaml: %v", err)
	}

	if len(st.Releases) != 2 {
		t.Fatalf("expected 2 releases, got %d", len(st.Releases))
	}
}
