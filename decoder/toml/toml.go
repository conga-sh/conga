// Package toml provides the TOML configuration decoder for CONGA applications.
//
// Register [Decode] with conga.App.RegisterDecoder to enable TOML configuration support:
//
//	app.RegisterDecoder(".toml", toml.Decode)
//
// # Key Mapping
//
// A top-level TOML key matches a flag's config tag as written or, when it has
// no tag, the flag name spelled in the app's key casing (conga.ConfigKeyCase,
// kebab-case by default). Other spellings and letter cases are unknown
// fields:
//
//	# config.toml
//	http-port = 8080
//	database-url = "postgres://localhost/db"
//	max-retries = 5
//
// Parse failures are returned as errors, and an empty document decodes to an empty map.
package toml

import (
	"github.com/pelletier/go-toml/v2"
)

// Decode decodes TOML bytes into the generic map[string]any structure the configuration engine matches against flag bindings.
func Decode(data []byte) (map[string]any, error) {
	var result map[string]any

	if err := toml.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	if result == nil {
		result = make(map[string]any)
	}

	return result, nil
}
