package helmexec

import (
	"bytes"
	"context"
	_ "embed"
	"os/exec"
	"reflect"
	"strings"
	"testing"
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
