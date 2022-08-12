package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/urfave/cli"
	"go.uber.org/zap"
	"golang.org/x/term"

	"github.com/helmfile/helmfile/pkg/maputil"
	"github.com/helmfile/helmfile/pkg/state"
)

// nolint: golint
type ConfigImpl struct {
	c *cli.Context

	set map[string]interface{}
}

func NewUrfaveCliConfigImpl(c *cli.Context) (ConfigImpl, error) {
	if c.NArg() > 0 {
		err := cli.ShowAppHelp(c)
		if err != nil {
			return ConfigImpl{}, err
		}
		return ConfigImpl{}, fmt.Errorf("err: extraneous arguments: %s", strings.Join(c.Args(), ", "))
	}

	conf := ConfigImpl{
		c: c,
	}

	optsSet := c.GlobalStringSlice("state-values-set")
	if len(optsSet) > 0 {
		set := map[string]interface{}{}
		for i := range optsSet {
			ops := strings.Split(optsSet[i], ",")
			for j := range ops {
				op := strings.SplitN(ops[j], "=", 2)
				k := maputil.ParseKey(op[0])
				v := op[1]

				maputil.Set(set, k, v)
			}
		}
		conf.set = set
	}

	return conf, nil
}

func NewUrfaveCliConfigImplIns(g *GlobalImpl) error {
	optsSet := g.RawStateValuesSet()
	if len(optsSet) > 0 {
		set := map[string]interface{}{}
		for i := range optsSet {
			ops := strings.Split(optsSet[i], ",")
			for j := range ops {
				op := strings.SplitN(ops[j], "=", 2)
				k := maputil.ParseKey(op[0])
				v := op[1]

				maputil.Set(set, k, v)
			}
		}
		g.SetSet(set)
	}

	return nil
}

func (c ConfigImpl) Set() []string {
	return c.c.StringSlice("set")
}

func (c ConfigImpl) SkipRepos() bool {
	return c.c.Bool("skip-repos")
}

func (c ConfigImpl) Wait() bool {
	return c.c.Bool("wait")
}

func (c ConfigImpl) WaitForJobs() bool {
	return c.c.Bool("wait-for-jobs")
}

func (c ConfigImpl) Values() []string {
	return c.c.StringSlice("values")
}

func (c ConfigImpl) Args() string {
	args := c.c.String("args")
	enableHelmDebug := c.c.GlobalBool("debug")

	if enableHelmDebug {
		args = fmt.Sprintf("%s %s", args, "--debug")
	}
	return args
}

func (c ConfigImpl) OutputDir() string {
	return strings.TrimRight(c.c.String("output-dir"), fmt.Sprintf("%c", os.PathSeparator))
}

func (c ConfigImpl) OutputDirTemplate() string {
	return c.c.String("output-dir-template")
}

func (c ConfigImpl) OutputFileTemplate() string {
	return c.c.String("output-file-template")
}

func (c ConfigImpl) Validate() bool {
	return c.c.Bool("validate")
}

func (c ConfigImpl) Concurrency() int {
	return c.c.Int("concurrency")
}

func (c ConfigImpl) HasCommandName(name string) bool {
	return c.c.Command.HasName(name)
}

func (c ConfigImpl) SkipNeeds() bool {
	if !c.IncludeNeeds() {
		return c.c.Bool("skip-needs")
	}

	return false
}

func (c ConfigImpl) IncludeNeeds() bool {
	return c.c.Bool("include-needs") || c.IncludeTransitiveNeeds()
}

func (c ConfigImpl) IncludeTransitiveNeeds() bool {
	return c.c.Bool("include-transitive-needs")
}

// DiffConfig

func (c ConfigImpl) SkipDeps() bool {
	return c.c.Bool("skip-deps")
}

func (c ConfigImpl) DetailedExitcode() bool {
	return c.c.Bool("detailed-exitcode")
}

func (c ConfigImpl) RetainValuesFiles() bool {
	return c.c.Bool("retain-values-files")
}

func (c ConfigImpl) IncludeTests() bool {
	return c.c.Bool("include-tests")
}

