// Package json provides the JSON configuration decoder for CONGA applications.
//
// Register [Decode] with conga.App.RegisterDecoder to enable JSON configuration support:
//
//	app.RegisterDecoder(".json", json.Decode)
//
// # Key Mapping
//
// A top-level JSON key matches a flag's config tag as written or, when it has
// no tag, the flag name spelled in the app's key casing (conga.ConfigKeyCase,
// kebab-case by default). Other spellings and letter cases are unknown
// fields:
//
//	{
//	  "port": 8080,
//	  "database-url": "postgres://localhost/db",
//	  "max-retries": 5
//	}
//
// # Strictness
//
// Decoding follows [encoding/json/v2] semantics and rejects inputs that RFC 8259
// leaves ambiguous: duplicate object member names and invalid UTF-8 in strings are
// errors rather than silently accepted. Reported errors carry the byte offset and
// JSON pointer of the offending value. Syntax errors, trailing content after the
// first JSON document, and non-object top-level values wrap [ErrParse]. A JSON
// null document decodes to an empty map.
package json

import (
	"encoding/json/v2"
	"errors"
	"fmt"
)

var (
	// ErrParse indicates a syntax or parsing error in JSON configuration.
	ErrParse = errors.New("json parse error")
)

// Decode decodes JSON bytes into the generic map[string]any structure the configuration engine matches against flag bindings.
func Decode(data []byte) (map[string]any, error) {
	var result map[string]any

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrParse, err)
	}

	if result == nil {
		result = make(map[string]any)
	}

	return result, nil
}
