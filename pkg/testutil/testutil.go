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
		wg.Done()
		_, ioCopyErr = io.Copy(&buf, reader)
		if ioCopyErr != nil {
			return
		}
		out <- buf.String()
	}()
	wg.Wait()
	f()
	if ioCopyErr != nil {
		return "", ioCopyErr
	}
	_ = writer.Close()
	return <-out, nil
}
