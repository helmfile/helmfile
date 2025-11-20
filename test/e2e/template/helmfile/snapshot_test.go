package helmfile

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	goruntime "runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/helmfile/chartify/helmtesting"
	"github.com/stretchr/testify/require"

	"github.com/helmfile/helmfile/pkg/envvar"
	"github.com/helmfile/helmfile/pkg/yaml"
)

var (
	// e.g. https_github_com_cloudposse_helmfiles_git.ref=0.xx.0
	chartGitFullPathRegex = regexp.MustCompile(`chart=.*git\.ref=.*/charts/.*`)
	// helm short version regex. e.g. v3.10.2+g50f003e
	helmShortVersionRegex = regexp.MustCompile(`v\d+\.\d+\.\d+\+[a-z0-9]+`)
)

type Config struct {
	LocalDockerRegistry struct {
		Enabled  bool   `yaml:"enabled"`
		Port     int    `yaml:"port"`
		ChartDir string `yaml:"chartDir"`
	} `yaml:"localDockerRegistry"`
	LocalChartRepoServer struct {
		Enabled  bool   `yaml:"enabled"`
		Port     int    `yaml:"port"`
		ChartDir string `yaml:"chartDir"`
	} `yaml:"localChartRepoServer"`
	ChartifyTempDir string   `yaml:"chartifyTempDir"`
	HelmfileArgs    []string `yaml:"helmfileArgs"`
}

// getFreePort asks the kernel for a free open port that is ready to use.
// This has a small race condition between the time we get the port and when we use it,
// but it's the standard approach for dynamic port allocation in tests.
func getFreePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()

	return l.Addr().(*net.TCPAddr).Port, nil
}

// waitForRegistry polls the Docker registry health endpoint until it's ready
// or the timeout is reached. Docker Registry v2 exposes /v2/ which returns
// 200 OK when the registry is healthy and ready to accept requests.
func waitForRegistry(t *testing.T, port int, timeout time.Duration) error {
	t.Helper()

	endpoint := fmt.Sprintf("http://localhost:%d/v2/", port)
	client := &http.Client{Timeout: 2 * time.Second}
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		resp, err := client.Get(endpoint)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				t.Logf("Registry at port %d is ready", port)
				return nil
			}
		}
		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("registry at port %d did not become ready within %v", port, timeout)
}

// prepareInputFile substitutes $REGISTRY_PORT placeholder in the input file
// with the actual allocated port for Docker registry tests. It also converts
// relative chart paths to absolute paths since the input file is copied to a
// temp directory.
func prepareInputFile(t *testing.T, originalFile, tmpDir string, hostPort int, chartsDir string) string {
	t.Helper()

	inputContent, err := os.ReadFile(originalFile)
	require.NoError(t, err, "Failed to read input file")

	// Replace $REGISTRY_PORT placeholder with actual port
	inputStr := string(inputContent)
	inputStr = strings.ReplaceAll(inputStr, "$REGISTRY_PORT", fmt.Sprintf("%d", hostPort))

	// Convert relative chart paths to absolute paths
	// This is necessary because the input file is copied to a temp directory,
	// breaking relative paths like ../../charts/raw-0.1.0
	inputStr = strings.ReplaceAll(inputStr, "../../charts/", chartsDir+"/")

	// Note: postrenderer paths are left as relative paths because they are resolved
	// relative to the working directory, not the helmfile file location.
	// Helm 3 uses the file path directly, Helm 4 extracts the plugin name from the path.

	// Write to temporary file
	tmpInputFile := filepath.Join(tmpDir, "input.yaml.gotmpl")
	err = os.WriteFile(tmpInputFile, []byte(inputStr), 0644)
	require.NoError(t, err, "Failed to write temporary input file")

	return tmpInputFile
}

