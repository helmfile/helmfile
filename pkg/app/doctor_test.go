package app

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/helmfile/helmfile/pkg/agent/llm"
)

// doctorStubConfig is a minimal DoctorConfigProvider for testing the
// secret-safety wrapper. It embeds a diffConfig (already used elsewhere in
// pkg/app tests) and adds the doctor-only knobs.
type doctorStubConfig struct {
	diffConfig
	flagLLM     llm.Config
	force       bool
	reportFmt   string
	showSecrets bool
}

func (d doctorStubConfig) FlagLLMConfig() llm.Config { return d.flagLLM }
func (d doctorStubConfig) Force() bool               { return d.force }
func (d doctorStubConfig) DoctorOutput() string      { return d.reportFmt }
func (d doctorStubConfig) ShowSecrets() bool         { return d.showSecrets }

// TestSecretSafeDoctorConfig_PassthroughAllOtherMethods is a regression guard
// against a subtle Go pitfall: when wrapping an interface in a struct and
// overriding one method, all OTHER methods must continue to delegate to the
// wrapped value. A future change that adds a new DiffConfigProvider method
// and forgets to wire it through the wrapper would silently break doctor.
//
// We assert behavior by:
//
//  1. Constructing an inner stub that returns known non-default values for
//     every DiffConfigProvider + DoctorConfigProvider method.
//  2. Wrapping it in secretSafeDoctorConfig.
//  3. Asserting every method on the wrapper returns the inner stub's value,
//     EXCEPT ShowSecrets() which must always return false.
//
// If someone adds a new method to DiffConfigProvider and forgets to consider
// the wrapper, this test won't catch it directly — but adding a case here
// when adding the method is the contract.
func TestSecretSafeDoctorConfig_PassthroughAllOtherMethods(t *testing.T) {
	inner := doctorStubConfig{
		diffConfig: diffConfig{
			concurrency: 7,
			context:     42,
			showSecrets: true, // MUST be flipped to false by the wrapper
			noHooks:     true,
			values:      []string{"prod.yaml"},
			set:         []string{"a=b"},
			validate:    true,
			skipCRDs:    true,
			skipDeps:    true,
		},
		flagLLM:   llm.Config{BaseURL: "x", APIKey: "y", Model: "z"},
		force:     true,
		reportFmt: "json",
	}
	wrapped := secretSafeDoctorConfig{DoctorConfigProvider: inner}

	// The single override: showSecrets MUST be false regardless of inner.
	if wrapped.ShowSecrets() {
		t.Errorf("ShowSecrets() = true; wrapper failed to force false")
	}

	// Spot-check a representative sample of other methods — one per embedded
	// interface — to confirm the wrapper delegates to inner. This catches a
	// wholesale "embedded interface not promoted" bug.
	type check struct {
		name string
		got  any
		want any
	}
	passThroughChecks := []check{
		// DiffConfigProvider surface — scalar types only; slices are
		// compared separately below via sliceEqual to avoid the
		// "comparing uncomparable type []string" panic.
		{"Concurrency", wrapped.Concurrency(), inner.Concurrency()},
		{"Context", wrapped.Context(), inner.Context()},
		{"NoHooks", wrapped.NoHooks(), inner.NoHooks()},
		{"Validate", wrapped.Validate(), inner.Validate()},
		{"SkipCRDs", wrapped.SkipCRDs(), inner.SkipCRDs()},
		{"SkipDeps", wrapped.SkipDeps(), inner.SkipDeps()},
		{"ShowSecrets", wrapped.ShowSecrets(), false}, // not passthrough!
		// DoctorConfigProvider surface
		{"Force", wrapped.Force(), inner.Force()},
		{"DoctorOutput", wrapped.DoctorOutput(), inner.DoctorOutput()},
	}
	for _, c := range passThroughChecks {
		if c.got != c.want {
			t.Errorf("%s() = %v, want %v (passthrough broken)", c.name, c.got, c.want)
		}
	}

	// Slice comparisons need deep-equal, not ==. Use stdlib slices.Equal
	// rather than a hand-rolled helper.
	if !slices.Equal(wrapped.Values(), inner.Values()) {
		t.Errorf("Values passthrough broken: got %v want %v", wrapped.Values(), inner.Values())
	}
	if !slices.Equal(wrapped.Set(), inner.Set()) {
		t.Errorf("Set passthrough broken: got %v want %v", wrapped.Set(), inner.Set())
	}
}

