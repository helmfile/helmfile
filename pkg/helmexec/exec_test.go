package helmexec

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/helmfile/helmfile/pkg/envvar"

	"github.com/Masterminds/semver/v3"
	"go.uber.org/zap"
)

// Mocking the command-line runner

type mockRunner struct {
	output []byte
	err    error
}

func (mock *mockRunner) ExecuteStdIn(cmd string, args []string, env map[string]string, stdin io.Reader) ([]byte, error) {
	return mock.output, mock.err
}

func (mock *mockRunner) Execute(cmd string, args []string, env map[string]string) ([]byte, error) {
	return mock.output, mock.err
}

func MockExecer(logger *zap.SugaredLogger, kubeContext string) *execer {
	execer := New("helm", logger, kubeContext, &mockRunner{})
	return execer
}

// Test methods

func TestNewHelmExec(t *testing.T) {
	buffer := bytes.NewBufferString("something")
	helm := MockExecer(NewLogger(buffer, "debug"), "dev")
	if helm.kubeContext != "dev" {
		t.Error("helmexec.New() - kubeContext")
	}
	if buffer.String() != "something" {
		t.Error("helmexec.New() - changed buffer")
	}
	if len(helm.extra) != 0 {
		t.Error("helmexec.New() - extra args not empty")
	}
}

func Test_SetExtraArgs(t *testing.T) {
	helm := MockExecer(NewLogger(os.Stdout, "info"), "dev")
	helm.SetExtraArgs()
	if len(helm.extra) != 0 {
		t.Error("helmexec.SetExtraArgs() - passing no arguments should not change extra field")
	}
	helm.SetExtraArgs("foo")
	if !reflect.DeepEqual(helm.extra, []string{"foo"}) {
		t.Error("helmexec.SetExtraArgs() - one extra argument missing")
	}
	helm.SetExtraArgs("alpha", "beta")
	if !reflect.DeepEqual(helm.extra, []string{"alpha", "beta"}) {
		t.Error("helmexec.SetExtraArgs() - two extra arguments missing (overwriting the previous value)")
	}
}

func Test_SetHelmBinary(t *testing.T) {
	helm := MockExecer(NewLogger(os.Stdout, "info"), "dev")
	if helm.helmBinary != "helm" {
		t.Error("helmexec.command - default command is not helm")
	}
	helm.SetHelmBinary("foo")
	if helm.helmBinary != "foo" {
		t.Errorf("helmexec.SetHelmBinary() - actual = %s expect = foo", helm.helmBinary)
	}
}

func Test_AddRepo_Helm_3_3_2(t *testing.T) {
	var buffer bytes.Buffer
	logger := NewLogger(&buffer, "debug")
	helm := &execer{
		helmBinary:  "helm",
		version:     *semver.MustParse("3.3.2"),
		logger:      logger,
		kubeContext: "dev",
		runner:      &mockRunner{},
	}
	err := helm.AddRepo("myRepo", "https://repo.example.com/", "", "cert.pem", "key.pem", "", "", "", "", "")
	expected := `Adding repo myRepo https://repo.example.com/
exec: helm --kube-context dev repo add myRepo https://repo.example.com/ --force-update --cert-file cert.pem --key-file key.pem
`
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if buffer.String() != expected {
		t.Errorf("helmexec.AddRepo()\nactual = %v\nexpect = %v", buffer.String(), expected)
	}
}

