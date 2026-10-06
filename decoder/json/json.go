// Package json provides the JSON configuration decoder for CONGA applications.
//
// Register [RawDecoder] with App.RegisterDecoder to enable JSON configuration support:
//
//	app.RegisterDecoder(".json", json.RawDecoder)
//
// # Key Mapping
//
// Top-level JSON keys match flag names using kebab-case, snake_case, or camelCase:
//
//	{
//	  "port": 8080,
//	  "database_url": "postgres://localhost/db",
//	  "maxRetries": 5
//	}
//
// Syntax errors and trailing content after the first JSON document wrap [ErrJSONParse].
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

// RawDecoder decodes JSON bytes into the generic map[string]any structure the configuration engine matches against flag bindings.
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
