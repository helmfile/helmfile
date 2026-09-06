package event

import (
	goContext "context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"github.com/helmfile/helmfile/pkg/environment"
	"github.com/helmfile/helmfile/pkg/envvar"
	"github.com/helmfile/helmfile/pkg/filesystem"
	"github.com/helmfile/helmfile/pkg/helmexec"
	"github.com/helmfile/helmfile/pkg/telemetry"
	"github.com/helmfile/helmfile/pkg/tmpl"
)

type Hook struct {
	Name     string            `yaml:"name"`
	Events   []string          `yaml:"events"`
	Command  string            `yaml:"command"`
	Kubectl  map[string]string `yaml:"kubectlApply,omitempty"`
	Args     []string          `yaml:"args"`
	ShowLogs bool              `yaml:"showlogs"`
}

type event struct {
	Name  string
	Error error
}

type Bus struct {
	Runner helmexec.Runner
	Hooks  []Hook

	// Ctx, when set, is used by the lazily-constructed default Runner so that
	// hook subprocesses join the current trace. It should carry trace context
	// without cancellation (see the WithoutCancel bridge in pkg/state); nil
	// falls back to context.TODO(), the historical behavior.
	Ctx goContext.Context

	BasePath      string
	StateFilePath string
	Namespace     string
	Chart         string

	Env environment.Environment
	Fs  *filesystem.FileSystem

	Logger *zap.SugaredLogger
}

var (
	disableHooks bool
)

func init() {
	disableHooks, _ = strconv.ParseBool(os.Getenv(envvar.DisableHooks))
}

func (bus *Bus) Trigger(evt string, evtErr error, context map[string]any) (bool, error) {
	if disableHooks && len(bus.Hooks) > 0 {
		return false, fmt.Errorf("%s is active, hooks are disabled", envvar.DisableHooks)
	}

	if bus.Runner == nil {
		ctx := bus.Ctx
		if ctx == nil {
			// It would be better to pass app.Ctx here, but it requires a lot of work.
			// It seems that this code only for running hooks, which took not to long time as helm.
			ctx = goContext.TODO()
		}
		bus.Runner = helmexec.ShellRunner{
			Dir:    bus.BasePath,
			Logger: bus.Logger,
			Ctx:    ctx,
		}
	}

	executed := false

	for _, hook := range bus.Hooks {
		contained := false
		for _, e := range hook.Events {
			contained = contained || e == evt
		}
		if !contained {
			continue
		}

		hookExecuted, err := bus.runHook(hook, evt, evtErr, context)
		if err != nil {
			return false, err
		}
		executed = executed || hookExecuted
	}

	return executed, nil
}

// runHook renders and executes a single hook; the returned bool reports
// whether the hook ran. The whole hook execution is wrapped in one
// helmfile.hook span (a no-op when telemetry is disabled), and the hook's
// subprocess span nests under it via hookRunner.
func (bus *Bus) runHook(hook Hook, evt string, evtErr error, context map[string]any) (executed bool, err error) {
	name := hook.Name
	if name == "" {
		if hook.Kubectl != nil {
			name = "kubectlApply"
		} else {
			name = hook.Command
		}
	}

	hookCtx, span := telemetry.Tracer(telemetry.ScopeHelmfile).Start(bus.spanParent(), "helmfile.hook",
		trace.WithAttributes(
			attribute.String("hook.event", evt),
			attribute.String("hook.name", name),
		),
	)
	defer func() {
		if err != nil {
			// The raw error embeds the rendered command (which may contain
			// templated credentials) and the runner's detailed exit error;
			// keep the span description generic.
			span.SetStatus(codes.Error, "hook failed")
		}
		span.End()
	}()

	if hook.Kubectl != nil {
		if hook.Command != "" {
			bus.Logger.Warnf("warn: ignoring command '%s' given within a kubectlApply hook", hook.Command)
		}
		hook.Command = "kubectl"
		if val, found := hook.Kubectl["filename"]; found {
			if _, found := hook.Kubectl["kustomize"]; found {
				return false, fmt.Errorf("hook[%s]: kustomize & filename cannot be used together", name)
			}
			hook.Args = append([]string{"apply", "-f"}, val)
		} else if val, found := hook.Kubectl["kustomize"]; found {
			hook.Args = append([]string{"apply", "-k"}, val)
		} else {
			return false, fmt.Errorf("hook[%s]: either kustomize or filename must be given", name)
		}
	}

	bus.Logger.Debugf("hook[%s]: stateFilePath=%s, basePath=%s\n", name, bus.StateFilePath, bus.BasePath)

	data := map[string]any{
		"Environment": bus.Env,
		"Namespace":   bus.Namespace,
		"Event": event{
			Name:  evt,
			Error: evtErr,
		},
	}
	for k, v := range context {
		data[k] = v
	}
	render := tmpl.NewTextRenderer(bus.Fs, bus.BasePath, data)

	bus.Logger.Debugf("hook[%s]: triggered by event \"%s\"\n", name, evt)

	command, err := render.RenderTemplateText(hook.Command)
	if err != nil {
		return false, fmt.Errorf("hook[%s]: %v", name, err)
	}

	args := make([]string, len(hook.Args))
	for i, raw := range hook.Args {
		args[i], err = render.RenderTemplateText(raw)
		if err != nil {
			return false, fmt.Errorf("hook[%s]: %v", name, err)
		}
	}

	bytes, err := bus.hookRunner(hookCtx).Execute(command, args, map[string]string{}, false)
	bus.Logger.Debugf("hook[%s]: %s\n", name, string(bytes))
	if hook.ShowLogs {
		prefix := fmt.Sprintf("\nhook[%s] logs | ", evt)
		bus.Logger.Infow(prefix + strings.ReplaceAll(string(bytes), "\n", prefix))
	}

	if err != nil {
		return false, fmt.Errorf("hook[%s]: command `%s` failed: %v", name, command, err)
	}

	return true, nil
}

// spanParent returns the context hook spans attach to.
func (bus *Bus) spanParent() goContext.Context {
	if bus.Ctx != nil {
		return bus.Ctx
	}
	return goContext.Background()
}

// hookRunner returns a runner whose context is the hook span's, so the
// subprocess span started inside ShellRunner nests under the hook span. The
// cancellation semantics are unchanged: hookCtx derives from Bus.Ctx, which
// by contract never carries cancellation. Non-ShellRunner runners (test
// fakes) are returned unchanged.
func (bus *Bus) hookRunner(hookCtx goContext.Context) helmexec.Runner {
	switch r := bus.Runner.(type) {
	case *helmexec.ShellRunner:
		clone := *r
		clone.Ctx = hookCtx
		return &clone
	case helmexec.ShellRunner:
		clone := r
		clone.Ctx = hookCtx
		return clone
	default:
		return bus.Runner
	}
}
