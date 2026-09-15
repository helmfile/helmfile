package state

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/helmfile/helmfile/pkg/envvar"
	"github.com/helmfile/helmfile/pkg/filesystem"
	"github.com/helmfile/helmfile/pkg/helmexec"
	"github.com/helmfile/helmfile/pkg/remote"
	"github.com/helmfile/helmfile/pkg/testhelper"
)

func TestStorage_resolveFile(t *testing.T) {
	type args struct {
		missingFileHandler *string
		title              string
		path               string
		opts               []resolveFileOption
	}

	cacheDir := remote.CacheDir()
	infoHandler := MissingFileHandlerInfo
	warnHandler := MissingFileHandlerWarn
	errorHandler := MissingFileHandlerError

	tests := []struct {
		name        string
		args        args
		wantFiles   []string
		wantSkipped bool
		wantErr     bool
		skipNonCI   bool
	}{
		{
			name: "non existing file in repo produce skip",
			args: args{
				path:               "git::https://github.com/helmfile/helmfile.git@examples/values/non-existing-file.yaml?ref=v0.145.2",
				title:              "values",
				missingFileHandler: &infoHandler,
			},
			wantSkipped: true,
			wantErr:     false,
		},
		{
			name: "non existing file in repo produce skip",
			args: args{
				path:               "git::https://github.com/helmfile/helmfile.git@examples/values/non-existing-file.yaml?ref=v0.145.2",
				title:              "values",
				missingFileHandler: &errorHandler,
			},
			wantSkipped: false,
			wantErr:     true,
		},
		{
			name: "non existing branch in repo produce error",
			args: args{
				path:               "git::https://github.com/helmfile/helmfile.git@examples/values/non-existing-file.yaml?ref=inexistent-branch-for-test",
				title:              "values",
				missingFileHandler: &infoHandler,
			},
			wantSkipped: false,
			wantErr:     true,
			skipNonCI:   true,
		},
		{
			name: "non existing branch in repo produce info when ignoreMissingGitBranch=true",
			args: args{
				path:               "git::https://github.com/helmfile/helmfile.git@examples/values/non-existing-file.yaml?ref=inexistent-branch-for-test",
				title:              "values",
				missingFileHandler: &infoHandler,
				opts: []resolveFileOption{
					ignoreMissingGitBranch(true),
				},
			},
			wantSkipped: true,
			wantErr:     false,
		},
		{
			name: "non existing branch in repo produce warn when ignoreMissingGitBranch=true",
			args: args{
				path:               "git::https://github.com/helmfile/helmfile.git@examples/values/non-existing-file.yaml?ref=inexistent-branch-for-test",
				title:              "values",
				missingFileHandler: &warnHandler,
				opts: []resolveFileOption{
					ignoreMissingGitBranch(true),
				},
			},
			wantSkipped: true,
			wantErr:     false,
		},
		{
			name: "non existing branch in repo produce error with error handler even if ignoreMissingGitBranch=true",
			args: args{
				path:               "git::https://github.com/helmfile/helmfile.git@examples/values/non-existing-file.yaml?ref=inexistent-branch-for-test",
				title:              "values",
				missingFileHandler: &errorHandler,
				opts: []resolveFileOption{
					ignoreMissingGitBranch(true),
				},
			},
			wantSkipped: false,
			wantErr:     true,
		},
		{
			name: "existing remote value fetched",
			args: args{
				path:               "git::https://github.com/helmfile/helmfile.git@examples/values/replica-values.yaml?ref=v0.145.2",
				title:              "values",
				missingFileHandler: &infoHandler,
			},
			wantFiles:   []string{fmt.Sprintf("%s/%s", cacheDir, "values/https_github_com_helmfile_helmfile_git.ref=v0.145.2/examples/values/replica-values.yaml")},
			wantSkipped: false,
			wantErr:     false,
		},
		{
			// examples/values/ at this tag contains replica-values.yaml plus the
			// dev/ and prod/ directories, so "*.yaml" matches exactly one file.
			name: "wildcard remote value expands to a single match",
			args: args{
				path:               "git::https://github.com/helmfile/helmfile.git@examples/values/*.yaml?ref=v0.145.2",
				title:              "values",
				missingFileHandler: &infoHandler,
			},
			wantFiles:   []string{fmt.Sprintf("%s/%s", cacheDir, "values/https_github_com_helmfile_helmfile_git.ref=v0.145.2/examples/values/replica-values.yaml")},
			wantSkipped: false,
			wantErr:     false,
		},
		{
			name: "non existing remote repo produce an error",
			args: args{
				path:               "https://github.com/helmfile/helmfiles.git@examples/values/replica-values.yaml?ref=v0.145.2",
				title:              "values",
				missingFileHandler: &infoHandler,
			},
			wantSkipped: false,
			wantErr:     true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skipNonCI && os.Getenv("CI") == "" {
				// CI uses HTTPS git remotes while local dev often has SSH configured via
				// git config url."ssh://".insteadOf. This causes different behavior for
				// non-existent branch scenarios.
				t.Skip("skipping test that requires CI environment (git SSH/HTTPS differences)")
			}

			st := NewStorage(cacheDir, helmexec.NewLogger(io.Discard, "debug"), filesystem.DefaultFileSystem())

			files, skipped, err := st.resolveFile(tt.args.missingFileHandler, tt.args.title, tt.args.path, tt.args.opts...)
			if (err != nil) != tt.wantErr {
				t.Errorf("resolveFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(files, tt.wantFiles) {
				t.Errorf("resolveFile() files = %v, want %v", files, tt.wantFiles)
			}
			if skipped != tt.wantSkipped {
				t.Errorf("resolveFile() skipped = %v, want %v", skipped, tt.wantSkipped)
			}
		})
	}
}

