package doctor

import (
	"regexp"
	"strings"
)

// RedactedPlaceholder is the default string substituted in place of detected
// secret values before the diff is sent to the LLM.
const RedactedPlaceholder = "<REDACTED>"

// SecretRedactor strips secret-looking content from helm diff output before
// it is sent to an LLM. Doctor ALWAYS applies this, regardless of helm-diff
// flags: if a user passed --show-secrets by mistake, doctor must still
// guarantee no secret reaches a third-party LLM endpoint.
//
// The redactor is defense-in-depth. Doctor first wraps the DiffConfigProvider
// to force ShowSecrets()=false, which makes helm-diff itself redact secrets
// with "<REDACTED>" placeholders. This redactor then catches residual leaks:
//
//   - chart hooks echoing secrets into stdout,
//   - mislabeled Secret resources,
//   - helm-diff plugin bugs,
//   - post-renderers that re-inject secrets.
//
// Tuning policy: prefer false positives over false negatives. A garbled diff
// is annoying; a leaked production TLS private key is an incident. Reviewers
// can always pass --suppress-secrets to drop Secret resources entirely.
type SecretRedactor struct {
	// Placeholder replaces detected secret values. Defaults to "<REDACTED>".
	Placeholder string
}

// NewSecretRedactor returns a SecretRedactor with the default placeholder.
func NewSecretRedactor() SecretRedactor {
	return SecretRedactor{Placeholder: RedactedPlaceholder}
}

// Redact applies all redaction patterns to diff and returns the sanitized
// text plus a count of replacements made. The count is surfaced in the doctor
// report footer so users can spot unexpected redaction (e.g. when a chart
// smuggled secrets into a non-Secret resource).
func (r SecretRedactor) Redact(diff string) (string, int) {
	ph := r.Placeholder
	if ph == "" {
		ph = RedactedPlaceholder
	}

	count := 0
	out := diff

	// Pattern 1: Secret resource blocks via state machine (state-based, not
	// regex, because helm-diff YAML has +/- prefixes and chart-specific nesting).
	out, n := redactSecretBlocks(out, ph)
	count += n

	// Pattern 2: Sensitive key/value lines (password, token, apiKey, ...).
	out, n = redactSensitiveKeyValues(out, ph)
	count += n

	// Pattern 3: Free-form long base64 (>=40 chars) and JWT-shaped tokens.
	out, n = redactLongBase64(out, ph)
	count += n

	return out, count
}

// --- pattern implementations ------------------------------------------------

// redactSecretBlocks walks the diff line by line, finds each `kind: Secret`
// resource block, and replaces the value portion of every `key: value` line
// under its `data:` or `stringData:` section. Keys are preserved so the LLM
// still sees *what* changed (e.g. "password was updated"), just not the value.
//
// State transitions:
//
//	inSecret=false → on "kind: Secret" line → inSecret=true, inData=false
//	inSecret=true  → on "kind:" or "apiVersion:" (new resource start) → inSecret=false
//	inSecret=true  → on "data:" or "stringData:" line → inData=true
//	inData=true    → on non-indented line → inData=false
//	inData=true    → on "  key: value" line → redact value
func redactSecretBlocks(diff, ph string) (string, int) {
	lines := strings.Split(diff, "\n")
	inSecret := false
	inData := false
	count := 0

	for i, raw := range lines {
		// Strip sign prefix and leading whitespace to inspect the YAML shape.
		// helm-diff prefixes each line with `+`, `-`, or space; we ignore it
		// for shape detection but keep it when rewriting the line.
		stripped := strings.TrimLeft(strings.TrimSpace(raw), "+-")
		stripped = strings.TrimSpace(stripped)
		if stripped == "" {
			continue
		}

		// Detect start of a Secret resource. hasSecretKind filters out
		// SecretList, SecretProviderClass, etc.
		if strings.HasPrefix(stripped, "kind:") && hasSecretKind(stripped) {
			inSecret = true
			inData = false
			continue
		}

		// Detect start of any other resource — ends the previous Secret.
		if inSecret && (strings.HasPrefix(stripped, "kind:") || strings.HasPrefix(stripped, "apiVersion:")) {
			inSecret = false
			inData = false
			continue
		}

		if !inSecret {
			continue
		}

		// Detect data: / stringData: section headers.
		if stripped == "data:" || stripped == "stringData:" {
			inData = true
			continue
		}

		if !inData {
			continue
		}

		// Inside a data: block. Detect when we leave it (less-indented line).
		if !indentedAfterSign(raw) {
			inData = false
			continue
		}

		// Redact "  key: value" lines. Keep the key, drop the value.
		// Multi-line scalar headers (|, >) are left for the long-base64 pass
		// to mop up their body lines.
		redacted, didRedact := redactKeyValueLine(raw, ph)
		if didRedact {
			lines[i] = redacted
			count++
		}
	}

	return strings.Join(lines, "\n"), count
}

// hasSecretKind reports whether stripped (e.g. "kind: Secret") refers to a
// Kubernetes Secret resource. We accept `Secret`, optionally followed by
// trailing comment or whitespace, but not `SecretList` or `SecretProviderClass`.
func hasSecretKind(s string) bool {
	rest := strings.TrimSpace(strings.TrimPrefix(s, "kind:"))
	rest = strings.Trim(rest, `"'`)
	// Drop trailing comments / whitespace.
	if spaceIdx := strings.IndexByte(rest, ' '); spaceIdx >= 0 {
		rest = rest[:spaceIdx]
	}
	if hashIdx := strings.IndexByte(rest, '#'); hashIdx >= 0 {
		rest = rest[:hashIdx]
		rest = strings.TrimSpace(rest)
	}
	return rest == "Secret"
}

