package cmd

import (
	"github.com/helmfile/helmfile/pkg/app"
	"github.com/helmfile/helmfile/pkg/config"
	"github.com/spf13/cobra"
)

// NewTestCmd returm build subcmd
func NewTestCmd(globalCfg *config.GlobalImpl) *cobra.Command {
	testOptions := config.NewTestOptions()
	testImpl := config.NewTestImpl(globalCfg, testOptions)

	cmd := &cobra.Command{
		Use:   "test",
		Short: "Test charts from state file (helm test)",
		RunE: func(cmd *cobra.Command, args []string) error {
			err := config.NewUrfaveCliConfigImplIns(testImpl.GlobalImpl)
			if err != nil {
				return err
			}

			if err := testImpl.ValidateConfig(); err != nil {
				return err
			}

			a := app.New(testImpl)
			return toCLIError(testImpl.GlobalImpl, a.Test(testImpl))
		},
	}
	testImpl.Cmd = cmd

	f := cmd.Flags()
	f.IntVar(&testOptions.Concurrency, "concurrency", 0, "maximum number of concurrent downloads of release charts")
	f.BoolVar(&testOptions.SkipDeps, "skip-deps", testOptions.SkipDeps, `skip running "helm repo update" and "helm dependency build"`)
	f.BoolVar(&testOptions.Cleanup, "cleanup", testOptions.Cleanup, "delete test pods upon completion")
	f.BoolVar(&testOptions.Logs, "logs", testOptions.Logs, "Dump the logs from test pods (this runs after all tests are complete, but before any cleanup)")
	f.StringVar(&testOptions.Args, "args", testOptions.Args, "pass args to helm exec")
	f.IntVar(&testOptions.Timeout, "timeout", 300, "maximum time for tests to run before being considered failed")

	return cmd
}
