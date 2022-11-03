package app

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"regexp"
	"testing"
)

func TestDownloadfile(t *testing.T) {
	var ts *httptest.Server
	cases := []struct {
		name        string
		handler     func(http.ResponseWriter, *http.Request)
		url         string
		filepath    string
		wantContent string
		wantError   string
	}{
		{
			name: "download success",
			handler: func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, "helmfile")
			},
			wantContent: "helmfile",
		},
		{
			name: "download 404",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(404)
				fmt.Fprint(w, "not found")
			},
			wantError: "download .*? error, code: 404",
		},
		{
			name: "download 500",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(500)
				fmt.Fprint(w, "server error")
			},
			wantError: "download .*? error, code: 500",
		},
		{
			name: "download path error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, "helmfile")
			},
			filepath:  "abc/down.txt",
			wantError: "open .*? no such file or directory",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			downfile := path.Join(dir, "down.txt")
			if c.filepath != "" {
				downfile = path.Join(dir, c.filepath)
			}

			ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				c.handler(w, r)
			}))
			defer ts.Close()
			url := ts.URL
			if c.url != "" {
				url = c.url
			}
			err := downloadfile(downfile, url)
			if c.wantError != "" {
				if err == nil {
					t.Errorf("download got no error, want error: %v", c.wantError)
				} else if matched, regexErr := regexp.MatchString(c.wantError, err.Error()); regexErr != nil || !matched {
					t.Errorf("download got error: %v, want error: %v", err, c.wantError)
				}
				return
			}
			content, err := os.ReadFile(downfile)
			if err != nil {
				t.Errorf("read download file error: %v", err)
			}
			if string(content) != c.wantContent {
				t.Errorf("download file content got: %v, want content: %v", string(content), c.wantContent)
			}
		})
	}
}
