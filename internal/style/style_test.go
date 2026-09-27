package style_test

import (
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/style"
)

func TestOf(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "bold red", style.Of(style.Bold, style.Red))
	assert.Equal(t, "dim italic green", style.Of(style.Dim, style.Italic, " ", style.Green))
	assert.Equal(t, "blue green", style.Of(" blue ", "", "  ", "green"), "tokens are trimmed and empties dropped")
	assert.Equal(t, "", style.Of("", "  "))
}

func TestParseAttributes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected func(style.Style) bool
	}{
		{name: "bold", input: "bold", expected: func(s style.Style) bool { return s.Bold }},
		{name: "dim", input: "dim", expected: func(s style.Style) bool { return s.Dim }},
		{name: "italic", input: "italic", expected: func(s style.Style) bool { return s.Italic }},
		{name: "underlined", input: "underlined", expected: func(s style.Style) bool { return s.Underlined }},
		{name: "blink", input: "blink", expected: func(s style.Style) bool { return s.Blink }},
		{name: "inverse", input: "inverse", expected: func(s style.Style) bool { return s.Inverse }},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			parsed := style.Parse(testCase.input)

			assert.True(t, testCase.expected(parsed), "expected %q to set its attribute", testCase.input)
			assert.Equal(t, "", parsed.Foreground, "attribute token must not set a color")
			assert.Equal(t, "", parsed.Background, "attribute token must not set a color")
		})
	}
}

func TestParseAllAttributesCombined(t *testing.T) {
	t.Parallel()

	parsed := style.Parse("bold dim italic underlined blink inverse")

	assert.True(t, parsed.Bold)
	assert.True(t, parsed.Dim)
	assert.True(t, parsed.Italic)
	assert.True(t, parsed.Underlined)
	assert.True(t, parsed.Blink)
	assert.True(t, parsed.Inverse)
}

func TestParseCaseInsensitive(t *testing.T) {
	t.Parallel()

	parsed := style.Parse("BOLD RED BG-BLUE")

	assert.True(t, parsed.Bold)
	assert.Equal(t, "red", parsed.Foreground)
	assert.Equal(t, "blue", parsed.Background)
}

func TestParseForeground(t *testing.T) {
	t.Parallel()

	tests := []string{
		"black", "red", "green", "yellow", "blue", "magenta", "cyan", "white",
		"bright-black", "bright-red", "bright-green", "bright-yellow",
		"bright-blue", "bright-magenta", "bright-cyan", "bright-white",
	}

	for _, token := range tests {
		t.Run(token, func(t *testing.T) {
			t.Parallel()

			parsed := style.Parse(token)

			assert.Equal(t, token, parsed.Foreground, "expected foreground token to be preserved")
			assert.Equal(t, "", parsed.Background)
		})
	}
}

func TestParseFgDefault(t *testing.T) {
	t.Parallel()

	parsed := style.Parse("fg-default")

	assert.Equal(t, "default", parsed.Foreground)
	assert.Equal(t, "", parsed.Background)
}

func TestParseBackground(t *testing.T) {
	t.Parallel()

	tests := []struct {
		token    string
		expected string
	}{
		{token: "bg-black", expected: "black"},
		{token: "bg-white", expected: "white"},
		{token: "bg-bright-red", expected: "bright-red"},
		{token: "bg-bright-white", expected: "bright-white"},
		{token: "bg-default", expected: "default"},
	}

	for _, testCase := range tests {
		t.Run(testCase.token, func(t *testing.T) {
			t.Parallel()

			parsed := style.Parse(testCase.token)

			assert.Equal(t, testCase.expected, parsed.Background, "expected background token to be classified")
			assert.Equal(t, "", parsed.Foreground)
		})
	}
}

func TestParseExtendedColors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		token  string
		fg, bg string
	}{
		{token: "color208", fg: "color208"},
		{token: "color0", fg: "color0"},
		{token: "bg-color120", bg: "color120"},
		{token: "#ff00ff", fg: "#ff00ff"},
		{token: "bg-#00ff00", bg: "#00ff00"},
		{token: "rgb(255,128,0)", fg: "rgb(255,128,0)"},
		{token: "bg-rgb(1,2,3)", bg: "rgb(1,2,3)"},
	}

	for _, testCase := range tests {
		t.Run(testCase.token, func(t *testing.T) {
			t.Parallel()

			parsed := style.Parse(testCase.token)

			assert.Equal(t, testCase.fg, parsed.Foreground)
			assert.Equal(t, testCase.bg, parsed.Background)
		})
	}
}

