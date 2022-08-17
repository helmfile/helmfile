package app

import (
	"bufio"
	"bytes"
	"io"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/variantdev/vals"

	ffs "github.com/helmfile/helmfile/pkg/filesystem"
	"github.com/helmfile/helmfile/pkg/helmexec"
	"github.com/helmfile/helmfile/pkg/testhelper"
	"github.com/helmfile/helmfile/pkg/testutil"
)

func TestListWithEnvironment(t *testing.T) {
	type testcase struct {
		environment string
		ns          string
		error       string
		selectors   []string
		expected    string
	}

	check := func(t *testing.T, tc testcase) {
		t.Helper()

		bs := &bytes.Buffer{}

		func() {
			t.Helper()

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

			valsRuntime, err := vals.New(vals.Options{CacheSize: 32})
			if err != nil {
				t.Errorf("unexpected error creating vals runtime: %v", err)
			}

			files := map[string]string{
				"/path/to/helmfile.d/helmfile_1.yaml": `
environments:
  development: {}
  shared: {}

releases:
- name: logging
  chart: incubator/raw
  namespace: kube-system

- name: kubernetes-external-secrets
  chart: incubator/raw
  namespace: kube-system
  needs:
  - kube-system/logging

- name: external-secrets
  chart: incubator/raw
  namespace: default
  labels:
    app: test
  needs:
  - kube-system/kubernetes-external-secrets

- name: my-release
  chart: incubator/raw
  namespace: default
  labels:
    app: test
  needs:
  - default/external-secrets


# Disabled releases are treated as missing
- name: disabled
  chart: incubator/raw
  namespace: kube-system
  installed: false

- name: test2
  chart: incubator/raw
  needs:
  - kube-system/disabled

- name: test3
  chart: incubator/raw
  needs:
  - test2
`,
				"/path/to/helmfile.d/helmfile_2.yaml": `
environments:
  test: {}
  shared: {}

repositories:
- name: bitnami
  url: https://charts.bitnami.com/bitnami

releases:
- name: cache
  namespace: my-app
  chart: bitnami/redis
  version: 17.0.7
  labels:
    app: test

- name: database
  namespace: my-app
  chart: bitnami/postgres
  version: 11.6.22
`,
				"/path/to/helmfile.d/helmfile_3.yaml": `
releases:
- name: global
  chart: incubator/raw
  namespace: kube-system
`,
			}

			app := appWithFs(&App{
				OverrideHelmBinary:  DefaultHelmBinary,
				fs:                  ffs.DefaultFileSystem(),
				OverrideKubeContext: "default",
				Env:                 tc.environment,
				Logger:              logger,
				valsRuntime:         valsRuntime,
			}, files)

			expectNoCallsToHelm(app)

			if tc.ns != "" {
				app.Namespace = tc.ns
			}

			if tc.selectors != nil {
				app.Selectors = tc.selectors
			}

			var listErr error
			out := testutil.CaptureStdout(func() {
				listErr = app.ListReleases(configImpl{})
			})

			var gotErr string
			if listErr != nil {
				gotErr = listErr.Error()
			}

			if d := cmp.Diff(tc.error, gotErr); d != "" {
				t.Fatalf("unexpected error: want (-), got (+): %s", d)
			}

			assert.Equal(t, tc.expected, out)
		}()

		testhelper.RequireLog(t, "app_list_test", bs)
	}

	t.Run("default environment includes all releases", func(t *testing.T) {
		check(t, testcase{
			environment: "default",
			expected: `NAME                       	NAMESPACE  	ENABLED	INSTALLED	LABELS  	CHART           	VERSION
logging                    	kube-system	true   	true     	        	incubator/raw   	       
kubernetes-external-secrets	kube-system	true   	true     	        	incubator/raw   	       
external-secrets           	default    	true   	true     	app:test	incubator/raw   	       
my-release                 	default    	true   	true     	app:test	incubator/raw   	       
disabled                   	kube-system	true   	false    	        	incubator/raw   	       
test2                      	           	true   	true     	        	incubator/raw   	       
test3                      	           	true   	true     	        	incubator/raw   	       
cache                      	my-app     	true   	true     	app:test	bitnami/redis   	17.0.7 
database                   	my-app     	true   	true     	        	bitnami/postgres	11.6.22
global                     	kube-system	true   	true     	        	incubator/raw   	       
`,
		})
	})

	t.Run("fail on unknown environment", func(t *testing.T) {
		check(t, testcase{
			environment: "staging",
			error:       `err: no releases found that matches specified selector() and environment(staging), in any helmfile`,
		})
	})

	t.Run("list releases matching selector and environment", func(t *testing.T) {
		check(t, testcase{
			environment: "development",
			selectors:   []string{"app=test"},
			expected: `NAME            	NAMESPACE	ENABLED	INSTALLED	LABELS                                                    	CHART        	VERSION
external-secrets	default  	true   	true     	app:test,chart:raw,name:external-secrets,namespace:default	incubator/raw	       
my-release      	default  	true   	true     	app:test,chart:raw,name:my-release,namespace:default      	incubator/raw	       
`,
		})
	})

	t.Run("filters releases for environment used in one file only", func(t *testing.T) {
		check(t, testcase{
			environment: "test",
			expected: `NAME    	NAMESPACE	ENABLED	INSTALLED	LABELS  	CHART           	VERSION
cache   	my-app   	true   	true     	app:test	bitnami/redis   	17.0.7 
database	my-app   	true   	true     	        	bitnami/postgres	11.6.22
`,
		})
	})

	t.Run("filters releases for environment used in multiple files", func(t *testing.T) {
		check(t, testcase{
			environment: "shared",
			// 'global' release has no environments, so is still excluded
			expected: `NAME                       	NAMESPACE  	ENABLED	INSTALLED	LABELS  	CHART           	VERSION
logging                    	kube-system	true   	true     	        	incubator/raw   	       
kubernetes-external-secrets	kube-system	true   	true     	        	incubator/raw   	       
external-secrets           	default    	true   	true     	app:test	incubator/raw   	       
my-release                 	default    	true   	true     	app:test	incubator/raw   	       
disabled                   	kube-system	true   	false    	        	incubator/raw   	       
test2                      	           	true   	true     	        	incubator/raw   	       
test3                      	           	true   	true     	        	incubator/raw   	       
cache                      	my-app     	true   	true     	app:test	bitnami/redis   	17.0.7 
database                   	my-app     	true   	true     	        	bitnami/postgres	11.6.22
`,
		})
	})
}
