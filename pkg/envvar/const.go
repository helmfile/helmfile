package envvar

const (
	DisableInsecureFeatures = "HELMFILE_DISABLE_INSECURE_FEATURES"

	// use helm status to check if a release exists before installing it
	UseHelmStatusToCheckReleaseExistence = "HELMFILE_USE_HELM_STATUS_TO_CHECK_RELEASE_EXISTENCE"

	// TODO: Remove this function once Helmfile v0.x
	SkipInsecureTemplateFunctions = "HELMFILE_SKIP_INSECURE_TEMPLATE_FUNCTIONS"

	DisableRunnerUniqueID = "HELMFILE_DISABLE_RUNNER_UNIQUE_ID"
	Experimental          = "HELMFILE_EXPERIMENTAL" // environment variable for experimental features, expecting "true" lower case
	Environment           = "HELMFILE_ENVIRONMENT"
	FilePath              = "HELMFILE_FILE_PATH"
	TempDir               = "HELMFILE_TEMPDIR"
	UpgradeNoticeDisabled = "HELMFILE_UPGRADE_NOTICE_DISABLED"
	V1Mode                = "HELMFILE_V1MODE"
	GoccyGoYaml           = "HELMFILE_GOCCY_GOYAML"
	CacheHome             = "HELMFILE_CACHE_HOME"
	LayeredEnvValuesFlag  = "HELMFILE_LAYERED_ENV_VALUES"
)
