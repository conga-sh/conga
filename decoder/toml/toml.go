// Package toml provides the TOML configuration decoder for CONGA applications.
//
// Register [RawDecoder] with App.RegisterDecoder to enable TOML configuration support:
//
//	app.RegisterDecoder(".toml", toml.RawDecoder)
//
// # Key Mapping
//
// Top-level TOML keys match flag names using kebab-case, snake_case, or camelCase:
//
//	# config.toml
//	http-port = 8080
//	database_url = "postgres://localhost/db"
//	maxRetries = 5
//
// Parse failures wrap [ErrTOMLParse], and an empty document decodes to an empty map.
package toml

import (
	"errors"
	"fmt"

	"github.com/pelletier/go-toml/v2"
)

var (
	// ErrTOMLParse indicates a syntax or parsing error in TOML configuration.
	ErrTOMLParse = errors.New("toml parse error")
)

// RawDecoder decodes TOML bytes into the generic map[string]any structure the configuration engine matches against flag bindings.
func RawDecoder(data []byte) (map[string]any, error) {
	var result map[string]any

	if err := toml.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrTOMLParse, err)
	}

	if result == nil {
		result = make(map[string]any)
	}

	return result, nil
}
