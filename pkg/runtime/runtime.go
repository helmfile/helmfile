package runtime

import (
	"fmt"
	"os"
	"strconv"

	"github.com/helmfile/helmfile/pkg/envvar"
)

// V1Mode is false by default for Helmfile v0.x and
// true by default for Helmfile v1.x
var (
	V1Mode bool

	// GoccyGoYaml is set to true in order to let Helmfile use
	// goccy/go-yaml instead of gopkg.in/yaml.v2.
	// It's false by default in Helmfile v0.x and true by default for Helmfile v1.x.
	GoccyGoYaml bool

	// We set this via ldflags at build-time so that we can use the
	// value specified at the build time as the runtime default.
	v1Mode string
)

func Info() string {
	yamlLib := "gopkg.in/yaml.v2"
	if GoccyGoYaml {
		yamlLib = "goccy/go-yaml"
	}

	return fmt.Sprintf("V1 mode = %v\nYAML library = %v", V1Mode, yamlLib)
}

func init() {
	// You can toggle the V1 mode at runtime via an envvar:
	// - Helmfile v1.x behaves like v0.x by running it with HELMFILE_V1MODE=false
	// - Helmfile v0.x behaves like v1.x by with HELMFILE_V1MODE=true
	switch os.Getenv(envvar.V1Mode) {
	case "true":
		V1Mode = true
	case "false":
		V1Mode = false
	default:
		V1Mode, _ = strconv.ParseBool(v1Mode)
	}

	// You can switch the YAML library at runtime via an envvar:
	switch os.Getenv(envvar.GoccyGoYaml) {
	case "true":
		GoccyGoYaml = true
	case "false":
		GoccyGoYaml = false
	default:
		GoccyGoYaml = V1Mode
	}
}
