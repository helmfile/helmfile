package state

import (
	"os"
	"testing"

	"github.com/helmfile/vals"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/helmfile/helmfile/pkg/envvar"
	"github.com/helmfile/helmfile/pkg/testhelper"
)

// newIssue2738TestState returns a HelmState wired with a regular vals runtime
// and a vals runtime created with the ExcludeSecret option (the one used for
// diffing without --show-secrets).
// See https://github.com/helmfile/helmfile/issues/2738
func newIssue2738TestState(t *testing.T, files map[string]string) *HelmState {
	t.Helper()

	const basePath = "/project"

	valsRuntime, err := vals.New(vals.Options{CacheSize: 32})
	require.NoError(t, err)

	valsExcludeSecretRuntime, err := vals.New(vals.Options{CacheSize: 32, ExcludeSecret: true})
	require.NoError(t, err)

	testFs := testhelper.NewTestFs(files)
	testFs.Cwd = basePath

	return &HelmState{
		logger:                   zaptest.NewLogger(t).Sugar(),
		fs:                       testFs.ToFileSystem(),
		valsRuntime:              valsRuntime,
		valsExcludeSecretRuntime: valsExcludeSecretRuntime,
		basePath:                 basePath,
		FilePath:                 basePath + "/helmfile.yaml",
		RenderedValues:           map[string]any{},
	}
}

// TestPrepareReleaseValuesEntries_SecretExclusion covers vals Eval point A:
// in diff mode without --show-secrets (excludeSecrets=true), `secretref+...`
// entries must stay unresolved while plain `ref+...` entries are still
// expanded. With --show-secrets (excludeSecrets=false), everything is
// resolved to plaintext as before.
func TestPrepareReleaseValuesEntries_SecretExclusion(t *testing.T) {
	st := newIssue2738TestState(t, map[string]string{})

	release := &ReleaseSpec{
		Name:  "myrelease",
		Chart: "mychart",
		Values: []any{
			map[string]any{
				"secretPassword": "secretref+echo://topsecret",
				"plainValue":     "ref+echo://plainvisible",
			},
		},
	}

	tests := []struct {
		name             string
		excludeSecrets   bool
		wantSecretLeaked bool
		wantPlain        string
	}{
		{
			name:             "diff without show-secrets keeps secretref unresolved",
			excludeSecrets:   true,
			wantSecretLeaked: false,
			wantPlain:        "plainvisible",
		},
		{
			name:             "diff with show-secrets resolves secretref to plaintext",
			excludeSecrets:   false,
			wantSecretLeaked: true,
			wantPlain:        "plainvisible",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rendered, err := st.prepareReleaseValuesEntries(release, tt.excludeSecrets)
			require.NoError(t, err)
			require.Len(t, rendered, 1)

			values := rendered[0].(map[string]any)
			secret := values["secretPassword"].(string)
			plain := values["plainValue"].(string)

			assert.Equal(t, tt.wantPlain, plain)
			if tt.wantSecretLeaked {
				assert.NotContains(t, secret, "secretref+", "secret value must be resolved when secrets are shown")
			} else {
				assert.Equal(t, "secretref+echo://topsecret", secret, "secret value must never be resolved while diffing")
			}
		})
	}
}

// TestRenderValuesFileToBytesWithData_SecretExclusion covers vals Eval point B:
// a values file containing `secretref+...` must not have its secrets resolved
// into the rendered bytes when excludeSecrets is true.
func TestRenderValuesFileToBytesWithData_SecretExclusion(t *testing.T) {
	valuesFile := "/project/values.yaml"
	valuesContent := `secretPassword: secretref+echo://topsecret
plainValue: ref+echo://plainvisible
`

	tests := []struct {
		name             string
		excludeSecrets   bool
		wantSecretLeaked bool
		wantPlain        string
	}{
		{
			name:             "diff without show-secrets keeps secretref literal",
			excludeSecrets:   true,
			wantSecretLeaked: false,
			wantPlain:        "plainvisible",
		},
		{
			name:             "diff with show-secrets renders plaintext",
			excludeSecrets:   false,
			wantSecretLeaked: true,
			wantPlain:        "plainvisible",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := newIssue2738TestState(t, map[string]string{
				valuesFile: valuesContent,
			})

			release := &ReleaseSpec{Name: "myrelease", Chart: "mychart"}
			tmplData := st.createReleaseTemplateData(release, map[string]any{})

			result, err := st.renderValuesFileToBytesWithData(valuesFile, tmplData, tt.excludeSecrets)
			require.NoError(t, err)

			assert.Contains(t, string(result), tt.wantPlain)
			if tt.wantSecretLeaked {
				assert.Contains(t, string(result), "topsecret")
				assert.NotContains(t, string(result), "secretref+")
			} else {
				assert.NotContains(t, string(result), "plainValue: topsecret", "resolved plaintext secret must not leak into rendered values file")
				assert.Contains(t, string(result), "secretPassword: secretref+echo://topsecret")
			}
		})
	}
}

// TestGenerateVanillaValuesFiles_SecretExclusion exercises the full
// values-files generation pipeline used by `helmfile diff` (the path that
// writes temporary values files) and asserts the generated files contain no
// plaintext secrets unless secrets are shown.
func TestGenerateVanillaValuesFiles_SecretExclusion(t *testing.T) {
	t.Setenv(envvar.TempDir, t.TempDir())

	valuesFile := "/project/values.yaml"
	valuesContent := `secretPassword: secretref+echo://topsecret
plainValue: ref+echo://plainvisible
`

	tests := []struct {
		name             string
		excludeSecrets   bool
		wantSecretLeaked bool
		wantPlain        string
	}{
		{
			name:             "diff without show-secrets writes no plaintext secret to temp values file",
			excludeSecrets:   true,
			wantSecretLeaked: false,
			wantPlain:        "plainvisible",
		},
		{
			name:             "diff with show-secrets writes plaintext secret to temp values file",
			excludeSecrets:   false,
			wantSecretLeaked: true,
			wantPlain:        "plainvisible",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := newIssue2738TestState(t, map[string]string{
				valuesFile: valuesContent,
			})

			release := &ReleaseSpec{
				Name:   "myrelease",
				Chart:  "mychart",
				Values: []any{"values.yaml"},
			}

			generatedFiles, err := st.generateVanillaValuesFiles(release, tt.excludeSecrets)
			require.NoError(t, err)
			require.Len(t, generatedFiles, 1)

			bs, err := os.ReadFile(generatedFiles[0])
			require.NoError(t, err)

			assert.Contains(t, string(bs), tt.wantPlain)
			if tt.wantSecretLeaked {
				assert.Contains(t, string(bs), "topsecret")
				assert.NotContains(t, string(bs), "secretref+")
			} else {
				assert.NotContains(t, string(bs), "plainValue: topsecret", "resolved plaintext secret must not leak into temporary values file")
				assert.Contains(t, string(bs), "secretPassword: secretref+echo://topsecret")
			}
		})
	}
}
