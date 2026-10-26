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

		assert.Equal(t, "hello world", stringx.Stringify("hello world"))
	})

	t.Run("fmt.Stringer non-nil", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "5s", stringx.Stringify(5*time.Second))
	})

	t.Run("fmt.Stringer custom pointer", func(t *testing.T) {
		t.Parallel()

		stringer := &customStringer{text: "custom"}

		assert.Equal(t, "custom", stringx.Stringify(stringer))
	})

	t.Run("fmt.Stringer typed nil", func(t *testing.T) {
		t.Parallel()

		// strictStringer.String dereferences fields without a nil-receiver
		// check, so Stringify must guard before invoking it.
		var nilStringer *strictStringer

		assert.Equal(t, "", stringx.Stringify(nilStringer))
	})

	t.Run("pointer-receiver fmt.Stringer reached through an addressable copy", func(t *testing.T) {
		t.Parallel()

		parsedURL, err := url.Parse("https://example.com/path")
		assert.NoError(t, err)

		// A url.URL value (not pointer) boxed in any cannot reach its
		// pointer-receiver String method without an addressable copy.
		value := *parsedURL

		assert.Equal(t, "https://example.com/path", stringx.Stringify(value))
	})

	t.Run("bool values", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "true", stringx.Stringify(true))
		assert.Equal(t, "false", stringx.Stringify(false))
	})

	t.Run("integers", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "42", stringx.Stringify(42))
		assert.Equal(t, "9876543210", stringx.Stringify(int64(9876543210)))
		assert.Equal(t, "123", stringx.Stringify(uint(123)))
		assert.Equal(t, "18446744073709551615", stringx.Stringify(uint64(18446744073709551615)))
	})

	t.Run("floats", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "3.14159", stringx.Stringify(3.14159))
	})

	t.Run("nil any", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "<nil>", stringx.Stringify(nil))
	})

	t.Run("encoding.TextMarshaler", func(t *testing.T) {
		t.Parallel()

		marshaler := customTextMarshaler{text: "marshaled-text"}

		assert.Equal(t, "marshaled-text", stringx.Stringify(marshaler))
	})

	t.Run("encoding.TextMarshaler pointer", func(t *testing.T) {
		t.Parallel()

		marshaler := &customPointerTextMarshaler{text: "ptr-text"}

		assert.Equal(t, "ptr-text", stringx.Stringify(marshaler))
	})

	t.Run("encoding.TextMarshaler typed nil", func(t *testing.T) {
		t.Parallel()

		var marshaler *customPointerTextMarshaler

		assert.Equal(t, "", stringx.Stringify(marshaler))
	})

	t.Run("map formatting", func(t *testing.T) {
		t.Parallel()

		mapValue := map[string]int{"b": 2, "a": 1, "c": 3}

		assert.Equal(t, "a=1, b=2, c=3", stringx.Stringify(mapValue))
		assert.Equal(t, "", stringx.Stringify(map[string]string{}))
	})

	t.Run("slice formatting", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "[a, b, c]", stringx.Stringify([]string{"a", "b", "c"}))
		assert.Equal(t, "[1, 2, 3]", stringx.Stringify([]int{1, 2, 3}))
		assert.Equal(t, "[true, false]", stringx.Stringify([]bool{true, false}))
		assert.Equal(t, "", stringx.Stringify([]string{}))
		assert.Equal(t, "raw-bytes", stringx.Stringify([]byte("raw-bytes")))
	})

	t.Run("unhandled fallback to fmt.Sprint", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "{10}", stringx.Stringify(unhandledType{field: 10}))
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
