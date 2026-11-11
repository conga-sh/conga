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
//
// # Numbers
//
// Numbers are decoded from their literal text rather than through float64, so
// integers bind exactly. An integer literal decodes to int64, or to uint64 when
// it exceeds the int64 range. A literal with a fraction or exponent decodes to
// float64. An integer outside both 64-bit ranges, or a number that overflows
// float64, wraps [ErrParse].
package json

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

var (
	// ErrParse indicates a syntax or parsing error in JSON configuration.
	ErrParse = errors.New("json parse error")
)

// Decode decodes JSON bytes into the generic map[string]any structure the configuration engine matches against flag bindings.
func Decode(data []byte) (map[string]any, error) {
	var result map[string]any

	if err := json.Unmarshal(data, &result, json.WithUnmarshalers(json.UnmarshalFromFunc(decodeNumber))); err != nil {
		var semantic *json.SemanticError
		if outOfRange := (*numberRangeError)(nil); errors.As(err, &semantic) && errors.As(semantic.Err, &outOfRange) {
			return nil, fmt.Errorf("%w: %s: %w", ErrParse, semantic.JSONPointer, outOfRange)
		}

		return nil, fmt.Errorf("%w: %w", ErrParse, err)
	}

	if result == nil {
		result = make(map[string]any)
	}

	return result, nil
}

// numberRangeError reports a JSON number that no int64, uint64, or float64
// holds. Decode unwraps it from the json package's SemanticError so the
// message names only the JSON pointer and the literal.
type numberRangeError struct {
	kind    string
	literal string
}

func (e *numberRangeError) Error() string {
	return e.kind + " " + e.literal + " out of range"
}

// decodeNumber decodes a JSON number held in an any from its literal text.
// Other value kinds return errors.ErrUnsupported so the default decoding
// applies.
func decodeNumber(decoder *jsontext.Decoder, target *any) error {
	if decoder.PeekKind() != '0' {
		return errors.ErrUnsupported
	}

	raw, err := decoder.ReadValue()
	if err != nil {
		return err
	}

	literal := string(raw)

	if !strings.ContainsAny(literal, ".eE") {
		if signed, err := strconv.ParseInt(literal, 10, 64); err == nil {
			*target = signed

			return nil
		}

		if unsigned, err := strconv.ParseUint(literal, 10, 64); err == nil {
			*target = unsigned

			return nil
		}

		return &numberRangeError{kind: "integer", literal: literal}
	}

	number, err := strconv.ParseFloat(literal, 64)
	if err != nil || math.IsInf(number, 0) {
		return &numberRangeError{kind: "number", literal: literal}
	}

	*target = number

	return nil
}
