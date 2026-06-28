package cmd

import (
	"github.com/spf13/pflag"

	"github.com/helmfile/helmfile/pkg/config"
)

// bindCommonDiffFlags registers the diff flags shared verbatim by
// `helmfile diff` and `helmfile doctor`. Keeping these in one place avoids
// hand-syncing two parallel flag blocks when a new diff flag is added.
//
// Flags NOT registered here (because their help text or default differs
// between diff and doctor) are registered inline in each command:
//
//   - --show-secrets      doctor overrides help: "ignored, secrets always redacted"
//   - --detailed-exitcode doctor overrides help: notes the exit-2 swallow
//   - --context           doctor defaults to 3 (more context for LLM)
//   - --output            diff uses "output"; doctor renames to "diff-output"
//     (doctor's --output selects the report format)
func bindCommonDiffFlags(f *pflag.FlagSet, opts *config.DiffOptions, globalArgs *string) {
	f.StringVar(&opts.DiffArgs, "diff-args", "", `Pass args to helm-diff`)
	f.StringVar(&opts.TemplateArgs, "template-args", "", `Pass extra args to the helm template/diff rendering (e.g. --template-args="--dry-run=server" to enable the helm lookup function). Overrides helmDefaults.templateArgs.`)
	f.StringVar(globalArgs, "args", "", "pass args to helm diff")
	f.StringArrayVar(&opts.Set, "set", nil, "additional values to be merged into the helm command --set flag")
	f.StringArrayVar(&opts.Values, "values", nil, "additional value files to be merged into the helm command --values flag")
	f.IntVar(&opts.Concurrency, "concurrency", 0, "maximum number of concurrent helm processes to run, 0 is unlimited")
	f.BoolVar(&opts.Validate, "validate", false, "validate your manifests against the Kubernetes cluster you are currently pointing at. Note that this requires access to a Kubernetes cluster to obtain information necessary for validating, like the diff of available API versions")
	f.BoolVar(&opts.SkipNeeds, "skip-needs", true, `do not automatically include releases from the target release's "needs" when --selector/-l flag is provided. Does nothing when --selector/-l flag is not provided. Defaults to true when --include-needs or --include-transitive-needs is not provided`)
	f.BoolVar(&opts.IncludeTests, "include-tests", false, "enable the diffing of the helm test hooks")
	f.BoolVar(&opts.IncludeNeeds, "include-needs", false, `automatically include releases from the target release's "needs" when --selector/-l flag is provided. Does nothing when --selector/-l flag is not provided`)
	f.BoolVar(&opts.EnforceNeedsAreInstalled, "enforce-needs-are-installed", false, "enforce that all 'needs' dependencies are installable before applying changes")
	f.BoolVar(&opts.IncludeTransitiveNeeds, "include-transitive-needs", false, `like --include-needs, but also includes transitive needs (needs of needs). Does nothing when --selector/-l flag is not provided. Overrides exclusions of other selectors and conditions.`)
	f.BoolVar(&opts.SkipDiffOnInstall, "skip-diff-on-install", false, "Skips running helm-diff on releases being newly installed on this apply. Useful when the release manifests are too huge to be reviewed, or it's too time-consuming to diff at all")
	f.BoolVar(&opts.NoHooks, "no-hooks", false, "do not diff changes made by hooks.")
	f.BoolVar(&opts.StripTrailingCR, "strip-trailing-cr", false, "strip trailing carriage return on input")
	f.BoolVar(&opts.SuppressSecrets, "suppress-secrets", false, "suppress secrets in the output. highly recommended to specify on CI/CD use-cases")
	f.StringArrayVar(&opts.Suppress, "suppress", nil, "suppress specified Kubernetes objects in the output. Can be provided multiple times. For example: --suppress KeycloakClient --suppress VaultSecret")
	f.BoolVar(&opts.ReuseValues, "reuse-values", false, `Override helmDefaults.reuseValues "helm diff upgrade --install --reuse-values"`)
	f.BoolVar(&opts.ResetValues, "reset-values", false, `Override helmDefaults.reuseValues "helm diff upgrade --install --reset-values"`)
	f.BoolVar(&opts.TakeOwnership, "take-ownership", false, "add --take-ownership flag to helm")
	f.StringVar(&opts.ServerSide, "server-side", "", `add --server-side flag to helm diff upgrade (Helm 4 only). Must be "true", "false", or "auto"`)
	f.StringVar(&opts.PostRenderer, "post-renderer", "", `pass --post-renderer to "helm template" or "helm upgrade --install"`)
	f.StringArrayVar(&opts.PostRendererArgs, "post-renderer-args", nil, `pass --post-renderer-args to "helm template" or "helm upgrade --install"`)
	f.StringArrayVar(&opts.SuppressOutputLineRegex, "suppress-output-line-regex", nil, "a list of regex patterns to suppress output lines from the diff output")
}