func Test_AddRepo(t *testing.T) {
	var buffer bytes.Buffer
	logger := NewLogger(&buffer, "debug")
	helm := MockExecer(logger, "dev")
	err := helm.AddRepo("myRepo", "https://repo.example.com/", "", "cert.pem", "key.pem", "", "", "", "", "")
	expected := `Adding repo myRepo https://repo.example.com/
exec: helm --kube-context dev repo add myRepo https://repo.example.com/ --cert-file cert.pem --key-file key.pem
`
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if buffer.String() != expected {
		t.Errorf("helmexec.AddRepo()\nactual = %v\nexpect = %v", buffer.String(), expected)
	}

	buffer.Reset()
	err = helm.AddRepo("myRepo", "https://repo.example.com/", "ca.crt", "", "", "", "", "", "", "")
	expected = `Adding repo myRepo https://repo.example.com/
exec: helm --kube-context dev repo add myRepo https://repo.example.com/ --ca-file ca.crt
`
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if buffer.String() != expected {
		t.Errorf("helmexec.AddRepo()\nactual = %v\nexpect = %v", buffer.String(), expected)
	}

	buffer.Reset()
	err = helm.AddRepo("myRepo", "https://repo.example.com/", "", "", "", "", "", "", "", "")
	expected = `Adding repo myRepo https://repo.example.com/
exec: helm --kube-context dev repo add myRepo https://repo.example.com/
`
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if buffer.String() != expected {
		t.Errorf("helmexec.AddRepo()\nactual = %v\nexpect = %v", buffer.String(), expected)
	}

	buffer.Reset()
	err = helm.AddRepo("acrRepo", "", "", "", "", "", "", "acr", "", "")
	expected = `Adding repo acrRepo (acr)
exec: az acr helm repo add --name acrRepo
exec: az acr helm repo add --name acrRepo: 
`
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if buffer.String() != expected {
		t.Errorf("helmexec.AddRepo()\nactual = %v\nexpect = %v", buffer.String(), expected)
	}

	buffer.Reset()
	err = helm.AddRepo("otherRepo", "", "", "", "", "", "", "unknown", "", "")
	expected = `ERROR: unknown type 'unknown' for repository otherRepo
`
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if buffer.String() != expected {
		t.Errorf("helmexec.AddRepo()\nactual = %v\nexpect = %v", buffer.String(), expected)
	}

	buffer.Reset()
	err = helm.AddRepo("myRepo", "https://repo.example.com/", "", "", "", "example_user", "example_password", "", "", "")
	expected = `Adding repo myRepo https://repo.example.com/
exec: helm --kube-context dev repo add myRepo https://repo.example.com/ --username example_user --password example_password
`
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if buffer.String() != expected {
		t.Errorf("helmexec.AddRepo()\nactual = %v\nexpect = %v", buffer.String(), expected)
	}

	buffer.Reset()
	err = helm.AddRepo("", "https://repo.example.com/", "", "", "", "", "", "", "", "")
	expected = `empty field name

`
	if err != nil && err.Error() != "empty field name" {
		t.Errorf("unexpected error: %v", err)
	}
	if buffer.String() != expected {
		t.Errorf("helmexec.AddRepo()\nactual = %v\nexpect = %v", buffer.String(), expected)
	}

	buffer.Reset()
	err = helm.AddRepo("myRepo", "https://repo.example.com/", "", "", "", "example_user", "example_password", "", "true", "")
	expected = `Adding repo myRepo https://repo.example.com/
exec: helm --kube-context dev repo add myRepo https://repo.example.com/ --username example_user --password example_password --pass-credentials
`
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if buffer.String() != expected {
		t.Errorf("helmexec.AddRepo()\nactual = %v\nexpect = %v", buffer.String(), expected)
	}

	buffer.Reset()
	err = helm.AddRepo("myRepo", "https://repo.example.com/", "", "", "", "", "", "", "", "true")
	expected = `Adding repo myRepo https://repo.example.com/
exec: helm --kube-context dev repo add myRepo https://repo.example.com/ --insecure-skip-tls-verify
`
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if buffer.String() != expected {
		t.Errorf("helmexec.AddRepo()\nactual = %v\nexpect = %v", buffer.String(), expected)
	}
}

