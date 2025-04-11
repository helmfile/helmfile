package app

const (
	HelmRequiredVersion           = "v3.16.4"
	HelmRecommendedVersion        = "v3.17.3"
	HelmDiffRecommendedVersion    = "v3.11.0"
	HelmSecretsRecommendedVersion = "v4.6.3"
	HelmGitRecommendedVersion     = "v1.3.0"
	HelmS3RecommendedVersion      = "v0.16.3"
	HelmInstallCommand            = "https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3"
)

var helmPlugins = []helmRecommendedPlugin{
	{
		name:    "diff",
		version: HelmDiffRecommendedVersion,
		repo:    "https://github.com/databus23/helm-diff",
	},
	{
		name:    "secrets",
		version: HelmSecretsRecommendedVersion,
		repo:    "https://github.com/jkroepke/helm-secrets",
	},
	{
		name:    "s3",
		version: HelmS3RecommendedVersion,
		repo:    "https://github.com/hypnoglow/helm-s3.git",
	},
	{
		name:    "helm-git",
		version: HelmGitRecommendedVersion,
		repo:    "https://github.com/aslafy-z/helm-git.git",
	},
}
