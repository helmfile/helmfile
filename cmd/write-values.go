package cmd

import (
	"github.com/spf13/cobra"

	"github.com/helmfile/helmfile/pkg/app"
	"github.com/helmfile/helmfile/pkg/config"
)

// NewWriteValuesCmd returns write-values subcmd
func NewWriteValuesCmd(globalCfg *config.GlobalImpl) *cobra.Command {
	writeValuesOptions := config.NewWriteValuesOptions()

	cmd := &cobra.Command{
		Use:   "write-values",
		Short: "Write values files for releases. Similar to `helmfile template`, write values files instead of manifests.",
		RunE: func(cmd *cobra.Command, args []string) error {
			writeValuesImpl := config.NewWriteValuesImpl(globalCfg, writeValuesOptions)
			err := config.NewCLIConfigImpl(writeValuesImpl.GlobalImpl)
			if err != nil {
				return err
			}

			if err := writeValuesImpl.ValidateConfig(); err != nil {
				return err
			}

			a := app.New(writeValuesImpl)
			return toCLIError(writeValuesImpl.GlobalImpl, a.WriteValues(writeValuesImpl))
		},
	}

	f := cmd.Flags()
	f.IntVar(&writeValuesOptions.Concurrency, "concurrency", 0, "maximum number of concurrent helm processes to run, 0 is unlimited")
	f.BoolVar(&writeValuesOptions.SkipDeps, "skip-deps", false, `skip running "helm repo update" and "helm dependency build"`)
	f.StringArrayVar(&writeValuesOptions.Set, "set", nil, "additional values to be merged into the helm command --set flag")
	f.StringArrayVar(&writeValuesOptions.Values, "values", nil, "additional value files to be merged into the helm command --values flag")
	f.StringVar(&writeValuesOptions.OutputFileTemplate, "output-file-template", "", "go text template for generating the output file. Default: {{ .State.BaseName }}-{{ .State.AbsPathSHA1 }}/{{ .Release.Name}}.yaml")

	return cmd
}
