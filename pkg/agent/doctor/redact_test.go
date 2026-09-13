package doctor

import (
	"strings"
	"testing"
)

func TestSecretRedactor_LeavesNonSecretDiffAlone(t *testing.T) {
	in := `  default, my-release, Deployment (apps) has changed:
  - spec.replicas: 1
  + spec.replicas: 3
`
	out, n := NewSecretRedactor().Redact(in)
	if out != in {
		t.Errorf("expected no change; got:\n%s", out)
	}
	if n != 0 {
		t.Errorf("replacement count = %d, want 0", n)
	}
}

func TestSecretRedactor_PreservesHelmDiffPlaceholder(t *testing.T) {
	// helm-diff with ShowSecrets=false already emits "<REDACTED>"; we must
	// not double-redact it (no count inflation, output stable).
	in := `  default, my-release, Secret (v1) has changed:
  - data.password: <REDACTED>
  + data.password: <REDACTED>
`
	out, n := NewSecretRedactor().Redact(in)
	if out != in {
		t.Errorf("expected output unchanged; got:\n%s", out)
	}
	if n != 0 {
		t.Errorf("replacement count = %d, want 0 (already redacted)", n)
	}
}

func TestSecretRedactor_CatchesLeakedSensitiveKeyValue(t *testing.T) {
	// User accidentally passed --show-secrets; the wrapper forces false but
	// if helm-diff still leaks (bug/hook), the text redactor must catch it.
	in := `  default, my-release, Secret (v1) has changed:
  - data.password: SGVsbG8=
  + data.password: bmV3cGFzcw==
`
	out, n := NewSecretRedactor().Redact(in)
	if strings.Contains(out, "SGVsbG8=") || strings.Contains(out, "bmV3cGFzcw==") {
		t.Errorf("secret value leaked into output:\n%s", out)
	}
	if n == 0 {
		t.Errorf("expected replacements, got 0")
	}
	// Both lines should be redacted.
	if c := strings.Count(out, RedactedPlaceholder); c < 2 {
		t.Errorf("expected >=2 placeholders, got %d in:\n%s", c, out)
	}
}

func TestSecretRedactor_CatchesFullSecretBlock(t *testing.T) {
	// Full Secret YAML as emitted by `helm template` style diffs.
	in := `+ # Source: mychart/templates/secret.yaml
+ apiVersion: v1
+ kind: Secret
+ metadata:
+   name: my-secret
+ data:
+   password: cGFzc3dvcmQxMjM=
+   tls.crt: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0t
+ stringData:
+   config.yaml: |
+     database_url: postgres://user:supersecret@db:5432
`
	out, n := NewSecretRedactor().Redact(in)
	for _, leaked := range []string{"cGFzc3dvcmQxMjM=", "LS0tLS1CRUdJTi", "supersecret"} {
		if strings.Contains(out, leaked) {
			t.Errorf("leaked %q into output:\n%s", leaked, out)
		}
	}
	if n == 0 {
		t.Errorf("expected replacements, got 0")
	}
}

func TestSecretRedactor_CatchesFreeFormLongBase64(t *testing.T) {
	// Catch a base64-looking token that is not inside a Secret resource and
	// not under a sensitive key (e.g. logged by a hook into a ConfigMap).
	in := `  + annotations:
  +   custom.io/signed-token: eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.signaturepartgoesheremorethan40charsXX`
	out, n := NewSecretRedactor().Redact(in)
	if strings.Contains(out, "eyJhbGciOi") {
		t.Errorf("JWT-shaped token leaked:\n%s", out)
	}
	if n == 0 {
		t.Errorf("expected replacements, got 0")
	}
}

func TestSecretRedactor_DoesNotFlagShortValues(t *testing.T) {
	// 10-char strings, resource versions, short labels — must not be flagged.
	in := `  - metadata.resourceVersion: 12345
  + metadata.resourceVersion: 67890
  - spec.template.metadata.labels.app: short
  + spec.template.metadata.labels.app: other
`
	out, n := NewSecretRedactor().Redact(in)
	if out != in {
		t.Errorf("short values should not be flagged; got:\n%s", out)
	}
	if n != 0 {
		t.Errorf("replacement count = %d, want 0", n)
	}
}

