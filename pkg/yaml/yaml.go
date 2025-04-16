package yaml

import (
	"bytes"
	"io"

	"github.com/goccy/go-yaml"
)

type Encoder interface {
	Encode(any) error
	Close() error
}

// NewEncoder creates and returns a function that is used to encode a Go object to a YAML document
func NewEncoder(w io.Writer) Encoder {
	return yaml.NewEncoder(w)
}

func Unmarshal(data []byte, v any) error {
	return yaml.Unmarshal(data, v)
}

// NewDecoder creates and returns a function that is used to decode a YAML document
// contained within the YAML document stream per each call.
// When strict is true, this function ensures that every field found in the YAML document
// to have the corresponding field in the decoded Go struct.
func NewDecoder(data []byte, strict bool) func(any) error {
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

func Marshal(v any) ([]byte, error) {
	var b bytes.Buffer
	yamlEncoder := yaml.NewEncoder(
		&b,
		yaml.Indent(2),
		yaml.UseSingleQuote(true),
		yaml.UseLiteralStyleIfMultiline(true),
	)
	err := yamlEncoder.Encode(v)
	defer func() {
		_ = yamlEncoder.Close()
	}()
	return b.Bytes(), err
}
