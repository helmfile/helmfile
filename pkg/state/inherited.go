package state

import (
	"fmt"
	"strings"

	"dario.cat/mergo"
	"go.uber.org/zap"

	"github.com/helmfile/helmfile/pkg/environment"
	"github.com/helmfile/helmfile/pkg/yaml"
)

// allowedInherits is the single source of truth for the keys accepted by the
// sub-helmfile `inherits:` field. It is unexported so external packages cannot
// mutate it at runtime; validation, docs and tests reach it via AllowedInherits
// (which returns a defensive copy) and IsValidInherit.
var allowedInherits = []string{
	"repositories",
	"helmDefaults",
	"commonLabels",
	"apiVersions",
	"kubeVersion",
	"templates",
	"environments",
}

// AllowedInherits returns a defensive copy of the keys accepted by the
// sub-helmfile `inherits:` field. The returned slice is safe for callers to
// mutate without affecting validation behavior.
func AllowedInherits() []string {
	out := make([]string, len(allowedInherits))
	copy(out, allowedInherits)
	return out
}

// IsValidInherit reports whether key is an allowed inherits: entry.
func IsValidInherit(key string) bool {
	for _, k := range allowedInherits {
		if k == key {
			return true
		}
	}
	return false
}

// InheritedConfig carries parent-helmfile config to a sub-helmfile. Only the
// fields requested via `inherits:` are populated; the rest stay zero/nil so the
// consumer can skip them. Env is resolved pre-load (see desiredStateLoader.Load)
// because environment values are baked into RenderedValues at load time; the
// other fields are merged post-load via MergeInherited.
type InheritedConfig struct {
	Repositories []RepositorySpec         `yaml:"repositories,omitempty"`
	HelmDefaults *HelmSpec                `yaml:"helmDefaults,omitempty"`
	CommonLabels map[string]string        `yaml:"commonLabels,omitempty"`
	ApiVersions  []string                 `yaml:"apiVersions,omitempty"`
	KubeVersion  string                   `yaml:"kubeVersion,omitempty"`
	Templates    map[string]TemplateSpec  `yaml:"templates,omitempty"`
	Env          *environment.Environment `yaml:"-"`
}

// BuildInheritedConfig extracts the requested fields from the parent state and
// returns a fully independent copy. The pure fields are deep-copied via a YAML
// round-trip (these structs ARE the helmfile model, so the round-trip is
// lossless for the user-declared fields and avoids hand-maintained per-field
// copy logic), and Env is deep-copied via environment.DeepCopy. The returned
// config therefore never aliases the parent state's memory.
func (st *HelmState) BuildInheritedConfig(want []string) (*InheritedConfig, error) {
	set := make(map[string]bool, len(want))
	for _, w := range want {
		set[w] = true
	}

	// src temporarily references the parent; the round-trip below decouples it.
	src := &InheritedConfig{}
	if set["repositories"] {
		src.Repositories = st.Repositories
	}
	if set["helmDefaults"] {
		src.HelmDefaults = &st.HelmDefaults
	}
	if set["commonLabels"] {
		src.CommonLabels = st.CommonLabels
	}
	if set["apiVersions"] {
		src.ApiVersions = st.ApiVersions
	}
	if set["kubeVersion"] {
		src.KubeVersion = st.KubeVersion
	}
	if set["templates"] {
		src.Templates = st.Templates
	}

	in := &InheritedConfig{}
	// Deep-copy the pure fields so the result never shares slices/maps with the
	// parent. Skipped only when no pure field was requested.
	if set["repositories"] || set["helmDefaults"] || set["commonLabels"] ||
		set["apiVersions"] || set["kubeVersion"] || set["templates"] {
		b, err := yaml.Marshal(src)
		if err != nil {
			return nil, fmt.Errorf("marshaling inherited config for deep copy: %w", err)
		}
		if err := yaml.Unmarshal(b, in); err != nil {
			return nil, fmt.Errorf("unmarshaling inherited config for deep copy: %w", err)
		}
	}

	// Env is tagged yaml:"-" so it does not survive the round-trip; deep-copy it
	// explicitly via its own DeepCopy (handles nested value maps).
	if set["environments"] {
		e := st.Env.DeepCopy()
		in.Env = &e
	}
	return in, nil
}

