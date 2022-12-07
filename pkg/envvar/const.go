package envvar

const DisableInsecureFeatures = "HELMFILE_DISABLE_INSECURE_FEATURES"
const DisableRunnerUniqueID = "HELMFILE_DISABLE_RUNNER_UNIQUE_ID"
const SkipInsecureTemplateFunctions = "HELMFILE_SKIP_INSECURE_TEMPLATE_FUNCTIONS"
const Experimental = "HELMFILE_EXPERIMENTAL" // environment variable for experimental features, expecting "true" lower case
const Environment = "HELMFILE_ENVIRONMENT"
const TempDir = "HELMFILE_TEMPDIR"
const Helm3 = "HELMFILE_HELM3"
const UpgradeNoticeDisabled = "HELMFILE_UPGRADE_NOTICE_DISABLED"
