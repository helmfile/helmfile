package helmexec

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/google/go-cmp/cmp"
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

func (mock *mockRunner) Execute(cmd string, args []string, env map[string]string, enableLiveOutput bool) ([]byte, error) {
	if len(mock.output) == 0 && strings.Join(args, " ") == "version --client --short" {
		return []byte("v3.2.4+ge29ce2a"), nil
	}
	return mock.output, mock.err
}

func MockExecer(logger *zap.SugaredLogger, kubeContext string) *execer {
	execer := New("helm", HelmExecOptions{}, logger, kubeContext, &mockRunner{})
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

func Test_SetEnableLiveOutput(t *testing.T) {
	helm := MockExecer(NewLogger(os.Stdout, "info"), "dev")
	if helm.options.EnableLiveOutput {
		t.Error("helmexec.options.EnableLiveOutput should not be enabled by default")
	}
	helm.SetEnableLiveOutput(true)
	if !helm.options.EnableLiveOutput {
		t.Errorf("helmexec.SetEnableLiveOutput() - actual = %t expect = true", helm.options.EnableLiveOutput)
	}
}

func Test_SetDisableForceUpdate(t *testing.T) {
	helm := MockExecer(NewLogger(os.Stdout, "info"), "dev")
	if helm.options.DisableForceUpdate {
		t.Error("helmexec.options.ForceUpdate should not be enabled by default")
	}
	helm.SetDisableForceUpdate(true)
	if !helm.options.DisableForceUpdate {
		t.Errorf("helmexec.SetDisableForceUpdate() - actual = %t expect = true", helm.options.DisableForceUpdate)
	}
}

func Test_AddRepo_Helm_3_3_2(t *testing.T) {
	var buffer bytes.Buffer
	logger := NewLogger(&buffer, "debug")
	helm := &execer{
		helmBinary:  "helm",
		version:     semver.MustParse("3.3.2"),
		logger:      logger,
		kubeContext: "dev",
		runner:      &mockRunner{},
	}
	err := helm.AddRepo("myRepo", "https://repo.example.com/", "", "cert.pem", "key.pem", "", "", "", false, false)
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

func Test_AddRepo_Helm_3_3_2_NoForceUpdate(t *testing.T) {
	var buffer bytes.Buffer
	logger := NewLogger(&buffer, "debug")
	helm := &execer{
		helmBinary:  "helm",
		options:     HelmExecOptions{DisableForceUpdate: true},
		version:     semver.MustParse("3.3.2"),
		logger:      logger,
		kubeContext: "dev",
		runner:      &mockRunner{},
	}
	err := helm.AddRepo("myRepo", "https://repo.example.com/", "", "cert.pem", "key.pem", "", "", "", false, false)
	expected := `Adding repo myRepo https://repo.example.com/
exec: helm --kube-context dev repo add myRepo https://repo.example.com/ --cert-file cert.pem --key-file key.pem
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
	err := helm.AddRepo("myRepo", "https://repo.example.com/", "", "cert.pem", "key.pem", "", "", "", false, false)
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
	err = helm.AddRepo("myRepo", "https://repo.example.com/", "ca.crt", "", "", "", "", "", false, false)
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
	err = helm.AddRepo("myRepo", "https://repo.example.com/", "", "", "", "", "", "", false, false)
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
	err = helm.AddRepo("acrRepo", "", "", "", "", "", "", "acr", false, false)
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
	err = helm.AddRepo("otherRepo", "", "", "", "", "", "", "unknown", false, false)
	expected = `ERROR: unknown type 'unknown' for repository otherRepo
`
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if buffer.String() != expected {
		t.Errorf("helmexec.AddRepo()\nactual = %v\nexpect = %v", buffer.String(), expected)
	}

	buffer.Reset()
	err = helm.AddRepo("myRepo", "https://repo.example.com/", "", "", "", "example_user", "example_password", "", false, false)
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
	err = helm.AddRepo("", "https://repo.example.com/", "", "", "", "", "", "", false, false)
	expected = `empty field name

`
	if err != nil && err.Error() != "empty field name" {
		t.Errorf("unexpected error: %v", err)
	}
	if buffer.String() != expected {
		t.Errorf("helmexec.AddRepo()\nactual = %v\nexpect = %v", buffer.String(), expected)
	}

	buffer.Reset()
	err = helm.AddRepo("myRepo", "https://repo.example.com/", "", "", "", "example_user", "example_password", "", true, false)
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
	err = helm.AddRepo("myRepo", "https://repo.example.com/", "", "", "", "", "", "", false, true)
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
exec: helm --kube-context dev upgrade --install release chart --timeout 10 --wait --wait-for-jobs --history-max 0
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
exec: helm --kube-context dev upgrade --install release chart --history-max 0
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
exec: helm --kube-context dev upgrade --install release https://example_user:example_password@repo.example.com/chart.tgz --history-max 0
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
	helm3Runner := mockRunner{output: []byte("v3.2.4+ge29ce2a")}
	helm := New("helm", HelmExecOptions{}, logger, "dev", &helm3Runner)
	err := helm.BuildDeps("foo", "./chart/foo", []string{"--skip-refresh"}...)
	expected := `Building dependency release=foo, chart=./chart/foo
exec: helm --kube-context dev dependency build ./chart/foo --skip-refresh
v3.2.4+ge29ce2a
`
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if buffer.String() != expected {
		t.Errorf("helmexec.BuildDeps()\nactual = %v\nexpect = %v", buffer.String(), expected)
	}

	buffer.Reset()
	err = helm.BuildDeps("foo", "./chart/foo")
	expected = `Building dependency release=foo, chart=./chart/foo
exec: helm --kube-context dev dependency build ./chart/foo
v3.2.4+ge29ce2a
`
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if buffer.String() != expected {
		t.Errorf("helmexec.BuildDeps()\nactual = %v\nexpect = %v", buffer.String(), expected)
	}

	buffer.Reset()
	helm.SetExtraArgs("--verify")
	err = helm.BuildDeps("foo", "./chart/foo", []string{"--skip-refresh"}...)
	expected = `Building dependency release=foo, chart=./chart/foo
exec: helm --kube-context dev dependency build ./chart/foo --skip-refresh --verify
v3.2.4+ge29ce2a
`
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if buffer.String() != expected {
		t.Errorf("helmexec.BuildDeps()\nactual = %v\nexpect = %v", buffer.String(), expected)
	}

	buffer.Reset()
	helm2Runner := mockRunner{output: []byte("Client: v2.16.1+ge13bc94")}
	helm = New("helm", HelmExecOptions{}, logger, "dev", &helm2Runner)
	err = helm.BuildDeps("foo", "./chart/foo")
	expected = `Building dependency release=foo, chart=./chart/foo
exec: helm --kube-context dev dependency build ./chart/foo
Client: v2.16.1+ge13bc94
`
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if buffer.String() != expected {
		t.Errorf("helmexec.BuildDeps()\nactual = %v\nexpect = %v", buffer.String(), expected)
	}
}

func Test_DecryptSecret(t *testing.T) {
	// Set secrets plugin version to 4.0.0
	if err := os.Setenv("HELM_PLUGINS", "../../test/plugins/secrets/4.0.0"); err != nil {
		t.Errorf("failed to set environment HELM_PLUGINS error: %s", err)
	}
	defer func() {
		if err := os.Unsetenv("HELM_PLUGINS"); err != nil {
			t.Errorf("failed to unset environment HELM_PLUGINS error: %s", err)
		}
	}()
	var buffer bytes.Buffer
	logger := NewLogger(&buffer, "debug")
	helm := MockExecer(logger, "dev")

	tmpFilePath := "path/to/temp/file"
	helm.writeTempFile = func(content []byte) (string, error) {
		return tmpFilePath, nil
	}

	_, err := helm.DecryptSecret(HelmContext{}, "secretName")
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	cwd, err := filepath.Abs(".")
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	// Run again for caching
	_, err = helm.DecryptSecret(HelmContext{}, "secretName")

	expected := fmt.Sprintf(`Preparing to decrypt secret %v/secretName
Decrypting secret %s/secretName
exec: helm --kube-context dev secrets decrypt %s/secretName
Decrypted %s/secretName into %s
Preparing to decrypt secret %s/secretName
Found secret in cache %s/secretName
Decrypted %s/secretName into %s
`, cwd, cwd, cwd, cwd, tmpFilePath, cwd, cwd, cwd, tmpFilePath)
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
	// Set secrets plugin version to 4.0.0
	if err := os.Setenv("HELM_PLUGINS", "../../test/plugins/secrets/4.0.0"); err != nil {
		t.Errorf("failed to set environment HELM_PLUGINS error: %s", err)
	}
	defer func() {
		if err := os.Unsetenv("HELM_PLUGINS"); err != nil {
			t.Errorf("failed to unset environment HELM_PLUGINS error: %s", err)
		}
	}()
	var buffer bytes.Buffer
	logger := NewLogger(&buffer, "debug")
	helm := MockExecer(logger, "dev")

	tmpFilePath := "path/to/temp/file"
	helm.writeTempFile = func(content []byte) (string, error) {
		return tmpFilePath, nil
	}

	secretName := "secretName.yaml.gotmpl"
	_, err := helm.DecryptSecret(HelmContext{}, secretName)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	cwd, err := filepath.Abs(".")
	if err != nil {
		t.Errorf("Error: %v", err)
	}

	expected := fmt.Sprintf(`Preparing to decrypt secret %v/secretName.yaml.gotmpl
Decrypting secret %s/secretName.yaml.gotmpl
exec: helm --kube-context dev secrets decrypt %s/secretName.yaml.gotmpl
Decrypted %s/secretName.yaml.gotmpl into %s
`, cwd, cwd, cwd, cwd, tmpFilePath)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	if d := cmp.Diff(expected, buffer.String()); d != "" {
		t.Errorf("helmexec.DecryptSecret(): want (-), got (+):\n%s", d)
	}
}

func Test_DiffRelease(t *testing.T) {
	var buffer bytes.Buffer
	logger := NewLogger(&buffer, "debug")
	helm := MockExecer(logger, "dev")
	err := helm.DiffRelease(HelmContext{}, "release", "chart", false, "--timeout 10", "--wait", "--wait-for-jobs")
	expected := `Comparing release=release, chart=chart
exec: helm --kube-context dev diff upgrade --allow-unreleased release chart --timeout 10 --wait --wait-for-jobs
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
exec: helm --kube-context dev diff upgrade --allow-unreleased release chart
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
exec: helm --kube-context dev diff upgrade --allow-unreleased release https://example_user:example_password@repo.example.com/chart.tgz
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
	_, err := helm.exec([]string{"version"}, env, nil)
	expected := `exec: helm version
`
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if buffer.String() != expected {
		t.Errorf("helmexec.exec()\nactual = %v\nexpect = %v", buffer.String(), expected)
	}

	helm = MockExecer(logger, "dev")
	ret, _ := helm.exec([]string{"diff"}, env, nil)
	if len(ret) != 0 {
		t.Error("helmexec.exec() - expected empty return value")
	}

	buffer.Reset()
	helm = MockExecer(logger, "dev")
	_, err = helm.exec([]string{"diff", "release", "chart", "--timeout 10", "--wait", "--wait-for-jobs"}, env, nil)
	expected = `exec: helm --kube-context dev diff release chart --timeout 10 --wait --wait-for-jobs
`
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if buffer.String() != expected {
		t.Errorf("helmexec.exec()\nactual = %v\nexpect = %v", buffer.String(), expected)
	}

	buffer.Reset()
	_, err = helm.exec([]string{"version"}, env, nil)
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
	_, err = helm.exec([]string{"version"}, env, nil)
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
	_, err = helm.exec([]string{"version"}, env, nil)
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
	tests := []struct {
		name        string
		helmBin     string
		helmVersion string
		chartName   string
		chartPath   string
		chartFlags  []string
		listResult  string
	}{
		{
			name:        "less then v3.7.0",
			helmBin:     "helm",
			helmVersion: "v3.6.0",
			chartName:   "chart",
			chartPath:   "path1",
			chartFlags:  []string{"--untar", "--untardir", "/tmp/dir"},
			listResult: `Pulling chart
exec: helm --kube-context dev chart pull chart --untar --untardir /tmp/dir
`,
		},
		{
			name:        "more then v3.7.0",
			helmBin:     "helm",
			helmVersion: "v3.10.0",
			chartName:   "repo/helm-charts:0.14.0",
			chartPath:   "path1",
			chartFlags:  []string{"--untardir", "/tmp/dir"},
			listResult: `Pulling repo/helm-charts:0.14.0
exec: helm --kube-context dev pull oci://repo/helm-charts --version 0.14.0 --destination path1 --untar --untardir /tmp/dir
`,
		},
	}
	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			buffer.Reset()
			helm := &execer{
				helmBinary:  tt.helmBin,
				version:     semver.MustParse(tt.helmVersion),
				logger:      logger,
				kubeContext: "dev",
				runner:      &mockRunner{},
			}
			err := helm.ChartPull(tt.chartName, tt.chartPath, tt.chartFlags...)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			isMatch, _ := regexp.MatchString(tt.listResult, buffer.String())
			if !isMatch {
				t.Errorf("helmexec.ChartPull()\nactual = %v\nexpect = %v", buffer.String(), tt.listResult)
			}
		})
	}
}

