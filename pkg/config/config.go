package config

import (
	"strings"

	"github.com/urfave/cli"
	"go.uber.org/zap"
	"golang.org/x/term"

	"github.com/helmfile/helmfile/pkg/maputil"
	"github.com/helmfile/helmfile/pkg/state"
)

func NewCLIConfigImpl(g *GlobalImpl) error {
	optsSet := g.RawStateValuesSet()
	if len(optsSet) > 0 {
		set := map[string]interface{}{}
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
