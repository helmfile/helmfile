package app

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/helmfile/helmfile/pkg/agent/doctor"
	"github.com/helmfile/helmfile/pkg/agent/llm"
)

// Doctor runs `helmfile diff` and (when an LLM is configured) asks the model
// to summarize the diff and flag risks.
//
// When no LLM is configured, doctor falls back to `helmfile diff` with
// ShowSecrets forced to false. Most diff flags are accepted; note that
// --output is reserved for the doctor report format (use --diff-output for
// helm-diff's plugin output format).
//
// LLM configuration precedence: env (HELMFILE_LLM_*) < helmfile.yaml (llm:)
// < CLI flags (--llm-*).
//
// Secrets are ALWAYS redacted before any byte leaves the process: (1) the
// DiffConfigProvider is wrapped so ShowSecrets() returns false, making
// helm-diff emit "<REDACTED>" placeholders; (2) a defense-in-depth
// SecretRedactor strips residual secret-looking content. The redaction
// count is surfaced in the report footer.
//
// See the `helmfile doctor --help` long description for the full user-facing
// documentation including exit codes.
func (a *App) Doctor(c DoctorConfigProvider) error {
	// Wrap c so that ShowSecrets() is forced to false regardless of user
	// flags. This is the primary secret-safety mechanism: it makes helm-diff
	// itself redact secret values with "<REDACTED>" placeholders.
	safeCfg := secretSafeDoctorConfig{DoctorConfigProvider: c}

	// Resolve LLM config (env < yaml < flag) and the release-name list.
	finalLLM, releases := a.resolveLLMConfig(safeCfg)

	a.Logger.Debugf("doctor: resolved llm config: configured=%v model=%s", finalLLM.IsConfigured(), finalLLM.Model)

	// Unconfigured → degrade to plain diff, byte-for-byte. We still hand
	// safeCfg in so ShowSecrets() is forced false here too: even when no LLM
	// is configured, doctor itself must never echo raw secrets to stdout
	// (a user might have piped doctor into a CI log by mistake).
	if !finalLLM.IsConfigured() {
		a.Logger.Debug("doctor: llm not configured, behaving as `helmfile diff` with ShowSecrets forced off")
		return a.Diff(safeCfg)
	}

	// Capture stdout while running diff. helmfile writes the rendered diff
	// to os.Stdout via fmt.Print (see pkg/state/state.go DiffReleases).
	diffText, diffErr := captureStdout(func() error {
		return a.Diff(safeCfg)
	})

	// Shared redactor — single source of truth for failure and success paths.
	redactor := doctor.NewSecretRedactor()

	// A "detected changes" exit (code 2 from helm-diff) is expected here and
	// must NOT short-circuit the analysis; doctor exists to react to changes.
	if diffErr != nil && !isDetectedChanges(diffErr) {
		// Real failure: dump whatever diff we got (through the shared
		// redactor) and bail.
		redactedForBail, n := redactor.Redact(diffText)
		if n > 0 {
			a.Logger.Warnf("doctor: %d secrets redacted from failure fallback output", n)
		}
		_, _ = fmt.Fprint(os.Stdout, redactedForBail)
		return diffErr
	}

	// Invoke the LLM. doctor.Analyze applies the shared redactor before the
	// diff ever leaves the process boundary.
	client := llm.NewClient(finalLLM)
	result := doctor.Analyze(a.ctx, diffText, doctor.Options{
		Client:      client,
		Environment: a.Env,
		Releases:    releases,
		Model:       finalLLM.Model,
		Redactor:    redactor,
	})
	if result.SecretsRedacted > 0 {
		a.Logger.Infof("doctor: %d secrets redacted before LLM transmission", result.SecretsRedacted)
	}

	// Render the report.
	report := renderReport(result, c.DoctorOutput())
	if report != "" {
		_, _ = fmt.Fprintln(os.Stdout, report)
	}

	// High-risk gate.
	if result.HasHighRisk() && !c.Force() {
		code := 2
		return &Error{msg: "doctor: high-severity risk detected (override with --force)", code: &code}
	}
	return nil
}

// resolveLLMConfig merges env < yaml < flag into the final LLM Config and
// harvests the release-name list matching the current selector.
//
// PERFORMANCE NOTE: ForEachState DOES run the full helmfile state loader
// (remote fetches, go-template rendering, base inheritance), so this is NOT
// free. For a large helmfile the load can take seconds. The main App.Diff()
// call below will load the state again, so doctor pays the load cost twice
// in the configured path. This is accepted as a known limitation: caching
// the loaded state across the App API would require touching core code,
// which doctor intentionally avoids. In the unconfigured path we return
// before App.Diff runs the second load, so the cost equals plain diff.
//
// On peek error: warns but does not fail. The second load inside App.Diff
// will surface a proper error.
func (a *App) resolveLLMConfig(safeCfg secretSafeDoctorConfig) (llm.Config, []string) {
	yamlLLM, releases, peekErr := a.peekDoctorContext(safeCfg)
	if peekErr != nil {
		a.Logger.Warnf("doctor: failed to peek helmfile.yaml for llm config: %v (continuing; LLM may be treated as unconfigured)", peekErr)
	}
	finalLLM := doctor.ResolveConfig(doctor.EnvConfig(), yamlLLM, safeCfg.FlagLLMConfig())
	return finalLLM, releases
}