func Test_ChartExport(t *testing.T) {
	var buffer bytes.Buffer
	logger := NewLogger(&buffer, "debug")
	tests := []struct {
		name          string
		helmBin       string
		helmVersion   string
		chartName     string
		chartPath     string
		chartFlags    []string
		listResult    string
		expectedError string
	}{
		{
			name:        "",
			helmBin:     "helm",
			helmVersion: "v3.6.0",
			chartName:   "chart",
			chartPath:   "path1",
			chartFlags:  []string{"--untar", "--untardir", "/tmp/dir"},
			listResult: `Exporting chart
exec: helm --kube-context dev chart export chart --destination path1 --untar --untardir /tmp/dir
`,
			expectedError: "",
		},
	}
	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			buffer.Reset()
			helm := &execer{
				helmBinary:  tt.helmBin,
				version:     semver.MustParse(tt.helmVersion),
				logger:      logger,
				kubeContext: "dev",
				runner:      &mockRunner{},
			}
			err := helm.ChartExport(tt.chartName, tt.chartPath, tt.chartFlags...)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if buffer.String() != tt.listResult {
				t.Errorf("helmexec.ChartExport()\nactual = %v\nexpect = %v", buffer.String(), tt.listResult)
			}
		})
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
		err := helm.AddRepo("myRepo", "https://repo.example.com/", "", "", "", "example_user", "example_password", "", false, false)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if buffer.String() != expected {
			t.Errorf("helmexec.AddRepo()\nactual = %v\nexpect = %v", buffer.String(), expected)
		}
	}
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
exec: helm --kube-context dev template release path/to/chart --values file.yml
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
exec: helm --kube-context dev template release https://example_user:example_password@repo.example.com/chart.tgz --values file.yml
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
	helm := New("helm", HelmExecOptions{}, NewLogger(os.Stdout, "info"), "dev", &helm2Runner)
	if helm.IsHelm3() {
		t.Error("helmexec.IsHelm3() - Detected Helm 3 with Helm 2 version")
	}

	helm3Runner := mockRunner{output: []byte("v3.0.0+ge29ce2a\n")}
	helm = New("helm", HelmExecOptions{}, NewLogger(os.Stdout, "info"), "dev", &helm3Runner)
	if !helm.IsHelm3() {
		t.Error("helmexec.IsHelm3() - Failed to detect Helm 3")
	}
}

