package config

import (
	"errors"
	"fmt"
	"io"
	"os"

	"go.uber.org/zap"
	"golang.org/x/term"

	"github.com/helmfile/helmfile/pkg/envvar"
	"github.com/helmfile/helmfile/pkg/maputil"
	"github.com/helmfile/helmfile/pkg/state"
)

// GlobalOptions is the global configuration for the Helmfile CLI.
type GlobalOptions struct {
	// HelmBinary is the path to the Helm binary.
	HelmBinary string
	// KustomizeBinary is the path to the Kustomize binary.
	KustomizeBinary string
	// File is the path to the Helmfile.
	File string
	// Environment is the name of the environment to use.
	Environment string
	// StateValuesSet is a list of state values to set on the command line.
	StateValuesSet []string
	// StateValuesSetString is a list of state values to set on the command line.
	StateValuesSetString []string
	// StateValuesFiles is a list of state values files to use.
	StateValuesFile []string
	// SkipDeps is true if the running "helm repo update" and "helm dependency build" should be skipped
	SkipDeps bool
	// SkipRefresh is true if the running "helm repo update" should be skipped
	SkipRefresh bool
	// StripArgsValuesOnExitError is true if the ARGS output on exit error should be suppressed
	StripArgsValuesOnExitError bool
	// DisableForceUpdate is true if force updating repos is not desirable when executing "helm repo add"
	DisableForceUpdate bool
	// Quiet is true if the output should be quiet.
	Quiet bool
	// Kubeconfig is the path to the kubeconfig file to use.
	Kubeconfig string
	// KubeContext is the name of the kubectl context to use.
	KubeContext string
	// Debug is true if the output should be verbose.
	Debug bool
	// Color is true if the output should be colorized.
	Color bool
	// NoColor is true if the output should not be colorized.
	NoColor bool
	// LogLevel is the log level to use.
	LogLevel string
	// Namespace is the namespace to use.
	Namespace string
	// Chart is the chart to use.
	Chart string
	// Selector is a list of selectors to use.
	Selector []string
	// AllowNoMatchingRelease is not exit with an error code if the provided selector has no matching releases.
	AllowNoMatchingRelease bool
	// logger is the logger to use.
	logger *zap.SugaredLogger
	// EnableLiveOutput enables live output from the Helm binary stdout/stderr into Helmfile own stdout/stderr
	EnableLiveOutput bool
	// Interactive is true if the user should be prompted for input.
	Interactive bool
	// Args is the list of arguments to pass to the Helm binary.
	Args string
	// LogOutput is the writer to use for writing logs.
	LogOutput io.Writer
}

// Logger returns the logger to use.
func (g *GlobalOptions) Logger() *zap.SugaredLogger {
	return g.logger
}

// GetLogLevel returns the log level to use.
func (g *GlobalOptions) SetLogger(logger *zap.SugaredLogger) {
	g.logger = logger
}

// GlobalImpl is the global configuration for the Helmfile CLI.
type GlobalImpl struct {
	*GlobalOptions
	set map[string]any
}

// NewGlobalImpl creates a new GlobalImpl.
func NewGlobalImpl(opts *GlobalOptions) *GlobalImpl {
	return &GlobalImpl{
		GlobalOptions: opts,
		set:           make(map[string]any),
	}
}

// Setset sets the set
func (g *GlobalImpl) SetSet(set map[string]any) {
	g.set = maputil.MergeMaps(g.set, set)
}

// HelmBinary returns the path to the Helm binary.
func (g *GlobalImpl) HelmBinary() string {
	return g.GlobalOptions.HelmBinary
}

// KustomizeBinary returns the path to the Kustomize binary.
func (g *GlobalImpl) KustomizeBinary() string {
	return g.GlobalOptions.KustomizeBinary
}

// Kubeconfig returns the path to the kubeconfig file to use.
func (g *GlobalImpl) Kubeconfig() string {
	return g.GlobalOptions.Kubeconfig
}

// KubeContext returns the name of the kubectl context to use.
func (g *GlobalImpl) KubeContext() string {
	return g.GlobalOptions.KubeContext
}

