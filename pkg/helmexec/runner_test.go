package helmexec

import (
	"bytes"
	"context"
	_ "embed"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestShellRunner_Execute(t *testing.T) {
	tests := []struct {
		name             string
		want             []byte
		stdoutWant       string
		enableLiveOutput bool
	}{
		{
			name:             "echo_template_no_live_output",
			want:             []byte("template\n"),
			enableLiveOutput: false,
		},
		{
			name:             "echo_template_enable_live_output",
			want:             nil,
			enableLiveOutput: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buffer bytes.Buffer
			shell := ShellRunner{
				Logger: NewLogger(&buffer, "debug"),
				Ctx:    context.TODO(),
			}
			got, err := shell.Execute("echo", strings.Split("template", " "), map[string]string{}, tt.enableLiveOutput)

			if err != nil {
				t.Errorf("Execute() has produced an error = %v", err)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ExecuteStdIn() got = %v, want %v", got, tt.want)
			}
		})
	}
}

//go:embed testdata/live-output-data.txt
var liveOutputData string

func TestLiveOutput(t *testing.T) {
	tests := []struct {
		name    string
		cmd     *exec.Cmd
		wantW   string
		wantErr bool
	}{
		{
			name:    "live_output_data",
			cmd:     exec.Command("cat", "testdata/live-output-data.txt"),
			wantW:   liveOutputData,
			wantErr: false,
		},
		{
			name:    "echo_template",
			cmd:     exec.Command("echo", "template"),
			wantW:   "template\n",
			wantErr: false,
		},
		{
			name: "helm_template",
			cmd:  exec.Command("helm", "template"),
			wantW: `Error: "helm template" requires at least 1 argument

Usage:  helm template [NAME] [CHART] [flags]
`,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &bytes.Buffer{}
			got, err := LiveOutput(context.Background(), tt.cmd, false, w)
			if (err != nil) != tt.wantErr {
				t.Errorf("LiveOutput() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotW := w.String(); gotW != tt.wantW {
				t.Errorf("LiveOutput() gotW = %v, want %v", gotW, tt.wantW)
			}
			if got != nil {
				t.Errorf("LiveOutput() got unespected %v", got)
			}
		})
	}
}

// TestOutput_NilProcessOnCanceledContext verifies that Output does not panic
// when the context is already canceled. The command is started synchronously
// before the select, so a canceled context no longer races on c.Process.
// See helmfile issue #2448.
func TestOutput_NilProcessOnCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cmd := exec.Command("sleep", "10")
	_, err := Output(ctx, cmd, false, &logWriterGenerator{
		log: NewLogger(&bytes.Buffer{}, "debug"),
	})
	if err == nil {
		t.Fatal("expected error due to canceled context, got nil")
	}
}

// TestLiveOutput_CanceledContextNoPanic verifies that LiveOutput does not
// panic when the context is canceled while or after a fast command runs.
func TestLiveOutput_CanceledContextNoPanic(t *testing.T) {
	for i := 0; i < 20; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			<-time.After(time.Millisecond)
			cancel()
		}()

		cmd := exec.Command("true")
		_, _ = LiveOutput(ctx, cmd, false, &bytes.Buffer{})
	}
}

// TestOutput_StartFailureErrorWrapping verifies that a Start() failure (e.g.
// binary not found) is wrapped through the same "unexpected error:" path as
// the previous c.Run()-in-goroutine implementation did. This guards against
// behavioral regressions in the error message format.
func TestOutput_StartFailureErrorWrapping(t *testing.T) {
	cmd := exec.Command("this-binary-does-not-exist-2448")
	_, err := Output(context.Background(), cmd, false, &logWriterGenerator{
		log: NewLogger(&bytes.Buffer{}, "debug"),
	})
	if err == nil {
		t.Fatal("expected error from non-existent command, got nil")
	}
	if !strings.Contains(err.Error(), "unexpected error:") {
		t.Errorf("expected error to be wrapped as 'unexpected error:', got: %v", err)
	}
}

// TestOutput_CanceledContextAfterExit verifies Output does not panic when the
// context is canceled right after a fast command finishes. This stresses the
// race between the command exiting (Process could be reaped) and ctx.Done().
func TestOutput_CanceledContextAfterExit(t *testing.T) {
	for i := 0; i < 50; i++ {
		ctx, cancel := context.WithCancel(context.Background())

		cmd := exec.Command("true")
		go func() {
			<-time.After(time.Millisecond)
			cancel()
		}()

		_, err := Output(ctx, cmd, false, &logWriterGenerator{
			log: NewLogger(&bytes.Buffer{}, "debug"),
		})
		// "true" should exit 0; even if canceled we tolerate a context error.
		_ = err
	}
}
