// Package jsonwire provides the minimal JSON string encoding used by CONGA's
// machine-readable outputs (shell completion and version reporting) without
// pulling in encoding/json.
package jsonwire

import (
	"unicode/utf8"
)

// hexDigits are the lowercase hexadecimal digits used in \uXX escapes.
const hexDigits = "0123456789abcdef"

// AppendString appends the RFC 8259 JSON string literal for input
// (including the surrounding quotes) to dst and returns the extended buffer.
//
// Escaping: '"' and '\' are escaped with a backslash; the control characters
// \b, \f, \n, \r, and \t use their short escapes; every other byte below 0x20
// uses \u00xx. Valid UTF-8 passes through unchanged; invalid UTF-8 bytes are
// replaced with \ufffd so the output is always valid UTF-8. encoding/json is
// deliberately not imported (ADR 005 dependency boundaries).
func AppendString(dst []byte, input string) []byte {
	dst = append(dst, '"')

	for i := 0; i < len(input); {
		char := input[i]

		if char >= utf8.RuneSelf {
			r, size := utf8.DecodeRuneInString(input[i:])
			if r == utf8.RuneError && size == 1 {
				dst = append(dst, `\ufffd`...)
			} else {
				dst = append(dst, input[i:i+size]...)
			}

			i += size

			continue
		}

		switch char {
		case '"':
			dst = append(dst, `\"`...)
		case '\\':
			dst = append(dst, `\\`...)
		case '\b':
			dst = append(dst, `\b`...)
		case '\f':
			dst = append(dst, `\f`...)
		case '\n':
			dst = append(dst, `\n`...)
		case '\r':
			dst = append(dst, `\r`...)
		case '\t':
			dst = append(dst, `\t`...)
		default:
			if char < 0x20 {
				dst = append(dst, `\u00`...)
				dst = append(dst, hexDigits[char>>4], hexDigits[char&0xf])
			} else {
				dst = append(dst, char)
			}
		}

		i++
	}

	return append(dst, '"')
}
