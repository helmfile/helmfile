package cmd

import (
	"github.com/helmfile/helmfile/pkg/app"
	"github.com/helmfile/helmfile/pkg/config"
	"github.com/spf13/cobra"
)

// NewLintCmd returm build subcmd
func NewLintCmd(globalCfg *config.GlobalImpl) *cobra.Command {
	lintOptions := config.NewLintOptions()
	lintImpl := config.NewLintImpl(globalCfg, lintOptions)

	cmd := &cobra.Command{
		Use:   "lint",
		Short: "Lint charts from state file (helm lint)",
		RunE: func(cmd *cobra.Command, args []string) error {
			err := config.NewUrfaveCliConfigImplIns(lintImpl.GlobalImpl)
			if err != nil {
				return err
			}

			if err := lintImpl.ValidateConfig(); err != nil {
				return err
			}

			a := app.New(lintImpl)
			return toCLIError(lintImpl.GlobalImpl, a.Lint(lintImpl))
		},
	}

	f := cmd.Flags()
	f.IntVar(&lintOptions.Concurrency, "concurrency", 0, "maximum number of concurrent downloads of release charts")
	f.BoolVar(&lintOptions.SkipDeps, "skip-deps", lintOptions.SkipDeps, `skip running "helm repo update" and "helm dependency build"`)
	f.StringVar(&lintOptions.Args, "args", lintOptions.Args, "pass args to helm exec")
	f.StringArrayVar(&lintOptions.Set, "set", lintOptions.Set, "additional values to be merged into the command")
	f.StringArrayVar(&lintOptions.Values, "values", lintOptions.Values, "additional value files to be merged into the command")

	return cmd
}
