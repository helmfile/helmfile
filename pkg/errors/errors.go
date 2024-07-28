/*
* MIT License
*
* Copyright (c) 2022 urfave/cli maintainers
*
* Permission is hereby granted, free of charge, to any person obtaining a copy
* of this software and associated documentation files (the "Software"), to deal
* in the Software without restriction, including without limitation the rights
* to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
* copies of the Software, and to permit persons to whom the Software is
* furnished to do so, subject to the following conditions:
*
* The above copyright notice and this permission notice shall be included in all
* copies or substantial portions of the Software.
*
* THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
* IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
* FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
* AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
* LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
* OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
* SOFTWARE.
 */

package errors

import (
	"fmt"
	"io"
	"os"
)

// OsExiter is the function used when the app exits. If not set defaults to os.Exit.
var OsExiter = os.Exit

// ErrWriter is used to write errors to the user. This can be anything
// implementing the io.Writer interface and defaults to os.Stderr.
var ErrWriter io.Writer = os.Stderr

type ErrorFormatter interface {
	Format(s fmt.State, verb rune)
}

// ExitCoder is the interface checked by `App` and `Command` for a custom exit
// code
type ExitCoder interface {
	error
	ExitCode() int
}

// ExitError fulfills both the builtin `error` interface and `ExitCoder`
type ExitError struct {
	exitCode int
	message  any
}

// NewExitError makes a new *ExitError
func NewExitError(message any, exitCode int) *ExitError {
	return &ExitError{
		exitCode: exitCode,
		message:  message,
	}
}

// Error returns the string message, fulfilling the interface required by
// `error`
func (ee *ExitError) Error() string {
	return fmt.Sprintf("%v", ee.message)
}

// ExitCode returns the exit code, fulfilling the interface required by
// `ExitCoder`
func (ee *ExitError) ExitCode() int {
	return ee.exitCode
}

// HandleExitCoder checks if the error fulfills the ExitCoder interface, and if
// so prints the error to stderr (if it is non-empty) and calls OsExiter with the
// given exit code.  If the given error is a MultiError, then this func is
// called on all members of the Errors slice and calls OsExiter with the last exit code.
func HandleExitCoder(err error) {
	if err == nil {
		return
	}

	if exitErr, ok := err.(ExitCoder); ok {
		if errMsg := err.Error(); errMsg != "" {
			fmt.Fprintln(ErrWriter, errMsg)
		}
		OsExiter(exitErr.ExitCode())
	} else {
		fmt.Fprintln(ErrWriter, err)
		OsExiter(3)
	}
}