// TestStorage_resolveFile_RemoteGlob covers wildcard expansion of remote
// references hermetically, with no network access. It relies on
// Remote.Fetch's cache-hit path: pre-populating the fake filesystem with
// files under the exact cache directory a real Fetch would compute makes
// DirectoryExistsAt true for that directory, so Fetch never invokes a getter.
func TestStorage_resolveFile_RemoteGlob(t *testing.T) {
	cacheDir := "/path/to/helmfile-cache"
	t.Setenv(envvar.CacheHome, cacheDir)

	// Mirrors the cache key Remote.Fetch computes for
	// "git::https://github.com/o/r.git@...?ref=main" (see remote.go's
	// srcDir/cacheKey construction): scheme + host + repo dir, with the
	// existing storage_test.go cases pinning the same replacer behavior.
	base := filepath.Join(cacheDir, "values", "https_github_com_o_r_git.ref=main", "dir")
	aFile := filepath.ToSlash(filepath.Join(base, "a.yaml"))
	bFile := filepath.ToSlash(filepath.Join(base, "b.yaml"))
	txtFile := filepath.ToSlash(filepath.Join(base, "c.txt"))
	subFile := filepath.ToSlash(filepath.Join(base, "sub", "d.yaml"))

	testfs := testhelper.NewTestFs(map[string]string{
		aFile:   "a: 1",
		bFile:   "b: 2",
		txtFile: "c",
		subFile: "d: 4",
	})

	infoHandler := MissingFileHandlerInfo
	errorHandler := MissingFileHandlerError

	tests := []struct {
		name            string
		path            string
		handler         *string
		wantFiles       []string
		wantSkipped     bool
		wantErr         bool
		wantErrContains string
	}{
		{
			name:      "literal file selector still resolves to itself",
			path:      "git::https://github.com/o/r.git@dir/a.yaml?ref=main",
			handler:   &infoHandler,
			wantFiles: []string{aFile},
		},
		{
			name:      "wildcard expands to every match, sorted, non-recursively",
			path:      "git::https://github.com/o/r.git@dir/*.yaml?ref=main",
			handler:   &infoHandler,
			wantFiles: []string{aFile, bFile}, // c.txt excluded by extension, sub/d.yaml excluded: no "**"
		},
		{
			name:        "wildcard matching nothing is skipped under Info handler",
			path:        "git::https://github.com/o/r.git@dir/*.json?ref=main",
			handler:     &infoHandler,
			wantSkipped: true,
		},
		{
			name:    "wildcard matching nothing errors under Error handler",
			path:    "git::https://github.com/o/r.git@dir/*.json?ref=main",
			handler: &errorHandler,
			wantErr: true,
		},
		{
			// An unclosed "[" is a legal filename character (e.g. a literal
			// remote file named "values-[foo.yaml" that doesn't exist at the
			// fetched ref), but it's also invalid filepath.Match syntax
			// (filepath.ErrBadPattern). Before wildcard support this selector
			// was only ever checked with FileExistsAt and simply treated as
			// missing; that behavior must be preserved rather than surfacing a
			// hard "syntax error in pattern" regardless of missingFileHandler.
			name:        "unclosed bracket pattern is treated as no match, not a hard error, under Info handler",
			path:        "git::https://github.com/o/r.git@dir/values-[foo.yaml?ref=main",
			handler:     &infoHandler,
			wantSkipped: true,
		},
		{
			name:            "unclosed bracket pattern still respects the Error handler as a plain missing-file error",
			path:            "git::https://github.com/o/r.git@dir/values-[foo.yaml?ref=main",
			handler:         &errorHandler,
			wantErr:         true,
			wantErrContains: "does not exist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := NewStorage(filepath.Join(cacheDir, "helmfile.yaml"), helmexec.NewLogger(io.Discard, "debug"), testfs.ToFileSystem())

			files, skipped, err := st.resolveFile(tt.handler, "values", tt.path)
			if (err != nil) != tt.wantErr {
				t.Fatalf("resolveFile() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				if tt.wantErrContains != "" && !strings.Contains(err.Error(), tt.wantErrContains) {
					t.Errorf("resolveFile() error = %q, want it to contain %q", err.Error(), tt.wantErrContains)
				}
				return
			}

			wantFiles := append([]string(nil), tt.wantFiles...)
			sort.Strings(wantFiles)
			if !reflect.DeepEqual(files, wantFiles) {
				t.Errorf("resolveFile() files = %v, want %v", files, wantFiles)
			}
			if skipped != tt.wantSkipped {
				t.Errorf("resolveFile() skipped = %v, want %v", skipped, tt.wantSkipped)
			}
		})
	}
}

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		name string
		base string
		path string
		want string
	}{
		{
			name: "unix path relative path",
			base: "/root",
			path: "local/timespan-application.yml",
			want: "/local/timespan-application.yml",
		},
		{
			name: "unix path absolute path",
			base: "/data",
			path: "/root/data/timespan-application.yml",
			want: "/root/data/timespan-application.yml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storageIns := NewStorage(tt.base, helmexec.NewLogger(io.Discard, "debug"), filesystem.DefaultFileSystem())
			if got := storageIns.normalizePath(tt.path); got != tt.want {
				t.Errorf("normalizePath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestJoinBase(t *testing.T) {
	tests := []struct {
		name string
		base string
		path string
		want string
	}{
		{
			name: "joinBase with non-root base",
			base: "/root",
			path: "local/timespan-application.yml",
			want: "/local/timespan-application.yml",
		},
		{
			name: "joinBase with root path",
			base: "/",
			path: "data/timespan-application.yml",
			want: "/data/timespan-application.yml",
		},
		{
			name: "windows joinBase",
			base: "",
			path: "data\\timespan-application.yml",
			want: "data\\timespan-application.yml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storageIns := NewStorage(tt.base, helmexec.NewLogger(io.Discard, "debug"), filesystem.DefaultFileSystem())
			if got := storageIns.JoinBase(tt.path); got != tt.want {
				t.Errorf("JoinBase() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNormalizeSetFilePath(t *testing.T) {
	st := &Storage{
		basePath: "/base/path",
	}

	tests := []struct {
		name     string
		path     string
		expected string
		osGOOS   string
	}{
		{
			name:     "Unix path on Unix",
			path:     "relative/path",
			expected: "/base/path/relative/path",
			osGOOS:   "linux",
		},
		{
			name:     "Windows path on Windows",
			path:     "relative\\path",
			expected: "/base/path/relative\\\\path",
			osGOOS:   "windows",
		},
		{
			name:     "Unix path on Windows",
			path:     "relative/path",
			expected: "/base/path/relative/path",
			osGOOS:   "windows",
		},
		{
			name:     "Absolute path on Unix",
			path:     "/absolute/path",
			expected: "/absolute/path",
			osGOOS:   "linux",
		},
		{
			name:     "Absolute path on Windows",
			path:     "C:\\absolute\\path",
			expected: "C:\\\\absolute\\\\path",
			osGOOS:   "windows",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := st.normalizeSetFilePath(tt.path, tt.osGOOS)
			if tt.osGOOS == "windows" {
				if result != tt.expected {
					t.Errorf("normalizeSetFilePath() = %v, want %v", result, tt.expected)
				}
			} else {
				expectedPath := filepath.Join(st.basePath, tt.path)
				if !filepath.IsAbs(tt.path) {
					if result != expectedPath {
						t.Errorf("normalizeSetFilePath() = %v, want %v", result, expectedPath)
					}
				} else {
					if result != tt.path {
						t.Errorf("normalizeSetFilePath() = %v, want %v", result, tt.path)
					}
				}
			}
		})
	}
}