// setupLocalDockerRegistry sets up a local Docker registry for OCI chart testing.
// It dynamically allocates a port if not configured, starts the registry container,
// and pushes test charts to it. Returns the allocated port.
func setupLocalDockerRegistry(t *testing.T, config Config, name, defaultChartsDir string) int {
	t.Helper()

	containerName := strings.Join([]string{"helmfile_docker_registry", name}, "_")

	hostPort := config.LocalDockerRegistry.Port
	if hostPort <= 0 {
		// Dynamically allocate an unused port to avoid conflicts
		var err error
		hostPort, err = getFreePort()
		require.NoError(t, err, "Failed to get free port for Docker registry")
		t.Logf("Allocated dynamic port %d for Docker registry in test %s", hostPort, name)
	}

	execDocker(t, "run", "--rm", "-d", "-p", fmt.Sprintf("%d:5000", hostPort), "--name", containerName, "registry:2")
	t.Cleanup(func() {
		execDocker(t, "stop", containerName)
	})

	// Wait for registry to be ready by polling its health endpoint
	err := waitForRegistry(t, hostPort, 30*time.Second)
	require.NoError(t, err, "Registry failed to become ready")

	// We helm-package and helm-push every test chart saved in the ./testdata/charts directory
	// to the local registry, so that they can be accessed by helmfile and helm invoked while testing.
	chartDir := config.LocalDockerRegistry.ChartDir
	if chartDir == "" {
		chartDir = defaultChartsDir
	}
	charts, err := os.ReadDir(chartDir)
	require.NoError(t, err)

	for _, c := range charts {
		chartPath := filepath.Join(chartDir, c.Name())
		if !c.IsDir() {
			t.Fatalf("%s is not a directory", c)
		}
		tgzFile := execHelmPackage(t, chartPath)
		_, err := execHelmPush(t, tgzFile, fmt.Sprintf("oci://localhost:%d/myrepo", hostPort))
		require.NoError(t, err, "Unable to run helm push to local registry: %v", err)
	}

	return hostPort
}

func TestHelmfileTemplateWithBuildCommand(t *testing.T) {
	t.Run("with go.yaml.in/yaml/v3", func(t *testing.T) {
		testHelmfileTemplateWithBuildCommand(t, true)
	})

	t.Run("with go.yaml.in/yaml/v2", func(t *testing.T) {
		testHelmfileTemplateWithBuildCommand(t, false)
	})
}

