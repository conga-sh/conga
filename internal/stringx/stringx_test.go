package stringx_test

import (
	"net/url"
	"testing"
	"time"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/stringx"
)

type customStringer struct {
	text string
}

func (c *customStringer) String() string {
	if c == nil {
		return ""
	}

	return c.text
}

type unhandledType struct {
	field int
}

func TestStringify(t *testing.T) {
	t.Parallel()

	t.Run("string identity", func(t *testing.T) {
		t.Parallel()

		result := stringx.Stringify("hello world")

		assert.Equal(t, "hello world", result)
	})

	t.Run("fmt.Stringer non-nil", func(t *testing.T) {
		t.Parallel()

		duration := 5 * time.Second

		result := stringx.Stringify(duration)

		assert.Equal(t, "5s", result)
	})

	t.Run("fmt.Stringer custom pointer", func(t *testing.T) {
		t.Parallel()

		stringer := &customStringer{text: "custom"}

		result := stringx.Stringify(stringer)

		assert.Equal(t, "custom", result)
	})

	t.Run("fmt.Stringer typed nil", func(t *testing.T) {
		t.Parallel()

		var nilStringer *customStringer

		result := stringx.Stringify(nilStringer)

		assert.Equal(t, "", result)
	})

	t.Run("pointer-receiver fmt.Stringer reached through an addressable copy", func(t *testing.T) {
		t.Parallel()

		parsedURL, err := url.Parse("https://example.com/path")
		assert.NoError(t, err)

		// A url.URL value (not pointer) boxed in any cannot reach its
		// pointer-receiver String method without an addressable copy.
		value := *parsedURL

		result := stringx.Stringify(value)

		assert.Equal(t, "https://example.com/path", result)
	})

	t.Run("bool values", func(t *testing.T) {
		t.Parallel()

		trueResult := stringx.Stringify(true)
		falseResult := stringx.Stringify(false)

		assert.Equal(t, "true", trueResult)
		assert.Equal(t, "false", falseResult)
	})

	t.Run("integers", func(t *testing.T) {
		t.Parallel()

		intResult := stringx.Stringify(42)
		int64Result := stringx.Stringify(int64(9876543210))
		uintResult := stringx.Stringify(uint(123))
		uint64Result := stringx.Stringify(uint64(18446744073709551615))

		assert.Equal(t, "42", intResult)
		assert.Equal(t, "9876543210", int64Result)
		assert.Equal(t, "123", uintResult)
		assert.Equal(t, "18446744073709551615", uint64Result)
	})

	t.Run("floats", func(t *testing.T) {
		t.Parallel()

		result := stringx.Stringify(3.14159)

		assert.Equal(t, "3.14159", result)
	})

	t.Run("nil any", func(t *testing.T) {
		t.Parallel()

		result := stringx.Stringify(nil)

		assert.Equal(t, "<nil>", result)
	})

	t.Run("unhandled fallback to fmt.Sprint", func(t *testing.T) {
		t.Parallel()

		sliceResult := stringx.Stringify([]int{1, 2, 3})
		structResult := stringx.Stringify(unhandledType{field: 10})

		assert.Equal(t, "[1 2 3]", sliceResult)
		assert.Equal(t, "{10}", structResult)
	})
}

func BenchmarkStringify_String(b *testing.B) {
	for b.Loop() {
		_ = stringx.Stringify("production")
	}
}

func BenchmarkStringify_Bool(b *testing.B) {
	for b.Loop() {
		_ = stringx.Stringify(true)
	}
}

func BenchmarkStringify_Int(b *testing.B) {
	for b.Loop() {
		_ = stringx.Stringify(8080)
	}
}