func Test_UpdateRepo(t *testing.T) {
	var buffer bytes.Buffer
	logger := NewLogger(&buffer, "debug")
	helm := MockExecer(logger, "dev")
	err := helm.UpdateRepo()
	expected := `Updating repo
exec: helm --kube-context dev repo update
`
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if buffer.String() != expected {
		t.Errorf("helmexec.UpdateRepo()\nactual = %v\nexpect = %v", buffer.String(), expected)
	}
}

func Test_SyncRelease(t *testing.T) {
	var buffer bytes.Buffer
	logger := NewLogger(&buffer, "debug")
	helm := MockExecer(logger, "dev")
	err := helm.SyncRelease(HelmContext{}, "release", "chart", "--timeout 10", "--wait", "--wait-for-jobs")
	expected := `Upgrading release=release, chart=chart
exec: helm --kube-context dev upgrade --install --reset-values release chart --timeout 10 --wait --wait-for-jobs
`
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if buffer.String() != expected {
		t.Errorf("helmexec.SyncRelease()\nactual = %v\nexpect = %v", buffer.String(), expected)
	}

	buffer.Reset()
	err = helm.SyncRelease(HelmContext{}, "release", "chart")
	expected = `Upgrading release=release, chart=chart
exec: helm --kube-context dev upgrade --install --reset-values release chart
`
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if buffer.String() != expected {
		t.Errorf("helmexec.SyncRelease()\nactual = %v\nexpect = %v", buffer.String(), expected)
	}

	buffer.Reset()
	err = helm.SyncRelease(HelmContext{}, "release", "https://example_user:example_password@repo.example.com/chart.tgz")
	expected = `Upgrading release=release, chart=https://example_user:xxxxx@repo.example.com/chart.tgz
exec: helm --kube-context dev upgrade --install --reset-values release https://example_user:example_password@repo.example.com/chart.tgz
`
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if buffer.String() != expected {
		t.Errorf("helmexec.SyncRelease()\nactual = %v\nexpect = %v", buffer.String(), expected)
	}
}

func Test_SyncReleaseTillerless(t *testing.T) {
	var buffer bytes.Buffer
	logger := NewLogger(&buffer, "debug")
	helm := MockExecer(logger, "dev")
	err := helm.SyncRelease(HelmContext{Tillerless: true, TillerNamespace: "foo"}, "release", "chart",
		"--timeout 10", "--wait", "--wait-for-jobs")
	expected := `Upgrading release=release, chart=chart
exec: helm --kube-context dev tiller run foo -- helm upgrade --install --reset-values release chart --timeout 10 --wait --wait-for-jobs
`
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if buffer.String() != expected {
		t.Errorf("helmexec.SyncRelease()\nactual = %v\nexpect = %v", buffer.String(), expected)
	}
}

func Test_UpdateDeps(t *testing.T) {
	var buffer bytes.Buffer
	logger := NewLogger(&buffer, "debug")
	helm := MockExecer(logger, "dev")
	err := helm.UpdateDeps("./chart/foo")
	expected := `Updating dependency ./chart/foo
exec: helm --kube-context dev dependency update ./chart/foo
`
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if buffer.String() != expected {
		t.Errorf("helmexec.UpdateDeps()\nactual = %v\nexpect = %v", buffer.String(), expected)
	}

	buffer.Reset()
	helm.SetExtraArgs("--verify")
	err = helm.UpdateDeps("./chart/foo")
	expected = `Updating dependency ./chart/foo
exec: helm --kube-context dev dependency update ./chart/foo --verify
`
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if buffer.String() != expected {
		t.Errorf("helmexec.AddRepo()\nactual = %v\nexpect = %v", buffer.String(), expected)
	}
}

