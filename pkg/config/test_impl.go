package config

import (
	"github.com/spf13/cobra"

	"github.com/helmfile/helmfile/pkg/state"
)

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
