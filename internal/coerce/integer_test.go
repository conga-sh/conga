package coerce_test

import (
	"reflect"
	"strconv"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/coerce"
	"github.com/conga-sh/conga/internal/intx"
)

// integerWidth describes one integer target type with the first literal
// above its range, in decimal and in hexadecimal.
type integerWidth struct {
	targetType     reflect.Type
	signed         bool
	overflow       string
	overflowHex    string
	negativeBelow  string // first literal below the range (signed types only)
	invalidMessage string // noun in "invalid <noun> value"
}

var integerWidths = []integerWidth{
	{targetType: reflect.TypeFor[int](), signed: true, overflow: "9223372036854775808", overflowHex: "0x8000000000000000", negativeBelow: "-9223372036854775809", invalidMessage: "integer"},
	{targetType: reflect.TypeFor[int8](), signed: true, overflow: "128", overflowHex: "0x80", negativeBelow: "-129", invalidMessage: "integer"},
	{targetType: reflect.TypeFor[int16](), signed: true, overflow: "32768", overflowHex: "0x8000", negativeBelow: "-32769", invalidMessage: "integer"},
	{targetType: reflect.TypeFor[int32](), signed: true, overflow: "2147483648", overflowHex: "0x80000000", negativeBelow: "-2147483649", invalidMessage: "integer"},
	{targetType: reflect.TypeFor[int64](), signed: true, overflow: "9223372036854775808", overflowHex: "0x8000000000000000", negativeBelow: "-9223372036854775809", invalidMessage: "integer"},
	{targetType: reflect.TypeFor[uint](), overflow: "18446744073709551616", overflowHex: "0x10000000000000000", invalidMessage: "unsigned integer"},
	{targetType: reflect.TypeFor[uint8](), overflow: "256", overflowHex: "0x100", invalidMessage: "unsigned integer"},
	{targetType: reflect.TypeFor[uint16](), overflow: "65536", overflowHex: "0x10000", invalidMessage: "unsigned integer"},
	{targetType: reflect.TypeFor[uint32](), overflow: "4294967296", overflowHex: "0x100000000", invalidMessage: "unsigned integer"},
	{targetType: reflect.TypeFor[uint64](), overflow: "18446744073709551616", overflowHex: "0x10000000000000000", invalidMessage: "unsigned integer"},
}

func TestScalar_IntegerSyntaxAccepted(t *testing.T) {
	t.Parallel()

	accepted := []struct {
		raw        string
		expected   int64
		signedOnly bool
	}{
		{raw: "0", expected: 0},
		{raw: "42", expected: 42},
		{raw: "0x2a", expected: 42},
		{raw: "0X2A", expected: 42},
		{raw: "0o52", expected: 42},
		{raw: "0O52", expected: 42},
		{raw: "0b101010", expected: 42},
		{raw: "0B101010", expected: 42},
		{raw: "-0", expected: 0, signedOnly: true},
		{raw: "+5", expected: 5, signedOnly: true},
		{raw: "-42", expected: -42, signedOnly: true},
		{raw: "-0x2a", expected: -42, signedOnly: true},
		{raw: "-0o52", expected: -42, signedOnly: true},
		{raw: "-0b101010", expected: -42, signedOnly: true},
	}

	for _, width := range integerWidths {
		for _, testCase := range accepted {
			if testCase.signedOnly && !width.signed {
				continue
			}

			t.Run(width.targetType.String()+"/"+testCase.raw, func(t *testing.T) {
				t.Parallel()

				expected := reflect.ValueOf(testCase.expected).Convert(width.targetType).Interface()

				value, err := coerce.Scalar(testCase.raw, width.targetType)

				assert.NoError(t, err)
				assert.Equal(t, expected, value)
			})
		}
	}
}

func TestScalar_IntegerSyntaxRejected(t *testing.T) {
	t.Parallel()

	rejected := []struct {
		raw          string
		hint         string
		signedOnly   bool
		unsignedOnly bool
	}{
		{raw: "0755", hint: "a leading zero is ambiguous: write 0o755 for octal or 755 for decimal"},
		{raw: "00", hint: "a leading zero is ambiguous: write 0o0 for octal or 0 for decimal"},
		{raw: "-07", hint: "a leading zero is ambiguous: write -0o7 for octal or -7 for decimal", signedOnly: true},
		{raw: "-07", unsignedOnly: true},
		{raw: "-0", unsignedOnly: true},
		{raw: "+5", unsignedOnly: true},
		{raw: "0x"},
		{raw: "0X"},
		{raw: "0o"},
		{raw: "0O"},
		{raw: "0b"},
		{raw: "0B"},
		{raw: "1_000"},
		{raw: "0x1_0"},
		{raw: "0b1_0"},
		{raw: "abc"},
	}

	for _, width := range integerWidths {
		for _, testCase := range rejected {
			if (testCase.signedOnly && !width.signed) || (testCase.unsignedOnly && width.signed) {
				continue
			}

			t.Run(width.targetType.String()+"/"+testCase.raw, func(t *testing.T) {
				t.Parallel()

				_, err := coerce.Scalar(testCase.raw, width.targetType)

				assert.Error(t, err)
				assert.Equal(t, "invalid "+width.invalidMessage+" value "+strconv.Quote(testCase.raw), err.Error())
				assert.Equal(t, testCase.hint, intx.HintOf(err))
				assert.IsError(t, err, coerce.ErrCoerce)
			})
		}
	}
}

func TestScalar_IntegerOutOfRange(t *testing.T) {
	t.Parallel()

	for _, width := range integerWidths {
		literals := []string{width.overflow, width.overflowHex}
		if width.signed {
			literals = append(literals, width.negativeBelow)
		}

		for _, raw := range literals {
			t.Run(width.targetType.String()+"/"+raw, func(t *testing.T) {
				t.Parallel()

				_, err := coerce.Scalar(raw, width.targetType)

				assert.Error(t, err)
				assert.Equal(t, "value "+strconv.Quote(raw)+" is out of range for "+width.targetType.String(), err.Error())
				assert.IsError(t, err, coerce.ErrCoerce)
				assert.IsError(t, err, strconv.ErrRange)
			})
		}
	}
}
