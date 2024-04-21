package state

type EnvironmentSpec struct {
	Values        []any    `yaml:"values,omitempty"`
	LayeredValues []any    `yaml:"layeredValues,omitempty"`
	Secrets       []string `yaml:"secrets,omitempty"`
	KubeContext   string   `yaml:"kubeContext,omitempty"`

	// MissingFileHandler instructs helmfile to fail when unable to find a environment values file listed
	// under `environments.NAME.values`.
	//
	// Possible values are  "Error", "Warn", "Info", "Debug". The default is "Error".
	//
	// Use "Warn", "Info", or "Debug" if you want helmfile to not fail when a values file is missing, while just leaving
	// a message about the missing file at the log-level.
	MissingFileHandler *string `yaml:"missingFileHandler,omitempty"`
	// MissingFileHandlerConfig is composed of various settings for the MissingFileHandler
	MissingFileHandlerConfig MissingFileHandlerConfig `yaml:"missingFileHandlerConfig,omitempty"`
}
