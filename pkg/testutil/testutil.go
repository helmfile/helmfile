package testutil

import (
	"bytes"
	"io"
	"log"
	"os"
	"sync"
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
	out := make(chan string)
	wg := new(sync.WaitGroup)
	wg.Add(1)
	var ioCopyErr error
	go func() {
		var buf bytes.Buffer
		_, ioCopyErr = io.Copy(&buf, reader)
		wg.Done()
		out <- buf.String()
	}()
	f()
	_ = writer.Close()
	wg.Wait()
	if ioCopyErr != nil {
		return "", ioCopyErr
	}
	return <-out, nil
}
