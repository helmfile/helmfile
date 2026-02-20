package hcllang

import (
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"

	ffs "github.com/helmfile/helmfile/pkg/filesystem"
	"github.com/helmfile/helmfile/pkg/helmexec"
)

func newHCLLoader() *HCLLoader {
	log := helmexec.NewLogger(io.Discard, "debug")
	return &HCLLoader{
		fs:     ffs.DefaultFileSystem(),
		logger: log,
	}
}

func TestHCL_localsTraversalsParser(t *testing.T) {
	l := newHCLLoader()
	files := []string{"testdata/values.1.hcl"}
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
		"testdata/values.1.hcl": {
			"myLocal":    0,
			"myLocalRef": 1,
		},
	}

	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Error(diff)
	}
}

func TestHCL_localsTraversalsAttrParser(t *testing.T) {
	l := newHCLLoader()
	files := []string{"testdata/values.1.hcl"}
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
				tr, _ := l.parseSingleAttrRef(v, LocalsBlockIdentifier)
				str += tr
			}
			actual[file][k] = str
		}
	}

	expected := map[string]map[string]string{
		"testdata/values.1.hcl": {
			"myLocal":    "",
			"myLocalRef": "myLocal",
		},
	}

	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Error(diff)
	}
}
func TestHCL_valuesTraversalsParser(t *testing.T) {
	l := newHCLLoader()
	files := []string{"testdata/values.1.hcl"}
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
		"val4": 1,
	}

	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Error(diff)
	}
}

func TestHCL_valuesTraversalsAttrParser(t *testing.T) {
	l := newHCLLoader()
	files := []string{"testdata/values.1.hcl"}
	l.AddFiles(files)

	fileValues, _, diags := l.readHCLs()
	if diags != nil {
		t.Errorf("Test file parsing error : %s", diags.Errs()[0].Error())
	}

	actual := make(map[string]string)
	for k, v := range fileValues {
		str := ""
		for _, v := range v.Expr.Variables() {
			tr, _ := l.parseSingleAttrRef(v, ValuesBlockIdentifier)
			str += tr
		}
		actual[k] = str
	}

	expected := map[string]string{
		"val1": "",
		"val2": "",
		"val3": "val1",
		"val4": "val1",
	}

	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Error(diff)
	}
}

func TestHCL_resultValidate(t *testing.T) {
	l := newHCLLoader()
	files := []string{"testdata/values.1.hcl"}
	l.AddFiles(files)
	actual, err := l.HCLRender()
	if err != nil {
		t.Errorf("Render error : %s", err.Error())
	}

	expected := map[string]any{
		"val1": float64(1),
		"val2": "LOCAL",
		"val3": "local1",
		"val4": float64(-1),
	}

	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Error(diff)
	}
}