func testHelmfileTemplateWithBuildCommand(t *testing.T, GoYamlV3 bool) {
	t.Setenv(envvar.GoYamlV3, strconv.FormatBool(GoYamlV3))

	localChartPortSets := make(map[int]struct{})

	_, filename, _, _ := goruntime.Caller(0)
	projectRoot := filepath.Join(filepath.Dir(filename), "..", "..", "..", "..")
	helmfileBin := filepath.Join(projectRoot, "helmfile")
	if goruntime.GOOS == "windows" {
		helmfileBin = helmfileBin + ".exe"
	}
	testdataDir := "testdata/snapshot"
	defaultChartsDir := "testdata/charts"

	entries, err := os.ReadDir(testdataDir)
	require.NoError(t, err)

	for _, e := range entries {
		if !e.IsDir() {
			t.Fatalf("Unexpected type of entry at %s", e.Name())
		}

		name := e.Name()

		wd, err := os.Getwd()
		require.NoError(t, err)

		// We read the config from `testdata/snapshot/$CASE_NAME/config.yaml`.
		// It's optional so the test won't fail even if the config file does not exist.

		var config Config

		configFile := filepath.Join(testdataDir, name, "config.yaml")
		if configData, err := os.ReadFile(configFile); err == nil {
			if err := yaml.Unmarshal(configData, &config); err != nil {
				t.Fatalf("Unable to load %s: %v", configFile, err)
			}
		}

		if config.LocalChartRepoServer.Enabled {
			if _, ok := localChartPortSets[config.LocalChartRepoServer.Port]; ok {
				t.Fatalf("Port %d is already in use", config.LocalChartRepoServer.Port)
			} else {
				localChartPortSets[config.LocalChartRepoServer.Port] = struct{}{}
			}
			if config.LocalChartRepoServer.ChartDir == "" {
				config.LocalChartRepoServer.ChartDir = defaultChartsDir
			}
			helmtesting.StartChartRepoServer(t, helmtesting.ChartRepoServerConfig{
				Port:      config.LocalChartRepoServer.Port,
				ChartsDir: config.LocalChartRepoServer.ChartDir,
			})
		}

		// We run `helmfile build` by default.
		// If you want to test `helmfile template`, set the following in the config.yaml:
		//
		// helmfileArgs:
		// - template
		helmfileArgs := config.HelmfileArgs
		if len(helmfileArgs) == 0 {
			helmfileArgs = append(helmfileArgs, "build")
		}

		t.Run(name, func(t *testing.T) {
			// Use the specific chartify tempdir for easy debugging and the test reproducibility.
			// We do snapshot testing in this test. The default chartify tempdir is a random directory created within the os temp dir.
			// Without making it a static path, it's unnecessarily hard to snapshot test it, as the dir path embedded in the output changes
			// on each test run.
			chartifyTempDir := config.ChartifyTempDir
			if chartifyTempDir == "" {
				chartifyTempDir = "chartify_temp"
			}

			// We set the envvar provided by chartify, CHARTIFY_TEMPDIR, to make the tempdir static.
			chartifyTempDir = filepath.Join(wd, chartifyTempDir)
			t.Setenv("CHARTIFY_TEMPDIR", chartifyTempDir)
			// Ensure there's no dangling and remaining tempdir from the previous run
			if err := os.RemoveAll(chartifyTempDir); err != nil {
				t.Fatalf("unable to remove chartify temp dir %q: %v", chartifyTempDir, err)
			}
			// Ensure it's removed on test completion
			t.Cleanup(func() {
				if err := os.RemoveAll(chartifyTempDir); err != nil {
					t.Fatalf("unable to remove chartify temp dir %q: %v", chartifyTempDir, err)
				}
			})

			// If localDockerRegistry.enabled is set to `true`,
			// run the docker registry v2 and push the test charts to the registry
			// so that it can be accessed by helm and helmfile as a oci registry based chart repository.
			var hostPort int
			if config.LocalDockerRegistry.Enabled {
				hostPort = setupLocalDockerRegistry(t, config, name, defaultChartsDir)
			}

			tmpDir := t.TempDir()
			// HELM_CACHE_HOME contains downloaded chart archives
			helmCacheHome := filepath.Join(tmpDir, "helm_cache")
			// HELMFILE_CACHE_HOME contains remote charts and manifests downloaded by Helmfile using the go-getter integration
			helmfileCacheHome := filepath.Join(tmpDir, "helmfile_cache")
			// HELM_CONFIG_HOME contains the registry auth file (registry.json) and the index of all the repos added via helm-repo-add (repositories.yaml).
			helmConfigHome := filepath.Join(tmpDir, "helm_config")

			t.Logf("Using HELM_CACHE_HOME=%s, HELMFILE_CACHE_HOME=%s, HELM_CONFIG_HOME=%s, WD=%s", helmCacheHome, helmfileCacheHome, helmConfigHome, wd)

			// Install post-renderer plugins for Helm 4
			if isHelm4(t) {
				helmDataHome := filepath.Join(tmpDir, "helm_data")
				helmPluginsDir := filepath.Join(helmDataHome, "plugins")
				if name == "postrenderer" {
					// Install the add-cm1 and add-cm2 plugins
					installTestPlugin(t, helmPluginsDir, "add-cm1", filepath.Join(wd, "testdata", "helm-plugins", "add-cm1"))
					installTestPlugin(t, helmPluginsDir, "add-cm2", filepath.Join(wd, "testdata", "helm-plugins", "add-cm2"))

					// Debug: List installed plugins
					if entries, err := os.ReadDir(helmPluginsDir); err == nil {
						t.Logf("Installed plugins in %s:", helmPluginsDir)
						for _, e := range entries {
							t.Logf("  - %s (dir=%v)", e.Name(), e.IsDir())
						}
					}
				}
			}

			inputFile := filepath.Join(testdataDir, name, "input.yaml.gotmpl")

			// If using dynamic Docker registry port, substitute $REGISTRY_PORT in input file
			if config.LocalDockerRegistry.Enabled {
				chartsDir := filepath.Join(wd, defaultChartsDir)
				inputFile = prepareInputFile(t, inputFile, tmpDir, hostPort, chartsDir)
			}

			outputFile := ""
			if GoYamlV3 {
				outputFile = filepath.Join(testdataDir, name, "gopkg.in-yaml.v3-output.yaml")
			} else {
				outputFile = filepath.Join(testdataDir, name, "gopkg.in-yaml.v2-output.yaml")
			}
			expectedOutputFile := filepath.Join(testdataDir, name, "output.yaml")

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			args := []string{"-f", inputFile}
			// Add --oci-plain-http flag for tests using local Docker registry (Helm 4 requirement)
			if config.LocalDockerRegistry.Enabled {
				args = append(args, "--oci-plain-http")
			}
			args = append(args, helmfileArgs...)
			cmd := exec.CommandContext(ctx, helmfileBin, args...)
			cmd.Env = os.Environ()
			// For Helm 4, we need to set HELM_DATA_HOME and plugins will be at $HELM_DATA_HOME/plugins
			helmDataHome := filepath.Join(tmpDir, "helm_data")
			helmPluginsDir := filepath.Join(helmDataHome, "plugins")
			cmd.Env = append(
				cmd.Env,
				envvar.TempDir+"=/tmp/helmfile",
				envvar.DisableRunnerUniqueID+"=1",
				"HELM_CACHE_HOME="+helmCacheHome,
				"HELM_CONFIG_HOME="+helmConfigHome,
				"HELM_DATA_HOME="+helmDataHome,
				"HELM_PLUGINS="+helmPluginsDir,
				"HELMFILE_CACHE_HOME="+helmfileCacheHome,
			)
			got, err := cmd.CombinedOutput()
			if err != nil {
				t.Logf("Output from %v: %s", args, string(got))
			}

			require.NoError(t, err, "Unable to run helmfile with args %v", args)

			gotStr := string(got)

			// Replace all random strings

			gotStr = strings.ReplaceAll(gotStr, fmt.Sprintf("chart=%s", wd), "chart=$WD")
			// Replace go-getter path with $GoGetterPath
			gotStr = chartGitFullPathRegex.ReplaceAllString(gotStr, `chart=$$GoGetterPath`)
			// Replace helm version with $HelmVersion
			gotStr = helmShortVersionRegex.ReplaceAllString(gotStr, `$$HelmVersion`)

			if config.LocalDockerRegistry.Enabled {
				// Normalize the dynamic port to $REGISTRY_PORT placeholder for test comparison
				gotStr = strings.ReplaceAll(gotStr, fmt.Sprintf("localhost:%d", hostPort), "localhost:$REGISTRY_PORT")
				gotStr = strings.ReplaceAll(gotStr, fmt.Sprintf("oci__localhost_%d", hostPort), "oci__localhost_$REGISTRY_PORT")

				sc := bufio.NewScanner(strings.NewReader(gotStr))
				for sc.Scan() {
					if !strings.HasPrefix(sc.Text(), "Templating ") {
						continue
					}
					releaseChartStr := strings.TrimPrefix(sc.Text(), "Templating ")
					releaseChartParts := strings.Split(releaseChartStr, ", ")
					if len(releaseChartParts) != 2 {
						t.Fatal("Found unexpected log output of templating oci based helm chart, want=\"Templating release=<release_name>, chart=<chart_name>\"")
					}
				}
			}

			gotStr = strings.ReplaceAll(gotStr, helmfileCacheHome, "$HELMFILE_CACHE_HOME")
			gotStr = strings.ReplaceAll(gotStr, wd, "__workingdir__")

			// Check for Helm 4 specific output file first if running with Helm 4
			helm4OutputFile := filepath.Join(testdataDir, name, "output-helm4.yaml")

			if isHelm4(t) {
				if stat, _ := os.Stat(helm4OutputFile); stat != nil {
					want, err := os.ReadFile(helm4OutputFile)
					require.NoError(t, err)
					require.Equal(t, string(want), gotStr)
				} else if stat, _ := os.Stat(outputFile); stat != nil {
					want, err := os.ReadFile(outputFile)
					require.NoError(t, err)
					require.Equal(t, string(want), gotStr)
				} else if stat, _ := os.Stat(expectedOutputFile); stat != nil {
					want, err := os.ReadFile(expectedOutputFile)
					require.NoError(t, err)
					require.Equal(t, string(want), gotStr)
				} else {
					// To update the test golden image(output-helm4.yaml), just remove it and rerun this test.
					// We automatically capture the output to `output-helm4.yaml` in the test case directory
					// when the output-helm4.yaml doesn't exist.
					t.Log("generate output-helm4.yaml file and write captured output to it")
					require.NoError(t, os.WriteFile(helm4OutputFile, []byte(gotStr), 0664))
				}
			} else if stat, _ := os.Stat(outputFile); stat != nil {
				want, err := os.ReadFile(outputFile)
				require.NoError(t, err)
				require.Equal(t, string(want), gotStr)
			} else if stat, _ := os.Stat(expectedOutputFile); stat != nil {
				want, err := os.ReadFile(expectedOutputFile)
				require.NoError(t, err)
				require.Equal(t, string(want), gotStr)
			} else {
				// To update the test golden image(output.yaml), just remove it and rerun this test.
				// We automatically capture the output to `output.yaml` in the test case directory
				// when the output.yaml doesn't exist.
				t.Log("generate output.yaml file and write captured output to it")
				require.NoError(t, os.WriteFile(outputFile, []byte(gotStr), 0664))
			}
		})
	}
}

