package app

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDownloadfile(t *testing.T) {
	cases := []struct {
		name        string
		handler     func(http.ResponseWriter, *http.Request)
		filepath    string
		wantContent string
		wantError   string
	}{
		{
			name: "successful download of file content",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				fmt.Fprint(w, "helmfile")
			},
			wantContent: "helmfile",
		},
		{
			name: "404 error when file not found",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				fmt.Fprint(w, "not found")
			},
			wantError: "download .*? error, code: 404",
		},
		{
			name: "500 error on server failure",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprint(w, "server error")
			},
			wantError: "download .*? error, code: 500",
		},
		{
			name: "error due to invalid file path",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				fmt.Fprint(w, "helmfile")
			},
			filepath:  "abc/down.txt",
			wantError: "open .*? no such file or directory",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			downfile := filepath.Join(dir, "down.txt")
			if c.filepath != "" {
				downfile = filepath.Join(dir, c.filepath)
			}

			ts := httptest.NewServer(http.HandlerFunc(c.handler))
			defer ts.Close()

			err := downloadfile(downfile, ts.URL)

			if c.wantError != "" {
				assert.Error(t, err)
				if err != nil {
					matched, regexErr := regexp.MatchString(c.wantError, err.Error())
					assert.NoError(t, regexErr)
					assert.True(t, matched, "expected error message to match regex: %s", c.wantError)
				}
				return
			}

			content, err := os.ReadFile(downfile)
			assert.NoError(t, err)
			assert.Equal(t, c.wantContent, string(content), "unexpected content in downloaded file")
		})
	}
}
