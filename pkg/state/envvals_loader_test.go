package state

import (
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/helmfile/helmfile/pkg/environment"
	ffs "github.com/helmfile/helmfile/pkg/filesystem"
	"github.com/helmfile/helmfile/pkg/helmexec"
	"github.com/helmfile/helmfile/pkg/remote"
)

func newLoader() *EnvironmentValuesLoader {
	log := helmexec.NewLogger(io.Discard, "debug")

	storage := &Storage{
		FilePath: "./helmfile.yaml",
		basePath: ".",
		fs:       ffs.DefaultFileSystem(),
		logger:   log,
	}

	return NewEnvironmentValuesLoader(storage, storage.fs, log, remote.NewRemote(log, "/tmp", storage.fs))
}

func newHCLLoader() *HCLLoader {
	log := helmexec.NewLogger(io.Discard, "debug")

	storage := &Storage{
		FilePath: "./helmfile.yaml",
		basePath: ".",
		fs:       ffs.DefaultFileSystem(),
		logger:   log,
	}

	return &HCLLoader{
		fs:     storage.fs,
		logger: log,
	}
}

// See https://github.com/roboll/helmfile/pull/1169
func TestEnvValsLoad_SingleValuesFile(t *testing.T) {
	l := newLoader()

	actual, err := l.LoadEnvironmentValues(nil, []any{"testdata/values.5.yaml"}, nil, "")
	if err != nil {
		t.Fatal(err)
	}

	expected := map[string]any{
		"affinity": map[string]any{},
	}

	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf(diff)
	}
}

func TestEnvValsLoad_EnvironmentNameFile(t *testing.T) {
	l := newLoader()

	expected := map[string]any{
		"envName": "test",
	}
	emptyExpected := map[string]any{
		"envName": nil,
	}

	tests := []struct {
		name     string
		env      *environment.Environment
		envName  string
		expected map[string]any
	}{
		{
			name:     "env is nil but envName is not",
			env:      nil,
			envName:  "test",
			expected: expected,
		},
		{
			name:     "envName is emplte but env is not",
			env:      environment.New("test"),
			envName:  "",
			expected: expected,
		},
		{
			name:     "envName and env is not nil",
			env:      environment.New("test"),
			envName:  "test",
			expected: expected,
		},
		{
			name:     "envName and env is nil",
			env:      nil,
			envName:  "",
			expected: emptyExpected,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := l.LoadEnvironmentValues(nil, []any{"testdata/values.6.yaml.gotmpl"}, tt.env, tt.envName)
			if err != nil {
				t.Fatal(err)
			}

			if diff := cmp.Diff(tt.expected, actual); diff != "" {
				t.Errorf(diff)
			}
		})
	}
}

// Fetch Environment values from remote
func TestEnvValsLoad_SingleValuesFileRemote(t *testing.T) {
	l := newLoader()

	actual, err := l.LoadEnvironmentValues(nil, []any{"git::https://github.com/helm/helm.git@cmd/helm/testdata/output/values.yaml?ref=v3.8.1"}, nil, "")
	if err != nil {
		t.Fatal(err)
	}

	expected := map[string]any{
		"name": string("value"),
	}

	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf(diff)
	}
}

// See https://github.com/roboll/helmfile/issues/1150
func TestEnvValsLoad_OverwriteNilValue_Issue1150(t *testing.T) {
	l := newLoader()

	actual, err := l.LoadEnvironmentValues(nil, []any{"testdata/values.1.yaml", "testdata/values.2.yaml"}, nil, "")
	if err != nil {
		t.Fatal(err)
	}

	expected := map[string]any{
		"components": map[string]any{
			"etcd-operator": map[string]any{
				"version": "0.10.3",
			},
		},
	}

	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf(diff)
	}
}

// See https://github.com/roboll/helmfile/issues/1154
func TestEnvValsLoad_OverwriteWithNilValue_Issue1154(t *testing.T) {
	l := newLoader()

	actual, err := l.LoadEnvironmentValues(nil, []any{"testdata/values.3.yaml", "testdata/values.4.yaml"}, nil, "")
	if err != nil {
		t.Fatal(err)
	}

	expected := map[string]any{
		"components": map[string]any{
			"etcd-operator": map[string]any{
				"version": "0.10.3",
			},
			"prometheus": nil,
		},
	}

	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf(diff)
	}
}

