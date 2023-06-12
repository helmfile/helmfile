package state

import (
	"fmt"
	"io"
	"reflect"
	"testing"

	"github.com/helmfile/helmfile/pkg/filesystem"
	"github.com/helmfile/helmfile/pkg/helmexec"
	"github.com/helmfile/helmfile/pkg/remote"
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