func Test_GetPluginVersion(t *testing.T) {
	v3ExpectedVersion := "3.15.0"
	v4ExpectedVersion := "4.0.0"
	v3PluginDirPath := "../../test/plugins/secrets/3.15.0"
	v4PluginDirPath := "../../test/plugins/secrets/4.0.0"

	v3SecretPluginVersion, err := GetPluginVersion("secrets", v3PluginDirPath)
	if err != nil {
		t.Errorf(err.Error())
	}
	if v3SecretPluginVersion.String() != v3ExpectedVersion {
		t.Errorf("secrets v3 plugin version is %v, expected %v", v3SecretPluginVersion.String(), v3ExpectedVersion)
	}

	v4SecretPluginVersion, err := GetPluginVersion("secrets", v4PluginDirPath)
	if err != nil {
		t.Errorf(err.Error())
	}
	if v4SecretPluginVersion.String() != v4ExpectedVersion {
		t.Errorf("secrets v4 plugin version is %v, expected %v", v4SecretPluginVersion.String(), v4ExpectedVersion)
	}
}

func Test_GetVersion(t *testing.T) {
	helm2Runner := mockRunner{output: []byte("Client: v2.16.1+ge13bc94\n")}
	helm := New("helm", HelmExecOptions{}, NewLogger(os.Stdout, "info"), "dev", &helm2Runner)
	ver := helm.GetVersion()
	if ver.Major != 2 || ver.Minor != 16 || ver.Patch != 1 {
		t.Errorf("helmexec.GetVersion - did not detect correct Helm2 version; it was: %+v", ver)
	}

	helm3Runner := mockRunner{output: []byte("v3.2.4+ge29ce2a\n")}
	helm = New("helm", HelmExecOptions{}, NewLogger(os.Stdout, "info"), "dev", &helm3Runner)
	ver = helm.GetVersion()
	if ver.Major != 3 || ver.Minor != 2 || ver.Patch != 4 {
		t.Errorf("helmexec.GetVersion - did not detect correct Helm3 version; it was: %+v", ver)
	}
}