// Namespace returns the namespace to use.
func (g *GlobalImpl) Namespace() string {
	return g.GlobalOptions.Namespace
}

// Chart returns the chart to use.
func (g *GlobalImpl) Chart() string {
	return g.GlobalOptions.Chart
}

// FileOrDir returns the path to the Helmfile.
func (g *GlobalImpl) FileOrDir() string {
	file := g.File
	if file == "" {
		file = os.Getenv(envvar.FilePath)
	}

	return file
}

// Selectors returns the selectors to use.
func (g *GlobalImpl) Selectors() []string {
	return g.Selector
}

// StateValuesSet returns the set
func (g *GlobalImpl) StateValuesSet() map[string]any {
	return g.set
}

// StateValuesSet returns the set
func (g *GlobalImpl) RawStateValuesSet() []string {
	return g.GlobalOptions.StateValuesSet
}

// RawStateValuesSetString returns the set
func (g *GlobalImpl) RawStateValuesSetString() []string {
	return g.StateValuesSetString
}

// StateValuesFiles returns the state values files
func (g *GlobalImpl) StateValuesFiles() []string {
	return g.StateValuesFile
}

// EnableLiveOutput return when to pipe the stdout and stderr from Helm live to the helmfile stdout
func (g *GlobalImpl) EnableLiveOutput() bool {
	return g.GlobalOptions.EnableLiveOutput
}

// SkipDeps return if running "helm repo update" and "helm dependency build" should be skipped
func (g *GlobalImpl) SkipDeps() bool {
	return g.GlobalOptions.SkipDeps
}

// SkipRefresh return if running "helm repo update"
func (g *GlobalImpl) SkipRefresh() bool {
	return g.GlobalOptions.SkipRefresh
}

// StripArgsValuesOnExitError return if the ARGS output on exit error should be suppressed
func (g *GlobalImpl) StripArgsValuesOnExitError() bool {
	return g.GlobalOptions.StripArgsValuesOnExitError
}

// DisableForceUpdate return when to disable forcing updates to repos upon adding
func (g *GlobalImpl) DisableForceUpdate() bool {
	return g.GlobalOptions.DisableForceUpdate
}

// Logger returns the logger
func (g *GlobalImpl) Logger() *zap.SugaredLogger {
	return g.logger
}

func (g *GlobalImpl) Color() bool {
	if c := g.GlobalOptions.Color; c {
		return c
	}

	if g.GlobalOptions.NoColor {
		return false
	}

	// We replicate the helm-diff behavior in helmfile
	// because when helmfile calls helm-diff, helm-diff has no access to term and therefore
	// we can't rely on helm-diff's ability to auto-detect term for color output.
	// See https://github.com/roboll/helmfile/issues/2043

	terminal := term.IsTerminal(int(os.Stdout.Fd()))
	// https://github.com/databus23/helm-diff/issues/281
	dumb := os.Getenv("TERM") == "dumb"
	return terminal && !dumb
}

// NoColor returns the no color flag
func (g *GlobalImpl) NoColor() bool {
	return g.GlobalOptions.NoColor
}

// Env returns the environment to use.
func (g *GlobalImpl) Env() string {
	var env string

	switch {
	case g.Environment != "":
		env = g.Environment
	case os.Getenv("HELMFILE_ENVIRONMENT") != "":
		env = os.Getenv("HELMFILE_ENVIRONMENT")
	default:
		env = state.DefaultEnv
	}
	return env
}

// ValidateConfig validates the global options.
func (g *GlobalImpl) ValidateConfig() error {
	if g.NoColor() && g.Color() {
		return errors.New("--color and --no-color cannot be specified at the same time")
	}
	return nil
}

// Interactive returns the Interactive
func (g *GlobalImpl) Interactive() bool {
	if g.GlobalOptions.Interactive {
		return true
	}
	return os.Getenv(envvar.Interactive) == "true"
}

// Args returns the args to use for helm
func (g *GlobalImpl) Args() string {
	args := g.GlobalOptions.Args
	enableHelmDebug := g.Debug

	if enableHelmDebug {
		args = fmt.Sprintf("%s %s", args, "--debug")
	}

	return args
}
