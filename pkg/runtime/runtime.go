package runtime

import (
	"fmt"
)

var (
	// GoccyGoYaml is set to true in order to let Helmfile use
	// goccy/go-yaml instead of gopkg.in/yaml.v2.
	GoccyGoYaml bool
)

func Info() string {
	yamlLib := "goccy/go-yaml"
	return fmt.Sprintf("YAML library = %v", yamlLib)
}

func init() {
	GoccyGoYaml = true
}
