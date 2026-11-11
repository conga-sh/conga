// Package yaml provides the YAML configuration decoder for CONGA applications.
//
// Register [Decode] with conga.App.RegisterDecoder to enable YAML configuration support:
//
//	app.RegisterDecoder(".yaml", yaml.Decode)
//	app.RegisterDecoder(".yml", yaml.Decode)
//
// # Key Mapping
//
// A top-level YAML key matches a flag's config tag as written or, when it has
// no tag, the flag name spelled in the app's key casing (conga.ConfigKeyCase,
// kebab-case by default). Other spellings and letter cases are unknown
// fields:
//
//	# config.yaml
//	http-port: 8080
//	database-url: "postgres://localhost/db"
//	max-retries: 5
//
// Parse failures wrap [ErrParse], and an empty document decodes to an empty map.
package yaml

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

var (
	// ErrParse indicates a syntax or parsing error in YAML configuration.
	ErrParse = errors.New("yaml parse error")
)

// Decode decodes YAML bytes into the generic map[string]any structure the configuration engine matches against flag bindings.
func Decode(data []byte) (map[string]any, error) {
	var result map[string]any

	dec := yaml.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&result); err != nil {
		if errors.Is(err, io.EOF) {
			return make(map[string]any), nil
		}

		return nil, fmt.Errorf("%w: %w", ErrParse, err)
	}

	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%w: trailing garbage after YAML document", ErrParse)
	}

	if result == nil {
		result = make(map[string]any)
	}

	return result, nil
}
