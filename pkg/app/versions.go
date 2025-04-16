package app

import "github.com/helmfile/helmfile/pkg/version"

var helmPlugins = []helmRecommendedPlugin{
	{
		name:    "diff",
		version: version.HelmDiffRecommendedVersion,
		repo:    "https://github.com/databus23/helm-diff",
	},
	{
		name:    "secrets",
		version: version.HelmSecretsRecommendedVersion,
		repo:    "https://github.com/jkroepke/helm-secrets",
	},
	{
		name:    "s3",
		version: version.HelmS3RecommendedVersion,
		repo:    "https://github.com/hypnoglow/helm-s3.git",
	},
	{
		name:    "helm-git",
		version: version.HelmGitRecommendedVersion,
		repo:    "https://github.com/aslafy-z/helm-git.git",
	},
}
