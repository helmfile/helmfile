package app

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func assertLogEqualsToSnapshot(t *testing.T, data string) {
	t.Helper()

	assertEqualsToSnapshot(t, "log", data)
}

func assertEqualsToSnapshot(t *testing.T, name string, data string) {
	type thisPkgLocator struct{}

	t.Helper()

	snapshotFileName := snapshotFileName(t, name)

	if os.Getenv("HELMFILE_UPDATE_SNAPSHOT") != "" {
		update(t, snapshotFileName, []byte(data))

		return
	}

	wantData, err := os.ReadFile(snapshotFileName)
	if err != nil {
		t.Fatalf(
			"Snapshot file %q does not exist. Rerun this test with `HELMFILE_UPDATE_SNAPSHOT=1 go test -v -run %s %s` to create the snapshot",
			snapshotFileName,
			t.Name(),
			reflect.TypeOf(thisPkgLocator{}).PkgPath(),
		)
	}

	want := string(wantData)

	if d := cmp.Diff(want, data); d != "" {
		t.Errorf("unexpected %s: want (-), got (+): %s", name, d)
		t.Errorf(
			"If you think this is due to the snapshot file being outdated, rerun this test with `HELMFILE_UPDATE_SNAPSHOT=1 go test -v -run %s %s` to update the snapshot",
			t.Name(),
			reflect.TypeOf(thisPkgLocator{}).PkgPath(),
		)
	}
}

func update(t *testing.T, snapshotFileName string, data []byte) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(snapshotFileName), 0755); err != nil {
		t.Fatalf("%v", err)
	}

	if err := os.WriteFile(snapshotFileName, data, 0644); err != nil {
		t.Fatalf("%v", err)
	}
}

func snapshotFileName(t *testing.T, name string) string {
	dir := filepath.Join(strings.Split(strings.ToLower(t.Name()), "/")...)

	return filepath.Join("testdata", dir, name)
}
