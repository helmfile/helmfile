package helmexec

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/attribute"
)

func TestRedactArgsLegacy(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "two-argument --set is masked",
			args: []string{"upgrade", "--set", "secret=1"},
			want: []string{"upgrade", "--set", redactedArg},
		},
		{
			name: "--set prefix covers --set-string/--set-file/--set-json",
			args: []string{"--set-string", "a=b", "--set-file", "f", "--set-json", "{}"},
			want: []string{"--set-string", redactedArg, "--set-file", redactedArg, "--set-json", redactedArg},
		},
		{
			name: "single-argument --set=k=v is NOT masked (documented legacy gap)",
			args: []string{"--set=secret=1"},
			want: []string{"--set=secret=1"},
		},
		{
			name: "credential flags are NOT masked (documented legacy gap)",
			args: []string{"--username", "bob", "--password", "hunter2"},
			want: []string{"--username", "bob", "--password", "hunter2"},
		},
		{
			name: "first argument is never masked",
			args: []string{"--set"},
			want: []string{"--set"},
		},
		{
			name: "empty args",
			args: []string{},
			want: []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RedactArgs(tt.args, RedactionLegacy)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRedactArgsStrict(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "legacy two-argument forms remain masked",
			args: []string{"upgrade", "--set", "secret=1", "--set-string", "s=2"},
			want: []string{"upgrade", "--set", redactedArg, "--set-string", redactedArg},
		},
		{
			name: "single-argument --set=k=v is masked",
			args: []string{"--set=secret=1", "--set-string=a=b", "--set-json={}"},
			want: []string{"--set=" + redactedArg, "--set-string=" + redactedArg, "--set-json=" + redactedArg},
		},
		{
			name: "credential flags are masked",
			args: []string{"registry", "login", "--username", "bob", "--password", "hunter2"},
			want: []string{"registry", "login", "--username", redactedArg, "--password", redactedArg},
		},
		{
			name: "key-file is masked",
			args: []string{"--key-file", "/etc/secrets/key"},
			want: []string{"--key-file", redactedArg},
		},
		{
			name: "password-stdin has no value to mask",
			args: []string{"--password-stdin"},
			want: []string{"--password-stdin"},
		},
		{
			name: "benign flags untouched",
			args: []string{"upgrade", "--install", "envoy", "./chart", "--namespace", "ingress", "--reset-values"},
			want: []string{"upgrade", "--install", "envoy", "./chart", "--namespace", "ingress", "--reset-values"},
		},
		{
			name: "prefix lookalikes untouched",
			args: []string{"--settlement", "x", "--usernames", "y"},
			want: []string{"--settlement", "x", "--usernames", "y"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RedactArgs(tt.args, RedactionStrict)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRedactArgsDoesNotMutateInput(t *testing.T) {
	orig := []string{"--set", "secret=1", "--set=x=y"}
	before := reflect.ValueOf(orig).Pointer()

	_ = RedactArgs(orig, RedactionStrict)

	assert.Equal(t, []string{"--set", "secret=1", "--set=x=y"}, orig, "input slice must not be mutated")
	assert.Equal(t, before, reflect.ValueOf(orig).Pointer(), "input backing array must be reused by the caller, not modified")
}

func TestExecSpanAttributes(t *testing.T) {
	tests := []struct {
		name         string
		cmd          string
		args         []string
		wantName     string
		wantAttrVals map[string][]string
	}{
		{
			name:     "helm invocation",
			cmd:      "/usr/local/bin/helm",
			args:     []string{"upgrade", "--install", "envoy", "./chart"},
			wantName: "helm.exec",
			wantAttrVals: map[string][]string{
				"exec.command":    {"helm"},
				"helm.subcommand": {"upgrade"},
			},
		},
		{
			name:     "helm with leading flags: subcommand is first positional",
			cmd:      "helm",
			args:     []string{"--kube-context", "ctx", "repo", "update"},
			wantName: "helm.exec",
			wantAttrVals: map[string][]string{
				"helm.subcommand": {"repo"},
			},
		},
		{
			name:     "non-helm binary",
			cmd:      "./scripts/migrate.sh",
			args:     []string{"--verbose"},
			wantName: "os.exec",
			wantAttrVals: map[string][]string{
				"exec.command": {"migrate.sh"},
			},
		},
		{
			name:     "secret args are redacted and flagged",
			cmd:      "helm",
			args:     []string{"upgrade", "--set", "pw=1", "--set=pw2=2"},
			wantName: "helm.exec",
			wantAttrVals: map[string][]string{
				"exec.args":     {"upgrade", "--set", redactedArg, "--set=" + redactedArg},
				"exec.redacted": {"true"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, attrs := execSpanAttributes(tt.cmd, tt.args)

			assert.Equal(t, tt.wantName, name)
			got := map[string][]string{}
			for _, kv := range attrs {
				switch kv.Value.Type() {
				case attribute.STRING:
					got[string(kv.Key)] = []string{kv.Value.AsString()}
				case attribute.STRINGSLICE:
					got[string(kv.Key)] = kv.Value.AsStringSlice()
				case attribute.BOOL:
					got[string(kv.Key)] = []string{fmt.Sprintf("%t", kv.Value.AsBool())}
				}
			}
			for key, want := range tt.wantAttrVals {
				assert.Equal(t, want, got[key], "attribute %s", key)
			}
		})
	}
}
