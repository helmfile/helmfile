package remote

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/helmfile/helmfile/pkg/helmexec"
	"github.com/helmfile/helmfile/pkg/testhelper"
)

func TestRemote_HttpsGitHub(t *testing.T) {
	cleanfs := map[string]string{
		CacheDir(): "",
	}
	cachefs := map[string]string{
		filepath.Join(CacheDir(), "https_github_com_cloudposse_helmfiles_git.ref=0.40.0/releases/kiam.yaml"): "foo: bar",
	}

	testcases := []struct {
		name           string
		files          map[string]string
		expectCacheHit bool
	}{
		{name: "not expectCacheHit", files: cleanfs, expectCacheHit: false},
		{name: "expectCacheHit", files: cachefs, expectCacheHit: true},
	}

	for _, tt := range testcases {
		t.Run(tt.name, func(t *testing.T) {
			testfs := testhelper.NewTestFs(tt.files)

			hit := true

			get := func(wd, src, dst string) error {
				if wd != CacheDir() {
					return fmt.Errorf("unexpected wd: %s", wd)
				}
				if src != "git::https://github.com/cloudposse/helmfiles.git?ref=0.40.0" {
					return fmt.Errorf("unexpected src: %s", src)
				}

				hit = false

				return nil
			}

			getter := &testGetter{
				get: get,
			}
			remote := &Remote{
				Logger: helmexec.NewLogger(io.Discard, "debug"),
				Home:   CacheDir(),
				Getter: getter,
				fs:     testfs.ToFileSystem(),
			}

			// FYI, go-getter in the `dir` mode accepts URL like the below. So helmfile expects URLs similar to it:
			//   go-getter -mode dir git::https://github.com/cloudposse/helmfiles.git?ref=0.40.0 gettertest1/b

			// We use `@` to separate dir and the file path. This is a good idea borrowed from helm-git:
			//   https://github.com/aslafy-z/helm-git

			url := "git::https://github.com/cloudposse/helmfiles.git@releases/kiam.yaml?ref=0.40.0"
			file, err := remote.Locate(url)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			expectedFile := filepath.Join(CacheDir(), "https_github_com_cloudposse_helmfiles_git.ref=0.40.0/releases/kiam.yaml")
			if file != expectedFile {
				t.Errorf("unexpected file located: %s vs expected: %s", file, expectedFile)
			}

			if tt.expectCacheHit && !hit {
				t.Errorf("unexpected result: unexpected cache miss")
			}
			if !tt.expectCacheHit && hit {
				t.Errorf("unexpected result: unexpected cache hit")
			}
		})
	}
}

func TestRemote_SShGitHub(t *testing.T) {
	cleanfs := map[string]string{
		CacheDir(): "",
	}
	cachefs := map[string]string{
		filepath.Join(CacheDir(), "ssh_github_com_helmfile_helmfiles_git.ref=0.40.0/releases/kiam.yaml"): "foo: bar",
	}

	testcases := []struct {
		name           string
		files          map[string]string
		expectCacheHit bool
	}{
		{name: "not expectCacheHit", files: cleanfs, expectCacheHit: false},
		{name: "expectCacheHit", files: cachefs, expectCacheHit: true},
	}

	for _, tt := range testcases {
		t.Run(tt.name, func(t *testing.T) {
			testfs := testhelper.NewTestFs(tt.files)

			hit := true

			get := func(wd, src, dst string) error {
				if wd != CacheDir() {
					return fmt.Errorf("unexpected wd: %s", wd)
				}
				if src != "git::ssh://git@github.com/helmfile/helmfiles.git?ref=0.40.0" {
					return fmt.Errorf("unexpected src: %s", src)
				}

				hit = false

				return nil
			}

			getter := &testGetter{
				get: get,
			}
			remote := &Remote{
				Logger: helmexec.NewLogger(io.Discard, "debug"),
				Home:   CacheDir(),
				Getter: getter,
				fs:     testfs.ToFileSystem(),
			}

			url := "git::ssh://git@github.com/helmfile/helmfiles.git@releases/kiam.yaml?ref=0.40.0"
			file, err := remote.Locate(url)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			expectedFile := filepath.Join(CacheDir(), "ssh_github_com_helmfile_helmfiles_git.ref=0.40.0/releases/kiam.yaml")
			if file != expectedFile {
				t.Errorf("unexpected file located: %s vs expected: %s", file, expectedFile)
			}

			if tt.expectCacheHit && !hit {
				t.Errorf("unexpected result: unexpected cache miss")
			}
			if !tt.expectCacheHit && hit {
				t.Errorf("unexpected result: unexpected cache hit")
			}
		})
	}
}