func TestSecretRedactor_StripsANSIBeforeComparing(t *testing.T) {
	// Color-coded helm diff output where secret was already redacted.
	// helm-diff may emit something like "\x1b[31m<REDACTED>\x1b[0m" — we must
	// recognize it as already redacted and not double-count.
	in := "  - data.password: \x1b[31m<REDACTED>\x1b[0m\n  + data.password: \x1b[32m<REDACTED>\x1b[0m\n"
	out, n := NewSecretRedactor().Redact(in)
	if n != 0 {
		t.Errorf("expected 0 replacements on colorized already-redacted; got %d", n)
	}
	if out != in {
		t.Errorf("expected input preserved; got:\n%s", out)
	}
}

func TestSecretRedactor_HonorsCustomPlaceholder(t *testing.T) {
	r := SecretRedactor{Placeholder: "***"}
	in := "  - data.password: SGVsbG8=\n"
	out, n := r.Redact(in)
	if !strings.Contains(out, "***") {
		t.Errorf("custom placeholder missing in:\n%s", out)
	}
	if strings.Contains(out, "SGVsbG8=") {
		t.Errorf("original value leaked in:\n%s", out)
	}
	if n == 0 {
		t.Errorf("expected replacements, got 0")
	}
}

func TestSecretRedactor_HandlesSensitiveKeyVariations(t *testing.T) {
	// Different naming conventions all need to be caught.
	cases := []string{
		"  - data.api_key: abc==\n",
		"  - data.api-key: abc==\n",
		"  - data.apiKey: abc==\n",
		"  - data.API_KEY: abc==\n",
		"  - data.client_secret: xyz==\n",
		"  - values.refresh-token: r==\n",
		"  - config.bearer: b==\n",
	}
	r := NewSecretRedactor()
	for _, in := range cases {
		out, _ := r.Redact(in)
		if strings.Contains(out, "abc==") || strings.Contains(out, "xyz==") || strings.Contains(out, "r==") || strings.Contains(out, "b==") {
			t.Errorf("sensitive key not redacted for input %q; got %q", in, out)
		}
	}
}

// TestSecretRedactor_RealisticMixedDiff is a regression guard against the
// state-machine rewrite of redactSecretBlocks. The earlier regex-based
// implementation leaked "supersecret" inside a Secret resource block when the
// diff also contained unrelated Secret/ConfigMap sections. We reproduce that
// exact input here so the bug cannot come back.
func TestSecretRedactor_RealisticMixedDiff(t *testing.T) {
	in := `  default, my-release, Secret (v1) has changed:
  - data.password: SGVsbG8=
  + data.password: cGFzc3dvcmQxMjM=
  default, api-config, ConfigMap (v1) has changed:
  - data.api_key: abcdef==
  + data.api_key: bmV3a2V5
  + data.apiKey: verylongbase64stringthatislongerthan40charsXXXXXXXXXXXXXXX
  - data.client_secret: short
  + data.client_secret: NEW
  + annotations:
  +   cluster.x-k8s.io/signed-token: eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjMifQ.signaturepart
  + # Source: mychart/templates/secret.yaml
  + apiVersion: v1
  + kind: Secret
  + metadata:
  +   name: db-secret
  + stringData:
  +   url: postgres://user:supersecret@db:5432
`
	out, n := NewSecretRedactor().Redact(in)

	// The known-bug sentinel must be gone.
	if strings.Contains(out, "supersecret") {
		t.Errorf("supersecret leaked into output:\n%s", out)
	}
	// Every other sensitive marker must also be gone.
	for _, leak := range []string{"SGVsbG8=", "cGFzc3dvcmQxMjM=", "bmV3a2V5", "eyJhbGc"} {
		if strings.Contains(out, leak) {
			t.Errorf("secret value %q leaked into output:\n%s", leak, out)
		}
	}
	if n == 0 {
		t.Errorf("expected replacements, got 0")
	}
	// Sanity: the structure marker `kind: Secret` survives so the LLM still
	// knows a Secret resource was involved.
	if !strings.Contains(out, "kind: Secret") {
		t.Errorf("expected `kind: Secret` to survive redaction; got:\n%s", out)
	}
}
