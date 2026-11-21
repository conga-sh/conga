package intx_test

import (
	"errors"
	"fmt"
	"strconv"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/intx"
)

func TestParseInt(t *testing.T) {
	t.Parallel()

	cases := []struct {
		raw      string
		bitSize  int
		expected int64
	}{
		{raw: "0", bitSize: 64, expected: 0},
		{raw: "-0", bitSize: 64, expected: 0},
		{raw: "+5", bitSize: 64, expected: 5},
		{raw: "755", bitSize: 64, expected: 755},
		{raw: "-755", bitSize: 64, expected: -755},
		{raw: "0x1f", bitSize: 64, expected: 31},
		{raw: "0X1F", bitSize: 64, expected: 31},
		{raw: "-0x10", bitSize: 64, expected: -16},
		{raw: "+0x10", bitSize: 64, expected: 16},
		{raw: "0o755", bitSize: 64, expected: 493},
		{raw: "0O755", bitSize: 64, expected: 493},
		{raw: "0b101", bitSize: 64, expected: 5},
		{raw: "0B101", bitSize: 64, expected: 5},
		{raw: "0x00ff", bitSize: 64, expected: 255},
		{raw: "-128", bitSize: 8, expected: -128},
		{raw: "127", bitSize: 8, expected: 127},
		{raw: "-0x80", bitSize: 8, expected: -128},
		{raw: "0x7f", bitSize: 8, expected: 127},
		{raw: "-32768", bitSize: 16, expected: -32768},
		{raw: "0x7fff", bitSize: 16, expected: 32767},
		{raw: "-2147483648", bitSize: 32, expected: -2147483648},
		{raw: "0x7fffffff", bitSize: 32, expected: 2147483647},
		{raw: "-9223372036854775808", bitSize: 64, expected: -9223372036854775808},
		{raw: "0x7fffffffffffffff", bitSize: 64, expected: 9223372036854775807},
		{raw: "0x7fffffffffffffff", bitSize: 0, expected: 9223372036854775807},
	}

	for _, testCase := range cases {
		t.Run(fmt.Sprintf("%s/%d", testCase.raw, testCase.bitSize), func(t *testing.T) {
			t.Parallel()

			value, err := intx.ParseInt(testCase.raw, testCase.bitSize)

			assert.NoError(t, err)
			assert.Equal(t, testCase.expected, value)
		})
	}
}

func TestParseUint(t *testing.T) {
	t.Parallel()

	cases := []struct {
		raw      string
		bitSize  int
		expected uint64
	}{
		{raw: "0", bitSize: 64, expected: 0},
		{raw: "755", bitSize: 64, expected: 755},
		{raw: "0x1f", bitSize: 64, expected: 31},
		{raw: "0X1F", bitSize: 64, expected: 31},
		{raw: "0o755", bitSize: 64, expected: 493},
		{raw: "0O755", bitSize: 64, expected: 493},
		{raw: "0b101", bitSize: 64, expected: 5},
		{raw: "0B101", bitSize: 64, expected: 5},
		{raw: "255", bitSize: 8, expected: 255},
		{raw: "0xff", bitSize: 8, expected: 255},
		{raw: "0xffff", bitSize: 16, expected: 65535},
		{raw: "0xffffffff", bitSize: 32, expected: 4294967295},
		{raw: "0xffffffffffffffff", bitSize: 64, expected: 18446744073709551615},
		{raw: "18446744073709551615", bitSize: 0, expected: 18446744073709551615},
	}

	for _, testCase := range cases {
		t.Run(fmt.Sprintf("%s/%d", testCase.raw, testCase.bitSize), func(t *testing.T) {
			t.Parallel()

			value, err := intx.ParseUint(testCase.raw, testCase.bitSize)

			assert.NoError(t, err)
			assert.Equal(t, testCase.expected, value)
		})
	}
}