// indentedAfterSign reports whether a diff line has indentation after the
// leading `+`/`-` sign. Used to tell YAML continuation lines from new keys.
//
//	"+   key: val"  → true
//	"+key: val"     → false (top-level YAML key)
//	"  - data.x: 1" → true
//	"- # comment"   → true (comments count as continuation for our purposes)
func indentedAfterSign(line string) bool {
	s := line
	// Strip leading sign chars.
	for len(s) > 0 && (s[0] == '+' || s[0] == '-') {
		s = s[1:]
	}
	// Strip leading spaces.
	return len(s) > 0 && (s[0] == ' ' || s[0] == '\t')
}

// redactKeyValueLine replaces the value portion of "  key: value" with ph.
// Returns (newLine, true) when a redaction happened, (orig, false) otherwise.
//
// Lines that are pure comments, blank, or multi-line scalar headers (|, >)
// without an inline value are left alone; the caller's broader long-base64
// pass will catch any encoded body that follows.
//
// ANSI color escapes are stripped before comparison so that an already-redacted
// line emitted by helm-diff in TTY mode (e.g. "\x1b[31m<REDACTED>\x1b[0m") is
// recognized and not double-counted.
func redactKeyValueLine(line, ph string) (string, bool) {
	// Find the first ": " separator. We don't use a regex here because the
	// line may contain colons inside the value (e.g. URLs like
	// "postgres://user:pass@host:5432").
	idx := strings.Index(line, ": ")
	if idx < 0 {
		return line, false
	}
	prefix := line[:idx+2]
	rawValue := strings.TrimSpace(line[idx+2:])
	value := stripANSI(rawValue)

	// Skip lines already redacted by helm-diff itself.
	if value == ph || value == "***" || value == "<redacted>" {
		return line, false
	}
	// Skip empty values and multi-line scalar headers.
	if value == "" || value == "|" || value == ">" || value == "|-" || value == ">-" {
		return line, false
	}
	return prefix + ph, true
}

// reSensitiveKeyValue matches compact helm-diff lines like:
//
//   - data.password: SGVsbG8=
//   - configuration.apiKey: bmV3a2V5
//     values.secret.token: xyz
//
// where the last path segment is a known-sensitive name. Case-insensitive so
// it catches apiKey / API_KEY / api_key uniformly.
//
// Groups:
//
//  1. prefix including the key
//  2. ": "
//  3. the value
var reSensitiveKeyValue = regexp.MustCompile(
	`(?im)^([+-]?\s+(?:[^:\n]*\.)?(?:password|passwd|pwd|secret|secrets|token|tokens|apikey|api[_-]?key|private[_-]?key|client[_-]?secret|access[_-]?token|refresh[_-]?token|bearer|credential|credentials|auth[_-]?token|session[_-]?token)(?:\.[A-Za-z_]+)?)(:\s*)(.+)$`,
)

func redactSensitiveKeyValues(diff, ph string) (string, int) {
	count := 0
	out := reSensitiveKeyValue.ReplaceAllStringFunc(diff, func(line string) string {
		m := reSensitiveKeyValue.FindStringSubmatch(line)
		if m == nil {
			return line
		}
		val := stripANSI(m[3])
		if val == ph || val == "***" || val == "<redacted>" {
			return line
		}
		count++
		return m[1] + m[2] + ph
	})
	return out, count
}

// reLongBase64 matches runs of base64 alphabet characters of length >= 40.
// 40 is chosen because:
//   - typical encoded secrets (16-byte token, 32-byte key) base64 to 24/44 chars,
//   - SHA-1 hashes hex-encoded are 40 chars,
//   - short base64 (e.g. resource versions) are usually < 20 chars and left alone.
//
// We accept the standard base64 alphabet `[A-Za-z0-9+/=]` plus the URL-safe
// variant `[A-Za-z0-9-_=]` since many tools emit JWTs and modern tokens in
// URL-safe form.
var reLongBase64 = regexp.MustCompile(`[A-Za-z0-9+/=_-]{40,}`)

// reJWT matches a signed JWT (header.payload.signature in URL-safe base64).
// JWTs start with "eyJ" (base64-encoded `{"`) and are joined by dots. Each
// segment is typically 8+ chars. Caught separately because the dots break
// reLongBase64.
var reJWT = regexp.MustCompile(`eyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}`)

func redactLongBase64(diff, ph string) (string, int) {
	count := 0
	// JWTs first (they contain dots so reLongBase64 would only catch segments).
	out := reJWT.ReplaceAllStringFunc(diff, func(s string) string {
		if s == ph {
			return s
		}
		count++
		return ph
	})
	// Then free-form long base64.
	out = reLongBase64.ReplaceAllStringFunc(out, func(s string) string {
		if s == ph {
			return s
		}
		count++
		return ph
	})
	return out, count
}

// reANSI matches ANSI color escape sequences used by helm-diff when stdout
// is a TTY. We strip them before comparing values so a colorized
// "*** REDACTED ***" is still recognized as already-redacted.
var reANSI = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string {
	return reANSI.ReplaceAllString(s, "")
}
