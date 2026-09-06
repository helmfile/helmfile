package helmexec

import (
	"net/url"
	"strings"
)

// redactedRefValue is the placeholder substituted for credential-bearing
// userinfo and query-parameter values in telemetry attributes.
const redactedRefValue = "xxxxx"

// sensitiveQueryKeys reports whether a query-parameter key is considered
// credential-bearing, mirroring the heuristic used by pkg/remote for cache
// keys (token/password/secret/key/signature substrings).
func sensitiveQueryKey(key string) bool {
	lk := strings.ToLower(key)
	return strings.Contains(lk, "token") || strings.Contains(lk, "password") ||
		strings.Contains(lk, "secret") || strings.Contains(lk, "key") ||
		strings.Contains(lk, "signature")
}

// RedactedRef sanitizes a remote reference (state-file path, chart reference,
// repository URL) for telemetry export: go-getter forced-form prefixes
// (git::, s3::, …) are preserved, URL userinfo is masked, and
// credential-bearing query parameters have their values replaced. Non-URL
// strings are returned unchanged. It is deliberately at least as strict as
// the log-time RedactedURL.
// maxForcedFormPrefix bounds the length of a recognized go-getter forced-form
// prefix ("git::", "s3::", "hg::" …); longer "::"-containing prefixes are
// treated as part of an opaque reference instead.
const maxForcedFormPrefix = 16

func RedactedRef(ref string) string {
	force := ""
	rest := ref
	// A go-getter forced form looks like "git::https://…": a short
	// alphanumeric prefix followed by "::" at the start of the reference.
	if i := strings.Index(rest, "::"); i > 0 && i <= maxForcedFormPrefix && isAlphanumericPrefix(rest[:i]) {
		force = rest[:i+len("::")]
		rest = rest[i+len("::"):]
	}

	u, err := url.Parse(rest)
	if err != nil {
		if strings.Contains(rest, "://") {
			// URL-like but malformed (e.g. a bad percent escape): fail
			// closed — export a fully redacted value rather than risk
			// leaking userinfo or query credentials.
			return force + redactedRefValue
		}
		return ref
	}
	if u.Scheme == "" || u.Host == "" {
		// Not a URL (e.g. "./charts/demo"); leave it alone.
		return ref
	}

	// Mask the whole userinfo: usernames are as likely to carry tokens as
	// passwords are (e.g. "https://x-access-token@host").
	if u.User != nil {
		u.User = url.User(redactedRefValue)
	}

	if u.RawQuery != "" {
		q, err := url.ParseQuery(u.RawQuery)
		if err != nil {
			// Malformed query: fail closed by dropping it entirely.
			u.RawQuery = ""
		} else {
			for key, values := range q {
				if sensitiveQueryKey(key) {
					for i := range values {
						values[i] = redactedRefValue
					}
				}
			}
			u.RawQuery = q.Encode()
		}
	}

	return force + u.String()
}

func isAlphanumericPrefix(s string) bool {
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			return false
		}
	}
	return len(s) > 0
}
