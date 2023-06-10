package testutil

import (
	"bytes"
	"io"
	"log"
	"os"
)

// CaptureStdout is a helper function to capture stdout.
func CaptureStdout(f func()) (string, error) {
	reader, writer, err := os.Pipe()
	if err != nil {
		return "", err
	}
	stdout := os.Stdout
	defer func() {
		os.Stdout = stdout
		log.SetOutput(os.Stderr)
	}()
	os.Stdout = writer
	log.SetOutput(writer)
	f()
	_ = writer.Close()
	var buf bytes.Buffer
	_, err = io.Copy(&buf, reader)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}