func Test_BuildDeps(t *testing.T) {
	var buffer bytes.Buffer
	logger := NewLogger(&buffer, "debug")
	helm := MockExecer(logger, "dev")
	err := helm.BuildDeps("foo", "./chart/foo")
	expected := `Building dependency release=foo, chart=./chart/foo
exec: helm --kube-context dev dependency build ./chart/foo
`
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if buffer.String() != expected {
		t.Errorf("helmexec.BuildDeps()\nactual = %v\nexpect = %v", buffer.String(), expected)
	}

	buffer.Reset()
	helm.SetExtraArgs("--verify")
	err = helm.BuildDeps("foo", "./chart/foo")
	expected = `Building dependency release=foo, chart=./chart/foo
exec: helm --kube-context dev dependency build ./chart/foo --verify
`
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if buffer.String() != expected {
		t.Errorf("helmexec.BuildDeps()\nactual = %v\nexpect = %v", buffer.String(), expected)
	}
}

func Test_DecryptSecret(t *testing.T) {
	var buffer bytes.Buffer
	logger := NewLogger(&buffer, "debug")
	helm := MockExecer(logger, "dev")

	tmpFilePath := "path/to/temp/file"
	helm.writeTempFile = func(content []byte) (string, error) {
		return tmpFilePath, nil
	}

	_, err := helm.DecryptSecret(HelmContext{}, "secretName")
	if err != nil {
		if _, ok := err.(*os.PathError); ok {
		} else {
			t.Errorf("Error: %v", err)
		}
	}
	cwd, err := filepath.Abs(".")
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	// Run again for caching
	_, err = helm.DecryptSecret(HelmContext{}, "secretName")

	expected := fmt.Sprintf(`Preparing to decrypt secret %v/secretName
Decrypting secret %s/secretName
exec: helm --kube-context dev secrets dec %s/secretName
Preparing to decrypt secret %s/secretName
Found secret in cache %s/secretName
`, cwd, cwd, cwd, cwd, cwd)
	if err != nil {
		if _, ok := err.(*os.PathError); ok {
		} else {
			t.Errorf("Error: %v", err)
		}
	}
	if d := cmp.Diff(expected, buffer.String()); d != "" {
		t.Errorf("helmexec.DecryptSecret(): want (-), got (+):\n%s", d)
	}
}

func Test_DecryptSecretWithGotmpl(t *testing.T) {
	var buffer bytes.Buffer
	logger := NewLogger(&buffer, "debug")
	helm := MockExecer(logger, "dev")

	tmpFilePath := "path/to/temp/file"
	helm.writeTempFile = func(content []byte) (string, error) {
		return tmpFilePath, nil
	}

	secretName := "secretName.yaml.gotmpl"
	_, decryptErr := helm.DecryptSecret(HelmContext{}, secretName)
	cwd, err := filepath.Abs(".")
	if err != nil {
		t.Errorf("Error: %v", err)
	}

	expected := fmt.Sprintf(`%s/%s.yaml.dec`, cwd, secretName)
	if d := cmp.Diff(expected, decryptErr.(*os.PathError).Path); d != "" {
		t.Errorf("helmexec.DecryptSecret(): want (-), got (+):\n%s", d)
	}
}

func Test_DiffRelease(t *testing.T) {
	var buffer bytes.Buffer
	logger := NewLogger(&buffer, "debug")
	helm := MockExecer(logger, "dev")
	err := helm.DiffRelease(HelmContext{}, "release", "chart", false, "--timeout 10", "--wait", "--wait-for-jobs")
	expected := `Comparing release=release, chart=chart
exec: helm --kube-context dev diff upgrade --reset-values --allow-unreleased release chart --timeout 10 --wait --wait-for-jobs
`
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if buffer.String() != expected {
		t.Errorf("helmexec.DiffRelease()\nactual = %v\nexpect = %v", buffer.String(), expected)
	}

	buffer.Reset()
	err = helm.DiffRelease(HelmContext{}, "release", "chart", false)
	expected = `Comparing release=release, chart=chart
exec: helm --kube-context dev diff upgrade --reset-values --allow-unreleased release chart
`
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if buffer.String() != expected {
		t.Errorf("helmexec.DiffRelease()\nactual = %v\nexpect = %v", buffer.String(), expected)
	}

	buffer.Reset()
	err = helm.DiffRelease(HelmContext{}, "release", "https://example_user:example_password@repo.example.com/chart.tgz", false)
	expected = `Comparing release=release, chart=https://example_user:xxxxx@repo.example.com/chart.tgz
exec: helm --kube-context dev diff upgrade --reset-values --allow-unreleased release https://example_user:example_password@repo.example.com/chart.tgz
`
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if buffer.String() != expected {
		t.Errorf("helmexec.DiffRelease()\nactual = %v\nexpect = %v", buffer.String(), expected)
	}
}

