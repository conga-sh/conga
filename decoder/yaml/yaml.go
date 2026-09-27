// Package yaml provides the YAML configuration decoder for CONGA applications.
//
// [RawDecoder] matches config.Decoder, parsing YAML into the generic key-value
// map that the configuration engine validates against command struct tags.
// Register it with App.RegisterDecoder; parse failures wrap [ErrYAMLParse],
// and an empty document decodes to an empty map.
package yaml

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

var (
	// ErrYAMLParse indicates a syntax or parsing error in YAML configuration.
	ErrYAMLParse = errors.New("yaml parse error")
)

// RawDecoder decodes YAML bytes into a generic map structure for CONGA's AST validator.
func RawDecoder(data []byte) (map[string]any, error) {
	var result map[string]any

	dec := yaml.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&result); err != nil {
		if errors.Is(err, io.EOF) {
			return make(map[string]any), nil
		}

		return nil, fmt.Errorf("%w: %w", ErrYAMLParse, err)
	}

	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%w: trailing garbage after YAML document", ErrYAMLParse)
	}

	if result == nil {
		result = make(map[string]any)
	}

	return result, nil
}
