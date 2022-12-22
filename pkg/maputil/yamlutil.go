package maputil

import (
	"bytes"

	v2 "gopkg.in/yaml.v2"
	"gopkg.in/yaml.v3"
)

var (
	// We'll derive the default from the build once
	// is merged
	GoYamlV3 bool = true
)

func YamlMarshal(v interface{}) ([]byte, error) {
	if GoYamlV3 {
		var b bytes.Buffer
		yamlEncoder := yaml.NewEncoder(&b)
		yamlEncoder.SetIndent(2)
		err := yamlEncoder.Encode(v)
		defer func() {
			_ = yamlEncoder.Close()
		}()
		return b.Bytes(), err
	}

	return v2.Marshal(v)
}
