package state

import (
	nativejson "encoding/json"
	"fmt"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/imdario/mergo"
	"github.com/variantdev/dag/pkg/dag"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/json"
	"go.uber.org/zap"

	"github.com/helmfile/helmfile/pkg/filesystem"
)

const (
	badIdentifierDetail         = "A name must start with a letter or underscore and may contain only letters, digits, underscores, and dashes."
	helmfileVarsBlockIdentifier = "helmfile_vars"
	helmfileVarsAccessorPrefix  = "hv"
)

type HCLLoader struct {
	hclFilesPath []string
	fs           *filesystem.FileSystem
	logger       *zap.SugaredLogger
}

func (hl *HCLLoader) AddFile(file string) {
	hl.hclFilesPath = append(hl.hclFilesPath, file)
}

func (hl *HCLLoader) AddFiles(files []string) {
	hl.hclFilesPath = append(hl.hclFilesPath, files...)
}

func (hl *HCLLoader) Length() int {
	return len(hl.hclFilesPath)
}

func (hl *HCLLoader) HCLRender() (map[string]any, error) {
	if hl.Length() == 0 {
		return nil, fmt.Errorf("Nothing to render")
	}

	helmfileVariables, diags := hl.readHCLs()
	if len(diags) > 0 {
		return nil, diags.Errs()[0]
	}

	// Create a graph + topological sort in order to interpolate in the right order
	dagGraph := dag.New()

	for _, hv := range helmfileVariables {
		var traversals []string
		for _, tr := range hv.Expr.Variables() {
			attr, diags := hl.parseSingleAttrRef(tr)
			if diags != nil {
				return nil, fmt.Errorf("%s", diags.Errs()[0])
			}
			traversals = append(traversals, attr)
		}
		hl.logger.Debugf("Adding Dependency : %s  => [%s]", hv.Name, strings.Join(traversals, ", "))
		dagGraph.Add(hv.Name, dag.Dependencies(traversals))
	}

	//Generate Dag Plan which will provide the order from which to interpolate vars
	plan, err := dagGraph.Plan(dag.SortOptions{
		WithDependencies: true,
	})
	if err != nil {
		if ude, ok := err.(*dag.UndefinedDependencyError); ok {
			var quotedVariableNames []string
			for _, d := range ude.Dependents {
				quotedVariableNames = append(quotedVariableNames, fmt.Sprintf("%q", d))
			}
			return nil, fmt.Errorf("variables %s depend(s) on undefined vars %q", strings.Join(quotedVariableNames, ", "), ude.UndefinedNode)
		} else {
			return nil, fmt.Errorf("error while building the DAG variable graph : %s", err.Error())
		}
	}

	// Interpolate vars
	values := map[string]cty.Value{}
	helmfileVariablesValues := map[string]cty.Value{}

	for groupIndex := 0; groupIndex < len(plan); groupIndex++ {
		dagNodesInGroup := plan[groupIndex]

		for _, node := range dagNodesInGroup {
			ctx := &hcl.EvalContext{
				Variables: values,
			}
			for _, v := range helmfileVariables {
				if v.Name == node.String() {
					// Decode Value
					helmfileVariablesValues[node.String()], diags = v.Expr.Value(ctx)
					if len(diags) > 0 {
						return nil, fmt.Errorf("error when trying to evaluate variable %s : %s", v.Name, diags.Errs()[0])
					}
					break
				}
			}
			// Update the eval context for the next value evaluation iteration
			values[helmfileVarsAccessorPrefix] = cty.ObjectVal(helmfileVariablesValues)
		}
	}
	nativeGovals, err := hl.convertToGo(values)
	if err != nil {
		return nil, err
	}
	return nativeGovals, nil
}

// HelmfileVariable represents a single entry from a "helmfile_variables" block file.
// The "helmfile_variables" block itself is not represented, because it serves only to
// provide context for us to interpret its contents.
type HelmfileVariable struct {
	Name  string
	Expr  hcl.Expression
	Range hcl.Range
}

func (hl *HCLLoader) readHCLs() (map[string]*HelmfileVariable, hcl.Diagnostics) {
	var variables map[string]*HelmfileVariable
	var diags hcl.Diagnostics
	for _, file := range hl.hclFilesPath {
		variables, diags = hl.readHCL(variables, file)
		if diags != nil {
			return nil, diags
		}
	}
	return variables, nil
}

