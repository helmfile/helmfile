package app

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/helmfile/vals"
	"github.com/stretchr/testify/assert"

	"github.com/helmfile/helmfile/pkg/testhelper"
)

func TestExtractDirSelectorTargets(t *testing.T) {
	tests := []struct {
		name      string
		selectors []string
		wantNil   bool
		wantPer   []int
	}{
		{
			name:      "nil selectors",
			selectors: nil,
			wantNil:   true,
		},
		{
			name:      "no dir selectors",
			selectors: []string{"name=foo", "namespace=bar"},
			wantNil:   true,
		},
		{
			name:      "single dir selector",
			selectors: []string{"dir=apps/x"},
			wantPer:   []int{1},
		},
		{
			name:      "dir with companion in same group",
			selectors: []string{"dir=apps/x,name=foo"},
			wantPer:   []int{1},
		},
		{
			name:      "dir spread across groups",
			selectors: []string{"dir=apps/x", "dir=apps/y"},
			wantPer:   []int{1, 1},
		},
		{
			name:      "negative dir is ignored",
			selectors: []string{"dir!=apps/x"},
			wantNil:   true,
		},
		{
			name:      "mixed group with no dir and group with dir",
			selectors: []string{"name=foo", "dir=apps/x"},
			wantPer:   []int{0, 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			groups := extractDirSelectorTargets(tt.selectors)
			if tt.wantNil {
				assert.Nil(t, groups)
				return
			}
			assert.Len(t, groups, len(tt.wantPer))
			for i, want := range tt.wantPer {
				assert.Len(t, groups[i].targets, want)
			}
		})
	}
}

func TestCouldContainDirMatch(t *testing.T) {
	tests := []struct {
		name        string
		entryPath   string
		entryParent string
		target      string
		want        bool
	}{
		{"entry is inside target subtree", "apps/x/sub/helmfile.yaml", "apps/x/sub", "apps/x", true},
		{"target is below entry parent", "apps/helmfile.yaml", "apps", "apps/x/sub", true},
		{"target equals entry parent", "apps/x/helmfile.yaml", "apps/x", "apps/x", true},
		{"entry parent equals target exactly", "apps/x", "apps", "apps", true},
		{"entry is sibling of target", "apps/y/helmfile.yaml", "apps/y", "apps/x", false},
		{"name-collision is not a match", "apps/xenial/helmfile.yaml", "apps/xenial", "apps/x", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, couldContainDirMatch(tt.entryPath, tt.entryParent, tt.target))
		})
	}
}

func TestShouldDescendForDirFilter(t *testing.T) {
	rootDir, err := filepath.Abs("/workspace/proj")
	assert.NoError(t, err)
	stateDir := filepath.Join(rootDir, "opendesk")

	tests := []struct {
		name     string
		entry    string
		groups   []dirSelectorGroup
		wantSkip bool
	}{
		{
			name:   "no groups means caller skipped check; treat as descend",
			entry:  "helmfile/apps/openproject/helmfile-child.yaml.gotmpl",
			groups: nil,
		},
		{
			name:   "target inside entry subtree → descend",
			entry:  "helmfile/apps/openproject/helmfile-child.yaml.gotmpl",
			groups: []dirSelectorGroup{{targets: []string{"opendesk/helmfile/apps/openproject"}}},
		},
		{
			name:     "sibling service → skip",
			entry:    "helmfile/apps/xwiki/helmfile-child.yaml.gotmpl",
			groups:   []dirSelectorGroup{{targets: []string{"opendesk/helmfile/apps/openproject"}}},
			wantSkip: true,
		},
		{
			name:   "entry is ancestor of target → descend",
			entry:  "helmfile_generic.yaml.gotmpl",
			groups: []dirSelectorGroup{{targets: []string{"opendesk/helmfile/apps/openproject"}}},
		},
		{
			name:   "group with no targets is permissive",
			entry:  "helmfile/apps/xwiki/helmfile-child.yaml.gotmpl",
			groups: []dirSelectorGroup{{targets: nil}, {targets: []string{"opendesk/helmfile/apps/openproject"}}},
		},
		{
			name:     "multiple targets in one group: all must allow",
			entry:    "helmfile/apps/openproject/helmfile-child.yaml.gotmpl",
			groups:   []dirSelectorGroup{{targets: []string{"opendesk/helmfile/apps/openproject", "opendesk/helmfile/apps/xwiki"}}},
			wantSkip: true,
		},
		{
			name:   "OR across groups: either matches descends",
			entry:  "helmfile/apps/openproject/helmfile-child.yaml.gotmpl",
			groups: []dirSelectorGroup{{targets: []string{"opendesk/helmfile/apps/xwiki"}}, {targets: []string{"opendesk/helmfile/apps/openproject"}}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldDescendForDirFilter(rootDir, stateDir, tt.entry, tt.groups)
			assert.Equal(t, !tt.wantSkip, got)
		})
	}
}

