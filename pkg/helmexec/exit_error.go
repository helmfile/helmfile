package helmexec

import (
	"fmt"
	"strings"
)

func newExitError(path string, args []string, exitStatus int, err error, stderr, combined string, stripArgsValuesOnExitError bool) ExitError {
	var out strings.Builder

	fmt.Fprintf(&out, "PATH:\n%s", Indent(path, "  "))

	out.WriteString("\n\nARGS:")
	// The legacy profile is byte-identical to the historical inline logic;
	// the goldens in exit_error_test.go pin its exact output.
	redacted := RedactArgs(args, RedactionLegacy)
	if !stripArgsValuesOnExitError {
		redacted = args
	}
	for i, a := range redacted {
		fmt.Fprintf(&out, "\n%s", Indent(fmt.Sprintf("%d: %s (%d bytes)", i, a, len(a)), "  "))
	}

	fmt.Fprintf(&out, "\n\nERROR:\n%s", Indent(err.Error(), "  "))

	fmt.Fprintf(&out, "\n\nEXIT STATUS\n%s", Indent(fmt.Sprintf("%d", exitStatus), "  "))

	if len(stderr) > 0 {
		fmt.Fprintf(&out, "\n\nSTDERR:\n%s", Indent(stderr, "  "))
	}

	if len(combined) > 0 {
		fmt.Fprintf(&out, "\n\nCOMBINED OUTPUT:\n%s", Indent(combined, "  "))
	}

	return ExitError{
		Message: fmt.Sprintf("command %q exited with non-zero status:\n\n%s", path, out.String()),
		Code:    exitStatus,
	}
}

// indents a block of text with an indent string
func Indent(text, indent string) string {
	var b strings.Builder

	b.Grow(len(text) * 2)

	lines := strings.Split(text, "\n")

	last := len(lines) - 1

	for i, j := range lines {
		if i > 0 && i < last && j != "" {
			b.WriteString("\n")
		}

		if j != "" {
			b.WriteString(indent + j)
		}
	}

	return b.String()
}

// ExitError is created whenever your shell command exits with a non-zero exit status
type ExitError struct {
	Message string
	Code    int
}

func (e ExitError) Error() string {
	return e.Message
}

func (e ExitError) ExitStatus() int {
	return e.Code
}
