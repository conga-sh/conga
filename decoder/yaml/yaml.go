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
//
// # Numbers and timestamps
//
// Integers decode to int, or to uint64 above the int64 range. A plain integer
// literal outside both 64-bit ranges wraps [ErrParse] instead of decoding to
// an approximate float64 or a string. Unquoted timestamps decode to
// time.Time, which binds to a time.Time flag and reaches a string flag in
// RFC 3339 form.
package yaml

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math/big"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	// ErrParse indicates a syntax or parsing error in YAML configuration.
	ErrParse = errors.New("yaml parse error")
)

// Decode decodes YAML bytes into the generic map[string]any structure the configuration engine matches against flag bindings.
func Decode(data []byte) (map[string]any, error) {
	var document yaml.Node

	dec := yaml.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&document); err != nil {
		if errors.Is(err, io.EOF) {
			return make(map[string]any), nil
		}

		return nil, fmt.Errorf("%w: %w", ErrParse, err)
	}

	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%w: trailing garbage after YAML document", ErrParse)
	}

	if err := checkIntegerRange(&document); err != nil {
		return nil, err
	}

	var result map[string]any
	if err := document.Decode(&result); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrParse, err)
	}

	if result == nil {
		result = make(map[string]any)
	}

	return result, nil
}

// checkIntegerRange rejects plain integer literals that no 64-bit integer
// holds. yaml.v3 resolves such literals to an approximate float64 (decimal)
// or to a string (0x, 0o, 0b), losing the value without an error.
func checkIntegerRange(node *yaml.Node) error {
	if node.Kind == yaml.AliasNode {
		return nil
	}

	if node.Kind == yaml.ScalarNode && node.Style == 0 && node.ShortTag() != "!!int" && outsideIntegerRange(node.Value) {
		return fmt.Errorf("%w: line %d: integer %s out of range", ErrParse, node.Line, node.Value)
	}

	for _, child := range node.Content {
		if err := checkIntegerRange(child); err != nil {
			return err
		}
	}

	return nil
}

// outsideIntegerRange reports whether literal is integer syntax (decimal, or
// 0x, 0o, 0b prefixed, with optional sign and underscores) whose value fits
// neither int64 nor uint64.
func outsideIntegerRange(literal string) bool {
	plain := strings.ReplaceAll(literal, "_", "")

	unsigned := strings.TrimLeft(plain, "+-")
	if unsigned == "" || len(plain)-len(unsigned) > 1 {
		return false
	}

	base := 10
	if len(unsigned) > 1 && unsigned[0] == '0' && strings.ContainsRune("xob", rune(unsigned[1])) {
		base = 0
	}

	integer, ok := new(big.Int).SetString(plain, base)
	if !ok {
		return false
	}

	return !integer.IsInt64() && !integer.IsUint64()
}
