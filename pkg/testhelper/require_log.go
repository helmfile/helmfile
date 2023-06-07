package testhelper

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func RequireLog(t *testing.T, dir string, bs *bytes.Buffer) {
	t.Helper()

	// Get the caller pkg used for instruction on rerunning the specific test
	pc, _, _, _ := runtime.Caller(1)
	funcName := runtime.FuncForPC(pc).Name()
	lastSlash := strings.LastIndexByte(funcName, '/')
	if lastSlash < 0 {
		lastSlash = 0
	}
	firstDot := strings.IndexByte(funcName[lastSlash:], '.') + lastSlash
	callerPkg := funcName[:firstDot]

	testNameComponents := strings.Split(t.Name(), "/")
	testBaseName := strings.ToLower(
		strings.ReplaceAll(
			testNameComponents[len(testNameComponents)-1],
			" ",
			"_",
		),
	)
	wantLogFileDir := filepath.Join("testdata", dir)
	wantLogFile := filepath.Join(wantLogFileDir, testBaseName)

	if os.Getenv("HELMFILE_UPDATE_SNAPSHOT") != "" {
		if err := os.MkdirAll(wantLogFileDir, 0755); err != nil {
			t.Fatalf("unable to create directory %q: %v", wantLogFileDir, err)
		}
		if err := os.WriteFile(wantLogFile, bs.Bytes(), 0644); err != nil {
			t.Fatalf("unable to update lint log snapshot: %v", err)
		}
		return
	}

	wantLogData, err := os.ReadFile(wantLogFile)
	if err != nil {
		t.Fatalf(
			"Snapshot file %q does not exist. Rerun this test with `HELMFILE_UPDATE_SNAPSHOT=1 go test -v -run %s %s` to create the snapshot",
			wantLogFile,
			t.Name(),
			callerPkg,
		)
	}

	wantLog := string(wantLogData)
	gotLog := bs.String()

	diff, exists := Diff(wantLog, gotLog, 3)
	if exists {
		t.Errorf("unexpected %s: want (-), got (+): %s", testBaseName, diff)
		t.Errorf(
			"If you think this is due to the snapshot file being outdated, rerun this test with `HELMFILE_UPDATE_SNAPSHOT=1 go test -v -run %s %s` to update the snapshot",
			t.Name(),
			callerPkg,
		)
	}
}
