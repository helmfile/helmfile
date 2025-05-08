package yaml

import (
	"bytes"
	"io"
	"os"

	"github.com/goccy/go-yaml"
	v2 "gopkg.in/yaml.v2"

	"github.com/helmfile/helmfile/pkg/envvar"
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

	return v2.NewEncoder(w)
}

func Unmarshal(data []byte, v any) error {
	if runtime.GoccyGoYaml {
		return yaml.Unmarshal(data, v)
	}

	return v2.Unmarshal(data, v)
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

	decoder := v2.NewDecoder(bytes.NewReader(data))
	decoder.SetStrict(strict)

	return func(v any) error {
		return decoder.Decode(v)
	}
}

func Marshal(v any) ([]byte, error) {
	if runtime.GoccyGoYaml {
		var b bytes.Buffer
		yamlEncoderOpts := []yaml.EncodeOption{
			yaml.Indent(2),
			yaml.UseSingleQuote(true),
			yaml.UseLiteralStyleIfMultiline(true),
		}
		// enable JSON style if the envvar is set
		if os.Getenv(envvar.EanbleGoccyGoYamlJsonStyle) == "true" {
			yamlEncoderOpts = append(yamlEncoderOpts, yaml.JSON(), yaml.Flow(false))
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

	return v2.Marshal(v)
}
