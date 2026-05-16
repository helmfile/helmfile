package state

type EnvironmentSpec struct {
	Values      []any    `yaml:"values,omitempty"`
	Secrets     []string `yaml:"secrets,omitempty"`
	KubeContext string   `yaml:"kubeContext,omitempty"`

	// MissingFileHandler instructs helmfile to fail when unable to find a environment values file listed
	// under `environments.NAME.values`.
	//
	// Possible values are  "Error", "Warn", "Info", "Debug". The default is "Error".
	//
	// Use "Warn", "Info", or "Debug" if you want helmfile to not fail when a values file is missing, while just leaving
	// a message about the missing file at the log-level.
	MissingFileHandler *string `yaml:"missingFileHandler,omitempty"`
	// MissingFileHandlerConfig is composed of various settings for the MissingFileHandler
	MissingFileHandlerConfig *MissingFileHandlerConfig `yaml:"missingFileHandlerConfig,omitempty"`

	// MergeStrategy controls precedence when multiple values files are listed under `values`.
	//
	// "override" (default): later files override earlier files (the historical helmfile behavior).
	// "fallback":           earlier files take precedence; later files only fill gaps.
	//
	// Under the "fallback" strategy, an explicit non-nil value in an earlier file (including
	// the zero values false, 0, "", and empty list) is preserved against any later file. Maps
	// are deep-merged, so an earlier map does not block later files from adding nested keys.
	// An explicit null in an earlier file falls through to a later file's value (matching how
	// helmfile's MergeMaps treats nil from the override side elsewhere). Subsequent .gotmpl
	// values files can also reference values from earlier files via .Values.
	MergeStrategy string `yaml:"mergeStrategy,omitempty"`
}
