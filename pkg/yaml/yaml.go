package yaml

import (
	"bytes"
	"io"

	"github.com/goccy/go-yaml"
	v3 "gopkg.in/yaml.v3"

	"github.com/helmfile/helmfile/pkg/runtime"
)

type Encoder interface {
	Encode(any) error
	Close() error
}

// NewEncoder creates and returns a function that is used to encode a Go object to a YAML document
func NewEncoder(w io.Writer) Encoder {
	if runtime.GoccyGoYaml {
		return yaml.NewEncoder(w)
	}

	v3Encoder := v3.NewEncoder(w)
	v3Encoder.SetIndent(2)
	return v3Encoder
}

func Marshal(v any) ([]byte, error) {
	var b bytes.Buffer
	if runtime.GoccyGoYaml {
		yamlEncoderOpts := []yaml.EncodeOption{
			yaml.Indent(2),
			yaml.UseSingleQuote(true),
			yaml.UseLiteralStyleIfMultiline(true),
		}
		yamlEncoder := yaml.NewEncoder(
			&b,
			yamlEncoderOpts...,
		)
		err := yamlEncoder.Encode(v)
		defer func() {
			_ = yamlEncoder.Close()
		}()
		return b.Bytes(), err
	}

	v3Encoder := v3.NewEncoder(&b)
	v3Encoder.SetIndent(2)
	err := v3Encoder.Encode(v)
	defer func() {
		_ = v3Encoder.Close()
	}()
	return b.Bytes(), err
}

// NewDecoder creates and returns a function that is used to decode a YAML document
// contained within the YAML document stream per each call.
// When strict is true, this function ensures that every field found in the YAML document
// to have the corresponding field in the decoded Go struct.
func NewDecoder(data []byte, strict bool) func(any) error {
	if runtime.GoccyGoYaml {
		var opts []yaml.DecodeOption
		if strict {
			opts = append(opts, yaml.DisallowUnknownField())
		}
		// allow duplicate keys
		opts = append(opts, yaml.AllowDuplicateMapKey())

		decoder := yaml.NewDecoder(
			bytes.NewReader(data),
			opts...,
		)

		return func(v any) error {
			return decoder.Decode(v)
		}
	}

	decoder := v3.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(strict)

	return func(v any) error {
		return decoder.Decode(v)
	}
}

func Unmarshal(data []byte, v any) error {
	if runtime.GoccyGoYaml {
		return yaml.Unmarshal(data, v)
	}

	return v3.Unmarshal(data, v)
}