func Test_IsVersionAtLeast(t *testing.T) {
	helm2Runner := mockRunner{output: []byte("Client: v2.16.1+ge13bc94\n")}
	helm := New("helm", HelmExecOptions{}, NewLogger(os.Stdout, "info"), "dev", &helm2Runner)
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

func Test_resolveOciChart(t *testing.T) {
	tests := []struct {
		name        string
		chartPath   string
		ociChartURL string
		ociChartTag string
	}{
		{
			name:        "normal",
			chartPath:   "chart/nginx:v1",
			ociChartURL: "oci://chart/nginx",
			ociChartTag: "v1",
		},
		{
			name:        "contains the port",
			chartPath:   "chart:5000/nginx:v1",
			ociChartURL: "oci://chart:5000/nginx",
			ociChartTag: "v1",
		},
		{
			name:        "no tag",
			chartPath:   "chart:5000/nginx",
			ociChartURL: "oci://chart:5000/nginx",
			ociChartTag: "",
		},
	}
	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			url, tag := resolveOciChart(tt.chartPath)
			if tt.ociChartURL != url || tt.ociChartTag != tag {
				actual := fmt.Sprintf("ociChartURL->%s  ociChartTag->%s", url, tag)
				expected := fmt.Sprintf("ociChartURL->%s ociChartTag->%s", tt.ociChartURL, tt.ociChartTag)
				t.Errorf("resolveOciChart()\nactual = %v\nexpect = %v", actual, expected)
			}
		})
	}
}

