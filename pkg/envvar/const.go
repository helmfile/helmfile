package envvar

const (
	DisableInsecureFeatures       = "HELMFILE_DISABLE_INSECURE_FEATURES"
	DisableRunnerUniqueID         = "HELMFILE_DISABLE_RUNNER_UNIQUE_ID"
	SkipInsecureTemplateFunctions = "HELMFILE_SKIP_INSECURE_TEMPLATE_FUNCTIONS"
	Experimental                  = "HELMFILE_EXPERIMENTAL" // environment variable for experimental features, expecting "true" lower case
	Environment                   = "HELMFILE_ENVIRONMENT"
	FilePath                      = "HELMFILE_FILE_PATH"
	TempDir                       = "HELMFILE_TEMPDIR"
	UpgradeNoticeDisabled         = "HELMFILE_UPGRADE_NOTICE_DISABLED"
	V1Mode                        = "HELMFILE_V1MODE"
	GoccyGoYaml                   = "HELMFILE_GOCCY_GOYAML"
	CacheHome                     = "HELMFILE_CACHE_HOME"
)
