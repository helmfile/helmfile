package helmexec

import (
	"fmt"
	"strings"
)

func newExitError(path string, args []string, exitStatus int, err error, stderr, combined string, stripArgsValuesOnExitError bool) ExitError {
	var out string

	out += fmt.Sprintf("PATH:\n%s", Indent(path, "  "))

	out += "\n\nARGS:"
	for i, a := range args {
		if i > 0 && strings.HasPrefix(args[i-1], "--set") && stripArgsValuesOnExitError {
			a = "*** STRIP ***"
		}
		out += fmt.Sprintf("\n%s", Indent(fmt.Sprintf("%d: %s (%d bytes)", i, a, len(a)), "  "))
	}

	out += fmt.Sprintf("\n\nERROR:\n%s", Indent(err.Error(), "  "))

	out += fmt.Sprintf("\n\nEXIT STATUS\n%s", Indent(fmt.Sprintf("%d", exitStatus), "  "))

	if len(stderr) > 0 {
		out += fmt.Sprintf("\n\nSTDERR:\n%s", Indent(stderr, "  "))
	}

	if len(combined) > 0 {
		out += fmt.Sprintf("\n\nCOMBINED OUTPUT:\n%s", Indent(combined, "  "))
	}

	return ExitError{
		Message: fmt.Sprintf("command %q exited with non-zero status:\n\n%s", path, out),
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