// See https://github.com/roboll/helmfile/issues/1168
func TestEnvValsLoad_OverwriteEmptyValue_Issue1168(t *testing.T) {
	l := newLoader()

	actual, err := l.LoadEnvironmentValues(nil, []any{"testdata/issues/1168/addons.yaml", "testdata/issues/1168/addons2.yaml"}, nil, "")
	if err != nil {
		t.Fatal(err)
	}

	expected := map[string]any{
		"addons": map[string]any{
			"mychart": map[string]any{
				"skip":      false,
				"name":      "mychart",
				"namespace": "kube-system",
				"chart":     "stable/mychart",
				"version":   "1.0.0",
			},
		},
	}

	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf(diff)
	}
}

func TestHCL_localsTraversalsParser(t *testing.T) {

	l := newHCLLoader()
	files := []string{"testdata/values.9.hcl"}
	l.AddFiles(files)

	_, filesLocals, diags := l.readHCLs()
	if diags != nil {
		t.Errorf("Test file parsing error : %s", diags.Errs()[0].Error())
	}

	actual := make(map[string]map[string]int)
	for file, locals := range filesLocals {
		actual[file] = make(map[string]int)
		for k, v := range locals {
			actual[file][k] = len(v.Expr.Variables())
		}
	}

	expected := map[string]map[string]int{
		"testdata/values.9.hcl": {
			"myLocal":    0,
			"myLocalRef": 1,
		},
	}

	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf(diff)
	}
}

func TestHCL_localsTraversalsAttrParser(t *testing.T) {

	l := newHCLLoader()
	files := []string{"testdata/values.9.hcl"}
	l.AddFiles(files)

	_, filesLocals, diags := l.readHCLs()
	if diags != nil {
		t.Errorf("Test file parsing error : %s", diags.Errs()[0].Error())
	}

	actual := make(map[string]map[string]string)
	for file, locals := range filesLocals {
		actual[file] = make(map[string]string)
		for k, v := range locals {
			str := ""
			for _, v := range v.Expr.Variables() {
				tr, _ := l.parseSingleAttrRef(v, localsBlockIdentifier)
				str += tr
			}
			actual[file][k] = str
		}
	}

	expected := map[string]map[string]string{
		"testdata/values.9.hcl": {
			"myLocal":    "",
			"myLocalRef": "myLocal",
		},
	}

	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf(diff)
	}
}
func TestHCL_valuesTraversalsParser(t *testing.T) {

	l := newHCLLoader()
	files := []string{"testdata/values.9.hcl"}
	l.AddFiles(files)

	fileValues, _, diags := l.readHCLs()
	if diags != nil {
		t.Errorf("Test file parsing error : %s", diags.Errs()[0].Error())
	}

	actual := make(map[string]int)
	for k, v := range fileValues {
		actual[k] = len(v.Expr.Variables())
	}

	expected := map[string]int{
		"val1": 0,
		"val2": 1,
		"val3": 2,
	}

	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf(diff)
	}
}

func TestHCL_valuesTraversalsAttrParser(t *testing.T) {

	l := newHCLLoader()
	files := []string{"testdata/values.9.hcl"}
	l.AddFiles(files)

	fileValues, _, diags := l.readHCLs()
	if diags != nil {
		t.Errorf("Test file parsing error : %s", diags.Errs()[0].Error())
	}

	actual := make(map[string]string)
	for k, v := range fileValues {
		str := ""
		for _, v := range v.Expr.Variables() {
			tr, _ := l.parseSingleAttrRef(v, valuesBlockIdentifier)
			str += tr
		}
		actual[k] = str
	}

	expected := map[string]string{
		"val1": "",
		"val2": "val1",
		"val3": "val1",
	}

	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf(diff)
	}
}

func TestEnvValsLoad_MultiHCL(t *testing.T) {
	l := newLoader()

	actual, err := l.LoadEnvironmentValues(nil, []any{"testdata/values.7.hcl", "testdata/values.8.hcl"}, nil, "")
	if err != nil {
		t.Fatal(err)
	}

	expected := map[string]any{
		"a": "a",
		"b": "b",
		"c": "ab",
		"map": map[string]any{
			"a": "a",
		},
		"list": []any{
			"b",
		},
		"nestedmap": map[string]any{
			"submap": map[string]any{
				"subsubmap": map[string]any{
					"hello": "ab",
				},
			},
		},
		"ternary":          true,
		"fromMap":          "aab",
		"expressionInText": "yes",
		"insideFor":        "b",
		"multi_block":      "block",
		"block":            "block",
		"crossfile":        "crossfile var",
		"crossfile_var":    "crossfile var",
		"localRef":         "localInValues7",
	}

	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf(diff)
	}
}