func (c ConfigImpl) Suppress() []string {
	return c.c.StringSlice("suppress")
}

func (c ConfigImpl) SuppressSecrets() bool {
	return c.c.Bool("suppress-secrets")
}

func (c ConfigImpl) ShowSecrets() bool {
	return c.c.Bool("show-secrets")
}

func (c ConfigImpl) SuppressDiff() bool {
	return c.c.Bool("suppress-diff")
}

// DeleteConfig

func (c ConfigImpl) Purge() bool {
	return c.c.Bool("purge")
}

// TestConfig

func (c ConfigImpl) Cleanup() bool {
	return c.c.Bool("cleanup")
}

func (c ConfigImpl) Logs() bool {
	return c.c.Bool("logs")
}

func (c ConfigImpl) Timeout() int {
	if !c.c.IsSet("timeout") {
		return state.EmptyTimeout
	}
	return c.c.Int("timeout")
}

// ListConfig

func (c ConfigImpl) Output() string {
	return c.c.String("output")
}

func (c ConfigImpl) KeepTempDir() bool {
	return c.c.Bool("keep-temp-dir")
}

// GlobalConfig

func (c ConfigImpl) HelmBinary() string {
	return c.c.GlobalString("helm-binary")
}

func (c ConfigImpl) KubeContext() string {
	return c.c.GlobalString("kube-context")
}

func (c ConfigImpl) Namespace() string {
	return c.c.GlobalString("namespace")
}

func (c ConfigImpl) Chart() string {
	return c.c.GlobalString("chart")
}

func (c ConfigImpl) FileOrDir() string {
	return c.c.GlobalString("file")
}

func (c ConfigImpl) Selectors() []string {
	return c.c.GlobalStringSlice("selector")
}

func (c ConfigImpl) StateValuesSet() map[string]interface{} {
	return c.set
}

func (c ConfigImpl) StateValuesFiles() []string {
	return c.c.GlobalStringSlice("state-values-file")
}

func (c ConfigImpl) Interactive() bool {
	return c.c.GlobalBool("interactive")
}

func (c ConfigImpl) Color() bool {
	if c := c.c.GlobalBool("color"); c {
		return c
	}

	if c.NoColor() {
		return false
	}

	// We replicate the helm-diff behavior in helmfile
	// because when when helmfile calls helm-diff, helm-diff has no access to term and therefore
	// we can't rely on helm-diff's ability to auto-detect term for color output.
	// See https://github.com/roboll/helmfile/issues/2043

	terminal := term.IsTerminal(int(os.Stdout.Fd()))
	// https://github.com/databus23/helm-diff/issues/281
	dumb := os.Getenv("TERM") == "dumb"
	return terminal && !dumb
}

func (c ConfigImpl) NoColor() bool {
	return c.c.GlobalBool("no-color")
}

func (c ConfigImpl) Context() int {
	return c.c.Int("context")
}

func (c ConfigImpl) DiffOutput() string {
	return c.c.String("output")
}

func (c ConfigImpl) SkipCleanup() bool {
	return c.c.Bool("skip-cleanup")
}

func (c ConfigImpl) SkipCRDs() bool {
	return c.c.Bool("skip-crds")
}

func (c ConfigImpl) SkipDiffOnInstall() bool {
	return c.c.Bool("skip-diff-on-install")
}

func (c ConfigImpl) EmbedValues() bool {
	return c.c.Bool("embed-values")
}

func (c ConfigImpl) IncludeCRDs() bool {
	return c.c.Bool("include-crds")
}

func (c ConfigImpl) SkipTests() bool {
	return c.c.Bool("skip-tests")
}

func (c ConfigImpl) Logger() *zap.SugaredLogger {
	return c.c.App.Metadata["logger"].(*zap.SugaredLogger)
}

func (c ConfigImpl) Env() string {
	env := c.c.GlobalString("environment")
	if env == "" {
		env = os.Getenv("HELMFILE_ENVIRONMENT")
		if env == "" {
			env = state.DefaultEnv
		}
	}
	return env
}