func Test_DiffReleaseTillerless(t *testing.T) {
	var buffer bytes.Buffer
	logger := NewLogger(&buffer, "debug")
	helm := MockExecer(logger, "dev")
	err := helm.DiffRelease(HelmContext{Tillerless: true}, "release", "chart", false, "--timeout 10", "--wait", "--wait-for-jobs")
	expected := `Comparing release=release, chart=chart
exec: helm --kube-context dev tiller run -- helm diff upgrade --reset-values --allow-unreleased release chart --timeout 10 --wait --wait-for-jobs
`
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if buffer.String() != expected {
		t.Errorf("helmexec.DiffRelease()\nactual = %v\nexpect = %v", buffer.String(), expected)
	}
}

func Test_DeleteRelease(t *testing.T) {
	var buffer bytes.Buffer
	logger := NewLogger(&buffer, "debug")
	helm := MockExecer(logger, "dev")
	err := helm.DeleteRelease(HelmContext{}, "release")
	expected := `Deleting release
exec: helm --kube-context dev delete release
`
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if buffer.String() != expected {
		t.Errorf("helmexec.DeleteRelease()\nactual = %v\nexpect = %v", buffer.String(), expected)
	}
}
func Test_DeleteRelease_Flags(t *testing.T) {
	var buffer bytes.Buffer
	logger := NewLogger(&buffer, "debug")
	helm := MockExecer(logger, "dev")
	err := helm.DeleteRelease(HelmContext{}, "release", "--purge")
	expected := `Deleting release
exec: helm --kube-context dev delete release --purge
`
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if buffer.String() != expected {
		t.Errorf("helmexec.DeleteRelease()\nactual = %v\nexpect = %v", buffer.String(), expected)
	}
}

func Test_TestRelease(t *testing.T) {
	var buffer bytes.Buffer
	logger := NewLogger(&buffer, "debug")
	helm := MockExecer(logger, "dev")
	err := helm.TestRelease(HelmContext{}, "release")
	expected := `Testing release
exec: helm --kube-context dev test release
`
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if buffer.String() != expected {
		t.Errorf("helmexec.TestRelease()\nactual = %v\nexpect = %v", buffer.String(), expected)
	}
}
func Test_TestRelease_Flags(t *testing.T) {
	var buffer bytes.Buffer
	logger := NewLogger(&buffer, "debug")
	helm := MockExecer(logger, "dev")
	err := helm.TestRelease(HelmContext{}, "release", "--cleanup", "--timeout", "60")
	expected := `Testing release
exec: helm --kube-context dev test release --cleanup --timeout 60
`
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if buffer.String() != expected {
		t.Errorf("helmexec.TestRelease()\nactual = %v\nexpect = %v", buffer.String(), expected)
	}
}

func Test_ReleaseStatus(t *testing.T) {
	var buffer bytes.Buffer
	logger := NewLogger(&buffer, "debug")
	helm := MockExecer(logger, "dev")
	err := helm.ReleaseStatus(HelmContext{}, "myRelease")
	expected := `Getting status myRelease
exec: helm --kube-context dev status myRelease
`
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if buffer.String() != expected {
		t.Errorf("helmexec.ReleaseStatus()\nactual = %v\nexpect = %v", buffer.String(), expected)
	}
}