func execDocker(t *testing.T, args ...string) {
	t.Helper()

	docker := exec.Command("docker", args...)
	out, err := docker.CombinedOutput()
	if err != nil {
		t.Logf("Docker output: %s", string(out))
		t.Fatalf("Unable to run docker: %v", err)
	}
}

func execHelmPackage(t *testing.T, localChart string) string {
	t.Helper()

	out := execHelm(t, "package", localChart)
	msg := strings.Split(out, " ")
	tgzAbsPath := msg[len(msg)-1]
	return strings.TrimSpace(tgzAbsPath)
}

// isHelm4 detects if the current Helm binary is version 4
func isHelm4(t *testing.T) bool {
	t.Helper()

	// First try to detect actual Helm version
	helmBinary := os.Getenv("HELM_BIN")
	if helmBinary == "" {
		helmBinary = "helm"
	}

	cmd := exec.Command(helmBinary, "version", "--template={{.Version}}")
	output, err := cmd.CombinedOutput()
	if err == nil {
		version := string(output)
		// Simple check: if it starts with "v4." it's Helm 4
		if len(version) > 2 && version[0] == 'v' && version[1] == '4' {
			return true
		}
		if len(version) > 2 && version[0] == 'v' && version[1] == '3' {
			return false
		}
	}

	// Fallback to environment variable
	return os.Getenv("HELMFILE_HELM4") == "1"
}

