// Package json provides the JSON configuration decoder for CONGA applications.
//
// [RawDecoder] matches config.Decoder, parsing JSON into the generic key-value
// map that the configuration engine validates against command struct tags.
// Register it with App.RegisterDecoder; syntax errors and trailing content
// after the first JSON document wrap [ErrJSONParse].
package json

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

var (
	// ErrJSONParse indicates a syntax or parsing error in JSON configuration.
	ErrJSONParse = errors.New("json parse error")
)

// RawDecoder decodes JSON bytes into a generic map structure for CONGA's AST validator.
func RawDecoder(data []byte) (map[string]any, error) {
	var result map[string]any

	dec := json.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&result); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrJSONParse, err)
	}

	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%w: trailing garbage after JSON document", ErrJSONParse)
	}

	return result, nil
}
