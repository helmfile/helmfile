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
	badIdentifierDetail   = "A name must start with a letter or underscore and may contain only letters, digits, underscores, and dashes."
	valuesBlockIdentifier = "values"
	valuesAccessorPrefix  = "hv"
	localsBlockIdentifier = "locals"
	localsAccessorPrefix  = "local"
)

// HelmfileHCLValue represents a single entry from a "values" or "locals" block file.
// The blocks itself is not represented, because it serves only to
// provide context for us to interpret its contents.
type HelmfileHCLValue struct {
	Name  string
	Expr  hcl.Expression
	Range hcl.Range
}

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
		return nil, fmt.Errorf("nothing to render")
	}

	HelmfileHCLValues, locals, diags := hl.readHCLs()
	if len(diags) > 0 {
		return nil, diags.Errs()[0]
	}

	// Decode all locals from all files first
	// in order for them to be usable in values blocks
	localsCty := map[string]map[string]cty.Value{}
	for k, local := range locals {
		dagPlan, err := hl.createDAGGraph(local, localsBlockIdentifier)
		if err != nil {
			return nil, err
		}
		localFileCty, err := hl.decodeGraph(dagPlan, localsBlockIdentifier, locals[k], nil)
		if err != nil {
			return nil, err
		}
		localsCty[k] = make(map[string]cty.Value)
		localsCty[k][localsAccessorPrefix] = localFileCty[localsAccessorPrefix]
	}

	// Decode Values
	dagHelmfileValuePlan, err := hl.createDAGGraph(HelmfileHCLValues, valuesBlockIdentifier)
	if err != nil {
		return nil, err
	}
	helmfileVarCty, err := hl.decodeGraph(dagHelmfileValuePlan, valuesBlockIdentifier, HelmfileHCLValues, localsCty)
	if err != nil {
		return nil, err
	}
	nativeGovals, err := hl.convertToGo(helmfileVarCty)
	if err != nil {
		return nil, err
	}
	return nativeGovals, nil
}

