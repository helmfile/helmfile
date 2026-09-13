package plugins

import (
	"io"
	"os"
	"testing"

	"github.com/helmfile/vals"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/helmfile/helmfile/pkg/envvar"
)

// resetInstance resets the singleton for testing
func resetInstance() {
	mu.Lock()
	defer mu.Unlock()
	instance = nil
}

// setenvForTest sets the environment variable to value, or unsets it when value is empty,
// restoring the original value when the test finishes.
func setenvForTest(t *testing.T, key, value string) {
	t.Helper()
	if value == "" {
		orig, had := os.LookupEnv(key)
		os.Unsetenv(key)
		t.Cleanup(func() {
			if had {
				os.Setenv(key, orig)
			} else {
				os.Unsetenv(key)
			}
		})
		return
	}
	t.Setenv(key, value)
}

func TestValsInstance(t *testing.T) {
	resetInstance()
	defer resetInstance()

	setenvForTest(t, envvar.DisableVals, "")
	setenvForTest(t, envvar.DisableValsStrict, "")

	i, err := ValsInstance()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	i2, _ := ValsInstance()
	if i != i2 {
		t.Error("Instances should be equal")
	}
}

func TestDisableVals(t *testing.T) {
	resetInstance()
	defer resetInstance()

	setenvForTest(t, envvar.DisableVals, "true")
	setenvForTest(t, envvar.DisableValsStrict, "")

	evaluator, err := ValsInstance()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should pass through values unchanged
	input := map[string]any{"key": "ref+echo://secret"}
	output, err := evaluator.Eval(input)
	if err != nil {
		t.Fatalf("passthrough should not error: %v", err)
	}

	if output["key"] != "ref+echo://secret" {
		t.Errorf("expected ref+ to pass through unchanged, got %v", output["key"])
	}
}

func TestDisableValsStrict(t *testing.T) {
	resetInstance()
	defer resetInstance()

	setenvForTest(t, envvar.DisableVals, "")
	setenvForTest(t, envvar.DisableValsStrict, "true")

	evaluator, err := ValsInstance()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should error on ref+
	input := map[string]any{"key": "ref+echo://secret"}
	_, err = evaluator.Eval(input)
	if err == nil {
		t.Fatal("strict mode should error on ref+")
	}
	if err != ErrValsDisabled {
		t.Errorf("expected ErrValsDisabled, got %v", err)
	}
}

func TestDisableValsStrictTakesPrecedence(t *testing.T) {
	resetInstance()
	defer resetInstance()

	setenvForTest(t, envvar.DisableVals, "true")
	setenvForTest(t, envvar.DisableValsStrict, "true")

	evaluator, err := ValsInstance()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Strict mode should win over pass-through mode
	input := map[string]any{"key": "ref+echo://secret"}
	_, err = evaluator.Eval(input)
	if err != ErrValsDisabled {
		t.Errorf("expected ErrValsDisabled when both env vars are set, got %v", err)
	}
}

func TestDisableValsStrictAllowsNonRef(t *testing.T) {
	resetInstance()
	defer resetInstance()

	setenvForTest(t, envvar.DisableVals, "")
	setenvForTest(t, envvar.DisableValsStrict, "true")

	evaluator, err := ValsInstance()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should pass through non-ref+ values, including strings that merely
	// contain "ref+" without a provider scheme (not a vals reference)
	input := map[string]any{
		"key":        "normal-value",
		"literal":    "some ref+ text without scheme",
		"secretRef2": "ref+2",
	}
	output, err := evaluator.Eval(input)
	if err != nil {
		t.Fatalf("strict mode should allow non-ref+ values: %v", err)
	}
	if output["key"] != "normal-value" {
		t.Errorf("expected value to pass through, got %v", output["key"])
	}
}

func TestDisableValsStrictDetectsSecretRef(t *testing.T) {
	resetInstance()
	defer resetInstance()

	setenvForTest(t, envvar.DisableVals, "")
	setenvForTest(t, envvar.DisableValsStrict, "true")

	evaluator, err := ValsInstance()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// secretref+ is also a vals reference and must be rejected
	input := map[string]any{"key": "secretref+vault://secret/data/x#y"}
	if _, err := evaluator.Eval(input); err != ErrValsDisabled {
		t.Errorf("expected ErrValsDisabled for secretref+ expression, got %v", err)
	}
}

