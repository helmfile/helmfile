package config

import (
	"github.com/spf13/cobra"

	"github.com/helmfile/helmfile/pkg/state"
)

// TestOptions is the options for the build command
type TestOptions struct {
	// Concurrency is the maximum number of concurrent helm processes to run, 0 is unlimited
	Concurrency int
	// SkipDeps is the skip deps flag
	SkipDeps bool
	// Cleanup is the cleanup flag
	Cleanup bool
	// Logs is the logs flagj
	Logs bool
	// Timeout is the timeout flag
	Timeout int
}

// NewTestOptions creates a new Apply
func NewTestOptions() *TestOptions {
	return &TestOptions{}
}

// TestImpl is impl for applyOptions
type TestImpl struct {
	*GlobalImpl
	*TestOptions
	Cmd *cobra.Command
}

// NewTestImpl creates a new TestImpl
func NewTestImpl(g *GlobalImpl, t *TestOptions) *TestImpl {
	return &TestImpl{
		GlobalImpl:  g,
		TestOptions: t,
	}
}

// Concurrency returns the concurrency
func (t *TestImpl) Concurrency() int {
	return t.TestOptions.Concurrency
}

// SkipDeps returns the skip deps
func (t *TestImpl) SkipDeps() bool {
	return t.TestOptions.SkipDeps
}

// Cleanup returns the cleanup
func (t *TestImpl) Cleanup() bool {
	return t.TestOptions.Cleanup
}

// Logs returns the logs
func (t *TestImpl) Logs() bool {
	return t.TestOptions.Logs
}

// Timeout returns the timeout
func (t *TestImpl) Timeout() int {
	if !t.Cmd.Flags().Changed("timeout") {
		return state.EmptyTimeout
	}
	return t.TestOptions.Timeout
}
