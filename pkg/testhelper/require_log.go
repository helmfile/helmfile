package testhelper

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func RequireLog(t *testing.T, dir string, bs *bytes.Buffer) {
	t.Helper()

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
	wantLogData, err := os.ReadFile(wantLogFile)
	updateLogFile := err != nil
	wantLog := string(wantLogData)
	gotLog := bs.String()
	if updateLogFile {
		if err := os.MkdirAll(wantLogFileDir, 0755); err != nil {
			t.Fatalf("unable to create directory %q: %v", wantLogFileDir, err)
		}
		if err := os.WriteFile(wantLogFile, bs.Bytes(), 0644); err != nil {
			t.Fatalf("unable to update lint log snapshot: %v", err)
		}
	}

	diff, exists := Diff(wantLog, gotLog, 3)
	if exists {
		t.Errorf("unexpected log:\nDIFF\n%s\nEOD\nPlease remove %s and rerun the test to recapture this test snapshot", diff, wantLogFile)
	}
}
