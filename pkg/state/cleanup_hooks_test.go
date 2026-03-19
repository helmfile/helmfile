package state

import (
	"errors"
	"io"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/helmfile/helmfile/pkg/environment"
	"github.com/helmfile/helmfile/pkg/event"
	ffs "github.com/helmfile/helmfile/pkg/filesystem"
	"github.com/helmfile/helmfile/pkg/helmexec"
)

type mockRunner struct {
	executeCalls []struct {
		cmd  string
		args []string
		env  map[string]string
	}
}

func (r *mockRunner) Execute(cmd string, args []string, env map[string]string, _ bool) ([]byte, error) {
	r.executeCalls = append(r.executeCalls, struct {
		cmd  string
		args []string
		env  map[string]string
	}{cmd: cmd, args: args, env: env})
	return []byte(""), nil
}

func (r *mockRunner) ExecuteStdIn(cmd string, args []string, env map[string]string, _ io.Reader) ([]byte, error) {
	return []byte(""), nil
}

func TestEventBusTriggerCleanupEventWithError(t *testing.T) {
	runner := &mockRunner{}

	core, _ := observer.New(zap.InfoLevel)
	logger := zap.New(core).Sugar()

	testError := errors.New("sync failed: release error")

	hooks := []event.Hook{
		{
			Name:     "cleanup-with-error",
			Events:   []string{"cleanup"},
			Command:  "echo",
			Args:     []string{"error is '{{ .Event.Error }}'"},
			ShowLogs: true,
		},
	}

	bus := &event.Bus{
		Hooks:         hooks,
		StateFilePath: "/path/to/helmfile.yaml",
		BasePath:      ".",
		Namespace:     "default",
		Env:           environment.Environment{Name: "default"},
		Logger:        logger,
		Fs:            ffs.DefaultFileSystem(),
		Runner:        runner,
	}

	data := map[string]any{
		"HelmfileCommand": "sync",
	}

	executed, err := bus.Trigger("cleanup", testError, data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !executed {
		t.Fatal("expected cleanup hook to be executed")
	}

	if len(runner.executeCalls) != 1 {
		t.Fatalf("expected 1 execute call, got %d", len(runner.executeCalls))
	}

	call := runner.executeCalls[0]
	if call.cmd != "echo" {
		t.Errorf("expected command 'echo', got %q", call.cmd)
	}

	if len(call.args) != 1 {
		t.Fatalf("expected 1 arg, got %d", len(call.args))
	}

	expectedArg := "error is 'sync failed: release error'"
	if !strings.Contains(call.args[0], "error is") {
		t.Errorf("expected arg to contain 'error is', got %q", call.args[0])
	}

	if call.args[0] != expectedArg {
		t.Errorf("expected arg %q, got %q", expectedArg, call.args[0])
	}
}

func TestEventBusTriggerCleanupEventWithNilError(t *testing.T) {
	runner := &mockRunner{}

	core, _ := observer.New(zap.InfoLevel)
	logger := zap.New(core).Sugar()

	hooks := []event.Hook{
		{
			Name:     "cleanup-nil-error",
			Events:   []string{"cleanup"},
			Command:  "echo",
			Args:     []string{"error is '{{ .Event.Error }}'"},
			ShowLogs: true,
		},
	}

	bus := &event.Bus{
		Hooks:         hooks,
		StateFilePath: "/path/to/helmfile.yaml",
		BasePath:      ".",
		Namespace:     "default",
		Env:           environment.Environment{Name: "default"},
		Logger:        logger,
		Fs:            ffs.DefaultFileSystem(),
		Runner:        runner,
	}

	data := map[string]any{
		"HelmfileCommand": "sync",
	}

	executed, err := bus.Trigger("cleanup", nil, data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !executed {
		t.Fatal("expected cleanup hook to be executed")
	}

	if len(runner.executeCalls) != 1 {
		t.Fatalf("expected 1 execute call, got %d", len(runner.executeCalls))
	}

	call := runner.executeCalls[0]
	expectedArg := "error is '<nil>'"
	if call.args[0] != expectedArg {
		t.Errorf("expected arg %q, got %q", expectedArg, call.args[0])
	}
}

var _ helmexec.Runner = &mockRunner{}
