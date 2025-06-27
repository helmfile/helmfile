package yaml

import (
	"bytes"
	"io"

	v2 "gopkg.in/yaml.v2"
	v3 "gopkg.in/yaml.v3"

	"github.com/helmfile/helmfile/pkg/runtime"
)

type Encoder interface {
	Encode(any) error
	Close() error
}

// NewEncoder creates and returns a function that is used to encode a Go object to a YAML document
func NewEncoder(w io.Writer) Encoder {
	if runtime.GoYamlV3 {
		v3Encoder := v3.NewEncoder(w)
		v3Encoder.SetIndent(2)
		return v3Encoder
	}
	return v2.NewEncoder(w)
}

func Marshal(v any) ([]byte, error) {
	var b bytes.Buffer
	yamlEncoder := NewEncoder(&b)
	err := yamlEncoder.Encode(v)
	defer func() {
		_ = yamlEncoder.Close()
	}()
	return b.Bytes(), err
}

// NewDecoder creates and returns a function that is used to decode a YAML document
// contained within the YAML document stream per each call.
// When strict is true, this function ensures that every field found in the YAML document
// to have the corresponding field in the decoded Go struct.
func NewDecoder(data []byte, strict bool) func(any) error {
	if runtime.GoYamlV3 {
		decoder := v3.NewDecoder(bytes.NewReader(data))
		decoder.KnownFields(strict)
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

func Unmarshal(data []byte, v any) error {
	if runtime.GoYamlV3 {
		return v3.Unmarshal(data, v)
	}

	return v2.Unmarshal(data, v)
}

// UnmarshalWithAppend unmarshals YAML data with support for key+ syntax
// This function first unmarshals the YAML normally, then processes any key+ syntax
func UnmarshalWithAppend(data []byte, v any) error {
	var rawData map[string]any
	if err := Unmarshal(data, &rawData); err != nil {
		return err
	}

	processor := NewAppendProcessor()
	processedData, err := processor.ProcessMap(rawData)
	if err != nil {
		return err
	}

	processedYAML, err := Marshal(processedData)
	if err != nil {
		return err
	}

	return Unmarshal(processedYAML, v)
}

// NewDecoderWithAppend creates and returns a function that is used to decode a YAML document
// with support for key+ syntax for appending values to lists
func NewDecoderWithAppend(data []byte, strict bool) func(any) error {
	if runtime.GoYamlV3 {
		decoder := v3.NewDecoder(bytes.NewReader(data))
		decoder.KnownFields(strict)
		return func(v any) error {
			var rawData map[string]any
			if err := decoder.Decode(&rawData); err != nil {
				return err
			}

			processor := NewAppendProcessor()
			processedData, err := processor.ProcessMap(rawData)
			if err != nil {
				return err
			}

			processedYAML, err := Marshal(processedData)
			if err != nil {
				return err
			}

			return v3.Unmarshal(processedYAML, v)
		}
	}

	decoder := v2.NewDecoder(bytes.NewReader(data))
	decoder.SetStrict(strict)

	return func(v any) error {
		var rawData map[string]any
		if err := decoder.Decode(&rawData); err != nil {
			return err
		}

		processor := NewAppendProcessor()
		processedData, err := processor.ProcessMap(rawData)
		if err != nil {
			return err
		}

		processedYAML, err := Marshal(processedData)
		if err != nil {
			return err
		}

		return v2.Unmarshal(processedYAML, v)
	}
}