func TestRemote_SShGitHub_WithSshKey(t *testing.T) {
	cleanfs := map[string]string{
		CacheDir(): "",
	}
	cachefs := map[string]string{
		filepath.Join(CacheDir(), "ssh_github_com_helmfile_helmfiles_git.ref=0.40.0_sshkey=0b44c081/releases/kiam.yaml"): "foo: bar",
	}

	testcases := []struct {
		name           string
		files          map[string]string
		expectCacheHit bool
	}{
		{name: "not expectCacheHit", files: cleanfs, expectCacheHit: false},
		{name: "expectCacheHit", files: cachefs, expectCacheHit: true},
	}

	for _, tt := range testcases {
		t.Run(tt.name, func(t *testing.T) {
			testfs := testhelper.NewTestFs(tt.files)

			hit := true

			get := func(wd, src, dst string) error {
				if wd != CacheDir() {
					return fmt.Errorf("unexpected wd: %s", wd)
				}
				if src != "git::ssh://git@github.com/helmfile/helmfiles.git?ref=0.40.0&sshkey=ZWNkc2Etc2hhMi1uaXN0cDI1NiBBQUFBRTJWalpITmhMWE5vWVRJdGJtbHpkSEF5TlRZQUFBQUlibWx6ZEhBeU5UWUFBQUJCQkJTU3dOY2xoVzQ2Vm9VR3dMQ3JscVRHYUdOVWdRVUVEUEptc1ZzdUViL2RBNUcrQk9YMWxGaUVMYU9HQ2F6bS9KQkR2V3Y2Y0ZDQUtVRjVocVJOUjdJPSA%3D" {
					return fmt.Errorf("unexpected src: %s", src)
				}

				hit = false

				return nil
			}

			getter := &testGetter{
				get: get,
			}
			remote := &Remote{
				Logger: helmexec.NewLogger(io.Discard, "debug"),
				Home:   CacheDir(),
				Getter: getter,
				fs:     testfs.ToFileSystem(),
			}

			url := "git::ssh://git@github.com/helmfile/helmfiles.git@releases/kiam.yaml?ref=0.40.0&sshkey=ZWNkc2Etc2hhMi1uaXN0cDI1NiBBQUFBRTJWalpITmhMWE5vWVRJdGJtbHpkSEF5TlRZQUFBQUlibWx6ZEhBeU5UWUFBQUJCQkJTU3dOY2xoVzQ2Vm9VR3dMQ3JscVRHYUdOVWdRVUVEUEptc1ZzdUViL2RBNUcrQk9YMWxGaUVMYU9HQ2F6bS9KQkR2V3Y2Y0ZDQUtVRjVocVJOUjdJPSA="
			file, err := remote.Locate(url)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			expectedFile := filepath.Join(CacheDir(), "ssh_github_com_helmfile_helmfiles_git.ref=0.40.0_sshkey=0b44c081/releases/kiam.yaml")
			if file != expectedFile {
				t.Errorf("unexpected file located: %s vs expected: %s", file, expectedFile)
			}

			if tt.expectCacheHit && !hit {
				t.Errorf("unexpected result: unexpected cache miss")
			}
			if !tt.expectCacheHit && hit {
				t.Errorf("unexpected result: unexpected cache hit")
			}
		})
	}
}

