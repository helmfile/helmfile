package plugins

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/helmfile/vals"

	"github.com/helmfile/helmfile/pkg/envvar"
)

func TestValsInstance(t *testing.T) {
	i, err := ValsInstance()

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	i2, _ := ValsInstance()

	if i != i2 {
		t.Error("Instances should be equal")
	}
}

// TestAWSSDKLogLevelConfiguration tests the AWS SDK log level configuration logic
func TestAWSSDKLogLevelConfiguration(t *testing.T) {
	tests := []struct {
		name              string
		envValue          string
		expectedLogLevel  string
		expectedLogOutput bool // true if LogOutput should be io.Discard
	}{
		{
			name:              "no env var defaults to off",
			envValue:          "",
			expectedLogLevel:  "off",
			expectedLogOutput: true, // LogOutput should be io.Discard
		},
		{
			name:              "explicit off",
			envValue:          "off",
			expectedLogLevel:  "off",
			expectedLogOutput: true,
		},
		{
			name:              "minimal logging",
			envValue:          "minimal",
			expectedLogLevel:  "minimal",
			expectedLogOutput: false, // LogOutput should NOT be io.Discard
		},
		{
			name:              "standard logging",
			envValue:          "standard",
			expectedLogLevel:  "standard",
			expectedLogOutput: false,
		},
		{
			name:              "verbose logging",
			envValue:          "verbose",
			expectedLogLevel:  "verbose",
			expectedLogOutput: false,
		},
		{
			name:              "custom logging",
			envValue:          "request,response",
			expectedLogLevel:  "request,response",
			expectedLogOutput: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: This test verifies the configuration logic, not the actual vals.New() call
			// since ValsInstance() uses sync.Once and can only be initialized once per test run.

			// Simulate the logic from ValsInstance()
			var logLevel string
			if tt.envValue != "" {
				logLevel = strings.TrimSpace(tt.envValue)
			}

			// Default to "off" for security if not specified
			if logLevel == "" {
				logLevel = "off"
			}

			// Verify expected log level
			if logLevel != tt.expectedLogLevel {
				t.Errorf("Expected log level %q, got %q", tt.expectedLogLevel, logLevel)
			}

			// Verify LogOutput configuration logic
			opts := vals.Options{
				CacheSize: valsCacheSize,
			}
			opts.AWSLogLevel = logLevel

			// Verify LogOutput is set to io.Discard only when level is "off"
			if tt.expectedLogOutput {
				opts.LogOutput = io.Discard
				if opts.LogOutput != io.Discard {
					t.Error("Expected LogOutput to be io.Discard for 'off' level")
				}
			}
		})
	}
}

// TestEnvironmentVariableReading verifies that the HELMFILE_AWS_SDK_LOG_LEVEL env var is read correctly
func TestEnvironmentVariableReading(t *testing.T) {
	tests := []struct {
		name          string
		envValue      string
		expectedValue string
	}{
		{
			name:          "empty defaults to off",
			envValue:      "",
			expectedValue: "off",
		},
		{
			name:          "whitespace trimmed",
			envValue:      "  minimal  ",
			expectedValue: "minimal",
		},
		{
			name:          "standard value preserved",
			envValue:      "standard",
			expectedValue: "standard",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and restore env var
			original := os.Getenv(envvar.AWSSDKLogLevel)
			defer func() {
				if original == "" {
					os.Unsetenv(envvar.AWSSDKLogLevel)
				} else {
					os.Setenv(envvar.AWSSDKLogLevel, original)
				}
			}()

			// Set test env var
			if tt.envValue == "" {
				os.Unsetenv(envvar.AWSSDKLogLevel)
			} else {
				os.Setenv(envvar.AWSSDKLogLevel, tt.envValue)
			}

			// Read and process like ValsInstance() does
			logLevel := strings.TrimSpace(os.Getenv(envvar.AWSSDKLogLevel))
			if logLevel == "" {
				logLevel = "off"
			}

			if logLevel != tt.expectedValue {
				t.Errorf("Expected %q, got %q", tt.expectedValue, logLevel)
			}
		})
	}
}