// installTestPlugin copies a test plugin directory to the helm plugins directory
func installTestPlugin(t *testing.T, helmPluginsDir, pluginName, sourcePath string) {
	t.Helper()

	targetPath := filepath.Join(helmPluginsDir, pluginName)

	// Create plugins directory if it doesn't exist
	if err := os.MkdirAll(helmPluginsDir, 0755); err != nil {
		t.Fatalf("Failed to create plugins directory: %v", err)
	}

	// Copy the entire plugin directory
	if err := copyDir(sourcePath, targetPath); err != nil {
		t.Fatalf("Failed to install plugin %s: %v", pluginName, err)
	}

	t.Logf("Installed plugin %s from %s to %s", pluginName, sourcePath, targetPath)

	// Verify the plugin was installed
	pluginYaml := filepath.Join(targetPath, "plugin.yaml")
	if _, err := os.Stat(pluginYaml); err != nil {
		t.Fatalf("Plugin %s does not have plugin.yaml after installation: %v", pluginName, err)
	}
}

// copyDir recursively copies a directory tree
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Calculate target path
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			// Create directory
			return os.MkdirAll(targetPath, info.Mode())
		}

		// Copy file
		sourceFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer sourceFile.Close()

		targetFile, err := os.Create(targetPath)
		if err != nil {
			return err
		}
		defer targetFile.Close()

		if _, err := io.Copy(targetFile, sourceFile); err != nil {
			return err
		}

		// Copy file permissions
		return os.Chmod(targetPath, info.Mode())
	})
}

// execHelmPush pushes helm package to oci based helm repository,
// then returns its digest.
func execHelmPush(t *testing.T, tgzPath, remoteUrl string) (string, error) {
	t.Helper()

	// Helm 4 requires --plain-http for HTTP-only OCI registries (not HTTPS with self-signed certs)
	args := []string{"push", tgzPath, remoteUrl}
	if isHelm4(t) {
		args = append(args, "--plain-http")
	}
	out := execHelm(t, args...)
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		if strings.HasPrefix(sc.Text(), "Digest:") {
			return strings.TrimPrefix(sc.Text(), "Digest: "), nil
		}
	}
	return "", fmt.Errorf("Unable to find chart digest from output string of helm push")
}

func execHelm(t *testing.T, args ...string) string {
	t.Helper()

	cmd := []string{"helm"}
	cmd = append(cmd, args...)
	c := strings.Join(cmd, " ")
	helm := exec.Command("helm", args...)
	out, err := helm.CombinedOutput()
	if err != nil {
		t.Logf("%s: %s", c, string(out))
		t.Fatalf("Unable to run %s: %v", c, err)
	}

	return string(out)
}
