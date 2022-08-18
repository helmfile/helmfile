package state

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"go.uber.org/zap"

	"github.com/helmfile/helmfile/pkg/remote"
)

func newLoader() *EnvironmentValuesLoader {
	log, err := zap.NewDevelopment(zap.AddStacktrace(zap.DebugLevel))
	if err != nil {
		panic(err)
	}

	sugar := log.Sugar()

	storage := &Storage{
		FilePath: "./helmfile.yaml",
		basePath: ".",
		glob:     filepath.Glob,
		logger:   sugar,
	}

	readFile := func(s string) ([]byte, error) { return []byte{}, nil }
	dirExists := func(d string) bool { return false }
	fileExists := func(f string) bool { return false }
	return NewEnvironmentValuesLoader(storage, os.ReadFile, os.ReadDir, sugar, remote.NewRemote(sugar, "/tmp", readFile, dirExists, fileExists))
}

// See https://github.com/roboll/helmfile/pull/1169
func TestEnvValsLoad_SingleValuesFile(t *testing.T) {
	l := newLoader()

	actual, err := l.LoadEnvironmentValues(nil, []interface{}{"testdata/values.5.yaml"}, nil)
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

// Fetch Environment values from remote
func TestEnvValsLoad_SingleValuesFileRemote(t *testing.T) {
	l := newLoader()

	actual, err := l.LoadEnvironmentValues(nil, []interface{}{"git::https://github.com/helm/helm.git@cmd/helm/testdata/output/values.yaml?ref=v3.8.0"}, nil)
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

	actual, err := l.LoadEnvironmentValues(nil, []interface{}{"testdata/values.1.yaml", "testdata/values.2.yaml"}, nil)
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

	actual, err := l.LoadEnvironmentValues(nil, []interface{}{"testdata/values.3.yaml", "testdata/values.4.yaml"}, nil)
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

	actual, err := l.LoadEnvironmentValues(nil, []interface{}{"testdata/issues/1168/addons.yaml", "testdata/issues/1168/addons2.yaml"}, nil)
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