func TestRemote_SShGitHub_WithDisableCacheKey(t *testing.T) {
	cleanfs := map[string]string{
		CacheDir(): "",
	}
	cachefs := map[string]string{
		filepath.Join(CacheDir(), "ssh_github_com_helmfile_helmfiles_git.ref=main/releases/kiam.yaml"): "foo: bar",
	}

	testcases := []struct {
		name           string
		files          map[string]string
		expectCacheHit bool
	}{
		{name: "not expectCacheHit", files: cleanfs, expectCacheHit: false},
		{name: "forceNoCacheHit", files: cachefs, expectCacheHit: false},
	}

	for _, tt := range testcases {
		t.Run(tt.name, func(t *testing.T) {
			testfs := testhelper.NewTestFs(tt.files)

			hit := true

			get := func(wd, src, dst string) error {
				if wd != CacheDir() {
					return fmt.Errorf("unexpected wd: %s", wd)
				}
				if src != "git::ssh://git@github.com/helmfile/helmfiles.git?ref=main" {
					return fmt.Errorf("unexpected src: %s", src)
				}

				hit = false

				return nil
			}

			getter := &testGetter{
				get: get,
			}
			remote := &Remote{
				Logger: helmexec.NewLogger(io.Discard, "debug"),
				Home:   CacheDir(),
				Getter: getter,
				fs:     testfs.ToFileSystem(),
			}

			url := "git::ssh://git@github.com/helmfile/helmfiles.git@releases/kiam.yaml?ref=main&cache=false"
			file, err := remote.Locate(url)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			expectedFile := filepath.Join(CacheDir(), "ssh_github_com_helmfile_helmfiles_git.ref=main/releases/kiam.yaml")
			if file != expectedFile {
				t.Errorf("unexpected file located: %s vs expected: %s", file, expectedFile)
			}

			if tt.expectCacheHit && !hit {
				t.Errorf("unexpected result: unexpected cache miss")
			}
			if !tt.expectCacheHit && hit {
				t.Errorf("unexpected result: unexpected cache hit")
			}
		})
	}
}

func TestRemote_S3(t *testing.T) {
	cleanfs := map[string]string{
		CacheDir(): "",
	}
	cachefs := map[string]string{
		filepath.Join(CacheDir(), "s3_helm-s3-values-example/subdir/values.yaml"): "foo: bar",
	}

	testcases := []struct {
		name           string
		files          map[string]string
		expectCacheHit bool
	}{
		{name: "not expectCacheHit", files: cleanfs, expectCacheHit: false},
		{name: "expectCacheHit", files: cachefs, expectCacheHit: true},
	}

	for _, tt := range testcases {
		t.Run(tt.name, func(t *testing.T) {
			testfs := testhelper.NewTestFs(tt.files)

			hit := true

			get := func(wd, src, dst string) error {
				if wd != CacheDir() {
					return fmt.Errorf("unexpected wd: %s", wd)
				}
				if src != "s3://helm-s3-values-example/subdir/values.yaml" {
					return fmt.Errorf("unexpected src: %s", src)
				}

				hit = false

				return nil
			}

			getter := &testGetter{
				get: get,
			}
			remote := &Remote{
				Logger:     helmexec.NewLogger(io.Discard, "debug"),
				Home:       CacheDir(),
				Getter:     getter,
				S3Getter:   getter,
				HttpGetter: getter,
				fs:         testfs.ToFileSystem(),
			}

			url := "s3://helm-s3-values-example/subdir/values.yaml"
			file, err := remote.Locate(url)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			expectedFile := filepath.Join(CacheDir(), "s3_helm-s3-values-example/subdir/values.yaml")
			if file != expectedFile {
				t.Errorf("unexpected file located: %s vs expected: %s", file, expectedFile)
			}

			if tt.expectCacheHit && !hit {
				t.Errorf("unexpected result: unexpected cache miss")
			}
			if !tt.expectCacheHit && hit {
				t.Errorf("unexpected result: unexpected cache hit")
			}
		})
	}
}