func TestParseInt_Rejected(t *testing.T) {
	t.Parallel()

	cases := []struct {
		raw     string
		bitSize int
		message string
		hint    string
		cause   error
	}{
		{raw: "0755", bitSize: 64, message: `integer "0755" has a leading zero`, hint: "a leading zero is ambiguous: write 0o755 for octal or 755 for decimal", cause: strconv.ErrSyntax},
		{raw: "-07", bitSize: 64, message: `integer "-07" has a leading zero`, hint: "a leading zero is ambiguous: write -0o7 for octal or -7 for decimal", cause: strconv.ErrSyntax},
		{raw: "+07", bitSize: 64, message: `integer "+07" has a leading zero`, hint: "a leading zero is ambiguous: write +0o7 for octal or +7 for decimal", cause: strconv.ErrSyntax},
		{raw: "00", bitSize: 64, message: `integer "00" has a leading zero`, hint: "a leading zero is ambiguous: write 0o0 for octal or 0 for decimal", cause: strconv.ErrSyntax},
		{raw: "-00", bitSize: 64, message: `integer "-00" has a leading zero`, hint: "a leading zero is ambiguous: write -0o0 for octal or -0 for decimal", cause: strconv.ErrSyntax},
		{raw: "0089", bitSize: 64, message: `integer "0089" has a leading zero`, hint: "a leading zero is not allowed: write 89", cause: strconv.ErrSyntax},
		{raw: "0x", bitSize: 64, message: `invalid integer "0x"`, cause: strconv.ErrSyntax},
		{raw: "0o", bitSize: 64, message: `invalid integer "0o"`, cause: strconv.ErrSyntax},
		{raw: "0B", bitSize: 64, message: `invalid integer "0B"`, cause: strconv.ErrSyntax},
		{raw: "-0x", bitSize: 64, message: `invalid integer "-0x"`, cause: strconv.ErrSyntax},
		{raw: "0x-5", bitSize: 64, message: `invalid integer "0x-5"`, cause: strconv.ErrSyntax},
		{raw: "0x+5", bitSize: 64, message: `invalid integer "0x+5"`, cause: strconv.ErrSyntax},
		{raw: "0xg", bitSize: 64, message: `invalid integer "0xg"`, cause: strconv.ErrSyntax},
		{raw: "0o8", bitSize: 64, message: `invalid integer "0o8"`, cause: strconv.ErrSyntax},
		{raw: "0b2", bitSize: 64, message: `invalid integer "0b2"`, cause: strconv.ErrSyntax},
		{raw: "1_000", bitSize: 64, message: `invalid integer "1_000"`, cause: strconv.ErrSyntax},
		{raw: "0x_10", bitSize: 64, message: `invalid integer "0x_10"`, cause: strconv.ErrSyntax},
		{raw: "0x1_0", bitSize: 64, message: `invalid integer "0x1_0"`, cause: strconv.ErrSyntax},
		{raw: "0_7", bitSize: 64, message: `invalid integer "0_7"`, cause: strconv.ErrSyntax},
		{raw: "", bitSize: 64, message: `invalid integer ""`, cause: strconv.ErrSyntax},
		{raw: "-", bitSize: 64, message: `invalid integer "-"`, cause: strconv.ErrSyntax},
		{raw: "--5", bitSize: 64, message: `invalid integer "--5"`, cause: strconv.ErrSyntax},
		{raw: "abc", bitSize: 64, message: `invalid integer "abc"`, cause: strconv.ErrSyntax},
		{raw: "1.5", bitSize: 64, message: `invalid integer "1.5"`, cause: strconv.ErrSyntax},
		{raw: " 5", bitSize: 64, message: `invalid integer " 5"`, cause: strconv.ErrSyntax},
		{raw: "128", bitSize: 8, message: `integer "128" is out of range`, cause: strconv.ErrRange},
		{raw: "-129", bitSize: 8, message: `integer "-129" is out of range`, cause: strconv.ErrRange},
		{raw: "0x80", bitSize: 8, message: `integer "0x80" is out of range`, cause: strconv.ErrRange},
		{raw: "32768", bitSize: 16, message: `integer "32768" is out of range`, cause: strconv.ErrRange},
		{raw: "0x80000000", bitSize: 32, message: `integer "0x80000000" is out of range`, cause: strconv.ErrRange},
		{raw: "9223372036854775808", bitSize: 64, message: `integer "9223372036854775808" is out of range`, cause: strconv.ErrRange},
		{raw: "-0x8000000000000001", bitSize: 64, message: `integer "-0x8000000000000001" is out of range`, cause: strconv.ErrRange},
	}

	for _, testCase := range cases {
		t.Run(fmt.Sprintf("%q/%d", testCase.raw, testCase.bitSize), func(t *testing.T) {
			t.Parallel()

			_, err := intx.ParseInt(testCase.raw, testCase.bitSize)

			assert.Error(t, err)
			assert.Equal(t, testCase.message, err.Error())
			assert.Equal(t, testCase.hint, intx.HintOf(err))
			assert.IsError(t, err, testCase.cause)
		})
	}
}

