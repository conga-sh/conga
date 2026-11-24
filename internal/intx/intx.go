// Package intx parses the integer literal syntax shared by every CONGA integer
// value: flag and argument values, defaults, enum values, range bounds,
// feature levels, and counter values.
//
// A literal is an optional sign (signed parsers only) followed by either a
// prefixed form, 0x or 0X with hexadecimal digits, 0o or 0O with octal digits,
// 0b or 0B with binary digits, or decimal digits. A decimal literal cannot
// start with a zero unless it is the single digit 0, because 0755 reads as
// octal in some tools and as decimal in others. Underscore digit separators
// are rejected.
//
// The package has no package-level state, so it adds no initialization work
// to the binaries that link it.
package intx

import (
	"errors"
	"strconv"
	"strings"
)

const (
	// KindSyntax reports a value that is not an integer literal.
	KindSyntax ErrorKind = iota + 1
	// KindRange reports an integer literal that does not fit the bit size.
	KindRange
	// KindLeadingZero reports a decimal literal with a leading zero, such as
	// 0755.
	KindLeadingZero
)

// ErrorKind identifies why a literal failed to parse.
type ErrorKind uint8

// Error describes a literal that failed to parse. It unwraps to
// strconv.ErrRange for [KindRange] and to strconv.ErrSyntax otherwise.
type Error struct {
	// Raw is the literal as given.
	Raw string
	// Kind is the failure mode.
	Kind ErrorKind
}

// Error renders the failure with the literal quoted.
func (e *Error) Error() string {
	switch e.Kind {
	case KindRange:
		return "integer " + strconv.Quote(e.Raw) + " is out of range"
	case KindLeadingZero:
		return "integer " + strconv.Quote(e.Raw) + " has a leading zero"
	default:
		return "invalid integer " + strconv.Quote(e.Raw)
	}
}

// Unwrap returns strconv.ErrRange for an out-of-range literal and
// strconv.ErrSyntax for every other failure.
func (e *Error) Unwrap() error {
	if e.Kind == KindRange {
		return strconv.ErrRange
	}

	return strconv.ErrSyntax
}

// Note returns the detail on a leading-zero literal, or "" for every other
// failure: "a leading zero is ambiguous" for digits that form an octal number
// and "a leading zero is not allowed" for the rest.
func (e *Error) Note() string {
	if e.Kind != KindLeadingZero {
		return ""
	}

	if e.octalDigits() {
		return "a leading zero is ambiguous"
	}

	return "a leading zero is not allowed"
}

// Hint returns the correction for a leading-zero literal, or "" for every
// other failure. Digits that form an octal number get both spellings ("write
// 0o755 for octal or 755 for decimal"); other digits get the decimal one
// ("write 89").
func (e *Error) Hint() string {
	if e.Kind != KindLeadingZero {
		return ""
	}

	sign, digits := cutSign(e.Raw)

	decimal := strings.TrimLeft(digits, "0")
	if decimal == "" {
		decimal = "0"
	}

	if !e.octalDigits() {
		return "write " + sign + decimal
	}

	return "write " + sign + "0o" + decimal + " for octal or " + sign + decimal + " for decimal"
}

// ParseInt parses raw as a signed integer that fits bitSize bits (0 means the
// size of int). Failures return an [*Error].
func ParseInt(raw string, bitSize int) (int64, error) {
	sign, digits := cutSign(raw)

	base, digits, err := splitBase(raw, digits)
	if err != nil {
		return 0, err
	}

	value, err := strconv.ParseInt(sign+digits, base, bitSize)
	if err != nil {
		return 0, newError(raw, err)
	}

	return value, nil
}

// ParseUint parses raw as an unsigned integer that fits bitSize bits (0 means
// the size of uint). A sign is a syntax error. Failures return an [*Error].
func ParseUint(raw string, bitSize int) (uint64, error) {
	base, digits, err := splitBase(raw, raw)
	if err != nil {
		return 0, err
	}

	value, err := strconv.ParseUint(digits, base, bitSize)
	if err != nil {
		return 0, newError(raw, err)
	}

	return value, nil
}

// IsSignedLiteral reports whether raw has the syntax [ParseInt] accepts,
// without checking whether the value fits any bit size: so -0x8000000000000001
// is a literal and -0755 or 0x1_0 are not.
func IsSignedLiteral(raw string) bool {
	_, digits := cutSign(raw)

	base, digits, err := splitBase(raw, digits)
	if err != nil || digits == "" {
		return false
	}

	for i := range len(digits) {
		if digitValue(digits[i]) >= base {
			return false
		}
	}

	return true
}

// octalDigits reports whether the literal's digits are all octal.
func (e *Error) octalDigits() bool {
	_, digits := cutSign(e.Raw)

	return strings.Trim(digits, "01234567") == ""
}

// cutSign splits a leading + or - from raw.
func cutSign(raw string) (string, string) {
	if raw != "" && (raw[0] == '+' || raw[0] == '-') {
		return raw[:1], raw[1:]
	}

	return "", raw
}

// splitBase reads the base prefix of the unsigned part of raw and returns the
// base with the digits after the prefix. Digits that are rejected outright (a
// prefix with no digits or a sign after the prefix, a leading zero) fail here;
// digits invalid for the base fail in strconv.
func splitBase(raw, unsigned string) (int, string, error) {
	if len(unsigned) < 2 || unsigned[0] != '0' {
		return 10, unsigned, nil
	}

	var base int

	switch unsigned[1] {
	case 'x', 'X':
		base = 16
	case 'o', 'O':
		base = 8
	case 'b', 'B':
		base = 2
	default:
		if strings.Trim(unsigned, "0123456789") == "" {
			return 0, "", &Error{Raw: raw, Kind: KindLeadingZero}
		}

		return 10, unsigned, nil
	}

	digits := unsigned[2:]
	if digits == "" || digits[0] == '+' || digits[0] == '-' {
		return 0, "", &Error{Raw: raw, Kind: KindSyntax}
	}

	return base, digits, nil
}

// digitValue returns the value of the digit character in bases up to 16,
// or 16 for any other character.
func digitValue(character byte) int {
	switch {
	case '0' <= character && character <= '9':
		return int(character - '0')
	case 'a' <= character && character <= 'f':
		return int(character-'a') + 10
	case 'A' <= character && character <= 'F':
		return int(character-'A') + 10
	default:
		return 16
	}
}

func newError(raw string, err error) error {
	if errors.Is(err, strconv.ErrRange) {
		return &Error{Raw: raw, Kind: KindRange}
	}

	return &Error{Raw: raw, Kind: KindSyntax}
}
