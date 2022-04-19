package remote

import (
	"fmt"
	"os"
	"path/filepath"
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

	type testcase struct {
		files          map[string]string
		expectCacheHit bool
	}

	testcases := []testcase{
		{files: cleanfs, expectCacheHit: false},
		{files: cachefs, expectCacheHit: true},
	}

	for i := range testcases {
		testcase := testcases[i]

		t.Run(fmt.Sprintf("case %d", i), func(t *testing.T) {
			testfs := testhelper.NewTestFs(testcase.files)

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
				Logger:     helmexec.NewLogger(os.Stderr, "debug"),
				Home:       CacheDir(),
				Getter:     getter,
				ReadFile:   testfs.ReadFile,
				FileExists: testfs.FileExistsAt,
				DirExists:  testfs.DirectoryExistsAt,
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

			if testcase.expectCacheHit && !hit {
				t.Errorf("unexpected result: unexpected cache miss")
			}
			if !testcase.expectCacheHit && hit {
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
		filepath.Join(CacheDir(), "ssh_github_com_cloudposse_helmfiles_git.ref=0.40.0/releases/kiam.yaml"): "foo: bar",
	}

	type testcase struct {
		files          map[string]string
		expectCacheHit bool
	}

	testcases := []testcase{
		{files: cleanfs, expectCacheHit: false},
		{files: cachefs, expectCacheHit: true},
	}

	for i := range testcases {
		testcase := testcases[i]

		t.Run(fmt.Sprintf("case %d", i), func(t *testing.T) {
			testfs := testhelper.NewTestFs(testcase.files)

			hit := true

			get := func(wd, src, dst string) error {
				if wd != CacheDir() {
					return fmt.Errorf("unexpected wd: %s", wd)
				}
				if src != "git::ssh://git@github.com/cloudposse/helmfiles.git?ref=0.40.0" {
					return fmt.Errorf("unexpected src: %s", src)
				}

				hit = false

				return nil
			}

			getter := &testGetter{
				get: get,
			}
			remote := &Remote{
				Logger:     helmexec.NewLogger(os.Stderr, "debug"),
				Home:       CacheDir(),
				Getter:     getter,
				ReadFile:   testfs.ReadFile,
				FileExists: testfs.FileExistsAt,
				DirExists:  testfs.DirectoryExistsAt,
			}

			url := "git::ssh://git@github.com/cloudposse/helmfiles.git@releases/kiam.yaml?ref=0.40.0"
			file, err := remote.Locate(url)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			expectedFile := filepath.Join(CacheDir(), "ssh_github_com_cloudposse_helmfiles_git.ref=0.40.0/releases/kiam.yaml")
			if file != expectedFile {
				t.Errorf("unexpected file located: %s vs expected: %s", file, expectedFile)
			}

			if testcase.expectCacheHit && !hit {
				t.Errorf("unexpected result: unexpected cache miss")
			}
			if !testcase.expectCacheHit && hit {
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
		filepath.Join(CacheDir(), "ssh_github_com_cloudposse_helmfiles_git.ref=0.40.0_sshkey=redacted/releases/kiam.yaml"): "foo: bar",
	}

	type testcase struct {
		files          map[string]string
		expectCacheHit bool
	}

	testcases := []testcase{
		{files: cleanfs, expectCacheHit: false},
		{files: cachefs, expectCacheHit: true},
	}

	for i := range testcases {
		testcase := testcases[i]

		t.Run(fmt.Sprintf("case %d", i), func(t *testing.T) {
			testfs := testhelper.NewTestFs(testcase.files)

			hit := true

			get := func(wd, src, dst string) error {
				if wd != CacheDir() {
					return fmt.Errorf("unexpected wd: %s", wd)
				}
				if src != "git::ssh://git@github.com/cloudposse/helmfiles.git?ref=0.40.0&sshkey=ZWNkc2Etc2hhMi1uaXN0cDI1NiBBQUFBRTJWalpITmhMWE5vWVRJdGJtbHpkSEF5TlRZQUFBQUlibWx6ZEhBeU5UWUFBQUJCQkJTU3dOY2xoVzQ2Vm9VR3dMQ3JscVRHYUdOVWdRVUVEUEptc1ZzdUViL2RBNUcrQk9YMWxGaUVMYU9HQ2F6bS9KQkR2V3Y2Y0ZDQUtVRjVocVJOUjdJPSA=" {
					return fmt.Errorf("unexpected src: %s", src)
				}

				hit = false

				return nil
			}

			getter := &testGetter{
				get: get,
			}
			remote := &Remote{
				Logger:     helmexec.NewLogger(os.Stderr, "debug"),
				Home:       CacheDir(),
				Getter:     getter,
				ReadFile:   testfs.ReadFile,
				FileExists: testfs.FileExistsAt,
				DirExists:  testfs.DirectoryExistsAt,
			}

			url := "git::ssh://git@github.com/cloudposse/helmfiles.git@releases/kiam.yaml?ref=0.40.0&sshkey=ZWNkc2Etc2hhMi1uaXN0cDI1NiBBQUFBRTJWalpITmhMWE5vWVRJdGJtbHpkSEF5TlRZQUFBQUlibWx6ZEhBeU5UWUFBQUJCQkJTU3dOY2xoVzQ2Vm9VR3dMQ3JscVRHYUdOVWdRVUVEUEptc1ZzdUViL2RBNUcrQk9YMWxGaUVMYU9HQ2F6bS9KQkR2V3Y2Y0ZDQUtVRjVocVJOUjdJPSA="
			file, err := remote.Locate(url)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			expectedFile := filepath.Join(CacheDir(), "ssh_github_com_cloudposse_helmfiles_git.ref=0.40.0_sshkey=redacted/releases/kiam.yaml")
			if file != expectedFile {
				t.Errorf("unexpected file located: %s vs expected: %s", file, expectedFile)
			}

			if testcase.expectCacheHit && !hit {
				t.Errorf("unexpected result: unexpected cache miss")
			}
			if !testcase.expectCacheHit && hit {
				t.Errorf("unexpected result: unexpected cache hit")
			}
		})
	}
}

func TestParse(t *testing.T) {
	type testcase struct {
		input                            string
		getter, scheme, dir, file, query string
		err                              string
	}

	testcases := []testcase{
		{
			input: "raw/incubator",
			err:   "parse url: missing scheme - probably this is a local file path? raw/incubator",
		},
		{
			input:  "git::https://github.com/stakater/Forecastle.git@deployments/kubernetes/chart/forecastle?ref=v1.0.54",
			getter: "git",
			scheme: "https",
			dir:    "/stakater/Forecastle.git",
			file:   "deployments/kubernetes/chart/forecastle",
			query:  "ref=v1.0.54",
		},
	}

	for i := range testcases {
		tc := testcases[i]

		t.Run(fmt.Sprintf("case %d", i), func(t *testing.T) {
			src, err := Parse(tc.input)

			var errMsg string
			if err != nil {
				errMsg = err.Error()
			}

			if diff := cmp.Diff(tc.err, errMsg); diff != "" {
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

			if diff := cmp.Diff(tc.getter, getter); diff != "" {
				t.Fatalf("Unexpected getter:\n%s", diff)
			}

			if diff := cmp.Diff(tc.scheme, scheme); diff != "" {
				t.Fatalf("Unexpected scheme:\n%s", diff)
			}

			if diff := cmp.Diff(tc.file, file); diff != "" {
				t.Fatalf("Unexpected file:\n%s", diff)
			}

			if diff := cmp.Diff(tc.dir, dir); diff != "" {
				t.Fatalf("Unexpected dir:\n%s", diff)
			}

			if diff := cmp.Diff(tc.query, query); diff != "" {
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
