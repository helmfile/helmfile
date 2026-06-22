package config

import (
	"time"

	"github.com/helmfile/helmfile/pkg/agent/llm"
)

// DoctorOptions is the doctor-specific options *only* (no diff flags here).
// Doctor is a strict superset of diff, so the diff flags are sourced via the
// embedded DiffImpl on DoctorImpl. Keeping the option types separate avoids
// Go's one-level-only field/method promotion tripping us up when DiffOptions
// and DoctorOptions both grow fields with the same name (e.g. Output).
type DoctorOptions struct {
	// LLMBaseURL overrides the OpenAI-compatible endpoint base URL.
	LLMBaseURL string
	// LLMAPIKey authenticates against the endpoint.
	LLMAPIKey string
	// LLMModel is the chat completion model identifier.
	LLMModel string
	// LLMTimeout is the per-request timeout. Parsed from a duration string
	// (e.g. "60s", "2m"). Zero means "use the llm package default".
	LLMTimeout time.Duration
	// LLMMaxTokens caps the completion length. Zero means default.
	LLMMaxTokens int

	// Force skips the high-risk exit-code-2 gate. Useful when CI wants the
	// report but does not want to block.
	Force bool

	// ReportFormat selects the doctor report format. "text" (markdown) by
	// default, "json" for structured CI consumption.
	//
	// Named ReportFormat (not Output) to avoid shadowing DiffOptions.Output
	// which is the helm-diff plugin format.
	ReportFormat string
}

// NewDoctorOptions creates a new DoctorOptions.
func NewDoctorOptions() *DoctorOptions {
	return &DoctorOptions{}
}

// DoctorImpl is the config provider implementation for the doctor command.
// It embeds DiffImpl so any code that consumes a DiffConfigProvider accepts a
// DoctorImpl unchanged (doctor IS-A diff for the purposes of helm-diff flags).
type DoctorImpl struct {
	// DiffImpl satisfies DiffConfigProvider and ConfigProvider by providing
	// GlobalImpl + DiffOptions + all their methods.
	*DiffImpl
	// DoctorOptions carries the doctor-only knobs (LLM endpoint, report format).
	*DoctorOptions
}

// NewDoctorImpl creates a new DoctorImpl. It wires a fresh DiffImpl around the
// same GlobalImpl so that --global flags behave identically to `helmfile diff`.
//
// doctorOpts may be nil; in that case a fresh DoctorOptions is allocated.
func NewDoctorImpl(g *GlobalImpl, doctorOpts *DoctorOptions) *DoctorImpl {
	if doctorOpts == nil {
		doctorOpts = NewDoctorOptions()
	}
	return &DoctorImpl{
		DiffImpl:      NewDiffImpl(g, NewDiffOptions()),
		DoctorOptions: doctorOpts,
	}
}

// FlagLLMConfig returns the LLM configuration sourced from CLI flags only.
// Empty fields mean "flag not set"; the doctor command merges this on top of
// env+yaml via ResolveConfig.
func (t *DoctorImpl) FlagLLMConfig() llm.Config {
	return llm.Config{
		BaseURL:     t.LLMBaseURL,
		APIKey:      t.LLMAPIKey,
		Model:       t.LLMModel,
		Timeout:     t.LLMTimeout,
		MaxTokens:   t.LLMMaxTokens,
		Temperature: 0, // let doctor/llm default apply
	}
}

// Force returns whether --force was passed.
func (t *DoctorImpl) Force() bool {
	return t.DoctorOptions.Force
}

// DoctorOutput returns the report format ("text" or "json").
// Named DoctorOutput to satisfy the DoctorConfigProvider interface; backed by
// ReportFormat to avoid colliding with DiffOptions.Output (helm-diff format).
func (t *DoctorImpl) DoctorOutput() string {
	return t.DoctorOptions.ReportFormat
}