func TestDisableValsStrictNestedRef(t *testing.T) {
	resetInstance()
	defer resetInstance()

	setenvForTest(t, envvar.DisableVals, "")
	setenvForTest(t, envvar.DisableValsStrict, "true")

	evaluator, err := ValsInstance()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should detect nested ref+ in map[string]any
	input := map[string]any{
		"outer": map[string]any{
			"inner": "ref+vault://secret",
		},
	}
	_, err = evaluator.Eval(input)
	if err == nil {
		t.Fatal("strict mode should detect nested ref+")
	}
}

func TestDisableValsStrictMapAnyAny(t *testing.T) {
	resetInstance()
	defer resetInstance()

	setenvForTest(t, envvar.DisableVals, "")
	setenvForTest(t, envvar.DisableValsStrict, "true")

	evaluator, err := ValsInstance()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should detect ref+ inside map[any]any (yaml.v2 nested maps)
	input := map[string]any{
		"outer": map[any]any{
			"inner": "ref+vault://secret",
		},
	}
	_, err = evaluator.Eval(input)
	if err == nil {
		t.Fatal("strict mode should detect ref+ in map[any]any")
	}
}

func TestDisableValsStrictArrayRef(t *testing.T) {
	resetInstance()
	defer resetInstance()

	setenvForTest(t, envvar.DisableVals, "")
	setenvForTest(t, envvar.DisableValsStrict, "true")

	evaluator, err := ValsInstance()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should detect ref+ in []any arrays
	input := map[string]any{
		"values": []any{"normal", "ref+awssecrets://db/password"},
	}
	_, err = evaluator.Eval(input)
	if err == nil {
		t.Fatal("strict mode should detect ref+ in arrays")
	}
}

func TestDisableValsStrictStringSlice(t *testing.T) {
	resetInstance()
	defer resetInstance()

	setenvForTest(t, envvar.DisableVals, "")
	setenvForTest(t, envvar.DisableValsStrict, "true")

	evaluator, err := ValsInstance()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should detect ref+ in []string arrays (matches renderValsSecrets usage)
	input := map[string]any{
		"values": []string{"normal", "ref+awssecrets://db/password"},
	}
	_, err = evaluator.Eval(input)
	if err == nil {
		t.Fatal("strict mode should detect ref+ in []string arrays")
	}
}

func TestDisableValsPassThroughStringSlice(t *testing.T) {
	resetInstance()
	defer resetInstance()

	setenvForTest(t, envvar.DisableVals, "true")
	setenvForTest(t, envvar.DisableValsStrict, "")

	evaluator, err := ValsInstance()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	input := map[string]any{
		"values": []string{"normal", "ref+awssecrets://db/password"},
	}
	out, err := evaluator.Eval(input)
	if err != nil {
		t.Fatalf("pass-through should not error: %v", err)
	}

	values, ok := out["values"].([]any)
	if !ok {
		t.Fatalf("expected out[\"values\"] to be []any, got %T", out["values"])
	}
	if values[0] != "normal" || values[1] != "ref+awssecrets://db/password" {
		t.Errorf("unexpected values: %v", values)
	}
}

func TestDisableValsPassThroughNestedTypes(t *testing.T) {
	resetInstance()
	defer resetInstance()

	setenvForTest(t, envvar.DisableVals, "true")
	setenvForTest(t, envvar.DisableValsStrict, "")

	evaluator, err := ValsInstance()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Nested map[any]any and []string should be normalized to map[string]any
	// and []any respectively, matching vals.Eval behavior
	input := map[string]any{
		"outer": map[any]any{
			"list": []string{"normal", "ref+echo://secret"},
		},
	}
	out, err := evaluator.Eval(input)
	if err != nil {
		t.Fatalf("pass-through should not error: %v", err)
	}

	outer, ok := out["outer"].(map[string]any)
	if !ok {
		t.Fatalf("expected out[\"outer\"] to be map[string]any, got %T", out["outer"])
	}
	list, ok := outer["list"].([]any)
	if !ok {
		t.Fatalf("expected outer[\"list\"] to be []any, got %T", outer["list"])
	}
	if list[0] != "normal" || list[1] != "ref+echo://secret" {
		t.Errorf("unexpected values: %v", list)
	}
}

