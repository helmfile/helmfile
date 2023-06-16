package config

import (
	"strings"

	"github.com/helmfile/helmfile/pkg/maputil"
)

func NewCLIConfigImpl(g *GlobalImpl) error {
	optsSet := g.RawStateValuesSet()
	if len(optsSet) > 0 {
		set := map[string]any{}
		for i := range optsSet {
			ops := strings.Split(optsSet[i], ",")
			for j := range ops {
				op := strings.SplitN(ops[j], "=", 2)
				k := maputil.ParseKey(op[0])
				v := op[1]

				maputil.Set(set, k, v)
			}
		}
		g.SetSet(set)
	}

	return nil
}