func TestCtyMergeValues_SimpleTypes(t *testing.T) {
	tests := []struct {
		name     string
		a        cty.Value
		b        cty.Value
		expected cty.Value
	}{
		{
			name:     "merge strings - b wins",
			a:        cty.StringVal("original"),
			b:        cty.StringVal("override"),
			expected: cty.StringVal("override"),
		},
		{
			name:     "merge numbers - b wins",
			a:        cty.NumberIntVal(42),
			b:        cty.NumberIntVal(100),
			expected: cty.NumberIntVal(100),
		},
		{
			name:     "merge bools - b wins",
			a:        cty.BoolVal(true),
			b:        cty.BoolVal(false),
			expected: cty.BoolVal(false),
		},
		{
			name:     "b is null - keep a",
			a:        cty.StringVal("keep"),
			b:        cty.NullVal(cty.String),
			expected: cty.StringVal("keep"),
		},
		{
			name:     "a is null - use b",
			a:        cty.NullVal(cty.String),
			b:        cty.StringVal("new"),
			expected: cty.StringVal("new"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ctyMergeValues(tt.a, tt.b)
			if !result.RawEquals(tt.expected) {
				t.Errorf("ctyMergeValues() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCtyMergeValues_Objects(t *testing.T) {
	tests := []struct {
		name     string
		a        cty.Value
		b        cty.Value
		expected cty.Value
	}{
		{
			name: "merge objects - shallow override",
			a: cty.ObjectVal(map[string]cty.Value{
				"key1": cty.StringVal("value1"),
				"key2": cty.StringVal("value2"),
			}),
			b: cty.ObjectVal(map[string]cty.Value{
				"key2": cty.StringVal("overridden"),
				"key3": cty.StringVal("new"),
			}),
			expected: cty.ObjectVal(map[string]cty.Value{
				"key1": cty.StringVal("value1"),
				"key2": cty.StringVal("overridden"),
				"key3": cty.StringVal("new"),
			}),
		},
		{
			name: "merge objects - nested merge",
			a: cty.ObjectVal(map[string]cty.Value{
				"parent": cty.ObjectVal(map[string]cty.Value{
					"child1": cty.StringVal("original"),
					"child2": cty.StringVal("keep"),
				}),
			}),
			b: cty.ObjectVal(map[string]cty.Value{
				"parent": cty.ObjectVal(map[string]cty.Value{
					"child1": cty.StringVal("override"),
					"child3": cty.StringVal("new"),
				}),
			}),
			expected: cty.ObjectVal(map[string]cty.Value{
				"parent": cty.ObjectVal(map[string]cty.Value{
					"child1": cty.StringVal("override"),
					"child2": cty.StringVal("keep"),
					"child3": cty.StringVal("new"),
				}),
			}),
		},
		{
			name: "merge objects - new keys in b",
			a: cty.ObjectVal(map[string]cty.Value{
				"existing": cty.StringVal("value"),
			}),
			b: cty.ObjectVal(map[string]cty.Value{
				"new1": cty.StringVal("newvalue1"),
				"new2": cty.NumberIntVal(42),
			}),
			expected: cty.ObjectVal(map[string]cty.Value{
				"existing": cty.StringVal("value"),
				"new1":     cty.StringVal("newvalue1"),
				"new2":     cty.NumberIntVal(42),
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ctyMergeValues(tt.a, tt.b)
			if !result.RawEquals(tt.expected) {
				t.Errorf("ctyMergeValues() = %#v, want %#v", result, tt.expected)
			}
		})
	}
}

func TestCtyMergeValues_Lists(t *testing.T) {
	tests := []struct {
		name     string
		a        cty.Value
		b        cty.Value
		expected cty.Value
	}{
		{
			name: "merge lists - b replaces a",
			a: cty.ListVal([]cty.Value{
				cty.StringVal("a"),
				cty.StringVal("b"),
			}),
			b: cty.ListVal([]cty.Value{
				cty.StringVal("x"),
				cty.StringVal("y"),
				cty.StringVal("z"),
			}),
			expected: cty.ListVal([]cty.Value{
				cty.StringVal("x"),
				cty.StringVal("y"),
				cty.StringVal("z"),
			}),
		},
		{
			name: "merge tuples - b replaces a",
			a: cty.TupleVal([]cty.Value{
				cty.StringVal("a"),
				cty.NumberIntVal(1),
			}),
			b: cty.TupleVal([]cty.Value{
				cty.StringVal("x"),
				cty.NumberIntVal(99),
			}),
			expected: cty.TupleVal([]cty.Value{
				cty.StringVal("x"),
				cty.NumberIntVal(99),
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ctyMergeValues(tt.a, tt.b)
			if !result.RawEquals(tt.expected) {
				t.Errorf("ctyMergeValues() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestHclParseError(t *testing.T) {
	hv := &HelmfileHCLValue{
		Name: "testVar",
		Range: hcl.Range{
			Filename: "test.hcl",
			Start:    hcl.Pos{Line: 10, Column: 5},
		},
	}

	diag := hclParseError("myKey", hv)

	if diag.Severity != hcl.DiagError {
		t.Errorf("Expected severity DiagError, got %v", diag.Severity)
	}

	if diag.Summary != "Unable to parse HCL expression" {
		t.Errorf("Unexpected summary: %s", diag.Summary)
	}

	expectedDetail := `The helmfile_var "myKey" defined at test.hcl:10 can't be parsed`
	if diag.Detail != expectedDetail {
		t.Errorf("Expected detail %q, got %q", expectedDetail, diag.Detail)
	}

	if diag.Subject != &hv.Range {
		t.Error("Expected Subject to point to hv.Range")
	}
}

func TestHCL_ValuesOverride(t *testing.T) {
	l := newHCLLoader()
	files := []string{"testdata/override.1.hcl", "testdata/override.2.hcl"}
	l.AddFiles(files)

	actual, err := l.HCLRender()
	if err != nil {
		t.Fatalf("Render error: %s", err.Error())
	}

	// Verify that values from file2 override file1
	if actual["env"] != "prod" {
		t.Errorf("Expected env=prod (from override), got %v", actual["env"])
	}

	// Verify nested object merge
	config, ok := actual["config"].(map[string]any)
	if !ok {
		t.Fatalf("Expected config to be a map, got %T", actual["config"])
	}

	if config["replicas"] != float64(3) {
		t.Errorf("Expected replicas=3 (overridden), got %v", config["replicas"])
	}

	if config["image"] != "v1.0" {
		t.Errorf("Expected image=v1.0 (preserved from file1), got %v", config["image"])
	}

	if config["debug"] != true {
		t.Errorf("Expected debug=true (new from file2), got %v", config["debug"])
	}

	// Verify list from file1 is preserved (file2 doesn't define tags)
	tags, ok := actual["tags"].([]any)
	if !ok || len(tags) != 2 {
		t.Errorf("Expected tags to be preserved from file1, got %v", actual["tags"])
	}

	// Verify new key from file2
	if actual["region"] != "us-east" {
		t.Errorf("Expected region=us-east (new in file2), got %v", actual["region"])
	}
}

func TestHCL_ValuesOverride_ThreeFiles(t *testing.T) {
	l := newHCLLoader()
	files := []string{"testdata/multi.1.hcl", "testdata/multi.2.hcl", "testdata/multi.3.hcl"}
	l.AddFiles(files)

	actual, err := l.HCLRender()
	if err != nil {
		t.Fatalf("Render error: %s", err.Error())
	}

	// Last file wins for simple values
	if actual["base"] != "file3" {
		t.Errorf("Expected base=file3 (last override), got %v", actual["base"])
	}

	// Check deep merge of shared object
	shared, ok := actual["shared"].(map[string]any)
	if !ok {
		t.Fatalf("Expected shared to be a map, got %T", actual["shared"])
	}

	if shared["key1"] != "v1" {
		t.Errorf("Expected key1=v1 (from file1, preserved), got %v", shared["key1"])
	}

	if shared["key2"] != "override2" {
		t.Errorf("Expected key2=override2 (from file2, not overridden by file3), got %v", shared["key2"])
	}

	if shared["key3"] != "final3" {
		t.Errorf("Expected key3=final3 (from file3, final override), got %v", shared["key3"])
	}

	if shared["key4"] != "v4" {
		t.Errorf("Expected key4=v4 (from file3, new key), got %v", shared["key4"])
	}

	if actual["final"] != "last" {
		t.Errorf("Expected final=last (new in file3), got %v", actual["final"])
	}
}