func Test_exec(t *testing.T) {
	var buffer bytes.Buffer
	logger := NewLogger(&buffer, "debug")
	helm := MockExecer(logger, "")
	env := map[string]string{}
	_, err := helm.exec([]string{"version"}, env)
	expected := `exec: helm version
`
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if buffer.String() != expected {
		t.Errorf("helmexec.exec()\nactual = %v\nexpect = %v", buffer.String(), expected)
	}

	helm = MockExecer(logger, "dev")
	ret, _ := helm.exec([]string{"diff"}, env)
	if len(ret) != 0 {
		t.Error("helmexec.exec() - expected empty return value")
	}

	buffer.Reset()
	helm = MockExecer(logger, "dev")
	_, err = helm.exec([]string{"diff", "release", "chart", "--timeout 10", "--wait", "--wait-for-jobs"}, env)
	expected = `exec: helm --kube-context dev diff release chart --timeout 10 --wait --wait-for-jobs
`
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if buffer.String() != expected {
		t.Errorf("helmexec.exec()\nactual = %v\nexpect = %v", buffer.String(), expected)
	}

	buffer.Reset()
	_, err = helm.exec([]string{"version"}, env)
	expected = `exec: helm --kube-context dev version
`
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if buffer.String() != expected {
		t.Errorf("helmexec.exec()\nactual = %v\nexpect = %v", buffer.String(), expected)
	}

	buffer.Reset()
	helm.SetExtraArgs("foo")
	_, err = helm.exec([]string{"version"}, env)
	expected = `exec: helm --kube-context dev version foo
`
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if buffer.String() != expected {
		t.Errorf("helmexec.exec()\nactual = %v\nexpect = %v", buffer.String(), expected)
	}

	buffer.Reset()
	helm = MockExecer(logger, "")
	helm.SetHelmBinary("overwritten")
	_, err = helm.exec([]string{"version"}, env)
	expected = `exec: overwritten version
`
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if buffer.String() != expected {
		t.Errorf("helmexec.exec()\nactual = %v\nexpect = %v", buffer.String(), expected)
	}
}

func Test_Lint(t *testing.T) {
	var buffer bytes.Buffer
	logger := NewLogger(&buffer, "debug")
	helm := MockExecer(logger, "dev")
	err := helm.Lint("release", "path/to/chart", "--values", "file.yml")
	expected := `Linting release=release, chart=path/to/chart
exec: helm --kube-context dev lint path/to/chart --values file.yml
`
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if buffer.String() != expected {
		t.Errorf("helmexec.Lint()\nactual = %v\nexpect = %v", buffer.String(), expected)
	}
}

func Test_Fetch(t *testing.T) {
	var buffer bytes.Buffer
	logger := NewLogger(&buffer, "debug")
	helm := MockExecer(logger, "dev")
	err := helm.Fetch("chart", "--version", "1.2.3", "--untar", "--untardir", "/tmp/dir")
	expected := `Fetching chart
exec: helm --kube-context dev fetch chart --version 1.2.3 --untar --untardir /tmp/dir
`
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if buffer.String() != expected {
		t.Errorf("helmexec.Fetch()\nactual = %v\nexpect = %v", buffer.String(), expected)
	}

	buffer.Reset()
	err = helm.Fetch("https://example_user:example_password@repo.example.com/chart.tgz", "--version", "1.2.3", "--untar", "--untardir", "/tmp/dir")
	expected = `Fetching https://example_user:xxxxx@repo.example.com/chart.tgz
exec: helm --kube-context dev fetch https://example_user:example_password@repo.example.com/chart.tgz --version 1.2.3 --untar --untardir /tmp/dir
`
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if buffer.String() != expected {
		t.Errorf("helmexec.Fetch()\nactual = %v\nexpect = %v", buffer.String(), expected)
	}
}

