package llm

import (
	"encoding/json"
	"fmt"
	"strings"
)

// systemPrompt returns the system message that frames the model as a
// Kubernetes/Helm reviewer and locks the output to a known JSON schema.
func systemPrompt() string {
	return strings.TrimSpace(`You are a senior Kubernetes and Helm reviewer.

You will be given the output of "helm diff" produced by helmfile. Your job is to:

1. Summarize what the diff actually changes, in one or two sentences a human operator can scan in under five seconds.
2. Identify risks across these categories (skip categories that do not apply):
   - data-loss: persistent volumes, PVCs, databases, or stateful workloads being deleted or recreated.
   - security: new privileges, host networking, host paths, removed network policies, plaintext secrets.
   - breaking-change: removed or renamed values, dropped labels/annotations relied on by other tools, apiVersion downgrades.
   - downtime: service disruptions, rolling-update storms, missing PodDisruptionBudget, missing readiness gates.
   - performance: huge resource requests/limits, removed HPA, expensive sidecars.
   - best-practice: missing namespace, hardcoded images, misaligned labels.
3. For each risk, give an actionable mitigation step (kubectl command, values override, or rollback plan).

Risk levels:
   - high: would cause data loss, security exposure, or an outage. Must be reviewed by a human before applying.
   - medium: likely causes a degraded rollout, minor downtime, or requires follow-up fix.
   - low: cosmetic or non-impacting, but worth noting.

Respond with ONLY a single JSON object matching this schema (no prose outside JSON, no markdown fences):

{
  "summary": "<one or two sentences>",
  "risks": [
    {
      "level": "low" | "medium" | "high",
      "category": "data-loss" | "security" | "breaking-change" | "downtime" | "performance" | "best-practice",
      "description": "<what the risk is, 1-3 sentences, concrete>",
      "suggestion": "<actionable mitigation>"
    }
  ],
  "affected_resources": ["Deployment/foo", "Service/bar"]
}

If the diff is empty, return {"summary":"No changes.","risks":[]}.

If the diff is too large or unparseable, return {"error":"<short reason>"} and nothing else.`)
}

// userPrompt assembles the user message containing the diff plus runtime
// context (environment, release names) for grounding.
//
// Security: the context block is JSON-encoded via encoding/json so malicious
// release names cannot inject directives into the prompt. JSON guarantees
// quotes / backslashes / control characters are escaped to their literal
// forms; using stdlib rather than a hand-rolled marshaler gives us RFC 8259
// compliance and auditability.
//
// The diff body is delimited by the literal banner "helm diff output:" so the
// model can tell where data begins. Large diffs are capped at 32KB as a
// defensive measure against blowing the model context window.
func userPrompt(diff string, extras AnalyzeInput) string {
	var b strings.Builder
	if extras.Environment != "" || len(extras.Releases) > 0 {
		ctxJSON, err := json.Marshal(promptContext(extras))
		if err != nil {
			// Should never happen for a struct of strings. Skip the context
			// block rather than fail — the diff alone is still useful.
			b.WriteString("Context: <unavailable>\n\n")
		} else {
			b.WriteString("Context (JSON):\n  ")
			b.Write(ctxJSON)
			b.WriteString("\n\n")
		}
	}
	b.WriteString("helm diff output:\n\n")
	const maxDiffBytes = 32 * 1024
	if len(diff) > maxDiffBytes {
		b.WriteString(diff[:maxDiffBytes])
		fmt.Fprintf(&b, "\n\n... [diff truncated: %d bytes total, only first %d sent] ...\n", len(diff), maxDiffBytes)
	} else {
		b.WriteString(diff)
	}
	return b.String()
}

// promptContext mirrors AnalyzeInput for JSON marshaling. Exists ONLY so
// json.Marshal produces a stable, predictable key order. Do not reuse outside
// the prompt builder.
type promptContext struct {
	Environment string   `json:"environment,omitempty"`
	Releases    []string `json:"releases,omitempty"`
}
