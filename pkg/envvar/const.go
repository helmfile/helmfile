package envvar

const (
	DisableInsecureFeatures = "HELMFILE_DISABLE_INSECURE_FEATURES"

	// use helm status to check if a release exists before installing it
	UseHelmStatusToCheckReleaseExistence = "HELMFILE_USE_HELM_STATUS_TO_CHECK_RELEASE_EXISTENCE"

	DisableRunnerUniqueID = "HELMFILE_DISABLE_RUNNER_UNIQUE_ID"
	Experimental          = "HELMFILE_EXPERIMENTAL" // environment variable for experimental features, expecting "true" lower case
	Environment           = "HELMFILE_ENVIRONMENT"
	FilePath              = "HELMFILE_FILE_PATH"
	TempDir               = "HELMFILE_TEMPDIR"
	UpgradeNoticeDisabled = "HELMFILE_UPGRADE_NOTICE_DISABLED"
	GoYamlV3              = "HELMFILE_GO_YAML_V3"
	CacheHome             = "HELMFILE_CACHE_HOME"
	Interactive           = "HELMFILE_INTERACTIVE"

	// AWSSDKLogLevel controls AWS SDK logging level
	// Valid values: "off" (default), "minimal", "standard", "verbose", or custom (e.g., "request,response")
	// - "off": No AWS SDK logging (secure default, prevents credential leakage)
	// - "minimal": Log retries only
	// - "standard": Log retries and requests (previous default behavior)
	// - "verbose": Log everything (requests, responses, bodies, signing)
	// - Custom: Comma-separated AWS SDK log modes
	// This is passed to vals Options.AWSLogLevel
	// Can be overridden by AWS_SDK_GO_LOG_LEVEL environment variable
	// See issue #2270 and vals PR #893
	AWSSDKLogLevel = "HELMFILE_AWS_SDK_LOG_LEVEL"
)
