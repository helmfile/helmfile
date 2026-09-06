package helmexec

import "strings"

// RedactionProfile selects how aggressively RedactArgs masks secret-bearing
// command-line arguments.
type RedactionProfile int

const (
	// RedactionLegacy reproduces the historical exit-error behavior
	// byte-for-byte: only the argument *following* a flag whose name starts
	// with "--set" is masked. The goldens in exit_error_test.go pin this
	// output; changing it changes observable error messages.
	RedactionLegacy RedactionProfile = iota

	// RedactionStrict masks every secret-bearing argument form known today:
	// in addition to the legacy behavior it covers single-argument forms
	// (--set=key=value) and credential flags (--username, --password,
	// --key-file). Used for telemetry span attributes; span visibility must
	// be at least as redacted as error messages.
	RedactionStrict
)

// redactedArg is the placeholder written in place of secret values; its exact
// bytes are pinned by exit_error_test.go.
const redactedArg = "*** STRIP ***"

// strictNextArgFlags are the flags whose following argument is a secret.
var strictNextArgFlags = []string{
	"--set",
	"--set-string",
	"--set-file",
	"--set-json",
	"--set-literal",
	"--username",
	"--password",
	"--key-file",
}

// RedactArgs returns a copy of args with secret-bearing values masked
// according to profile. The input slice is never mutated.
func RedactArgs(args []string, profile RedactionProfile) []string {
	if len(args) == 0 {
		return args
	}

	out := make([]string, len(args))
	copy(out, args)

	for i := range out {
		var prev string
		if i > 0 {
			prev = out[i-1]
		}

		switch profile {
		case RedactionLegacy:
			if strings.HasPrefix(prev, "--set") {
				out[i] = redactedArg
			}
		case RedactionStrict:
			if isStrictNextArgFlag(prev) {
				out[i] = redactedArg
			} else if flag := strictInlineFlag(out[i]); flag != "" {
				out[i] = flag + "=" + redactedArg
			}
		}
	}
	return out
}

func isStrictNextArgFlag(arg string) bool {
	for _, flag := range strictNextArgFlags {
		if arg == flag {
			return true
		}
	}
	return false
}

// strictInlineFlag returns the flag name when arg is a single-argument secret
// form such as "--set=key=value", or "" otherwise.
func strictInlineFlag(arg string) string {
	for _, flag := range strictNextArgFlags {
		if strings.HasPrefix(arg, flag+"=") {
			return flag
		}
	}
	return ""
}

func equalArgs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
