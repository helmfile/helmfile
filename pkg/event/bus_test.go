package event

import (
	"fmt"
	"io"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/helmfile/helmfile/pkg/environment"
	ffs "github.com/helmfile/helmfile/pkg/filesystem"
)

type runner struct {
}

func (r *runner) ExecuteStdIn(cmd string, args []string, env map[string]string, stdin io.Reader) ([]byte, error) {
	return []byte(""), nil
}

func (r *runner) Execute(cmd string, args []string, env map[string]string, enableLiveOutput bool) ([]byte, error) {
	if cmd == "ng" {
		return nil, fmt.Errorf("cmd failed due to invalid cmd: %s", cmd)
	}
	for _, a := range args {
		if a == "ng" {
			return nil, fmt.Errorf("cmd failed due to invalid arg: %s", a)
		}
	}
	return []byte(""), nil
}

func TestTrigger(t *testing.T) {
	cases := []struct {
		name           string
		hook           *Hook
		triggeredEvt   string
		expectedResult bool
		expectedErr    string
	}{
		{
			"okhook1",
			&Hook{"okhook1", []string{"foo"}, "ok", nil, []string{}, true},
			"foo",
			true,
			"",
		},
		{
			"okhooké",
			&Hook{"okhook2", []string{"foo"}, "ok", nil, []string{}, false},
			"foo",
			true,
			"",
		},
		{
			"missinghook1",
			&Hook{"okhook1", []string{"foo"}, "ok", nil, []string{}, false},
			"bar",
			false,
			"",
		},
		{
			"nohook1",
			nil,
			"bar",
			false,
			"",
		},
		{
			"nghook1",
			&Hook{"nghook1", []string{"foo"}, "ng", nil, []string{}, false},
			"foo",
			false,
			"hook[nghook1]: command `ng` failed: cmd failed due to invalid cmd: ng",
		},
		{
			"nghook2",
			&Hook{"nghook2", []string{"foo"}, "ok", nil, []string{"ng"}, false},
			"foo",
			false,
			"hook[nghook2]: command `ok` failed: cmd failed due to invalid arg: ng",
		},
		{
			"okkubeapply1",
			&Hook{"okkubeapply1", []string{"foo"}, "", map[string]string{"kustomize": "kustodir"}, []string{}, false},
			"foo",
			true,
			"",
		},
		{
			"okkubeapply2",
			&Hook{"okkubeapply2", []string{"foo"}, "", map[string]string{"filename": "resource.yaml"}, []string{}, false},
			"foo",
			true,
			"",
		},
		{
			"kokubeapply",
			&Hook{"kokubeapply", []string{"foo"}, "", map[string]string{"kustomize": "kustodir", "filename": "resource.yaml"}, []string{}, true},
			"foo",
			false,
			"hook[kokubeapply]: kustomize & filename cannot be used together",
		},
		{
			"kokubeapply2",
			&Hook{"kokubeapply2", []string{"foo"}, "", map[string]string{}, []string{}, true},
			"foo",
			false,
			"hook[kokubeapply2]: either kustomize or filename must be given",
		},
		{
			"kokubeapply3",
			&Hook{"", []string{"foo"}, "", map[string]string{}, []string{}, true},
			"foo",
			false,
			"hook[kubectlApply]: either kustomize or filename must be given",
		},
		{
			"warnkubeapply1",
			&Hook{"warnkubeapply1", []string{"foo"}, "ok", map[string]string{"filename": "resource.yaml"}, []string{}, true},
			"foo",
			true,
			"",
		},
		{
			"warnkubeapply2",
			&Hook{"warnkubeapply2", []string{"foo"}, "", map[string]string{"filename": "resource.yaml"}, []string{"ng"}, true},
			"foo",
			true,
			"",
		},
		{
			"warnkubeapply3",
			&Hook{"warnkubeapply3", []string{"foo"}, "ok", map[string]string{"filename": "resource.yaml"}, []string{"ng"}, true},
			"foo",
			true,
			"",
		},
	}
	readFile := func(filename string) ([]byte, error) {
		return nil, fmt.Errorf("unexpected call to readFile: %s", filename)
	}
	glob := func(pattern string) ([]string, error) {
		return nil, nil
	}
	getWD := func() (string, error) {
		return "path", nil
	}
	for _, c := range cases {
		hooks := []Hook{}
		if c.hook != nil {
			hooks = append(hooks, *c.hook)
		}
		observer, observedLogs := observer.New(zap.InfoLevel)
		zeLogger := zap.New(observer).Sugar()
		bus := &Bus{
			Hooks:         hooks,
			StateFilePath: "path/to/helmfile.yaml",
			BasePath:      "path/to",
			Namespace:     "myns",
			Env:           environment.Environment{Name: "prod"},
			Logger:        zeLogger,
			Fs:            &ffs.FileSystem{ReadFile: readFile, Glob: glob, Getwd: getWD},
		}

		bus.Runner = &runner{}
		data := map[string]any{
			"Release":         "myrel",
			"HelmfileCommand": "mycmd",
		}
		ok, err := bus.Trigger(c.triggeredEvt, nil, data)

		if ok != c.expectedResult {
			t.Errorf("unexpected result for case \"%s\": expected=%v, actual=%v", c.name, c.expectedResult, ok)
		}

		if c.expectedErr != "" {
			if err == nil {
				t.Errorf("error expected for case \"%s\", but not occurred", c.name)
			} else if err.Error() != c.expectedErr {
				t.Errorf("unexpected error for case \"%s\": expected=%s, actual=%v", c.name, c.expectedErr, err)
			}
		} else {
			if err != nil {
				t.Errorf("unexpected error for case \"%s\": %v", c.name, err)
			}
		}
		if observedLogs.Len() != 0 && !hooks[0].ShowLogs {
			t.Errorf("unexpected error for case \"%s\": Logs should not be created : %v", c.name, observedLogs.All())
		}
	}
}
