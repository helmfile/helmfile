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
	GoccyGoYaml           = "HELMFILE_GOCCY_GOYAML"
	CacheHome             = "HELMFILE_CACHE_HOME"
	Interactive           = "HELMFILE_INTERACTIVE"
)
