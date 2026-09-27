// Package toml provides the TOML configuration decoder for CONGA applications.
//
// [RawDecoder] matches config.Decoder, parsing TOML into the generic key-value
// map that the configuration engine validates against command struct tags.
// Register it with App.RegisterDecoder; parse failures wrap [ErrTOMLParse],
// and an empty document decodes to an empty map.
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

// RawDecoder decodes TOML bytes into a generic map structure for CONGA's AST validator.
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