// TestProcessNestedHelmfiles_DirFilterSkipsSiblings drives the full visit
// chain against an opendesk-shaped fixture: a root helmfile that includes an
// aggregator, which in turn lists several sibling service helmfiles. With
// `-l dir=…/openproject` only the matching service helmfile should be opened,
// asserted by counting how often a helmfile-child.yaml file is read. The
// release names also let us confirm the post-load filter sees only the
// expected entry.
func TestProcessNestedHelmfiles_DirFilterSkipsSiblings(t *testing.T) {
	files := map[string]string{
		"/workspace/helmfile.yaml": `
helmfiles:
  - opendesk/helmfile_generic.yaml
`,
		"/workspace/opendesk/helmfile_generic.yaml": `
helmfiles:
  - apps/openproject/helmfile-child.yaml
  - apps/xwiki/helmfile-child.yaml
  - apps/jitsi/helmfile-child.yaml
`,
		"/workspace/opendesk/apps/openproject/helmfile-child.yaml": `
releases:
  - name: openproject-web
    chart: stable/openproject
`,
		"/workspace/opendesk/apps/xwiki/helmfile-child.yaml": `
releases:
  - name: xwiki
    chart: stable/xwiki
`,
		"/workspace/opendesk/apps/jitsi/helmfile-child.yaml": `
releases:
  - name: jitsi
    chart: stable/jitsi
`,
	}

	testFs := testhelper.NewTestFs(files)
	testFs.Cwd = "/workspace"

	valsRuntime, err := vals.New(vals.Options{CacheSize: 32})
	assert.NoError(t, err)

	app := &App{
		OverrideHelmBinary:              DefaultHelmBinary,
		OverrideKubeContext:             "default",
		DisableKubeVersionAutoDetection: true,
		Env:                             "default",
		Logger:                          newAppTestLogger(),
		valsRuntime:                     valsRuntime,
		FileOrDir:                       "/workspace/helmfile.yaml",
		Selectors:                       []string{"dir=opendesk/apps/openproject"},
	}
	app = injectFs(app, testFs)
	expectNoCallsToHelm(app)

	err = app.ForEachState(Noop, false, SetFilter(true))
	assert.NoError(t, err)

	childReads := 0
	for _, r := range testFs.SuccessfulReads() {
		if strings.HasSuffix(r, "helmfile-child.yaml") {
			childReads++
		}
	}
	assert.Equal(t, 1, childReads, "exactly one helmfile-child.yaml should be read (the matching one), got reads=%v", testFs.SuccessfulReads())
}

// TestProcessNestedHelmfiles_DirFilterExcludesTopLevelReleases verifies that a
// dir filter targeting a nested subtree excludes releases declared directly
// in the root helmfile (whose dirLabel is ".") while still descending into the
// matching sub-helmfile. The root file itself must still be read because its
// `helmfiles:` directive is what points to the nested branch. Uses distinct
// filenames per level so the read log is unambiguous.
func TestProcessNestedHelmfiles_DirFilterExcludesTopLevelReleases(t *testing.T) {
	files := map[string]string{
		"/workspace/root.yaml": `
releases:
  - name: root-release
    chart: stable/root
helmfiles:
  - apps/openproject/leaf.yaml
`,
		"/workspace/apps/openproject/leaf.yaml": `
releases:
  - name: openproject-web
    chart: stable/openproject
`,
	}

	testFs := testhelper.NewTestFs(files)
	testFs.Cwd = "/workspace"

	valsRuntime, err := vals.New(vals.Options{CacheSize: 32})
	assert.NoError(t, err)

	app := &App{
		OverrideHelmBinary:              DefaultHelmBinary,
		OverrideKubeContext:             "default",
		DisableKubeVersionAutoDetection: true,
		Env:                             "default",
		Logger:                          newAppTestLogger(),
		valsRuntime:                     valsRuntime,
		FileOrDir:                       "/workspace/root.yaml",
		Selectors:                       []string{"dir=apps/openproject"},
	}
	app = injectFs(app, testFs)
	expectNoCallsToHelm(app)

	err = app.ForEachState(Noop, false, SetFilter(true))
	assert.NoError(t, err)

	reads := testFs.SuccessfulReads()
	readSet := map[string]bool{}
	for _, r := range reads {
		readSet[r] = true
	}
	assert.True(t, readSet["root.yaml"], "root helmfile must be parsed to learn its helmfiles: list, reads=%v", reads)
	assert.True(t, readSet["leaf.yaml"], "matching nested helmfile must be descended into, reads=%v", reads)
}

// TestProcessNestedHelmfiles_DirDotIsRejected pins down the current behavior
// that `dir=.` and equivalent forms (`dir=./`, `dir=apps/..`) are rejected at
// selector parse time. Semantics for `.` were under discussion: matching
// everything (no-op), matching only root-level releases (useful but uneven),
// or rejecting outright. Outright rejection is the current decision; this
// test makes any change to that policy a deliberate code change.
func TestProcessNestedHelmfiles_DirDotIsRejected(t *testing.T) {
	files := map[string]string{
		"/workspace/root.yaml": `
releases:
  - name: root-release
    chart: stable/root
`,
	}
	for _, selector := range []string{"dir=.", "dir=./", "dir=apps/.."} {
		t.Run(selector, func(t *testing.T) {
			testFs := testhelper.NewTestFs(files)
			testFs.Cwd = "/workspace"

			valsRuntime, err := vals.New(vals.Options{CacheSize: 32})
			assert.NoError(t, err)

			app := &App{
				OverrideHelmBinary:              DefaultHelmBinary,
				OverrideKubeContext:             "default",
				DisableKubeVersionAutoDetection: true,
				Env:                             "default",
				Logger:                          newAppTestLogger(),
				valsRuntime:                     valsRuntime,
				FileOrDir:                       "/workspace/root.yaml",
				Selectors:                       []string{selector},
			}
			app = injectFs(app, testFs)
			expectNoCallsToHelm(app)

			err = app.ForEachState(Noop, false, SetFilter(true))
			assert.Error(t, err, "dir=. and equivalents should be rejected at parse time")
		})
	}
}
