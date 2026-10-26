package jsonwire_test

import (
	"encoding/json"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/jsonwire"
)

func TestAppendStringEscapeTable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty", input: "", want: `""`},
		{name: "plain", input: "deploy", want: `"deploy"`},
		{name: "double quote", input: `he"llo`, want: `"he\"llo"`},
		{name: "backslash", input: `a\b`, want: `"a\\b"`},
		{name: "newline", input: "a\nb", want: `"a\nb"`},
		{name: "tab", input: "a\tb", want: `"a\tb"`},
		{name: "carriage return", input: "a\rb", want: `"a\rb"`},
		{name: "backspace", input: "a\bb", want: `"a\bb"`},
		{name: "form feed", input: "a\fb", want: `"a\fb"`},
		{name: "nul control", input: "a\x00b", want: `"a\u0000b"`},
		{name: "unit separator control", input: "a\x1fb", want: `"a\u001fb"`},
		{name: "delete passes through", input: "a\x7fb", want: "\"a\x7fb\""},
		{name: "multibyte utf-8 passes through", input: "héllo 中文", want: `"héllo 中文"`},
		{name: "invalid utf-8 replaced", input: "ab\xffc", want: `"ab\ufffdc"`},
		{name: "combined", input: "a\"b\\c\nd\x01", want: `"a\"b\\c\nd\u0001"`},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.want, string(jsonwire.AppendString(nil, test.input)))
		})
	}
}

func TestAppendStringRoundTripsThroughEncodingJSON(t *testing.T) {
	t.Parallel()

	inputs := []string{
		"",
		"deploy",
		`quo"te and \back slash/`,
		"tab\tnewline\nret\r",
		"control\x00\x1f\x7f",
		"héllo 中文 🎉",
	}

	for _, input := range inputs {
		encoded := jsonwire.AppendString(nil, input)

		assert.True(t, json.Valid(encoded), "output must be valid JSON: %q", encoded)

		var decoded string

		assert.NoError(t, json.Unmarshal(encoded, &decoded))
		assert.Equal(t, input, decoded)
	}
}

func TestAppendStringReplacesInvalidUTF8WithValidJSON(t *testing.T) {
	t.Parallel()

	encoded := jsonwire.AppendString(nil, "bad\xffseed")

	assert.True(t, json.Valid(encoded))

	var decoded string

	assert.NoError(t, json.Unmarshal(encoded, &decoded))
	assert.Equal(t, "bad\uFFFDseed", decoded)
}

func TestAppendStringAppendsToExistingBuffer(t *testing.T) {
	t.Parallel()

	dst := []byte(`{"value":`)
	dst = jsonwire.AppendString(dst, "deploy")

	assert.Equal(t, `{"value":"deploy"`, string(dst))
}
