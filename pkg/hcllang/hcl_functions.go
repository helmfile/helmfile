package hcllang

import (
	"github.com/hashicorp/hcl/v2/ext/tryfunc"
	"github.com/imdario/mergo"
	ctyyaml "github.com/zclconf/go-cty-yaml"
	"github.com/zclconf/go-cty/cty/function"
	"github.com/zclconf/go-cty/cty/function/stdlib"
)

func HCLFunctions(additionnalFunctions map[string]function.Function) (map[string]function.Function, error) {
	var hclFunctions = map[string]function.Function{
		"abs":             stdlib.AbsoluteFunc,
		"can":             tryfunc.CanFunc,
		"ceil":            stdlib.CeilFunc,
		"chomp":           stdlib.ChompFunc,
		"coalesce":        stdlib.CoalesceFunc,
		"coalescelist":    stdlib.CoalesceListFunc,
		"compact":         stdlib.CompactFunc,
		"concat":          stdlib.ConcatFunc,
		"contains":        stdlib.ContainsFunc,
		"csvdecode":       stdlib.CSVDecodeFunc,
		"distinct":        stdlib.DistinctFunc,
		"element":         stdlib.ElementFunc,
		"chunklist":       stdlib.ChunklistFunc,
		"flatten":         stdlib.FlattenFunc,
		"floor":           stdlib.FloorFunc,
		"format":          stdlib.FormatFunc,
		"formatdate":      stdlib.FormatDateFunc,
		"formatlist":      stdlib.FormatListFunc,
		"indent":          stdlib.IndentFunc,
		"index":           stdlib.IndentFunc,
		"join":            stdlib.JoinFunc,
		"jsondecode":      stdlib.JSONDecodeFunc,
		"jsonencode":      stdlib.JSONEncodeFunc,
		"keys":            stdlib.KeysFunc,
		"length":          stdlib.LengthFunc,
		"log":             stdlib.LogFunc,
		"lookup":          stdlib.LookupFunc,
		"lower":           stdlib.LowerFunc,
		"max":             stdlib.MaxFunc,
		"merge":           stdlib.MergeFunc,
		"min":             stdlib.MinFunc,
		"parseint":        stdlib.ParseIntFunc,
		"pow":             stdlib.PowFunc,
		"range":           stdlib.RangeFunc,
		"regex":           stdlib.RegexFunc,
		"regexall":        stdlib.RegexAllFunc,
		"reverse":         stdlib.ReverseListFunc,
		"setintersection": stdlib.SetIntersectionFunc,
		"setproduct":      stdlib.SetProductFunc,
		"setsubtract":     stdlib.SetSubtractFunc,
		"setunion":        stdlib.SetUnionFunc,
		"signum":          stdlib.SignumFunc,
		"slice":           stdlib.SliceFunc,
		"sort":            stdlib.SortFunc,
		"split":           stdlib.SplitFunc,
		"strrev":          stdlib.ReverseFunc,
		"substr":          stdlib.SubstrFunc,
		"timeadd":         stdlib.TimeAddFunc,
		"title":           stdlib.TitleFunc,
		"trim":            stdlib.TrimFunc,
		"trimprefix":      stdlib.TrimPrefixFunc,
		"trimspace":       stdlib.TrimSpaceFunc,
		"trimsuffix":      stdlib.TrimSuffixFunc,
		"try":             tryfunc.TryFunc,
		"upper":           stdlib.UpperFunc,
		"values":          stdlib.ValuesFunc,
		"yamldecode":      ctyyaml.YAMLDecodeFunc,
		"yamlencode":      ctyyaml.YAMLEncodeFunc,
		"zipmap":          stdlib.ZipmapFunc,
	}

	if additionnalFunctions != nil {
		err := mergo.MapWithOverwrite(hclFunctions, additionnalFunctions)
		if err != nil {
			return nil, err
		}
	}
	return hclFunctions, nil
}
