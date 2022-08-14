package state

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/helmfile/helmfile/pkg/helmexec"
	"github.com/helmfile/helmfile/pkg/remote"
)

func TestStorage_resolveFile(t *testing.T) {
	type args struct {
		missingFileHandler *string
		title              string
		path               string
	}

	cacheDir := remote.CacheDir()
	infoHandler := MissingFileHandlerInfo
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
			st := NewStorage(cacheDir, helmexec.NewLogger(os.Stderr, "debug"), filepath.Glob)

			files, skipped, err := st.resolveFile(tt.args.missingFileHandler, tt.args.title, tt.args.path)
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