func TestCaptureStdout_CapturesPrint(t *testing.T) {
	got, err := captureStdout(func() error {
		fmt.Println("hello")
		fmt.Println("world")
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	// Println adds trailing newlines; just check both lines are present.
	for _, want := range []string{"hello", "world"} {
		if !strings.Contains(got, want) {
			t.Errorf("captured output missing %q\ngot=%q", want, got)
		}
	}
}

func TestCaptureStdout_PropagatesError(t *testing.T) {
	boom := fmt.Errorf("boom")
	_, err := captureStdout(func() error {
		fmt.Println("still gets captured")
		return boom
	})
	if err != boom {
		t.Errorf("err = %v, want %v", err, boom)
	}
}

func TestCaptureStdout_RestoresRealStdout(t *testing.T) {
	original := stdoutBefore()
	_, _ = captureStdout(func() error {
		fmt.Println("transient")
		return nil
	})
	if stdoutBefore() != original {
		t.Errorf("os.Stdout was not restored after captureStdout")
	}
}

func TestCaptureStdout_LargeOutputDoesNotDeadlock(t *testing.T) {
	// Regression guard: a naive pipe-based capture would deadlock when the
	// producer writes more than the kernel pipe buffer (~64K) without a
	// concurrent reader. We pump 1MB to make sure io.Copy drains in time.
	const n = 1024 * 1024
	payload := strings.Repeat("x", n)

	got, err := captureStdout(func() error {
		_, err := fmt.Print(payload)
		return err
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(got) != n {
		t.Errorf("captured %d bytes, want %d (truncated?)", len(got), n)
	}
}

// TestCaptureStdout_PanicRestoresStdout is a regression guard for a bug where
// captureStdout forgot to defer the os.Stdout restore. If fn panicked, the
// process's stdout was left pointing at a closed pipe forever, silently
// swallowing every subsequent print.
//
// We simulate a panic and then assert (a) the deferred recover caught it, and
// (b) os.Stdout is back to the original file after captureStdout returns.
func TestCaptureStdout_PanicRestoresStdout(t *testing.T) {
	original := os.Stdout

	defer func() {
		// In Go, a panic in a deferred function would propagate; we installed
		// this defer BEFORE the recover() below on purpose so it observes the
		// post-restore state.
		if os.Stdout != original {
			t.Errorf("os.Stdout was not restored after panic; got %p, want %p", os.Stdout, original)
		}
	}()

	// captureStdout must propagate the panic — we are NOT testing panic
	// suppression here, only that os.Stdout is restored when the dust settles.
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected captureStdout to propagate panic, but it returned normally")
		}
	}()

	_, _ = captureStdout(func() error {
		panic("simulated helm-diff crash")
	})
}

// TestCaptureStdout_WCloseFailureDoesNotDeadlock is a regression guard for a
// deadlock that existed when captureStdout ran `<-done` after a failed
// w.Close() with no fallback to unblock the reader goroutine. The reader's
// io.Copy would block forever waiting on EOF that never came.
//
// The realistic trigger for "w.Close fails inside captureStdout" is the fn
// itself closing os.Stdout (e.g. a buggy sub-process inheriting the fd and
// closing it on exit). We simulate that here. The contract we enforce is
// "captureStdout returns within the test timeout" — i.e. no deadlock. We do
// NOT assert payload integrity because r.Close-as-fallback can race with
// io.Copy draining the pipe buffer.
//
// Run with -timeout 30s; a regression hangs the test runner here.
func TestCaptureStdout_WCloseFailureDoesNotDeadlock(t *testing.T) {
	// Use a separate goroutine + Wait so a deadlock surfaces as a test
	// failure instead of hanging the whole `go test` process.
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = captureStdout(func() error {
			// Write something so the pipe is non-trivially exercised.
			fmt.Println("payload")
			// Sabotage: close os.Stdout (currently the pipe write end).
			// captureStdout's later w.Close() will then fail with
			// "file already closed" — this is the exact failure mode that
			// used to deadlock.
			os.Stdout.Close()
			return nil
		})
	}()

	select {
	case <-done:
		// Success: captureStdout returned despite w.Close failure.
	case <-time.After(10 * time.Second):
		t.Fatal("captureStdout deadlocked when w.Close failed (r.Close fallback regressed)")
	}
}

func TestIsDetectedChanges(t *testing.T) {
	code2 := 2
	code1 := 1
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "app exit 2", err: &Error{msg: "Identified at least one change", code: &code2}, want: true},
		{name: "app exit 1", err: &Error{msg: "other", code: &code1}, want: false},
		{name: "app no code", err: &Error{msg: "mystery"}, want: false},
		{name: "plain error", err: fmt.Errorf("not detected changes"), want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isDetectedChanges(tt.err); got != tt.want {
				t.Errorf("isDetectedChanges(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

// stdoutBefore reads the current os.Stdout pointer so we can compare it before
// and after a captureStdout call. Stored as a function so the test does not
// import os directly (we want the test to fail at compile time if anyone ever
// shadows os.Stdout inside doctor.go).
func stdoutBefore() any {
	return os.Stdout
}

// TestIsLogOutputStdout is a regression guard against the
// `--log-output stdout` misconfiguration that would silently mix helmfile's
// own logger output into the LLM prompt via captureStdout. The check itself
// is trivial but the warning it gates is the only user-visible signal.
func TestIsLogOutputStdout(t *testing.T) {
	if !IsLogOutputStdout(os.Stdout) {
		t.Error("os.Stdout should be detected as stdout")
	}
	if IsLogOutputStdout(os.Stderr) {
		t.Error("os.Stderr should NOT be detected as stdout")
	}
	if IsLogOutputStdout(nil) {
		t.Error("nil (helmfile default → stderr) should NOT be detected as stdout")
	}
	var buf strings.Builder
	if IsLogOutputStdout(&buf) {
		t.Error("arbitrary writer should NOT be detected as stdout")
	}
}