func TestNormalValsProcessing(t *testing.T) {
	resetInstance()
	defer resetInstance()

	// Ensure both are unset
	setenvForTest(t, envvar.DisableVals, "")
	setenvForTest(t, envvar.DisableValsStrict, "")

	evaluator, err := ValsInstance()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// ref+echo should expand to the value after ://
	input := map[string]any{"key": "ref+echo://myvalue"}
	output, err := evaluator.Eval(input)
	if err != nil {
		t.Fatalf("normal vals should process ref+echo: %v", err)
	}

	if output["key"] != "myvalue" {
		t.Errorf("expected 'myvalue', got %v", output["key"])
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
		expectError                bool
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
			name:                       "failOnMissingKey uppercase TRUE",
			awsLogLevel:                "",
			failOnMissingKey:           "TRUE",
			expectedLogLevel:           "off",
			expectedFailOnMissingKey:   true,
			expectedLogOutputDiscarded: true,
		},
		{
			name:                       "failOnMissingKey numeric 1",
			awsLogLevel:                "",
			failOnMissingKey:           "1",
			expectedLogLevel:           "off",
			expectedFailOnMissingKey:   true,
			expectedLogOutputDiscarded: true,
		},
		{
			name:                       "failOnMissingKey numeric 0",
			awsLogLevel:                "",
			failOnMissingKey:           "0",
			expectedLogLevel:           "off",
			expectedFailOnMissingKey:   false,
			expectedLogOutputDiscarded: true,
		},
		{
			name:             "failOnMissingKey invalid value",
			awsLogLevel:      "",
			failOnMissingKey: "invalid",
			expectError:      true,
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
			name:                       "aws log level OFF uppercase",
			awsLogLevel:                "OFF",
			failOnMissingKey:           "",
			expectedLogLevel:           "off",
			expectedFailOnMissingKey:   false,
			expectedLogOutputDiscarded: true,
		},
		{
			name:                       "aws log level Off mixed case",
			awsLogLevel:                "Off",
			failOnMissingKey:           "",
			expectedLogLevel:           "off",
			expectedFailOnMissingKey:   false,
			expectedLogOutputDiscarded: true,
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
			t.Setenv(envvar.AWSSDKLogLevel, tt.awsLogLevel)
			t.Setenv(envvar.ValsFailOnMissingKeyInMap, tt.failOnMissingKey)

			opts, err := buildValsOptions()

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), envvar.ValsFailOnMissingKeyInMap)
				return
			}

			require.NoError(t, err)

			assert.Equal(t, tt.expectedLogLevel, opts.AWSLogLevel)
			assert.Equal(t, tt.expectedFailOnMissingKey, opts.FailOnMissingKeyInMap)
			assert.Equal(t, valsCacheSize, opts.CacheSize)

			isDiscarded := opts.LogOutput == io.Discard
			assert.Equal(t, tt.expectedLogOutputDiscarded, isDiscarded)
		})
	}
}

// TestAWSSDKLogLevelConfiguration tests the AWS SDK log level configuration logic
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
			name:              "OFF uppercase",
			envValue:          "OFF",
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
			t.Setenv(envvar.AWSSDKLogLevel, tt.envValue)

			opts, err := buildValsOptions()
			require.NoError(t, err)

			assert.Equal(t, tt.expectedLogLevel, opts.AWSLogLevel)

			isDiscarded := opts.LogOutput == io.Discard
			assert.Equal(t, tt.expectedLogOutput, isDiscarded)
		})
	}
}

func TestBuildValsOptionsIntegration(t *testing.T) {
	t.Run("valid configuration produces working vals options", func(t *testing.T) {
		t.Setenv(envvar.AWSSDKLogLevel, "off")
		t.Setenv(envvar.ValsFailOnMissingKeyInMap, "true")

		opts, err := buildValsOptions()
		require.NoError(t, err)

		assert.Equal(t, valsCacheSize, opts.CacheSize)
		assert.Equal(t, "off", opts.AWSLogLevel)
		assert.True(t, opts.FailOnMissingKeyInMap)
		assert.Equal(t, io.Discard, opts.LogOutput)

		rt, err := vals.New(opts)
		require.NoError(t, err)
		assert.NotNil(t, rt)
	})
}
