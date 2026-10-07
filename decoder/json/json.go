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
// # Strictness
//
// Decoding follows [encoding/json/v2] semantics and rejects inputs that RFC 8259
// leaves ambiguous: duplicate object member names and invalid UTF-8 in strings are
// errors rather than silently accepted. Reported errors carry the byte offset and
// JSON pointer of the offending value. Syntax errors, trailing content after the
// first JSON document, and non-object top-level values wrap [ErrJSONParse].
package json

import (
	"encoding/json/v2"
	"errors"
	"fmt"
)

var (
	// ErrJSONParse indicates a syntax or parsing error in JSON configuration.
	ErrJSONParse = errors.New("json parse error")
)

// RawDecoder decodes JSON bytes into the generic map[string]any structure the configuration engine matches against flag bindings.
func RawDecoder(data []byte) (map[string]any, error) {
	var result map[string]any

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrJSONParse, err)
	}

	return result, nil
}