func TestParseUnknownTokensIgnored(t *testing.T) {
	t.Parallel()

	tests := []string{
		"gray", "grey", "underline", "reverse", "bright_black", "fg-blue",
		"fg:red", "default", "bogus", "color256", "color-1", "color",
		"#fff", "#gg0000", "rgb(1,2)", "rgb(256,0,0)", "rgb(-1,0,0)",
	}

	for _, token := range tests {
		t.Run(token, func(t *testing.T) {
			t.Parallel()

			parsed := style.Parse(token)

			assert.Equal(t, style.Style{}, parsed, "expected %q to be ignored by the closed vocabulary", token)
		})
	}
}

func TestStyleSGR(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		style    style.Style
		expected string
	}{
		{name: "empty", style: style.Style{}, expected: ""},
		{name: "untranslatable color dropped", style: style.Style{Foreground: "bogus"}, expected: ""},
		{name: "bold", style: style.Style{Bold: true}, expected: "1"},
		{name: "dim", style: style.Style{Dim: true}, expected: "2"},
		{name: "italic", style: style.Style{Italic: true}, expected: "3"},
		{name: "underlined", style: style.Style{Underlined: true}, expected: "4"},
		{name: "blink", style: style.Style{Blink: true}, expected: "5"},
		{name: "inverse", style: style.Style{Inverse: true}, expected: "7"},
		{name: "attribute order is fixed", style: style.Style{Bold: true, Dim: true, Italic: true, Underlined: true, Blink: true, Inverse: true}, expected: "1;2;3;4;5;7"},
		{name: "foreground after attributes", style: style.Style{Foreground: style.Red, Bold: true}, expected: "1;31"},
		{name: "background last", style: style.Style{Foreground: style.Red, Background: style.Blue}, expected: "31;44"},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, testCase.expected, testCase.style.SGR())
		})
	}
}

func TestSGRForeground(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"black": "30", "red": "31", "green": "32", "yellow": "33",
		"blue": "34", "magenta": "35", "cyan": "36", "white": "37",
		"fg-default":   "39",
		"bright-black": "90", "bright-red": "91", "bright-green": "92", "bright-yellow": "93",
		"bright-blue": "94", "bright-magenta": "95", "bright-cyan": "96", "bright-white": "97",
	}

	for token, expected := range tests {
		t.Run(token, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, expected, style.SGR(token))
		})
	}
}

func TestSGRBackground(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"bg-black": "40", "bg-red": "41", "bg-green": "42", "bg-yellow": "43",
		"bg-blue": "44", "bg-magenta": "45", "bg-cyan": "46", "bg-white": "47",
		"bg-default":      "49",
		"bg-bright-black": "100", "bg-bright-red": "101", "bg-bright-green": "102", "bg-bright-yellow": "103",
		"bg-bright-blue": "104", "bg-bright-magenta": "105", "bg-bright-cyan": "106", "bg-bright-white": "107",
	}

	for token, expected := range tests {
		t.Run(token, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, expected, style.SGR(token))
		})
	}
}

func TestSGRExtendedColors(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"color0":         "38;5;0",
		"color208":       "38;5;208",
		"color255":       "38;5;255",
		"bg-color120":    "48;5;120",
		"#ff00ff":        "38;2;255;0;255",
		"#000000":        "38;2;0;0;0",
		"bg-#ff00ff":     "48;2;255;0;255",
		"rgb(255,128,0)": "38;2;255;128;0",
		"bg-rgb(1,2,3)":  "48;2;1;2;3",
	}

	for token, expected := range tests {
		t.Run(token, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, expected, style.SGR(token))
		})
	}
}

func TestSGRInvalidColorsDropped(t *testing.T) {
	t.Parallel()

	tests := []string{
		"color256", "color-1", "color", "#fff", "#gg0000",
		"rgb(1,2)", "rgb(a,b,c)", "rgb()", "rgb(1,2,3,4)",
		"gray", "underline", "fg-blue", "default", "bogus",
	}

	for _, token := range tests {
		t.Run(token, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, "", style.SGR(token), "expected untranslatable token to be dropped")
		})
	}
}

func TestSGRCombined(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"bold red":                    "1;31",
		"dim bg-blue italic":          "2;3;44",
		"bold red bg-blue":            "1;31;44",
		"underlined color208 inverse": "4;7;38;5;208",
		"inverse #00ff00 bg-color17":  "7;38;2;0;255;0;48;5;17",
		"bold   red":                  "1;31",
		"BOLD RED":                    "1;31",
		"bold dim":                    "1;2",
		"":                            "",
	}

	for input, expected := range tests {
		t.Run(input, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, expected, style.SGR(input))
		})
	}
}

func TestSGRFreeFunctionMatchesStyleMethod(t *testing.T) {
	t.Parallel()

	assert.Equal(t, style.Parse("bold red").SGR(), style.SGR("bold red"))
}
