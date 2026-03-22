package plugins

import (
	"io"
	"testing"

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

func TestBuildValsOptions(t *testing.T) {
	tests := []struct {
		name                       string
		awsLogLevel                string
		failOnMissingKey           string
		expectedLogLevel           string
		expectedFailOnMissingKey   bool
		expectedLogOutputDiscarded bool
	}{
		{
			name:                       "defaults",
			awsLogLevel:                "",
			failOnMissingKey:           "",
			expectedLogLevel:           "off",
			expectedFailOnMissingKey:   false,
			expectedLogOutputDiscarded: true,
		},
		{
			name:                       "explicit failOnMissingKey true",
			awsLogLevel:                "",
			failOnMissingKey:           "true",
			expectedLogLevel:           "off",
			expectedFailOnMissingKey:   true,
			expectedLogOutputDiscarded: true,
		},
		{
			name:                       "failOnMissingKey false",
			awsLogLevel:                "",
			failOnMissingKey:           "false",
			expectedLogLevel:           "off",
			expectedFailOnMissingKey:   false,
			expectedLogOutputDiscarded: true,
		},
		{
			name:                       "failOnMissingKey with whitespace",
			awsLogLevel:                "",
			failOnMissingKey:           "  true  ",
			expectedLogLevel:           "off",
			expectedFailOnMissingKey:   true,
			expectedLogOutputDiscarded: true,
		},
		{
			name:                       "aws log level verbose",
			awsLogLevel:                "verbose",
			failOnMissingKey:           "",
			expectedLogLevel:           "verbose",
			expectedFailOnMissingKey:   false,
			expectedLogOutputDiscarded: false,
		},
		{
			name:                       "aws log level with whitespace",
			awsLogLevel:                "  minimal  ",
			failOnMissingKey:           "",
			expectedLogLevel:           "minimal",
			expectedFailOnMissingKey:   false,
			expectedLogOutputDiscarded: false,
		},
		{
			name:                       "both options set",
			awsLogLevel:                "standard",
			failOnMissingKey:           "true",
			expectedLogLevel:           "standard",
			expectedFailOnMissingKey:   true,
			expectedLogOutputDiscarded: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.awsLogLevel != "" {
				t.Setenv(envvar.AWSSDKLogLevel, tt.awsLogLevel)
			}
			if tt.failOnMissingKey != "" {
				t.Setenv(envvar.ValsFailOnMissingKeyInMap, tt.failOnMissingKey)
			}

			opts := buildValsOptions()

			if opts.AWSLogLevel != tt.expectedLogLevel {
				t.Errorf("AWSLogLevel: expected %q, got %q", tt.expectedLogLevel, opts.AWSLogLevel)
			}
			if opts.FailOnMissingKeyInMap != tt.expectedFailOnMissingKey {
				t.Errorf("FailOnMissingKeyInMap: expected %v, got %v", tt.expectedFailOnMissingKey, opts.FailOnMissingKeyInMap)
			}
			if opts.CacheSize != valsCacheSize {
				t.Errorf("CacheSize: expected %d, got %d", valsCacheSize, opts.CacheSize)
			}

			isDiscarded := opts.LogOutput == io.Discard
			if isDiscarded != tt.expectedLogOutputDiscarded {
				t.Errorf("LogOutput discarded: expected %v, got %v", tt.expectedLogOutputDiscarded, isDiscarded)
			}
		})
	}
}

func TestAWSSDKLogLevelConfiguration(t *testing.T) {
	tests := []struct {
		name              string
		envValue          string
		expectedLogLevel  string
		expectedLogOutput bool
	}{
		{
			name:              "no env var defaults to off",
			envValue:          "",
			expectedLogLevel:  "off",
			expectedLogOutput: true,
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
			expectedLogOutput: false,
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
			if tt.envValue != "" {
				t.Setenv(envvar.AWSSDKLogLevel, tt.envValue)
			}

			opts := buildValsOptions()

			if opts.AWSLogLevel != tt.expectedLogLevel {
				t.Errorf("Expected log level %q, got %q", tt.expectedLogLevel, opts.AWSLogLevel)
			}

			isDiscarded := opts.LogOutput == io.Discard
			if isDiscarded != tt.expectedLogOutput {
				t.Errorf("Expected LogOutput discarded %v, got %v", tt.expectedLogOutput, isDiscarded)
			}
		})
	}
}