// MergeInherited merges the 6 pure fields into the child state. Semantics:
// "child wins, parent fills gaps" — matching bases: precedence.
//
//   - repositories / apiVersions: parent-first append, de-duplicated by value
//     (repositories by Name, child's entry wins on conflict).
//   - helmDefaults: deep struct merge via mergo without override, so the
//     parent fills the child's zero-valued sub-fields and the child's non-zero
//     sub-fields win. Caveat: Go value types cannot distinguish "unset" from
//     zero, so a child that explicitly sets a field to its zero value (e.g.
//     atomic: false to disable) will see the parent's value fill in. The
//     common case — child omits helmDefaults entirely and inherits the parent's
//     — works correctly. This intentionally differs from bases:, which uses
//     WithOverride and would wipe the parent whenever the child's block is
//     absent, making inheritance useless.
//   - commonLabels / templates: per-key union, child's key wins.
//   - kubeVersion: parent fills only when the child left it empty.
//
// Env (environments) is intentionally NOT handled here; it must be injected
// pre-load because RenderedValues is computed at load time.
func (st *HelmState) MergeInherited(in *InheritedConfig) error {
	if in == nil {
		return nil
	}

	if in.Repositories != nil {
		combined := append([]RepositorySpec{}, in.Repositories...)
		combined = append(combined, st.Repositories...)
		st.Repositories = dedupReposByName(combined)
	}

	if in.HelmDefaults != nil {
		if err := mergo.Merge(&st.HelmDefaults, *in.HelmDefaults); err != nil {
			return fmt.Errorf("merging inherited helmDefaults: %w", err)
		}
	}

	if in.CommonLabels != nil {
		if st.CommonLabels == nil {
			st.CommonLabels = map[string]string{}
		}
		for k, v := range in.CommonLabels {
			if _, ok := st.CommonLabels[k]; !ok {
				st.CommonLabels[k] = v
			}
		}
	}

	if in.Templates != nil {
		if st.Templates == nil {
			st.Templates = map[string]TemplateSpec{}
		}
		for k, v := range in.Templates {
			if _, ok := st.Templates[k]; !ok {
				st.Templates[k] = v
			}
		}
	}

	if in.ApiVersions != nil {
		combined := append([]string{}, in.ApiVersions...)
		combined = append(combined, st.ApiVersions...)
		st.ApiVersions = dedupStrings(combined)
	}

	if in.KubeVersion != "" && st.KubeVersion == "" {
		st.KubeVersion = in.KubeVersion
	}

	return nil
}

// WarnUninheritedRepos emits a one-time warning per repository that a release
// references, which is present in the parent's effective repository set
// (parentRepoNames — derived from the parent's st.Repositories, so it includes
// repositories brought in via bases: or inherits:) but missing from the child's
// effective repositories (after any inheritance merge). It is a footgun guard
// for issue #1495: when repositories are not inherited, a release chart like
// "release-charts/myapp" fails with a confusing "repo not found". The warning
// suggests `inherits: [repositories]`.
//
// When `inherits: [repositories]` is in effect, the parent's repos are already
// part of the child's st.Repositories, so the condition is naturally false and
// nothing is logged.
func (st *HelmState) WarnUninheritedRepos(parentRepoNames []string, logger *zap.SugaredLogger) {
	if logger == nil || len(parentRepoNames) == 0 || len(st.Releases) == 0 {
		return
	}

	parentSet := make(map[string]bool, len(parentRepoNames))
	for _, n := range parentRepoNames {
		parentSet[n] = true
	}
	childSet := make(map[string]bool, len(st.Repositories))
	for _, r := range st.Repositories {
		childSet[r.Name] = true
	}

	warned := map[string]bool{}
	for _, rel := range st.Releases {
		chart := rel.Chart
		if chart == "" {
			continue
		}
		parts := strings.SplitN(chart, "/", 2)
		if len(parts) < 2 || parts[0] == "" {
			continue // local path, URL, or bare chart name — not a named-repo ref
		}
		repo := parts[0]
		// Skip schemes (oci://, https://, ...) and relative paths (./, ../):
		// these are never named-repository references, so they must not match a
		// parent repo name even by coincidence.
		if strings.Contains(repo, ":") || repo == "." || repo == ".." {
			continue
		}
		if parentSet[repo] && !childSet[repo] && !warned[repo] {
			warned[repo] = true
			logger.Warnf(
				`release %q references repository %q which is available in the parent helmfile but not inherited. `+
					`Add "inherits: [repositories]" to this sub-helmfile entry, or declare the repository here.`,
				rel.Name, repo,
			)
		}
	}
}

// dedupReposByName de-duplicates a repository list by Name, keeping the LAST
// occurrence so a child entry overrides a parent entry with the same name.
func dedupReposByName(repos []RepositorySpec) []RepositorySpec {
	if len(repos) == 0 {
		return repos
	}
	idx := map[string]int{}
	for i, r := range repos {
		idx[r.Name] = i // last index wins
	}
	out := make([]RepositorySpec, 0, len(repos))
	for i, r := range repos {
		if idx[r.Name] == i {
			out = append(out, r)
		}
	}
	return out
}

// dedupStrings de-duplicates a string slice, preserving first-seen order.
func dedupStrings(in []string) []string {
	if len(in) == 0 {
		return in
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
