package config

import (
	"errors"
	"os"

	"go.uber.org/zap"
	"golang.org/x/term"

	"github.com/helmfile/helmfile/pkg/state"
)

// GlobalOptions is the global configuration for the Helmfile CLI.
type GlobalOptions struct {
	// helmBinary is the path to the Helm binary.
	HelmBinary string
	// File is the path to the Helmfile.
	File string
	// Environment is the name of the environment to use.
	Environment string
	// StateValuesSet is a list of state values to set on the command line.
	StateValuesSet []string
	// StateValuesFiles is a list of state values files to use.
	StateValuesFile []string
	// Quiet is true if the output should be quiet.
	Quiet bool
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
	set map[string]interface{}
}

// NewGlobalImpl creates a new GlobalImpl.
func NewGlobalImpl(opts *GlobalOptions) *GlobalImpl {
	return &GlobalImpl{
		GlobalOptions: opts,
		set:           make(map[string]interface{}),
	}
}

// Setset sets the set
func (g *GlobalImpl) SetSet(set map[string]interface{}) {
	g.set = set
}

// HelmBinary returns the path to the Helm binary.
func (g *GlobalImpl) HelmBinary() string {
	return g.GlobalOptions.HelmBinary
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
	return g.GlobalOptions.File
}

// Selectors returns the selectors to use.
func (g *GlobalImpl) Selectors() []string {
	return g.GlobalOptions.Selector
}

// StateValuesSet returns the set
func (g *GlobalImpl) StateValuesSet() map[string]interface{} {
	return g.set
}

// StateValuesSet returns the set
func (g *GlobalImpl) RawStateValuesSet() []string {
	return g.GlobalOptions.StateValuesSet
}

// StateValuesFiles returns the state values files
func (g *GlobalImpl) StateValuesFiles() []string {
	return g.GlobalOptions.StateValuesFile
}

// EnableLiveOutput return when to pipe the stdout and stderr from Helm live to the helmfile stdout
func (g *GlobalImpl) EnableLiveOutput() bool {
	return g.GlobalOptions.EnableLiveOutput
}

// Logger returns the logger
func (g *GlobalImpl) Logger() *zap.SugaredLogger {
	return g.GlobalOptions.logger
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
	case g.GlobalOptions.Environment != "":
		env = g.GlobalOptions.Environment
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
