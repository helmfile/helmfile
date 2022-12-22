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

// YamlDecoder creates and returns a function that is used to decode a YAML document
// contained within the YAML document stream per each call.
// When strict is true, this function ensures that every field found in the YAML document
// to have the corresponding field in the decoded Go struct.
func YamlDecoder(data []byte, strict bool) func(interface{}) error {
	if GoYamlV3 {
		decoder := yaml.NewDecoder(bytes.NewReader(data))
		decoder.KnownFields(strict)

		return func(v interface{}) error {
			return decoder.Decode(v)
		}
	}

	decoder := v2.NewDecoder(bytes.NewReader(data))
	decoder.SetStrict(strict)

	return func(v interface{}) error {
		return decoder.Decode(v)
	}
}

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
