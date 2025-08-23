package hcllang

import (
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"

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
