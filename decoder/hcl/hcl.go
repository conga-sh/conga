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
// Parse and semantic failures (undefined variables) and these rejections are
// returned as errors, and an empty document decodes to an empty map.
package hcl

import (
	"fmt"
	"math"
	"math/big"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
)

const (
	structureBodyType          structureKind = iota // the body is not an hclsyntax body
	structureLabeledBlock                           // a block carries labels
	structureDuplicateBlock                         // a block type is declared twice
	structureAttributeConflict                      // a block shares an attribute's name
)

// Decode decodes HCL bytes into the generic map[string]any structure the configuration engine matches against flag bindings.
func Decode(data []byte) (map[string]any, error) {
	file, diags := hclsyntax.ParseConfig(data, "config.hcl", hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return nil, diags
	}

	body, ok := file.Body.(*hclsyntax.Body)
	if !ok {
		return nil, &structureError{kind: structureBodyType, bodyType: fmt.Sprintf("%T", file.Body)}
	}

	return decodeHCLBody(body)
}

// numberRangeError reports an HCL number that no int64, uint64, or float64
// holds, matching the JSON decoder's wording.
type numberRangeError struct {
	literal string
}

func (e *numberRangeError) Error() string {
	return "number " + e.literal + " out of range"
}

// structureKind identifies how an HCL document's structure is rejected.
type structureKind uint8

// structureError reports an HCL document whose structure the decoder rejects.
// The decoder found the problem itself, so it has no cause.
type structureError struct {
	kind      structureKind
	position  hcl.Range // where the block is declared
	blockType string
	labels    []string  // the block's labels (structureLabeledBlock)
	other     hcl.Range // the first declaration or the conflicting attribute
	bodyType  string    // the unexpected body's type (structureBodyType)
}

func (e *structureError) Error() string {
	switch e.kind {
	case structureBodyType:
		return "unexpected HCL body type " + e.bodyType
	case structureLabeledBlock:
		return fmt.Sprintf("%s: block %q has labels %q; labeled blocks are not supported, use an unlabeled block",
			e.position, e.blockType, e.labels)
	case structureDuplicateBlock:
		return fmt.Sprintf("%s: duplicate block %q, first declared at %s", e.position, e.blockType, e.other)
	case structureAttributeConflict:
		return fmt.Sprintf("%s: block %q conflicts with the attribute of the same name at %s", e.position, e.blockType, e.other)
	default:
		return "invalid HCL structure"
	}
}

func decodeHCLBody(body *hclsyntax.Body) (map[string]any, error) {
	result := make(map[string]any)

	for name, attr := range body.Attributes {
		value, diags := attr.Expr.Value(nil)
		if diags.HasErrors() {
			return nil, diags
		}

		converted, err := convertCtyValue(value)
		if err != nil {
			return nil, fmt.Errorf("%s: attribute %q: %w", attr.SrcRange, name, err)
		}

		result[name] = converted
	}

	firstBlocks := make(map[string]hcl.Range)

	for _, block := range body.Blocks {
		if len(block.Labels) > 0 {
			return nil, &structureError{kind: structureLabeledBlock, position: block.DefRange(), blockType: block.Type, labels: block.Labels}
		}

		if first, isDuplicate := firstBlocks[block.Type]; isDuplicate {
			return nil, &structureError{kind: structureDuplicateBlock, position: block.DefRange(), blockType: block.Type, other: first}
		}

		if attr, isAttribute := body.Attributes[block.Type]; isAttribute {
			return nil, &structureError{kind: structureAttributeConflict, position: block.DefRange(), blockType: block.Type, other: attr.SrcRange}
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
