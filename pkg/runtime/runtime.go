package runtime

import (
	"fmt"
	"os"

	"github.com/helmfile/helmfile/pkg/envvar"
)

var (
	// GoccyGoYaml is set to true in order to let Helmfile use
	// goccy/go-yaml instead of gopkg.in/yaml.v3.
	// It's false by default in Helmfile until the GoccyGoYaml is ready to be used
	GoccyGoYaml bool
)

func Info() string {
	yamlLib := "gopkg.in/yaml.v3"
	if GoccyGoYaml {
		yamlLib = "goccy/go-yaml"
	}

	return fmt.Sprintf("YAML library = %v", yamlLib)
}

func init() {
	// You can switch the YAML library at runtime via an envvar:
	switch os.Getenv(envvar.GoccyGoYaml) {
	case "true":
		GoccyGoYaml = true
	case "false":
		GoccyGoYaml = false
	default:
		GoccyGoYaml = false
	}
}
