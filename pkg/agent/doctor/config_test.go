package doctor

import (
	"testing"
	"time"

	"github.com/helmfile/helmfile/pkg/agent/llm"
)

func TestEnvConfig_ReadsEnvVars(t *testing.T) {
	t.Setenv("HELMFILE_LLM_BASE_URL", "https://env.example/v1")
	t.Setenv("HELMFILE_LLM_API_KEY", "envkey")
	t.Setenv("HELMFILE_LLM_MODEL", "envmodel")
	t.Setenv("HELMFILE_LLM_TIMEOUT", "90s")
	t.Setenv("HELMFILE_LLM_MAX_TOKENS", "2048")

	cfg := EnvConfig()
	if cfg.BaseURL != "https://env.example/v1" {
		t.Errorf("BaseURL = %q", cfg.BaseURL)
	}
	if cfg.APIKey != "envkey" {
		t.Errorf("APIKey = %q", cfg.APIKey)
	}
	if cfg.Model != "envmodel" {
		t.Errorf("Model = %q", cfg.Model)
	}
	if cfg.Timeout != 90*time.Second {
		t.Errorf("Timeout = %v", cfg.Timeout)
	}
	if cfg.MaxTokens != 2048 {
		t.Errorf("MaxTokens = %d", cfg.MaxTokens)
	}
}

// TestEnvConfig_NoEnvReturnsEmpty uses t.Setenv with empty values rather than
// os.Unsetenv. t.Setenv automatically restores the prior value via t.Cleanup,
// so it is safe under `go test -parallel` and never leaks state to other
// tests in the same process. Empty string is observationally equivalent to
// an unset variable for os.Getenv callers.
func TestEnvConfig_NoEnvReturnsEmpty(t *testing.T) {
	t.Setenv("HELMFILE_LLM_BASE_URL", "")
	t.Setenv("HELMFILE_LLM_API_KEY", "")
	t.Setenv("HELMFILE_LLM_MODEL", "")
	t.Setenv("HELMFILE_LLM_TIMEOUT", "")
	t.Setenv("HELMFILE_LLM_MAX_TOKENS", "")

	cfg := EnvConfig()
	if cfg.IsConfigured() {
		t.Errorf("EnvConfig should be empty when no env vars set, got %+v", cfg)
	}
}

func TestEnvConfig_IgnoresBogusTimeout(t *testing.T) {
	t.Setenv("HELMFILE_LLM_TIMEOUT", "not-a-duration")
	t.Setenv("HELMFILE_LLM_MAX_TOKENS", "not-a-number")
	cfg := EnvConfig()
	if cfg.Timeout != 0 {
		t.Errorf("Timeout should be 0 on parse error, got %v", cfg.Timeout)
	}
	if cfg.MaxTokens != 0 {
		t.Errorf("MaxTokens should be 0 on parse error, got %v", cfg.MaxTokens)
	}
}

func TestResolveConfig_PreservesLayerPrecedence(t *testing.T) {
	env := llm.Config{
		BaseURL: "https://env.example",
		APIKey:  "envkey",
		Model:   "envmodel",
	}
	yaml := llm.Config{
		BaseURL:   "https://yaml.example",
		MaxTokens: 1024,
	}
	flag := llm.Config{
		Model:     "flagmodel",
		MaxTokens: 8192,
	}

	got := ResolveConfig(env, yaml, flag)

	if got.BaseURL != "https://yaml.example" {
		t.Errorf("BaseURL = %q, want yaml to override env", got.BaseURL)
	}
	if got.APIKey != "envkey" {
		t.Errorf("APIKey = %q, want env value (no override)", got.APIKey)
	}
	if got.Model != "flagmodel" {
		t.Errorf("Model = %q, want flag value", got.Model)
	}
	if got.MaxTokens != 8192 {
		t.Errorf("MaxTokens = %d, want flag to override yaml", got.MaxTokens)
	}
}

func TestResolveConfig_AllEmptyStaysEmpty(t *testing.T) {
	got := ResolveConfig(llm.Config{}, llm.Config{}, llm.Config{})
	if got.IsConfigured() {
		t.Errorf("all-empty resolve should be unconfigured, got %+v", got)
	}
}
