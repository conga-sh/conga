// Package hcl provides the HCL configuration decoder for CONGA applications.
//
// Register [Decode] with conga.App.RegisterDecoder to enable HCL configuration support:
//
//	app.RegisterDecoder(".hcl", hcl.Decode)
//
// # Key Mapping
//
// A top-level HCL attribute matches a flag's config tag as written or, when
// it has no tag, the flag name spelled in the app's key casing
// (conga.ConfigKeyCase, kebab-case by default). Other spellings and letter
// cases are unknown fields:
//
//	# config.hcl
//	http-port = 8080
//	database-url = "postgres://localhost/db"
//	max-retries = 5
//
// Blocks decode as nested maps keyed by block type. A labeled block
// (server "web" { ... }), a repeated block type, and a block that shares its
// name with an attribute are rejected.
//
// # Numbers
//
// Numbers decode to int64, or to uint64 above the int64 range, or to float64
// when they have a fraction. HCL keeps a number's value but not its spelling,
// so a whole number outside both 64-bit integer ranges is rejected whether it
// was written in full or with an exponent (1e30), as is a number that
// overflows float64.
//
// Parse and semantic failures (undefined variables) and these rejections wrap
// [ErrParse], and an empty document decodes to an empty map.
package hcl

import (
	"errors"
	"fmt"
	"math"
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

	for name, attr := range body.Attributes {
		value, diags := attr.Expr.Value(nil)
		if diags.HasErrors() {
			return nil, fmt.Errorf("%w: %w", ErrParse, diags)
		}

		converted, err := convertCtyValue(value)
		if err != nil {
			return nil, fmt.Errorf("%w: %s: attribute %q: %w", ErrParse, attr.SrcRange, name, err)
		}

		result[name] = converted
	}

	firstBlocks := make(map[string]hcl.Range)

	for _, block := range body.Blocks {
		if len(block.Labels) > 0 {
			return nil, fmt.Errorf("%w: %s: block %q has labels %q; labeled blocks are not supported, use an unlabeled block",
				ErrParse, block.DefRange(), block.Type, block.Labels)
		}

		if first, isDuplicate := firstBlocks[block.Type]; isDuplicate {
			return nil, fmt.Errorf("%w: %s: duplicate block %q, first declared at %s",
				ErrParse, block.DefRange(), block.Type, first)
		}

		if attr, isAttribute := body.Attributes[block.Type]; isAttribute {
			return nil, fmt.Errorf("%w: %s: block %q conflicts with the attribute of the same name at %s",
				ErrParse, block.DefRange(), block.Type, attr.SrcRange)
		}

		firstBlocks[block.Type] = block.DefRange()

		nested, err := decodeHCLBody(block.Body)
		if err != nil {
			return nil, err
		}

		result[block.Type] = nested
	}

	return result, nil
}

func convertCtyValue(value cty.Value) (any, error) {
	if value.IsNull() || !value.IsKnown() {
		return nil, nil //nolint:nilnil // a null HCL value decodes to a nil map entry, which is not an error
	}

	switch value.Type() {
	case cty.String:
		return value.AsString(), nil
	case cty.Bool:
		return value.True(), nil
	case cty.Number:
		return convertCtyNumber(value.AsBigFloat())
	default:
		if !value.CanIterateElements() {
			return value.GoString(), nil
		}

		if value.Type().IsObjectType() || value.Type().IsMapType() {
			result := make(map[string]any)

			for it := value.ElementIterator(); it.Next(); {
				key, elem := it.Element()

				converted, err := convertCtyValue(elem)
				if err != nil {
					return nil, err
				}

				result[key.AsString()] = converted
			}

			return result, nil
		}

		var list []any

		for it := value.ElementIterator(); it.Next(); {
			_, elem := it.Element()

			converted, err := convertCtyValue(elem)
			if err != nil {
				return nil, err
			}

			list = append(list, converted)
		}

		return list, nil
	}
}

// numberRangeError reports an HCL number that no int64, uint64, or float64
// holds, matching the JSON decoder's wording.
type numberRangeError struct {
	literal string
}

func (e *numberRangeError) Error() string {
	return "number " + e.literal + " out of range"
}

// convertCtyNumber narrows an HCL number to int64, uint64, or float64,
// rejecting values no 64-bit type holds.
func convertCtyNumber(bigFloat *big.Float) (any, error) {
	if bigFloat.IsInt() {
		if signed, accuracy := bigFloat.Int64(); accuracy == big.Exact {
			return signed, nil
		}

		if unsigned, accuracy := bigFloat.Uint64(); accuracy == big.Exact {
			return unsigned, nil
		}

		return nil, &numberRangeError{literal: formatOversized(bigFloat)}
	}

	number, _ := bigFloat.Float64()
	if math.IsInf(number, 0) {
		return nil, &numberRangeError{literal: formatOversized(bigFloat)}
	}

	return number, nil
}

// formatOversized renders an out-of-range number for an error message: in
// full when it is short enough to compare with the source, in exponent form
// otherwise.
func formatOversized(bigFloat *big.Float) string {
	const maxPlainDigits = 40

	if plain := bigFloat.Text('f', -1); len(plain) <= maxPlainDigits {
		return plain
	}

	return bigFloat.Text('g', 10)
}