func (hl *HCLLoader) readHCL(hvars map[string]*HelmfileVariable, file string) (map[string]*HelmfileVariable, hcl.Diagnostics) {
	src, err := hl.fs.ReadFile(file)
	if err != nil {
		return nil, hcl.Diagnostics{
			{
				Severity: hcl.DiagError,
				Summary:  fmt.Sprintf("%s", err),
				Detail:   "could not read file",
				Subject:  &hcl.Range{},
			},
		}
	}

	// Parse file as HCL
	p := hclparse.NewParser()
	hclFile, diags := p.ParseHCL(src, file)
	if hclFile == nil || hclFile.Body == nil || diags != nil {
		return nil, diags
	}

	helmfileVariablesSchema := &hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{
			{
				Type: helmfileVarsBlockIdentifier,
			},
		},
	}
	// make sure content has a struct with helmfile_vars Schema defined
	content, diags := hclFile.Body.Content(helmfileVariablesSchema)
	if diags != nil {
		return nil, diags
	}
	for k, v := range hvars {
		hl.logger.Errorf("%s=%v", k, v.Expr)
	}
	// Decode blocks to return HelmfileVariable object => (each var with expr + Name )
	for _, block := range content.Blocks {
		helmfileBlockVars, diags := hl.decodeHelmfileVariablesBlock(block)
		if diags != nil {
			return nil, diags
		}
		// make sure vars are unique across blocks
		for k := range helmfileBlockVars {
			if hvars[k] != nil {
				var diags hcl.Diagnostics
				diags = append(diags, &hcl.Diagnostic{
					Severity: hcl.DiagError,
					Summary:  "Duplicate helmfile_vars definition",
					Detail: fmt.Sprintf("The helmfile_var %q was already defined at %s:%d",
						k, hvars[k].Range.Filename, hvars[k].Range.Start.Line),
					Subject: &helmfileBlockVars[k].Range,
				})
				return nil, diags
			}
		}
		err = mergo.Merge(&hvars, &helmfileBlockVars)
		if err != nil {
			var diags hcl.Diagnostics
			diags = append(diags, &hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "Merge failed",
				Detail:   err.Error(),
				Subject:  nil,
			})
			return nil, diags
		}
	}

	return hvars, nil
}

func (hl *HCLLoader) decodeHelmfileVariablesBlock(block *hcl.Block) (map[string]*HelmfileVariable, hcl.Diagnostics) {
	attrs, diags := block.Body.JustAttributes()
	if len(attrs) == 0 || diags != nil {
		return nil, diags
	}

	hfVars := map[string]*HelmfileVariable{}
	for name, attr := range attrs {
		if !hclsyntax.ValidIdentifier(name) {
			diags = append(diags, &hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "Invalid helmfile_vars variable name",
				Detail:   badIdentifierDetail,
				Subject:  &attr.NameRange,
			})
		}

		hfVars[name] = &HelmfileVariable{
			Name:  name,
			Expr:  attr.Expr,
			Range: attr.Range,
		}
	}
	return hfVars, diags
}

func (hl *HCLLoader) parseSingleAttrRef(traversal hcl.Traversal) (string, hcl.Diagnostics) {
	var diags hcl.Diagnostics

	root := traversal.RootName()
	rootRange := traversal[0].SourceRange()

	if len(traversal) < 2 {
		diags = diags.Append(&hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Invalid reference",
			Detail:   fmt.Sprintf("The %q object cannot be accessed directly. Instead, access it from one of its root.", root),
			Subject:  &rootRange,
		})
		return "", diags
	}

	if len(traversal) > 1 {
		if attrTrav, ok := traversal[1].(hcl.TraverseAttr); ok {
			return attrTrav.Name, diags
		}
	}
	diags = diags.Append(&hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  "Invalid reference",
		Detail:   fmt.Sprintf("The %q object does not support this operation.", root),
		Subject:  traversal[1].SourceRange().Ptr(),
	})
	return "", diags
}

func (hl *HCLLoader) convertToGo(src map[string]cty.Value) (map[string]any, error) {
	// Ugly workaround on value conversion
	// CTY conversion to go natives requires much processing and complexity
	// All of this, in our context, can go away because of the CTY capability to dump a cty.Value as Json
	// The Json document outputs 2 keys : "type" and "value" which describe the mapping between the two
	// We only care about the value
	b, err := json.Marshal(src[helmfileVarsAccessorPrefix], cty.DynamicPseudoType)
	if err != nil {
		return nil, fmt.Errorf("Could not marshal cty value : %s", err.Error())
	}

	var jsonunm map[string]any
	err = nativejson.Unmarshal(b, &jsonunm)
	if err != nil {
		return nil, fmt.Errorf("Could not unmarshall json : %s", err.Error())
	}

	if result, ok := jsonunm["value"].(map[string]any); ok {
		return result, nil
	} else {
		return nil, fmt.Errorf("Could extract a map object from json \"value\" key")
	}
}
