package app

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"
	"go.uber.org/zap"

	"github.com/helmfile/helmfile/pkg/helmexec"
)

func runWithLogCapture(t *testing.T, f func(*testing.T, *zap.SugaredLogger)) *bytes.Buffer {
	t.Helper()

	bs := &bytes.Buffer{}

	logReader, logWriter := io.Pipe()

	logFlushed := &sync.WaitGroup{}
	// Ensure all the log is consumed into `bs` by calling `logWriter.Close()` followed by `logFlushed.Wait()`
	logFlushed.Add(1)
	go func() {
		scanner := bufio.NewScanner(logReader)
		for scanner.Scan() {
			bs.Write(scanner.Bytes())
			bs.WriteString("\n")
		}
		logFlushed.Done()
	}()

	defer func() {
		// This is here to avoid data-trace on bytes buffer `bs` to capture logs
		if err := logWriter.Close(); err != nil {
			panic(err)
		}
		logFlushed.Wait()
	}()

	logger := helmexec.NewLogger(logWriter, "debug")

	f(t, logger)

	return bs
}

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