// renderReport picks the right renderer for the requested format and returns
// the formatted string. Empty string means "nothing to print".
func renderReport(r doctor.Result, format string) string {
	switch doctor.ParseFormat(format) {
	case doctor.FormatJSON:
		return doctor.ReportJSON(r)
	default:
		return doctor.ReportText(r)
	}
}

// IsLogOutputStdout reports whether helmfile's logger writer is os.Stdout.
// Used by doctor to warn the user that their log lines will leak into the
// LLM prompt when captureStdout swaps os.Stdout.
//
// helmfile stores the configured log writer on GlobalOptions.LogOutput
// (io.Writer). When nil, the default is os.Stderr — see cmd/root.go
// PersistentPreRunE. Comparing the pointer to os.Stdout catches the explicit
// `--log-output stdout` misconfiguration that would mix logs into the diff.
//
// Exported so the cmd layer can call it before invoking App.Doctor.
func IsLogOutputStdout(w io.Writer) bool {
	return w == os.Stdout
}

// to false. Doctor must never let helm-diff emit secret plaintext, even when
// the user passed --show-secrets by mistake or has a legacy helmfile.yaml
// that sets showSecrets: true via templates.
//
// All other DiffConfigProvider methods are passed through unchanged.
type secretSafeDoctorConfig struct {
	DoctorConfigProvider
}

// ShowSecrets is forced to false. This propagates through App.Diff →
// run.diff → st.DiffReleases(..., showSecrets=false, ...) → helm-diff
// plugin's own redaction logic, which substitutes "<REDACTED>" for secret
// values. doctor's own SecretRedactor handles any residual leaks.
func (s secretSafeDoctorConfig) ShowSecrets() bool { return false }

// peekDoctorContext walks the helmfile(s) once to harvest the first configured
// `llm:` block plus the full set of release names that match the current
// selector. Used by Doctor to drive config precedence without running helm.
//
// This DOES run the full state loader (remote fetches, template rendering,
// base inheritance). It is NOT a cheap YAML-only peek — see the performance
// note in App.Doctor for why we accept the double-load cost.
//
// Returns (zero-value llm.Config, nil, nil) when no `llm:` block exists.
// Returns the ForEachState error verbatim so callers can decide whether to
// warn or fail.
func (a *App) peekDoctorContext(c DoctorConfigProvider) (llm.Config, []string, error) {
	var yamlLLM llm.Config
	var releases []string
	var mu sync.Mutex

	err := a.ForEachState(func(run *Run) (bool, []error) {
		mu.Lock()
		defer mu.Unlock()

		st := run.State()
		if st != nil {
			if !yamlLLM.IsConfigured() && st.LLM.IsConfigured() {
				yamlLLM = st.LLM
			}
			for _, r := range st.Releases {
				if r.Desired() {
					releases = append(releases, r.Name)
				}
			}
		}
		return false, nil
	}, c.IncludeNeeds(), SetFilter(true))

	return yamlLLM, releases, err
}

// captureStdout temporarily swaps os.Stdout for a pipe, runs fn, and returns
// whatever fn wrote. Used by doctor to capture helm-diff output for LLM
// analysis without touching helmfile core.
//
// Robustness:
//
//   - os.Stdout is always restored, even if fn panics (defer).
//   - The reader goroutine always terminates: cleanup closes both ends of
//     the pipe idempotently, so io.Copy sees EOF or "read on closed pipe".
//     Cleanup does NOT wait on `done` — on the panic path buffer completeness
//     is moot, and on the happy path the body already waited.
//
// Go semantic warning: `return buf.String(), fnErr` evaluates buf.String()
// BEFORE deferred funcs run, so the body must `<-done` (wait for io.Copy to
// drain) before returning. The deferred cleanup only fires on panic, where
// it closes w+r so the goroutine exits but does NOT wait for it.
func captureStdout(fn func() error) (string, error) {
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		return "", fmt.Errorf("doctor: failed to create stdout pipe: %w", err)
	}

	buf := &bytes.Buffer{}
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(buf, r)
		close(done)
	}()

	// Panic-safety net: close both ends so the reader goroutine exits even if
	// the body's w.Close + <-done are never reached. Idempotent on happy path.
	defer func() {
		os.Stdout = oldStdout
		_ = w.Close()
		_ = r.Close()
	}()

	os.Stdout = w

	fnErr := fn()

	// Signal EOF and wait for the reader to drain before evaluating buf.String().
	_ = w.Close()
	<-done

	return buf.String(), fnErr
}

// isDetectedChanges reports whether err is helmfile's "detected at least one
// change" signal from `helm diff --detailed-exitcode` (exit code 2). doctor
// must swallow this: a non-empty diff is the whole point of running doctor.
func isDetectedChanges(err error) bool {
	if err == nil {
		return false
	}
	var appErr *Error
	if errors.As(err, &appErr) {
		if appErr.code != nil && *appErr.code == 2 {
			return true
		}
	}
	return false
}