func TestParseUint_Rejected(t *testing.T) {
	t.Parallel()

	cases := []struct {
		raw     string
		bitSize int
		message string
		hint    string
		cause   error
	}{
		{raw: "0755", bitSize: 64, message: `integer "0755" has a leading zero`, hint: "a leading zero is ambiguous: write 0o755 for octal or 755 for decimal", cause: strconv.ErrSyntax},
		{raw: "00", bitSize: 64, message: `integer "00" has a leading zero`, hint: "a leading zero is ambiguous: write 0o0 for octal or 0 for decimal", cause: strconv.ErrSyntax},
		{raw: "-0", bitSize: 64, message: `invalid integer "-0"`, cause: strconv.ErrSyntax},
		{raw: "+5", bitSize: 64, message: `invalid integer "+5"`, cause: strconv.ErrSyntax},
		{raw: "-1", bitSize: 64, message: `invalid integer "-1"`, cause: strconv.ErrSyntax},
		{raw: "-07", bitSize: 64, message: `invalid integer "-07"`, cause: strconv.ErrSyntax},
		{raw: "-0x10", bitSize: 64, message: `invalid integer "-0x10"`, cause: strconv.ErrSyntax},
		{raw: "0x", bitSize: 64, message: `invalid integer "0x"`, cause: strconv.ErrSyntax},
		{raw: "0x-5", bitSize: 64, message: `invalid integer "0x-5"`, cause: strconv.ErrSyntax},
		{raw: "1_000", bitSize: 64, message: `invalid integer "1_000"`, cause: strconv.ErrSyntax},
		{raw: "256", bitSize: 8, message: `integer "256" is out of range`, cause: strconv.ErrRange},
		{raw: "0x100", bitSize: 8, message: `integer "0x100" is out of range`, cause: strconv.ErrRange},
		{raw: "0o200000", bitSize: 16, message: `integer "0o200000" is out of range`, cause: strconv.ErrRange},
		{raw: "0b100000000000000000000000000000000", bitSize: 32, message: `integer "0b100000000000000000000000000000000" is out of range`, cause: strconv.ErrRange},
		{raw: "18446744073709551616", bitSize: 64, message: `integer "18446744073709551616" is out of range`, cause: strconv.ErrRange},
	}

	for _, testCase := range cases {
		t.Run(fmt.Sprintf("%q/%d", testCase.raw, testCase.bitSize), func(t *testing.T) {
			t.Parallel()

			_, err := intx.ParseUint(testCase.raw, testCase.bitSize)

			assert.Error(t, err)
			assert.Equal(t, testCase.message, err.Error())
			assert.Equal(t, testCase.hint, intx.HintOf(err))
			assert.IsError(t, err, testCase.cause)
		})
	}
}

func TestHintOf(t *testing.T) {
	t.Parallel()

	t.Run("wrapped leading zero error", func(t *testing.T) {
		t.Parallel()

		_, parseErr := intx.ParseInt("010", 64)
		wrapped := fmt.Errorf("flag --mode: %w", parseErr)

		hint := intx.HintOf(wrapped)

		assert.Equal(t, "a leading zero is ambiguous: write 0o10 for octal or 10 for decimal", hint)
	})

	t.Run("error without an integer error", func(t *testing.T) {
		t.Parallel()

		hint := intx.HintOf(errors.New("other"))

		assert.Equal(t, "", hint)
	})

	t.Run("nil error", func(t *testing.T) {
		t.Parallel()

		hint := intx.HintOf(nil)

		assert.Equal(t, "", hint)
	})
}

func TestIsSignedLiteral(t *testing.T) {
	t.Parallel()

	cases := []struct {
		raw      string
		expected bool
	}{
		{raw: "0", expected: true},
		{raw: "-0", expected: true},
		{raw: "+5", expected: true},
		{raw: "-42", expected: true},
		{raw: "-0x8000000000000001", expected: true},
		{raw: "99999999999999999999999", expected: true},
		{raw: "0X1F", expected: true},
		{raw: "-0o17", expected: true},
		{raw: "0B101", expected: true},
		{raw: "0755", expected: false},
		{raw: "-00", expected: false},
		{raw: "0x", expected: false},
		{raw: "-0x-1", expected: false},
		{raw: "0xg", expected: false},
		{raw: "0o8", expected: false},
		{raw: "0b2", expected: false},
		{raw: "1_000", expected: false},
		{raw: "0x1_0", expected: false},
		{raw: "--5", expected: false},
		{raw: "-", expected: false},
		{raw: "", expected: false},
		{raw: "1.5", expected: false},
	}

	for _, testCase := range cases {
		t.Run(strconv.Quote(testCase.raw), func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, testCase.expected, intx.IsSignedLiteral(testCase.raw))
		})
	}
}