func (hl *HCLLoader) createDAGGraph(HelmfileHCLValues map[string]*HelmfileHCLValue, blockType string) (*dag.Topology, error) {
	dagGraph := dag.New()

	for _, hv := range HelmfileHCLValues {
		var traversals []string
		for _, tr := range hv.Expr.Variables() {
			attr, diags := hl.parseSingleAttrRef(tr, blockType)
			if diags != nil {
				return nil, fmt.Errorf("%s", diags.Errs()[0])
			}
			if attr != "" {
				traversals = append(traversals, attr)
			}
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
	return &plan, nil
}

func (hl *HCLLoader) decodeGraph(dagTopology *dag.Topology, blocktype string, vars map[string]*HelmfileHCLValue, additionalLocalContext map[string]map[string]cty.Value) (map[string]cty.Value, error) {
	values := map[string]cty.Value{}
	HelmfileHCLValuesValues := map[string]cty.Value{}
	var diags hcl.Diagnostics
	for groupIndex := 0; groupIndex < len(*dagTopology); groupIndex++ {
		dagNodesInGroup := (*dagTopology)[groupIndex]

		for _, node := range dagNodesInGroup {
			v := vars[node.String()]
			if blocktype != localsBlockIdentifier && additionalLocalContext[v.Range.Filename] != nil {
				values[localsAccessorPrefix] = additionalLocalContext[v.Range.Filename][localsAccessorPrefix]
			}
			ctx := &hcl.EvalContext{
				Variables: values,
			}
			// Decode Value
			HelmfileHCLValuesValues[node.String()], diags = v.Expr.Value(ctx)
			if len(diags) > 0 {
				return nil, fmt.Errorf("error when trying to evaluate variable %s : %s", v.Name, diags.Errs()[0])
			}
			switch blocktype {
			case valuesBlockIdentifier:
				// Update the eval context for the next value evaluation iteration
				values[valuesAccessorPrefix] = cty.ObjectVal(HelmfileHCLValuesValues)
				// Set back local to nil to avoid an unexpected behavior when the next iteration is in another file
				values[localsAccessorPrefix] = cty.NilVal
			case localsBlockIdentifier:
				values[localsAccessorPrefix] = cty.ObjectVal(HelmfileHCLValuesValues)
			}
		}
	}
	return values, nil
}

func (hl *HCLLoader) readHCLs() (map[string]*HelmfileHCLValue, map[string]map[string]*HelmfileHCLValue, hcl.Diagnostics) {
	var variables map[string]*HelmfileHCLValue
	var local map[string]*HelmfileHCLValue
	locals := map[string]map[string]*HelmfileHCLValue{}
	var diags hcl.Diagnostics
	for _, file := range hl.hclFilesPath {
		variables, local, diags = hl.readHCL(variables, file)
		if diags != nil {
			return nil, nil, diags
		}
		locals[file] = make(map[string]*HelmfileHCLValue)
		locals[file] = local
	}
	return variables, locals, nil
}

func (hl *HCLLoader) readHCL(hvars map[string]*HelmfileHCLValue, file string) (map[string]*HelmfileHCLValue, map[string]*HelmfileHCLValue, hcl.Diagnostics) {
	src, err := hl.fs.ReadFile(file)
	if err != nil {
		return nil, nil, hcl.Diagnostics{
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
		return nil, nil, diags
	}

	HelmfileHCLValuesSchema := &hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{
			{
				Type: valuesBlockIdentifier,
			},
			{
				Type: localsBlockIdentifier,
			},
		},
	}
	// make sure content has a struct with helmfile_vars Schema defined
	content, diags := hclFile.Body.Content(HelmfileHCLValuesSchema)
	if diags != nil {
		return nil, nil, diags
	}

	var helmfileLocalsVars map[string]*HelmfileHCLValue
	// Decode blocks to return HelmfileHCLValue object => (each var with expr + Name )

	if len(content.Blocks.OfType(localsBlockIdentifier)) > 1 {
		return nil, nil, hcl.Diagnostics{
			&hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "A file can only support exactly 1 `locals` block",
				Subject:  &content.Blocks[0].DefRange,
			}}
	}
	for _, block := range content.Blocks {
		var helmfileBlockVars map[string]*HelmfileHCLValue
		if block.Type == valuesBlockIdentifier {
			helmfileBlockVars, diags = hl.decodeHelmfileHCLValuesBlock(block)
			if diags != nil {
				return nil, nil, diags
			}
		}

		if block.Type == localsBlockIdentifier {
			helmfileLocalsVars, diags = hl.decodeHelmfileHCLValuesBlock(block)
			if diags != nil {
				return nil, nil, diags
			}
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
				return nil, nil, diags
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
			return nil, nil, diags
		}
	}

	return hvars, helmfileLocalsVars, nil
}

func (hl *HCLLoader) decodeHelmfileHCLValuesBlock(block *hcl.Block) (map[string]*HelmfileHCLValue, hcl.Diagnostics) {
	attrs, diags := block.Body.JustAttributes()
	if len(attrs) == 0 || diags != nil {
		return nil, diags
	}

	hfVars := map[string]*HelmfileHCLValue{}
	for name, attr := range attrs {
		if !hclsyntax.ValidIdentifier(name) {
			diags = append(diags, &hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "Invalid helmfile_vars variable name",
				Detail:   badIdentifierDetail,
				Subject:  &attr.NameRange,
			})
		}

		hfVars[name] = &HelmfileHCLValue{
			Name:  name,
			Expr:  attr.Expr,
			Range: attr.Range,
		}
	}
	return hfVars, diags
}

func (hl *HCLLoader) parseSingleAttrRef(traversal hcl.Traversal, blockType string) (string, hcl.Diagnostics) {
	var diags hcl.Diagnostics

	root := traversal.RootName()
	// In `values` blocks, Locals are always precomputed, so they don't need to be in the graph
	if root == localsAccessorPrefix && blockType != localsBlockIdentifier {
		return "", nil
	}
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
	b, err := json.Marshal(src[valuesAccessorPrefix], cty.DynamicPseudoType)
	if err != nil {
		return nil, fmt.Errorf("could not marshal cty value : %s", err.Error())
	}

	var jsonunm map[string]any
	err = nativejson.Unmarshal(b, &jsonunm)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshall json : %s", err.Error())
	}

	if result, ok := jsonunm["value"].(map[string]any); ok {
		return result, nil
	} else {
		return nil, fmt.Errorf("could extract a map object from json \"value\" key")
	}
}