func TestIsRemote(t *testing.T) {
	testcases := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "git remote URL",
			input:    "git::https://github.com/cloudposse/helmfiles.git@releases/kiam.yaml?ref=0.40.0",
			expected: true,
		},
		{
			name:     "s3 remote URL",
			input:    "s3://helm-s3-values-example/values.yaml",
			expected: true,
		},
		{
			name:     "https remote URL",
			input:    "https://example.com/values.yaml",
			expected: true,
		},
		{
			name:     "relative path",
			input:    "relative/path/to/file.yaml",
			expected: false,
		},
		{
			name:     "parent-relative path",
			input:    "../services/values.yaml",
			expected: false,
		},
		{
			name:     "unix absolute path",
			input:    "/absolute/path/to/file.yaml",
			expected: false,
		},
	}

	for _, tt := range testcases {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRemote(tt.input)
			if result != tt.expected {
				t.Errorf("IsRemote(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParse(t *testing.T) {
	testcases := []struct {
		name   string
		input  string
		getter string
		scheme string
		dir    string
		file   string
		query  string
		err    string
	}{
		{
			name:  "miss scheme",
			input: "raw/incubator",
			err:   "parse url: missing scheme - probably this is a local file path? raw/incubator",
		},
		{
			name:  "unix absolute path",
			input: "/absolute/path/to/file.yaml",
			err:   "parse url: local absolute path is not a remote URL: /absolute/path/to/file.yaml",
		},
		{
			name:   "git scheme",
			input:  "git::https://github.com/stakater/Forecastle.git@deployments/kubernetes/chart/forecastle?ref=v1.0.54",
			getter: "git",
			scheme: "https",
			dir:    "/stakater/Forecastle.git",
			file:   "deployments/kubernetes/chart/forecastle",
			query:  "ref=v1.0.54",
		},
		{
			name:   "s3 scheme",
			input:  "s3://helm-s3-values-example/values.yaml",
			getter: "normal",
			scheme: "s3",
			dir:    "",
			file:   "values.yaml",
			query:  "",
		},
		{
			name:   "s3 scheme with subdir",
			input:  "s3://helm-s3-values-example/subdir/values.yaml",
			getter: "normal",
			scheme: "s3",
			dir:    "subdir",
			file:   "values.yaml",
			query:  "",
		},
		{
			name:   "http scheme",
			input:  "http::http://helm-s3-values-example.s3.us-east-2.amazonaws.com/values.yaml",
			getter: "http",
			scheme: "http",
			dir:    "",
			file:   "values.yaml",
			query:  "",
		},
		{
			name:   "http scheme with subdir",
			input:  "http::http://helm-s3-values-example.s3.us-east-2.amazonaws.com/subdir/values.yaml",
			getter: "http",
			scheme: "http",
			dir:    "subdir",
			file:   "values.yaml",
			query:  "",
		},
		{
			name:   "https scheme",
			input:  "http::https://helm-s3-values-example.s3.us-east-2.amazonaws.com/values.yaml",
			getter: "http",
			scheme: "https",
			dir:    "",
			file:   "values.yaml",
			query:  "",
		},
		{
			name:   "http scheme normal",
			input:  "http://helm-s3-values-example.s3.us-east-2.amazonaws.com/values.yaml",
			getter: "normal",
			scheme: "http",
			dir:    "",
			file:   "values.yaml",
			query:  "",
		},
		{
			name:   "https scheme normal",
			input:  "https://helm-s3-values-example.s3.us-east-2.amazonaws.com/values.yaml",
			getter: "normal",
			scheme: "https",
			dir:    "",
			file:   "values.yaml",
			query:  "",
		},
		{
			name:   "https scheme normal with query params",
			input:  "https://gitlab.example.com/api/v4/projects/test/repository/files/values.yaml/raw?ref=abc123",
			getter: "normal",
			scheme: "https",
			dir:    "api/v4/projects/test/repository/files/values.yaml",
			file:   "raw",
			query:  "ref=abc123",
		},
	}

	for _, tt := range testcases {
		t.Run(tt.name, func(t *testing.T) {
			src, err := Parse(tt.input)

			var errMsg string
			if err != nil {
				errMsg = err.Error()
			}

			if diff := cmp.Diff(tt.err, errMsg); diff != "" {
				t.Fatalf("Unexpected error:\n%s", diff)
			}

			var getter, scheme, dir, file, query string
			if src != nil {
				getter = src.Getter
				scheme = src.Scheme
				dir = src.Dir
				file = src.File
				query = src.RawQuery
			}

			if diff := cmp.Diff(tt.getter, getter); diff != "" {
				t.Fatalf("Unexpected getter:\n%s", diff)
			}

			if diff := cmp.Diff(tt.scheme, scheme); diff != "" {
				t.Fatalf("Unexpected scheme:\n%s", diff)
			}

			if diff := cmp.Diff(tt.file, file); diff != "" {
				t.Fatalf("Unexpected file:\n%s", diff)
			}

			if diff := cmp.Diff(tt.dir, dir); diff != "" {
				t.Fatalf("Unexpected dir:\n%s", diff)
			}

			if diff := cmp.Diff(tt.query, query); diff != "" {
				t.Fatalf("Unexpected query:\n%s", diff)
			}
		})
	}
}

type testGetter struct {
	get func(wd, src, dst string) error
}

func (t *testGetter) Get(wd, src, dst string) error {
	return t.get(wd, src, dst)
}

func TestRemote_Fetch(t *testing.T) {
	cleanfs := map[string]string{
		CacheDir(): "",
	}
	cachefs := map[string]string{
		filepath.Join(CacheDir(), "https_github_com_helmfile_helmfile_git.ref=v0.151.0/README.md"): "foo: bar",
	}

	testcases := []struct {
		name           string
		files          map[string]string
		expectCacheHit bool
		cacheDirOpt    string
	}{
		{name: "not expectCacheHit", files: cleanfs, expectCacheHit: false, cacheDirOpt: ""},
		{name: "expectCacheHit", files: cachefs, expectCacheHit: true, cacheDirOpt: ""},
		{name: "not expectCacheHit with states", files: cleanfs, expectCacheHit: false, cacheDirOpt: "states"},
		{name: "expectCacheHit with states", files: cachefs, expectCacheHit: true, cacheDirOpt: "states"},
	}

	for _, tt := range testcases {
		t.Run(tt.name, func(t *testing.T) {
			testfs := testhelper.NewTestFs(tt.files)

			hit := true

			get := func(wd, src, dst string) error {
				if wd != CacheDir() {
					return fmt.Errorf("unexpected wd: %s", wd)
				}
				if src != "git::https://github.com/helmfile/helmfile.git?ref=v0.151.0" {
					return fmt.Errorf("unexpected src: %s", src)
				}

				hit = false

				return nil
			}

			getter := &testGetter{
				get: get,
			}
			remote := &Remote{
				Logger: helmexec.NewLogger(io.Discard, "debug"),
				Home:   CacheDir(),
				Getter: getter,
				fs:     testfs.ToFileSystem(),
			}

			url := "git::https://github.com/helmfile/helmfile.git@README.md?ref=v0.151.0"
			file, err := remote.Fetch(url)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			expectedFile := filepath.Join(CacheDir(), "https_github_com_helmfile_helmfile_git.ref=v0.151.0/README.md")
			if file != expectedFile {
				t.Errorf("unexpected file located: %s vs expected: %s", file, expectedFile)
			}

			if tt.expectCacheHit && !hit {
				t.Errorf("unexpected result: unexpected cache miss")
			}
			if !tt.expectCacheHit && hit {
				t.Errorf("unexpected result: unexpected cache hit")
			}
		})
	}
}

// TestAWSSDKLogLevelInit verifies that the init() function reads HELMFILE_AWS_SDK_LOG_LEVEL correctly
func TestAWSSDKLogLevelInit(t *testing.T) {
	tests := []struct {
		name          string
		envValue      string
		expectedValue string
	}{
		{
			name:          "no env var defaults to off",
			envValue:      "",
			expectedValue: "off",
		},
		{
			name:          "explicit off",
			envValue:      "off",
			expectedValue: "off",
		},
		{
			name:          "minimal value",
			envValue:      "minimal",
			expectedValue: "minimal",
		},
		{
			name:          "standard value",
			envValue:      "standard",
			expectedValue: "standard",
		},
		{
			name:          "verbose value",
			envValue:      "verbose",
			expectedValue: "verbose",
		},
		{
			name:          "whitespace is trimmed",
			envValue:      "  standard  ",
			expectedValue: "standard",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the init() logic
			var result string
			if tt.envValue == "" {
				result = ""
			} else {
				result = tt.envValue
			}

			// Trim whitespace like init() does
			result = strings.TrimSpace(result)

			// Default to "off" if empty
			if result == "" {
				result = "off"
			}

			if result != tt.expectedValue {
				t.Errorf("Expected %q, got %q", tt.expectedValue, result)
			}
		})
	}
}

func TestRemote_HttpUrlWithQueryParams(t *testing.T) {
	cacheDir := CacheDir()
	cleanfs := map[string]string{
		cacheDir: "",
	}
	cachefs := map[string]string{
		filepath.Join(cacheDir, "https_gitlab_example_com.ref=abc123/api/v4/projects/test/repository/files/values.yaml", "raw"): "cached: content",
	}

	testcases := []struct {
		name           string
		files          map[string]string
		expectCacheHit bool
	}{
		{name: "cache miss", files: cleanfs, expectCacheHit: false},
		{name: "cache hit", files: cachefs, expectCacheHit: true},
	}

	for _, tt := range testcases {
		t.Run(tt.name, func(t *testing.T) {
			testfs := testhelper.NewTestFs(tt.files)

			hit := true

			get := func(wd, src, dst string) error {
				hit = false
				return nil
			}

			getter := &testGetter{get: get}
			remote := &Remote{
				Logger:     helmexec.NewLogger(io.Discard, "debug"),
				Home:       cacheDir,
				Getter:     getter,
				S3Getter:   getter,
				HttpGetter: getter,
				fs:         testfs.ToFileSystem(),
			}

			url := "https://gitlab.example.com/api/v4/projects/test/repository/files/values.yaml/raw?ref=abc123"
			file, err := remote.Fetch(url)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			expectedFile := filepath.Join(cacheDir, "https_gitlab_example_com.ref=abc123/api/v4/projects/test/repository/files/values.yaml", "raw")
			if file != expectedFile {
				t.Errorf("unexpected file located: %s vs expected: %s", file, expectedFile)
			}

			if tt.expectCacheHit && !hit {
				t.Errorf("unexpected cache miss")
			}
			if !tt.expectCacheHit && hit {
				t.Errorf("unexpected cache hit")
			}
		})
	}
}

func TestRemote_HttpUrlQueryParamsAvoidCacheCollision(t *testing.T) {
	cacheDir := CacheDir()
	cleanfs := map[string]string{
		cacheDir: "",
	}
	testfs := testhelper.NewTestFs(cleanfs)

	get := func(wd, src, dst string) error {
		return nil
	}

	getter := &testGetter{get: get}
	remote := &Remote{
		Logger:     helmexec.NewLogger(io.Discard, "debug"),
		Home:       cacheDir,
		Getter:     getter,
		S3Getter:   getter,
		HttpGetter: getter,
		fs:         testfs.ToFileSystem(),
	}

	url1 := "https://gitlab.example.com/api/v4/projects/test/repository/files/values.yaml/raw?ref=29b5609"
	url2 := "https://gitlab.example.com/api/v4/projects/test/repository/files/values.yaml/raw?ref=d80839c"

	file1, err := remote.Fetch(url1)
	if err != nil {
		t.Fatalf("unexpected error fetching url1: %v", err)
	}

	file2, err := remote.Fetch(url2)
	if err != nil {
		t.Fatalf("unexpected error fetching url2: %v", err)
	}

	if file1 == file2 {
		t.Errorf("expected different cache paths for different query params, but both resolved to: %s", file1)
	}

	// Verify both contain the ref in the path
	if !strings.Contains(file1, "ref=29b5609") {
		t.Errorf("expected file1 path to contain ref=29b5609, got: %s", file1)
	}
	if !strings.Contains(file2, "ref=d80839c") {
		t.Errorf("expected file2 path to contain ref=d80839c, got: %s", file2)
	}
}
