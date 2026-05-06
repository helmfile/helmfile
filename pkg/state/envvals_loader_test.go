package state

import (
	"io"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/helmfile/helmfile/pkg/environment"
	ffs "github.com/helmfile/helmfile/pkg/filesystem"
	"github.com/helmfile/helmfile/pkg/helmexec"
	"github.com/helmfile/helmfile/pkg/remote"
)

func newLoader() *EnvironmentValuesLoader {
	log := helmexec.NewLogger(io.Discard, "debug")

	storage := &Storage{
		FilePath: "./helmfile.yaml",
		basePath: ".",
		fs:       ffs.DefaultFileSystem(),
		logger:   log,
	}

	return NewEnvironmentValuesLoader(storage, storage.fs, log, remote.NewRemote(log, "/tmp", storage.fs))
}

// See https://github.com/roboll/helmfile/pull/1169
func TestEnvValsLoad_SingleValuesFile(t *testing.T) {
	l := newLoader()

	actual, err := l.LoadEnvironmentValues(nil, []any{"testdata/values.5.yaml"}, nil, "", "")
	if err != nil {
		t.Fatal(err)
	}

	expected := map[string]any{
		"affinity": map[string]any{},
	}

	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Error(diff)
	}
}

func TestEnvValsLoad_EnvironmentNameFile(t *testing.T) {
	l := newLoader()

	expected := map[string]any{
		"envName": "test",
	}
	emptyExpected := map[string]any{
		"envName": nil,
	}

	tests := []struct {
		name     string
		env      *environment.Environment
		envName  string
		expected map[string]any
	}{
		{
			name:     "env is nil but envName is not",
			env:      nil,
			envName:  "test",
			expected: expected,
		},
		{
			name:     "envName is emplte but env is not",
			env:      environment.New("test"),
			envName:  "",
			expected: expected,
		},
		{
			name:     "envName and env is not nil",
			env:      environment.New("test"),
			envName:  "test",
			expected: expected,
		},
		{
			name:     "envName and env is nil",
			env:      nil,
			envName:  "",
			expected: emptyExpected,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := l.LoadEnvironmentValues(nil, []any{"testdata/values.6.yaml.gotmpl"}, tt.env, tt.envName, "")
			if err != nil {
				t.Fatal(err)
			}

			if diff := cmp.Diff(tt.expected, actual); diff != "" {
				t.Error(diff)
			}
		})
	}
}

// Fetch Environment values from remote
func TestEnvValsLoad_SingleValuesFileRemote(t *testing.T) {
	l := newLoader()

	actual, err := l.LoadEnvironmentValues(nil, []any{"git::https://github.com/helm/helm.git@cmd/helm/testdata/output/values.yaml?ref=v3.8.1"}, nil, "", "")
	if err != nil {
		t.Fatal(err)
	}

	expected := map[string]any{
		"name": string("value"),
	}

	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Error(diff)
	}
}

// See https://github.com/roboll/helmfile/issues/1150
func TestEnvValsLoad_OverwriteNilValue_Issue1150(t *testing.T) {
	l := newLoader()

	actual, err := l.LoadEnvironmentValues(nil, []any{"testdata/values.1.yaml", "testdata/values.2.yaml"}, nil, "", "")
	if err != nil {
		t.Fatal(err)
	}

	expected := map[string]any{
		"components": map[string]any{
			"etcd-operator": map[string]any{
				"version": "0.10.3",
			},
		},
	}

	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Error(diff)
	}
}

// See https://github.com/roboll/helmfile/issues/1154
func TestEnvValsLoad_OverwriteWithNilValue_Issue1154(t *testing.T) {
	l := newLoader()

	actual, err := l.LoadEnvironmentValues(nil, []any{"testdata/values.3.yaml", "testdata/values.4.yaml"}, nil, "", "")
	if err != nil {
		t.Fatal(err)
	}

	expected := map[string]any{
		"components": map[string]any{
			"etcd-operator": map[string]any{
				"version": "0.10.3",
			},
			"prometheus": nil,
		},
	}

	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Error(diff)
	}
}

// See https://github.com/roboll/helmfile/issues/1168
func TestEnvValsLoad_OverwriteEmptyValue_Issue1168(t *testing.T) {
	l := newLoader()

	actual, err := l.LoadEnvironmentValues(nil, []any{"testdata/issues/1168/addons.yaml", "testdata/issues/1168/addons2.yaml"}, nil, "", "")
	if err != nil {
		t.Fatal(err)
	}

	expected := map[string]any{
		"addons": map[string]any{
			"mychart": map[string]any{
				"skip":      false,
				"name":      "mychart",
				"namespace": "kube-system",
				"chart":     "stable/mychart",
				"version":   "1.0.0",
			},
		},
	}

	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Error(diff)
	}
}

