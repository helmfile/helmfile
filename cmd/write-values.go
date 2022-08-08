package cmd

import (
	"github.com/helmfile/helmfile/pkg/app"
	"github.com/helmfile/helmfile/pkg/config"
	"github.com/spf13/cobra"
)

// NewWriteValuesCmd returns write subcmd
func NewWriteValuesCmd(globalCfg *config.GlobalImpl) *cobra.Command {
	writeValuesOptions := config.NewWriteValuesOptions()
	writeValuesImpl := config.NewWriteValuesImpl(globalCfg, writeValuesOptions)

	cmd := &cobra.Command{
		Use:   "write-values",
		Short: "Write values files for releases. Similar to `helmfile template`, write values files instead of manifests.",
		RunE: func(cmd *cobra.Command, args []string) error {
			err := config.NewUrfaveCliConfigImplIns(writeValuesImpl.GlobalImpl)
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
	f.IntVar(&writeValuesOptions.Concurrency, "concurrency", 0, "maximum number of concurrent downloads of release charts")
	f.BoolVar(&writeValuesOptions.SkipDeps, "skip-deps", writeValuesOptions.SkipDeps, `skip running "helm repo update" and "helm dependency build"`)
	f.StringArrayVar(&writeValuesOptions.Set, "set", writeValuesOptions.Set, "additional values to be merged into the command")
	f.StringArrayVar(&writeValuesOptions.Values, "values", writeValuesOptions.Values, "additional value files to be merged into the command")
	f.StringVar(&writeValuesOptions.OutputFileTemplate, "output-file-template", writeValuesOptions.OutputFileTemplate, "go text template for generating the output file. Default: {{ .State.BaseName }}-{{ .State.AbsPathSHA1 }}/{{ .Release.Name}}.yaml")

	return cmd
}
