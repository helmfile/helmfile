package yaml

import (
	"bytes"
	"io"

	"github.com/goccy/go-yaml"
	v2 "gopkg.in/yaml.v2"

	"github.com/helmfile/helmfile/pkg/runtime"
)

type Encoder interface {
	Encode(interface{}) error
	Close() error
}

// NewEncoder creates and returns a function that is used to encode a Go object to a YAML document
func NewEncoder(w io.Writer) Encoder {
	if runtime.GoccyGoYaml {
		return yaml.NewEncoder(w)
	}

	return v2.NewEncoder(w)
}

func Unmarshal(data []byte, v interface{}) error {
	if runtime.GoccyGoYaml {
		return yaml.Unmarshal(data, v)
	}

	return v2.Unmarshal(data, v)
}

// NewDecoder creates and returns a function that is used to decode a YAML document
// contained within the YAML document stream per each call.
// When strict is true, this function ensures that every field found in the YAML document
// to have the corresponding field in the decoded Go struct.
func NewDecoder(data []byte, strict bool) func(interface{}) error {
	if runtime.GoccyGoYaml {
		var opts []yaml.DecodeOption
		if strict {
			opts = append(opts, yaml.DisallowUnknownField())
		}

		decoder := yaml.NewDecoder(
			bytes.NewReader(data),
			opts...,
		)

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

func Marshal(v interface{}) ([]byte, error) {
	if runtime.GoccyGoYaml {
		var b bytes.Buffer
		yamlEncoder := yaml.NewEncoder(
			&b,
			yaml.Indent(2),
		)
		err := yamlEncoder.Encode(v)
		defer func() {
			_ = yamlEncoder.Close()
		}()
		return b.Bytes(), err
	}

	return v2.Marshal(v)
}
