// Package hcl provides the HCL configuration decoder for CONGA applications.
//
// Register [Decode] with conga.App.RegisterDecoder to enable HCL configuration support:
//
//	app.RegisterDecoder(".hcl", hcl.Decode)
//
// # Key Mapping
//
// Top-level HCL attributes match flag names using kebab-case, snake_case, or camelCase:
//
//	# config.hcl
//	http-port = 8080
//	database_url = "postgres://localhost/db"
//	maxRetries = 5
//
// Blocks decode as nested maps keyed by block type; duplicate block types are
// rejected. Parse and semantic failures (undefined variables) wrap [ErrParse],
// and an empty document decodes to an empty map.
package hcl

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
)

var (
	// ErrParse indicates a syntax or semantic error in HCL configuration.
	ErrParse = errors.New("hcl parse error")
)

// Decode decodes HCL bytes into the generic map[string]any structure the configuration engine matches against flag bindings.
func Decode(data []byte) (map[string]any, error) {
	file, diags := hclsyntax.ParseConfig(data, "config.hcl", hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return nil, fmt.Errorf("%w: %w", ErrParse, diags)
	}

	body, ok := file.Body.(*hclsyntax.Body)
	if !ok {
		return nil, fmt.Errorf("%w: unexpected HCL body type %T", ErrParse, file.Body)
	}

	return decodeHCLBody(body)
}

func decodeHCLBody(body *hclsyntax.Body) (map[string]any, error) {
	result := make(map[string]any)
	seenBlocks := make(map[string]bool)

	for name, attr := range body.Attributes {
		value, diags := attr.Expr.Value(nil)
		if diags.HasErrors() {
			return nil, fmt.Errorf("%w: %w", ErrParse, diags)
		}

		result[name] = convertCtyValue(value)
	}

	for _, block := range body.Blocks {
		if seenBlocks[block.Type] {
			return nil, fmt.Errorf("%w: duplicate block %q", ErrParse, block.Type)
		}

		seenBlocks[block.Type] = true

		nested, err := decodeHCLBody(block.Body)
		if err != nil {
			return nil, err
		}

		result[block.Type] = nested
	}

	return result, nil
}

func convertCtyValue(value cty.Value) any {
	if value.IsNull() || !value.IsKnown() {
		return nil
	}

	switch value.Type() {
	case cty.String:
		return value.AsString()
	case cty.Bool:
		return value.True()
	case cty.Number:
		bigFloat := value.AsBigFloat()
		if bigFloat.IsInt() {
			if i64, acc := bigFloat.Int64(); acc == big.Exact {
				return i64
			}

			if u64, acc := bigFloat.Uint64(); acc == big.Exact {
				return u64
			}

			return bigFloat.Text('f', -1)
		}

		f64, _ := bigFloat.Float64()

		return f64
	default:
		if value.CanIterateElements() {
			if value.Type().IsObjectType() || value.Type().IsMapType() {
				result := make(map[string]any)

				for it := value.ElementIterator(); it.Next(); {
					key, elem := it.Element()
					result[key.AsString()] = convertCtyValue(elem)
				}

				return result
			}

			var list []any

			for it := value.ElementIterator(); it.Next(); {
				_, elem := it.Element()
				list = append(list, convertCtyValue(elem))
			}

			return list
		}

		return value.GoString()
	}
}