func Test_ShowChart(t *testing.T) {
	showChartRunner := mockRunner{output: []byte("name: my-chart\nversion: 3.2.0\n")}
	helm := &execer{
		helmBinary:  "helm",
		version:     semver.MustParse("3.3.2"),
		logger:      NewLogger(os.Stdout, "info"),
		kubeContext: "dev",
		runner:      &showChartRunner,
	}

	metadata, err := helm.ShowChart("my-chart")
	if err != nil {
		t.Errorf("helmexec.ShowChart() - unexpected error: %v", err)
	}
	if metadata.Name != "my-chart" {
		t.Errorf("helmexec.ShowChart() - expected chart name was %s, received: %s", "my-chart", metadata.Name)
	}
	if metadata.Version != "3.2.0" {
		t.Errorf("helmexec.ShowChart() - expected chart version was %s, received: %s", "3.2.0", metadata.Version)
	}
}

func TestParseHelmVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    *semver.Version
		wantErr bool
	}{
		{
			name:    "helm 2",
			version: "Client: v2.16.1+ge13bc94\n",
			want:    semver.MustParse("v2.16.1+ge13bc94"),
			wantErr: false,
		},
		{
			name:    "helm 3",
			version: "Client: v3.2.4+ge29ce2a\n",
			want:    semver.MustParse("v3.2.4+ge29ce2a"),
			wantErr: false,
		},
		{
			name:    "helm 3 with os arch and build info",
			version: "Client v3.7.1+7.el8+g8f33223\n",
			want:    semver.MustParse("v3.7.1+7.el8"),
			wantErr: false,
		},
		{
			name:    "empty version",
			version: "",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "invalid version",
			version: "oooooo",
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseHelmVersion(tt.version)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseHelmVersion() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseHelmVersion() = %v, want %v", got, tt.want)
			}
		})
	}
}
