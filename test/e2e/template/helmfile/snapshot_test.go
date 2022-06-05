package helmfile

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestHelmfileTemplateWithBuildCommand(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	projectRoot := filepath.Join(filepath.Dir(filename), "..", "..", "..", "..")
	helmfileBin := filepath.Join(projectRoot, "helmfile")
	testdataDir := "testdata/snapshot"

	entries, err := os.ReadDir(testdataDir)
	require.NoError(t, err)

	for _, e := range entries {
		if !e.IsDir() {
			t.Fatalf("Unexpected type of entry at %s", e.Name())
		}

		name := e.Name()

		t.Run(name, func(t *testing.T) {
			inputFile := filepath.Join(testdataDir, name, "input.yaml")

			want, err := os.ReadFile(filepath.Join(testdataDir, name, "output.yaml"))
			require.NoError(t, err)

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			cmd := exec.CommandContext(ctx, helmfileBin, "-f", inputFile, "build")
			got, err := cmd.CombinedOutput()
			if err != nil {
				t.Logf("%s", string(got))
			}
			require.NoError(t, err)

			require.Equal(t, string(want), string(got))
		})
	}
}