func Test_ChartPull(t *testing.T) {
	var buffer bytes.Buffer
	logger := NewLogger(&buffer, "debug")
	helm := MockExecer(logger, "dev")
	err := helm.ChartPull("chart", "--version", "1.2.3", "--untar", "--untardir", "/tmp/dir")
	expected := `Pulling chart
Exporting chart
exec: helm --kube-context dev chart pull chart --version 1.2.3 --untar --untardir /tmp/dir
`
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if buffer.String() != expected {
		t.Errorf("helmexec.ChartPull()\nactual = %v\nexpect = %v", buffer.String(), expected)
	}
}

func Test_ChartExport(t *testing.T) {
	var buffer bytes.Buffer
	logger := NewLogger(&buffer, "debug")
	helm := MockExecer(logger, "dev")
	err := helm.ChartExport("chart", "--version", "1.2.3", "--untar", "--untardir", "/tmp/dir")
	expected := `Exporting chart
exec: helm --kube-context dev chart export chart --destination --version 1.2.3 --untar --untardir /tmp/dir
`
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if buffer.String() != expected {
		t.Errorf("helmexec.ChartExport()\nactual = %v\nexpect = %v", buffer.String(), expected)
	}
}

var logLevelTests = map[string]string{
	"debug": `Adding repo myRepo https://repo.example.com/
exec: helm repo add myRepo https://repo.example.com/ --username example_user --password example_password
`,
	"info": `Adding repo myRepo https://repo.example.com/
`,
	"warn": ``,
}

func Test_LogLevels(t *testing.T) {
	var buffer bytes.Buffer
	for logLevel, expected := range logLevelTests {
		buffer.Reset()
		logger := NewLogger(&buffer, logLevel)
		helm := MockExecer(logger, "")
		err := helm.AddRepo("myRepo", "https://repo.example.com/", "", "", "", "example_user", "example_password", "", "", "")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if buffer.String() != expected {
			t.Errorf("helmexec.AddRepo()\nactual = %v\nexpect = %v", buffer.String(), expected)
		}
	}
}

func Test_getTillerlessEnv(t *testing.T) {
	context := HelmContext{Tillerless: true, TillerNamespace: "foo", WorkerIndex: 1}

	os.Unsetenv("KUBECONFIG")
	actual := context.getTillerlessEnv()
	if val, found := actual["HELM_TILLER_SILENT"]; !found || val != "true" {
		t.Errorf("getTillerlessEnv() HELM_TILLER_SILENT\nactual = %s\nexpect = true", val)
	}
	// This feature is disabled until it is fixed in helm
	/*if val, found := actual["HELM_TILLER_PORT"]; !found || val != "44135" {
		t.Errorf("getTillerlessEnv() HELM_TILLER_PORT\nactual = %s\nexpect = 44135", val)
	}*/
	if val, found := actual["KUBECONFIG"]; found {
		t.Errorf("getTillerlessEnv() KUBECONFIG\nactual = %s\nexpect = nil", val)
	}

	os.Setenv("KUBECONFIG", "toto")
	actual = context.getTillerlessEnv()
	cwd, _ := os.Getwd()
	expected := path.Join(cwd, "toto")
	if val, found := actual["KUBECONFIG"]; !found || val != expected {
		t.Errorf("getTillerlessEnv() KUBECONFIG\nactual = %s\nexpect = %s", val, expected)
	}
	os.Unsetenv("KUBECONFIG")
}

func Test_mergeEnv(t *testing.T) {
	actual := env2map(mergeEnv([]string{"A=1", "B=c=d", "E=2"}, map[string]string{"B": "3", "F": "4"}))
	expected := map[string]string{"A": "1", "B": "3", "E": "2", "F": "4"}
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("mergeEnv()\nactual = %v\nexpect = %v", actual, expected)
	}
}