func TestEnvValsLoad_MultiHCL(t *testing.T) {
	l := newLoader()

	actual, err := l.LoadEnvironmentValues(nil, []any{"testdata/values.7.hcl", "testdata/values.8.hcl"}, nil, "", "")
	if err != nil {
		t.Fatal(err)
	}

	expected := map[string]any{
		"a": "a",
		"b": "b",
		"c": "ab",
		"map": map[string]any{
			"a": "a",
		},
		"list": []any{
			"b",
		},
		"nestedmap": map[string]any{
			"submap": map[string]any{
				"subsubmap": map[string]any{
					"hello": "ab",
				},
			},
		},
		"ternary":          true,
		"fromMap":          "aab",
		"expressionInText": "yes",
		"insideFor":        "b",
		"multi_block":      "block",
		"block":            "block",
		"crossfile":        "crossfile var",
		"crossfile_var":    "crossfile var",
		"localRef":         "localInValues7",
	}

	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Error(diff)
	}
}

func TestEnvValsLoad_EnvironmentValues(t *testing.T) {
	l := newLoader()
	env := environment.New("test")
	env.Values["foo"] = "bar"

	actual, err := l.LoadEnvironmentValues(nil, []any{"testdata/values.9.yaml.gotmpl"}, env, "", "")
	if err != nil {
		t.Fatal(err)
	}

	expected := map[string]any{
		"foo": "bar",
	}

	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Error(diff)
	}
}

// --- mergeStrategy: fallback ---

// Earlier files take precedence. Same conflicting key in two files →
// the value from default.yaml (loaded first) wins.
func TestEnvValsLoad_FallbackStrategy_EarlierWins(t *testing.T) {
	l := newLoader()

	actual, err := l.LoadEnvironmentValues(nil,
		[]any{"testdata/mergestrategy/default.yaml", "testdata/mergestrategy/fallback.yaml"},
		nil, "", MergeStrategyFallback)
	if err != nil {
		t.Fatal(err)
	}

	cluster := actual["cluster"].(map[string]any)
	if got := cluster["domain"]; got != "example.com" {
		t.Errorf("cluster.domain: want %q (from default.yaml), got %v", "example.com", got)
	}
}

// Later files only fill keys that are missing from earlier files.
func TestEnvValsLoad_FallbackStrategy_FillsGaps(t *testing.T) {
	l := newLoader()

	actual, err := l.LoadEnvironmentValues(nil,
		[]any{"testdata/mergestrategy/default.yaml", "testdata/mergestrategy/fallback.yaml"},
		nil, "", MergeStrategyFallback)
	if err != nil {
		t.Fatal(err)
	}

	cluster := actual["cluster"].(map[string]any)
	if got := cluster["region"]; got != "us-east-1" {
		t.Errorf("cluster.region: want %q (from fallback.yaml, missing in default.yaml), got %v", "us-east-1", got)
	}
	service := actual["service"].(map[string]any)
	if got := service["port"]; got != 8080 {
		t.Errorf("service.port: want 8080 (from fallback.yaml), got %v", got)
	}
}

// Nested maps merge recursively: top-level cluster is not replaced
// wholesale; both files contribute keys.
func TestEnvValsLoad_FallbackStrategy_DeepMerge(t *testing.T) {
	l := newLoader()

	actual, err := l.LoadEnvironmentValues(nil,
		[]any{"testdata/mergestrategy/default.yaml", "testdata/mergestrategy/fallback.yaml"},
		nil, "", MergeStrategyFallback)
	if err != nil {
		t.Fatal(err)
	}

	expected := map[string]any{
		"cluster": map[string]any{
			"domain": "example.com", // from default (wins)
			"region": "us-east-1",   // from fallback (gap filled)
		},
		"service": map[string]any{
			"port": 8080, // from fallback (gap filled)
		},
	}
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("deep merge mismatch (-want +got):\n%s", diff)
	}
}

// First-wins precedence holds across an arbitrarily long chain — not just
// pairwise. Three files exercise the accumulator state across iterations.
func TestEnvValsLoad_FallbackStrategy_ChainedFiles(t *testing.T) {
	l := newLoader()

	actual, err := l.LoadEnvironmentValues(nil,
		[]any{
			"testdata/mergestrategy/chain_a.yaml",
			"testdata/mergestrategy/chain_b.yaml",
			"testdata/mergestrategy/chain_c.yaml",
		},
		nil, "", MergeStrategyFallback)
	if err != nil {
		t.Fatal(err)
	}

	expected := map[string]any{
		"letter":    "a",      // only in a
		"only_a":    "from-a", // only in a
		"only_b":    "from-b", // only in b
		"only_c":    "from-c", // only in c
		"both_ab":   "from-a", // a and b → a wins (earlier)
		"both_bc":   "from-b", // b and c → b wins (earlier)
		"all_three": "from-a", // a, b, c → a wins (earliest)
	}
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("chain mismatch (-want +got):\n%s", diff)
	}
}

