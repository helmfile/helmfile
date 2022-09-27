package maputil

import (
	"bytes"

	"gopkg.in/yaml.v3"
)

func YamlMarshal(v interface{}) ([]byte, error) {
	var b bytes.Buffer
	yamlEncoder := yaml.NewEncoder(&b)
	yamlEncoder.SetIndent(2)
	err := yamlEncoder.Encode(v)
	defer func() {
		_ = yamlEncoder.Close()
	}()
	return b.Bytes(), err
}
