package environment

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

// See https://github.com/roboll/helmfile/issues/1150
func TestMerge_OverwriteNilValue_Issue1150(t *testing.T) {
	dst := &Environment{
		Name: "dst",
		Values: map[string]interface{}{
			"components": map[string]interface{}{
				"etcd-operator": nil,
			},
		},
		Defaults: nil,
	}

	src := &Environment{
		Name: "src",
		Values: map[string]interface{}{
			"components": map[string]interface{}{
				"etcd-operator": map[string]interface{}{
					"version": "0.10.3",
				},
			},
		},
		Defaults: nil,
	}

	merged, err := dst.Merge(src)
	if err != nil {
		t.Fatal(err)
	}

	actual := merged.Values

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
func TestMerge_OverwriteWithNilValue_Issue1154(t *testing.T) {
	dst := &Environment{
		Name: "dst",
		Values: map[string]interface{}{
			"components": map[string]interface{}{
				"etcd-operator": map[string]interface{}{
					"version": "0.10.0",
				},
			},
		},
		Defaults: nil,
	}

	src := &Environment{
		Name: "src",
		Values: map[string]interface{}{
			"components": map[string]interface{}{
				"etcd-operator": map[string]interface{}{
					"version": "0.10.3",
				},
				"prometheus": nil,
			},
		},
		Defaults: nil,
	}

	merged, err := dst.Merge(src)
	if err != nil {
		t.Fatal(err)
	}

	actual := merged.Values

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
