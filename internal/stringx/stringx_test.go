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

type strictStringer struct {
	text string
}

func (s *strictStringer) String() string {
	return s.text
}

type unhandledType struct {
	field int
}

type customTextMarshaler struct {
	text string
}

func (c customTextMarshaler) MarshalText() ([]byte, error) {
	return []byte(c.text), nil
}

type customPointerTextMarshaler struct {
	text string
}

func (c *customPointerTextMarshaler) MarshalText() ([]byte, error) {
	return []byte(c.text), nil
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

		// strictStringer.String dereferences fields without a nil-receiver
		// check, so Stringify must guard before invoking it.
		var nilStringer *strictStringer

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

	t.Run("encoding.TextMarshaler", func(t *testing.T) {
		t.Parallel()

		marshaler := customTextMarshaler{text: "marshaled-text"}
		result := stringx.Stringify(marshaler)

		assert.Equal(t, "marshaled-text", result)
	})

	t.Run("encoding.TextMarshaler pointer", func(t *testing.T) {
		t.Parallel()

		marshaler := &customPointerTextMarshaler{text: "ptr-text"}

		result := stringx.Stringify(marshaler)

		assert.Equal(t, "ptr-text", result)
	})

	t.Run("encoding.TextMarshaler typed nil", func(t *testing.T) {
		t.Parallel()

		var marshaler *customPointerTextMarshaler

		result := stringx.Stringify(marshaler)

		assert.Equal(t, "", result)
	})

	t.Run("map formatting", func(t *testing.T) {
		t.Parallel()

		mapValue := map[string]int{"b": 2, "a": 1, "c": 3}
		result := stringx.Stringify(mapValue)

		assert.Equal(t, "a=1, b=2, c=3", result)
		assert.Equal(t, "", stringx.Stringify(map[string]string{}))
	})

	t.Run("slice formatting", func(t *testing.T) {
		t.Parallel()

		stringSliceResult := stringx.Stringify([]string{"a", "b", "c"})
		intSliceResult := stringx.Stringify([]int{1, 2, 3})
		boolSliceResult := stringx.Stringify([]bool{true, false})
		emptySliceResult := stringx.Stringify([]string{})
		byteSliceResult := stringx.Stringify([]byte("raw-bytes"))

		assert.Equal(t, "[a, b, c]", stringSliceResult)
		assert.Equal(t, "[1, 2, 3]", intSliceResult)
		assert.Equal(t, "[true, false]", boolSliceResult)
		assert.Equal(t, "", emptySliceResult)
		assert.Equal(t, "raw-bytes", byteSliceResult)
	})

	t.Run("unhandled fallback to fmt.Sprint", func(t *testing.T) {
		t.Parallel()

		structResult := stringx.Stringify(unhandledType{field: 10})

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