func Test_Template(t *testing.T) {
	var buffer bytes.Buffer
	logger := NewLogger(&buffer, "debug")
	helm := MockExecer(logger, "dev")
	err := helm.TemplateRelease("release", "path/to/chart", "--values", "file.yml")
	expected := `Templating release=release, chart=path/to/chart
exec: helm --kube-context dev template path/to/chart --name release --values file.yml
`
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if buffer.String() != expected {
		t.Errorf("helmexec.Template()\nactual = %v\nexpect = %v", buffer.String(), expected)
	}

	buffer.Reset()
	err = helm.TemplateRelease("release", "https://example_user:example_password@repo.example.com/chart.tgz", "--values", "file.yml")
	expected = `Templating release=release, chart=https://example_user:xxxxx@repo.example.com/chart.tgz
exec: helm --kube-context dev template https://example_user:example_password@repo.example.com/chart.tgz --name release --values file.yml
`
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if buffer.String() != expected {
		t.Errorf("helmexec.Template()\nactual = %v\nexpect = %v", buffer.String(), expected)
	}
}

func Test_IsHelm3(t *testing.T) {
	helm2Runner := mockRunner{output: []byte("Client: v2.16.0+ge13bc94\n")}
	helm := New("helm", NewLogger(os.Stdout, "info"), "dev", &helm2Runner)
	if helm.IsHelm3() {
		t.Error("helmexec.IsHelm3() - Detected Helm 3 with Helm 2 version")
	}

	helm3Runner := mockRunner{output: []byte("v3.0.0+ge29ce2a\n")}
	helm = New("helm", NewLogger(os.Stdout, "info"), "dev", &helm3Runner)
	if !helm.IsHelm3() {
		t.Error("helmexec.IsHelm3() - Failed to detect Helm 3")
	}

	os.Setenv(envvar.Helm3, "1")
	helm2Runner = mockRunner{output: []byte("Client: v2.16.0+ge13bc94\n")}
	helm = New("helm", NewLogger(os.Stdout, "info"), "dev", &helm2Runner)
	if !helm.IsHelm3() {
		t.Errorf("helmexec.IsHelm3() - Helm3 not detected when %s is set", envvar.Helm3)
	}
	os.Setenv(envvar.Helm3, "")
}

func Test_GetVersion(t *testing.T) {
	helm2Runner := mockRunner{output: []byte("Client: v2.16.1+ge13bc94\n")}
	helm := New("helm", NewLogger(os.Stdout, "info"), "dev", &helm2Runner)
	ver := helm.GetVersion()
	if ver.Major != 2 || ver.Minor != 16 || ver.Patch != 1 {
		t.Error(fmt.Sprintf("helmexec.GetVersion - did not detect correct Helm2 version; it was: %+v", ver))
	}

	helm3Runner := mockRunner{output: []byte("v3.2.4+ge29ce2a\n")}
	helm = New("helm", NewLogger(os.Stdout, "info"), "dev", &helm3Runner)
	ver = helm.GetVersion()
	if ver.Major != 3 || ver.Minor != 2 || ver.Patch != 4 {
		t.Error(fmt.Sprintf("helmexec.GetVersion - did not detect correct Helm3 version; it was: %+v", ver))
	}
}

func Test_IsVersionAtLeast(t *testing.T) {
	helm2Runner := mockRunner{output: []byte("Client: v2.16.1+ge13bc94\n")}
	helm := New("helm", NewLogger(os.Stdout, "info"), "dev", &helm2Runner)
	if !helm.IsVersionAtLeast("2.1.0") {
		t.Error("helmexec.IsVersionAtLeast - 2.16.1 not atleast 2.1")
	}

	if helm.IsVersionAtLeast("2.19.0") {
		t.Error("helmexec.IsVersionAtLeast - 2.16.1 is atleast 2.19")
	}

	if helm.IsVersionAtLeast("3.2.0") {
		t.Error("helmexec.IsVersionAtLeast - 2.16.1 is atleast 3.2")
	}
}
