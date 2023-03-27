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

	actual, err := l.LoadEnvironmentValues(nil, []interface{}{"testdata/values.5.yaml"}, nil, "")
	if err != nil {
		t.Fatal(err)
	}

	expected := map[string]interface{}{
		"affinity": map[string]interface{}{},
	}

	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf(diff)
	}
}

func TestEnvValsLoad_EnvironmentNameFile(t *testing.T) {
	l := newLoader()

	expected := map[string]interface{}{
		"envName": "test",
	}
	emptyExpected := map[string]interface{}{
		"envName": nil,
	}

	tests := []struct {
		name     string
		env      *environment.Environment
		envName  string
		expected map[string]interface{}
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
			actual, err := l.LoadEnvironmentValues(nil, []interface{}{"testdata/values.6.yaml.gotmpl"}, tt.env, tt.envName)
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

	actual, err := l.LoadEnvironmentValues(nil, []interface{}{"git::https://github.com/helm/helm.git@cmd/helm/testdata/output/values.yaml?ref=v3.8.0"}, nil, "")
	if err != nil {
		t.Fatal(err)
	}

	expected := map[string]interface{}{
		"name": string("value"),
	}

	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf(diff)
	}
}

// See https://github.com/roboll/helmfile/issues/1150
func TestEnvValsLoad_OverwriteNilValue_Issue1150(t *testing.T) {
	l := newLoader()

	actual, err := l.LoadEnvironmentValues(nil, []interface{}{"testdata/values.1.yaml", "testdata/values.2.yaml"}, nil, "")
	if err != nil {
		t.Fatal(err)
	}

	expected := map[string]interface{}{
		"components": map[string]interface{}{
			"etcd-operator": map[string]interface{}{
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

	actual, err := l.LoadEnvironmentValues(nil, []interface{}{"testdata/values.3.yaml", "testdata/values.4.yaml"}, nil, "")
	if err != nil {
		t.Fatal(err)
	}

	expected := map[string]interface{}{
		"components": map[string]interface{}{
			"etcd-operator": map[string]interface{}{
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

	actual, err := l.LoadEnvironmentValues(nil, []interface{}{"testdata/issues/1168/addons.yaml", "testdata/issues/1168/addons2.yaml"}, nil, "")
	if err != nil {
		t.Fatal(err)
	}

	expected := map[string]interface{}{
		"addons": map[string]interface{}{
			"mychart": map[string]interface{}{
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
