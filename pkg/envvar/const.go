package envvar

const (
	DisableInsecureFeatures = "HELMFILE_DISABLE_INSECURE_FEATURES" // insecure feature use will panic
	SkipInsecureFeatures    = "HELMFILE_SKIP_INSECURE_FEATURES"    // insecure features have no effect
	DisableRunnerUniqueID   = "HELMFILE_DISABLE_RUNNER_UNIQUE_ID"
	Experimental            = "HELMFILE_EXPERIMENTAL" // environment variable for experimental features, expecting "true" lower case
	Environment             = "HELMFILE_ENVIRONMENT"
	TempDir                 = "HELMFILE_TEMPDIR"
	Helm3                   = "HELMFILE_HELM3"
	UpgradeNoticeDisabled   = "HELMFILE_UPGRADE_NOTICE_DISABLED"
)
