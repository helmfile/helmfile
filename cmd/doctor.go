package cmd

import (
	"github.com/spf13/cobra"

	"github.com/helmfile/helmfile/pkg/app"
	"github.com/helmfile/helmfile/pkg/config"
)

// NewDoctorCmd returns the doctor subcmd.
//
// `helmfile doctor` runs `helmfile diff` and, when an OpenAI-compatible LLM
// endpoint is configured, asks the model to summarize the diff and flag risks.
// When no LLM is configured the command falls back to `helmfile diff` with
// --show-secrets forced off. Note: --output is reserved for the doctor report
// format; helm-diff's output format is exposed as --diff-output.
//
// Configuration precedence: env (HELMFILE_LLM_*) < helmfile.yaml (llm:) < flags (--llm-*).
//
// SECURITY: secrets are ALWAYS redacted. --show-secrets is silently ignored
// (helm-diff forced to emit <REDACTED>) and a defense-in-depth text redactor
// strips residual secret-looking content before LLM transmission. The redaction
// count is reported in the output footer.
func NewDoctorCmd(globalCfg *config.GlobalImpl) *cobra.Command {
	doctorOptions := config.NewDoctorOptions()
	// Construct the DoctorImpl ONCE up front so cobra flag bindings and the
	// RunE callback share the SAME DiffOptions pointer. See
	// TestDoctorCmd_DiffOptionsFlagBindingIsLive for the regression this guard.
	doctorImpl := config.NewDoctorImpl(globalCfg, doctorOptions)
	diffOpts := doctorImpl.DiffImpl.DiffOptions

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "AI-assisted diff analysis: summarize changes and flag risks",
		Long: `Runs ` + "`helmfile diff`" + ` and asks an LLM to summarize the changes and flag risks.

With no LLM configured (HELMFILE_LLM_API_KEY / HELMFILE_LLM_MODEL / helmfile.yaml llm: block /
--llm-base-url / --llm-api-key), falls back to ` + "`helmfile diff`" + ` with --show-secrets forced off.
Most diff flags are accepted; --output is reserved for the doctor report format
(use --diff-output for helm-diff's plugin output format).

SECURITY: secrets are ALWAYS redacted before any byte leaves the process.
` + "`--show-secrets`" + ` is silently ignored by doctor (helm-diff is forced to emit
<REDACTED> placeholders) and a defense-in-depth text redactor strips any residual
secret-looking content from the diff before it is sent to the LLM. The redaction
count is reported in the output footer so you can spot unexpected leaks.

Exit codes:
  0  success, or only low/medium risks, or LLM call failed (degraded)
  2  at least one high-severity risk and --force not passed
  1  other error (state load failure, helm-diff runtime failure, etc.)

The "detected changes" exit-2 from helm-diff --detailed-exitcode is intentionally
swallowed: doctor's whole job is to react to changes.
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Warn loudly if the user pointed helmfile's logger at stdout:
			// captureStdout swaps os.Stdout, so log lines would be captured
			// with the diff and shipped to the LLM.
			if app.IsLogOutputStdout(doctorImpl.DiffImpl.GlobalImpl.GlobalOptions.LogOutput) {
				doctorImpl.DiffImpl.GlobalImpl.GlobalOptions.Logger().Warnf(
					"doctor: --log-output is set to stdout; log lines will be " +
						"captured with the diff and sent to the LLM. Prefer the " +
						"default (stderr) when running doctor.",
				)
			}

			err := config.NewCLIConfigImpl(doctorImpl.DiffImpl.GlobalImpl)
			if err != nil {
				return err
			}

			if err := doctorImpl.ValidateConfig(); err != nil {
				return err
			}

			a := app.New(doctorImpl)
			return toCLIError(doctorImpl.DiffImpl.GlobalImpl, a.Doctor(doctorImpl))
		},
	}

	f := cmd.Flags()

	// === LLM-specific flags ===
	f.StringVar(&doctorOptions.LLMBaseURL, "llm-base-url", "",
		`OpenAI-compatible Chat Completions endpoint base URL, e.g. "https://api.openai.com/v1" or "https://one-api.internal/v1". Overrides HELMFILE_LLM_BASE_URL and helmfile.yaml llm.baseURL.`)
	f.StringVar(&doctorOptions.LLMAPIKey, "llm-api-key", "",
		`API key for the LLM endpoint. Overrides HELMFILE_LLM_API_KEY and helmfile.yaml llm.apiKey. Prefer helmfile.yaml with {{ env "..." }} over passing this on the CLI.`)
	f.StringVar(&doctorOptions.LLMModel, "llm-model", "",
		`Chat completion model identifier, e.g. "gpt-4o" or "claude-3-5-sonnet" (via gateway). Overrides HELMFILE_LLM_MODEL and helmfile.yaml llm.model.`)
	f.DurationVar(&doctorOptions.LLMTimeout, "llm-timeout", 0,
		"Per-request timeout for the LLM call. Defaults to 60s. Example: --llm-timeout 120s")
	f.IntVar(&doctorOptions.LLMMaxTokens, "llm-max-tokens", 0,
		"Maximum tokens for the LLM completion. Defaults to 4096.")

	// === Doctor-specific flags ===
	f.BoolVar(&doctorOptions.Force, "force", false,
		"Skip the high-risk exit-code-2 gate. Use this when CI wants the report but should not block.")
	// --output here is the DOCTOR REPORT format (text/json). It intentionally
	// shadows helm-diff's --output (renamed --diff-output below) because in
	// the doctor context users expect --output to mean the report. The JSON
	// "diff" field is always post-redaction — doctor never exposes raw
	// pre-redaction diff through stdout/JSON.
	f.StringVar(&doctorOptions.ReportFormat, "output", "",
		`Doctor report format: "text" (markdown, default) or "json" (structured). The JSON "diff" field is always post-redaction.`)

	// === Common diff surface (shared with `helmfile diff`) ===
	bindCommonDiffFlags(f, diffOpts, &globalCfg.GlobalOptions.Args)

	// === Diff flags whose default or help differs in doctor ===
	f.BoolVar(&diffOpts.ShowSecrets, "show-secrets", false,
		"Ignored by doctor. Secrets are always redacted (see --suppress-secrets to drop Secret resources entirely). Accepted only for parity with `helmfile diff` so existing flags files do not break.")
	f.BoolVar(&diffOpts.DetailedExitcode, "detailed-exitcode", false,
		"return a detailed exit code (note: doctor swallows the 'detected changes' code-2 since it is the whole point of running it)")
	f.IntVar(&diffOpts.Context, "context", 3,
		"output NUM lines of context around changes. doctor defaults to 3 so the LLM sees enough surrounding YAML to ground its analysis")
	f.StringVar(&diffOpts.Output, "diff-output", "",
		"output format for diff plugin (helm-diff --output). Renamed from --output to avoid colliding with doctor's --output flag.")

	return cmd
}
