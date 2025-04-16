package config

import (
	"io"

	"go.uber.org/zap"
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