// Explicit zero values in the earlier file MUST be preserved. Without the
// hand-rolled fallbackDeepMerge, mergo's isEmptyValue would silently let
// `enabled: true` from fallback overwrite `enabled: false` from default.
func TestEnvValsLoad_FallbackStrategy_PreservesExplicitZeroValues(t *testing.T) {
	l := newLoader()

	actual, err := l.LoadEnvironmentValues(nil,
		[]any{"testdata/mergestrategy/zero_default.yaml", "testdata/mergestrategy/zero_fallback.yaml"},
		nil, "", MergeStrategyFallback)
	if err != nil {
		t.Fatal(err)
	}

	expected := map[string]any{
		"enabled":  false,
		"replicas": 0,
		"name":     "",
		"tags":     []any{},
	}
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("explicit zero values not preserved (-want +got):\n%s", diff)
	}
}

// Explicit nil in the earlier file is also preserved.
func TestEnvValsLoad_FallbackStrategy_PreservesExplicitNil(t *testing.T) {
	l := newLoader()

	actual, err := l.LoadEnvironmentValues(nil,
		[]any{"testdata/mergestrategy/nil_default.yaml", "testdata/mergestrategy/nil_fallback.yaml"},
		nil, "", MergeStrategyFallback)
	if err != nil {
		t.Fatal(err)
	}

	expected := map[string]any{"value": nil}
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("explicit nil not preserved (-want +got):\n%s", diff)
	}
}

// Inline map entries (not file paths) also honor the fallback strategy.
func TestEnvValsLoad_FallbackStrategy_InlineMapEntry(t *testing.T) {
	l := newLoader()

	inline := map[string]any{
		"cluster": map[string]any{"domain": "inline.example"},
		"extra":   "from-inline",
	}

	actual, err := l.LoadEnvironmentValues(nil,
		[]any{inline, "testdata/mergestrategy/fallback.yaml"},
		nil, "", MergeStrategyFallback)
	if err != nil {
		t.Fatal(err)
	}

	cluster := actual["cluster"].(map[string]any)
	if got := cluster["domain"]; got != "inline.example" {
		t.Errorf("cluster.domain: want inline value to win, got %v", got)
	}
	if got := actual["extra"]; got != "from-inline" {
		t.Errorf("extra: want %q, got %v", "from-inline", got)
	}
	// fallback.yaml still fills gaps the inline map did not set.
	if got := cluster["region"]; got != "us-east-1" {
		t.Errorf("cluster.region: want %q from fallback file, got %v", "us-east-1", got)
	}
}

// Regression guard: explicit "override" matches today's behavior
// (last file wins).
func TestEnvValsLoad_OverrideStrategy_PreservesCurrentBehavior(t *testing.T) {
	l := newLoader()

	actual, err := l.LoadEnvironmentValues(nil,
		[]any{"testdata/mergestrategy/default.yaml", "testdata/mergestrategy/fallback.yaml"},
		nil, "", MergeStrategyOverride)
	if err != nil {
		t.Fatal(err)
	}

	cluster := actual["cluster"].(map[string]any)
	if got := cluster["domain"]; got != "cluster.local" {
		t.Errorf("cluster.domain under override: want %q (from fallback.yaml), got %v", "cluster.local", got)
	}
}

// Empty strategy is identical to explicit "override".
func TestEnvValsLoad_DefaultStrategy_MatchesOverride(t *testing.T) {
	l := newLoader()

	asDefault, err := l.LoadEnvironmentValues(nil,
		[]any{"testdata/mergestrategy/default.yaml", "testdata/mergestrategy/fallback.yaml"},
		nil, "", "")
	if err != nil {
		t.Fatal(err)
	}
	asOverride, err := l.LoadEnvironmentValues(nil,
		[]any{"testdata/mergestrategy/default.yaml", "testdata/mergestrategy/fallback.yaml"},
		nil, "", MergeStrategyOverride)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(asOverride, asDefault); diff != "" {
		t.Errorf("default strategy diverges from override (-override +default):\n%s", diff)
	}
}

// Unknown strategy values produce a clear error that names both the bad
// value and the valid options.
func TestEnvValsLoad_InvalidStrategy_Errors(t *testing.T) {
	l := newLoader()

	_, err := l.LoadEnvironmentValues(nil,
		[]any{"testdata/mergestrategy/default.yaml"},
		nil, "prod", "bogus")
	if err == nil {
		t.Fatal("expected error for invalid mergeStrategy, got nil")
	}
	for _, want := range []string{"prod", "bogus", MergeStrategyOverride, MergeStrategyFallback} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error message missing %q: %v", want, err)
		}
	}
}
