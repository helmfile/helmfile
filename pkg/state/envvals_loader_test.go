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
		t.Error(diff)
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
				t.Error(diff)
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
		t.Error(diff)
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
		t.Error(diff)
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
		t.Error(diff)
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
		t.Error(diff)
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
		t.Error(diff)
	}
}

func TestEnvValsLoad_EnvironmentValues(t *testing.T) {
	l := newLoader()
	env := environment.New("test")
	env.Values["foo"] = "bar"

	actual, err := l.LoadEnvironmentValues(nil, []any{"testdata/values.9.yaml.gotmpl"}, env, "")
	if err != nil {
		t.Fatal(err)
	}

	expected := map[string]any{
		"foo": "bar",
	}

	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Error(diff)
	}
}
