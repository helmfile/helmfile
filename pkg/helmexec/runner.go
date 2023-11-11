package helmexec

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"go.uber.org/zap"

	"github.com/helmfile/helmfile/pkg/envvar"
)

// Runner interface for shell commands
type Runner interface {
	Execute(cmd string, args []string, env map[string]string, enableLiveOutput bool) ([]byte, error)
	ExecuteStdIn(cmd string, args []string, env map[string]string, stdin io.Reader) ([]byte, error)
}

// ShellRunner implemention for shell commands
type ShellRunner struct {
	Dir string

	StripArgsValuesOnExitError bool

	Logger *zap.SugaredLogger
	Ctx    context.Context
}

// Execute a shell command
func (shell ShellRunner) Execute(cmd string, args []string, env map[string]string, enableLiveOutput bool) ([]byte, error) {
	preparedCmd := exec.Command(cmd, args...)
	preparedCmd.Dir = shell.Dir
	preparedCmd.Env = mergeEnv(os.Environ(), env)

	if !enableLiveOutput {
		return Output(shell.Ctx, preparedCmd, shell.StripArgsValuesOnExitError, &logWriterGenerator{
			log: shell.Logger,
		})
	} else {
		return LiveOutput(shell.Ctx, preparedCmd, shell.StripArgsValuesOnExitError, os.Stdout)
	}
}

// Execute a shell command
func (shell ShellRunner) ExecuteStdIn(cmd string, args []string, env map[string]string, stdin io.Reader) ([]byte, error) {
	preparedCmd := exec.Command(cmd, args...)
	preparedCmd.Dir = shell.Dir
	preparedCmd.Env = mergeEnv(os.Environ(), env)
	preparedCmd.Stdin = stdin
	return Output(shell.Ctx, preparedCmd, shell.StripArgsValuesOnExitError, &logWriterGenerator{
		log: shell.Logger,
	})
}

func Output(ctx context.Context, c *exec.Cmd, stripArgsValuesOnExitError bool, logWriterGenerators ...*logWriterGenerator) ([]byte, error) {
	if c.Stdout != nil {
		return nil, errors.New("exec: Stdout already set")
	}
	if c.Stderr != nil {
		return nil, errors.New("exec: Stderr already set")
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	var combined bytes.Buffer

	var logWriters []io.Writer

	var id string
	if os.Getenv(envvar.DisableRunnerUniqueID) == "" {
		id = newExecutionID()
	}
	path := filepath.Base(c.Path)
	for _, g := range logWriterGenerators {
		var logPrefix string
		if id == "" {
			logPrefix = fmt.Sprintf("%s> ", path)
		} else {
			logPrefix = fmt.Sprintf("%s:%s> ", path, id)
		}
		logWriters = append(logWriters, g.Writer(logPrefix))
	}

	c.Stdout = io.MultiWriter(append([]io.Writer{&stdout, &combined}, logWriters...)...)
	c.Stderr = io.MultiWriter(append([]io.Writer{&stderr, &combined}, logWriters...)...)

	var err error
	ch := make(chan error)
	go func() {
		ch <- c.Run()
	}()
	select {
	case err = <-ch:
	case <-ctx.Done():
		_ = c.Process.Signal(os.Interrupt)
		err = <-ch
	}

	if err != nil {
		// TrimSpace is necessary, because otherwise helmfile prints the redundant new-lines after each error like:
		//
		//   err: release "envoy2" in "helmfile.yaml" failed: exit status 1: Error: could not find a ready tiller pod
		//   <redundant new line!>
		//   err: release "envoy" in "helmfile.yaml" failed: exit status 1: Error: could not find a ready tiller pod
		switch ee := err.(type) {
		case *exec.ExitError:
			// Propagate any non-zero exit status from the external command, rather than throwing it away,
			// so that helmfile could return its own exit code accordingly
			waitStatus := ee.Sys().(syscall.WaitStatus)
			exitStatus := waitStatus.ExitStatus()
			err = newExitError(c.Path, c.Args, exitStatus, ee, stderr.String(), combined.String(), stripArgsValuesOnExitError)
		default:
			panic(fmt.Sprintf("unexpected error: %v", err))
		}
	}

	return stdout.Bytes(), err
}

func LiveOutput(ctx context.Context, c *exec.Cmd, stripArgsValuesOnExitError bool, stdout io.Writer) ([]byte, error) {
	reader, writer := io.Pipe()
	ch := make(chan error)
	doneCh := make(chan struct{})

	go func() {
		defer close(doneCh)
		reader := bufio.NewReader(reader)

		for {
			// prefer readstring over scanner to handle potential large output
			// https://stackoverflow.com/a/29444042
			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					break
				}
				ch <- err
			}

			line = strings.TrimSuffix(line, "\n")
			fmt.Fprintln(stdout, line)
		}
	}()

	c.Stdout = writer
	c.Stderr = writer
	err := c.Start()

	if err == nil {
		go func() {
			ch <- c.Wait()
		}()

		select {
		case err = <-ch:
		case <-ctx.Done():
			_ = c.Process.Signal(os.Interrupt)
			err = <-ch
		}
		_ = writer.Close()
		<-doneCh
	}

	if err != nil {
		switch ee := err.(type) {
		case *exec.ExitError:
			// Propagate any non-zero exit status from the external command, rather than throwing it away,
			// so that helmfile could return its own exit code accordingly
			waitStatus := ee.Sys().(syscall.WaitStatus)
			exitStatus := waitStatus.ExitStatus()
			err = newExitError(c.Path, c.Args, exitStatus, ee, "", "", stripArgsValuesOnExitError)
		default:
			panic(fmt.Sprintf("unexpected error: %v", err))
		}
	}

	return nil, err
}

func mergeEnv(orig []string, new map[string]string) []string {
	wanted := env2map(orig)
	for k, v := range new {
		wanted[k] = v
	}
	return map2env(wanted)
}

func map2env(wanted map[string]string) []string {
	result := []string{}
	for k, v := range wanted {
		result = append(result, k+"="+v)
	}
	return result
}

func env2map(env []string) map[string]string {
	wanted := map[string]string{}
	for _, cur := range env {
		pair := strings.SplitN(cur, "=", 2)

		var v string

		// An environment can completely miss `=` and the right side.
		// If we didn't deal with that, this may fail due to an index-out-of-range error
		if len(pair) > 1 {
			v = pair[1]
		}

		wanted[pair[0]] = v
	}
	return wanted
}
