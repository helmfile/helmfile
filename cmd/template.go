package cmd

import (
	"github.com/spf13/cobra"

	"github.com/helmfile/helmfile/pkg/app"
	"github.com/helmfile/helmfile/pkg/config"
)

// NewTemplateCmd returm template subcmd
func NewTemplateCmd(globalCfg *config.GlobalImpl) *cobra.Command {
	templateOptions := config.NewTemplateOptions()

	cmd := &cobra.Command{
		Use:   "template",
		Short: "Template releases defined in state file",
		RunE: func(cmd *cobra.Command, args []string) error {
			templateImpl := config.NewTemplateImpl(globalCfg, templateOptions)
			err := config.NewCLIConfigImpl(templateImpl.GlobalImpl)
			if err != nil {
				return err
			}

			if err := templateImpl.ValidateConfig(); err != nil {
				return err
			}

			a := app.New(templateImpl)
			return toCLIError(templateImpl.GlobalImpl, a.Template(templateImpl))
		},
	}

	f := cmd.Flags()
	f.StringVar(&globalCfg.GlobalOptions.Args, "args", "", "pass args to helm template")
	f.StringArrayVar(&templateOptions.Set, "set", nil, "additional values to be merged into the helm command --set flag")
	f.StringArrayVar(&templateOptions.Values, "values", nil, "additional value files to be merged into the helm command --values flag")
	f.StringVar(&templateOptions.OutputDir, "output-dir", "", "output directory to pass to helm template (helm template --output-dir)")
	f.StringVar(&templateOptions.OutputDirTemplate, "output-dir-template", "", "go text template for generating the output directory. Default: {{ .OutputDir }}/{{ .State.BaseName }}-{{ .State.AbsPathSHA1 }}-{{ .Release.Name}}")
	f.IntVar(&templateOptions.Concurrency, "concurrency", 0, "maximum number of concurrent helm processes to run, 0 is unlimited")
	f.BoolVar(&templateOptions.Validate, "validate", false, "validate your manifests against the Kubernetes cluster you are currently pointing at. Note that this requires access to a Kubernetes cluster to obtain information necessary for validating, like the template of available API versions")
	f.BoolVar(&templateOptions.IncludeCRDs, "include-crds", false, "include CRDs in the templated output")
	f.BoolVar(&templateOptions.SkipTests, "skip-tests", false, "skip tests from templated output")
	f.BoolVar(&templateOptions.SkipNeeds, "skip-needs", true, `do not automatically include releases from the target release's "needs" when --selector/-l flag is provided. Does nothing when --selector/-l flag is not provided. Defaults to true when --include-needs or --include-transitive-needs is not provided`)
	f.BoolVar(&templateOptions.IncludeNeeds, "include-needs", false, `automatically include releases from the target release's "needs" when --selector/-l flag is provided. Does nothing when --selector/-l flag is not provided`)
	f.BoolVar(&templateOptions.IncludeTransitiveNeeds, "include-transitive-needs", false, `like --include-needs, but also includes transitive needs (needs of needs). Does nothing when --selector/-l flag is not provided. Overrides exclusions of other selectors and conditions.`)
	f.BoolVar(&templateOptions.SkipCleanup, "skip-cleanup", false, "Stop cleaning up temporary values generated by helmfile and helm-secrets. Useful for debugging. Don't use in production for security")
	f.StringVar(&templateOptions.PostRenderer, "post-renderer", "", `pass --post-renderer to "helm template" or "helm upgrade --install"`)
	f.StringArrayVar(&templateOptions.PostRendererArgs, "post-renderer-args", nil, `pass --post-renderer-args to "helm template" or "helm upgrade --install"`)
	f.StringVar(&templateOptions.SkipSchemaValidation, "skip-schema-validation", "", `pass skip-schema-validation to "helm template" or "helm upgrade --install"`)
	f.StringVar(&templateOptions.KubeVersion, "kube-version", "", `pass --kube-version to "helm template". Overrides kubeVersion in helmfile.yaml`)
	f.StringArrayVar(&templateOptions.ShowOnly, "show-only", nil, `pass --show-only to "helm template"`)

	return cmd
}
