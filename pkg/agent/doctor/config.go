package doctor

import (
	"fmt"
	"os"
	"time"

	"github.com/helmfile/helmfile/pkg/agent/llm"
)

// EnvConfig reads LLM configuration from process environment variables.
// These are the lowest-precedence source (env < yaml < flag).
func EnvConfig() llm.Config {
	cfg := llm.Config{
		BaseURL: os.Getenv("HELMFILE_LLM_BASE_URL"),
		APIKey:  os.Getenv("HELMFILE_LLM_API_KEY"),
		Model:   os.Getenv("HELMFILE_LLM_MODEL"),
	}
	if t := os.Getenv("HELMFILE_LLM_TIMEOUT"); t != "" {
		if d, err := time.ParseDuration(t); err == nil {
			cfg.Timeout = d
		}
	}
	if mt := os.Getenv("HELMFILE_LLM_MAX_TOKENS"); mt != "" {
		var n int
		if _, err := fmt.Sscanf(mt, "%d", &n); err == nil && n > 0 {
			cfg.MaxTokens = n
		}
	}
	return cfg
}

// ResolveConfig merges env < yaml < flag into a single Config.
//
// Any non-zero field at a higher precedence layer overrides the lower one.
// Returns the merged config (caller can call IsConfigured() to decide whether
// to wire up an LLM client at all).
func ResolveConfig(env, yamlCfg, flag llm.Config) llm.Config {
	merged := env
	merged = llm.Merge(merged, yamlCfg)
	merged = llm.Merge(merged, flag)
	return merged
}
