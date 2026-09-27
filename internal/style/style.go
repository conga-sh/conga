// Package style provides the terminal styling vocabulary for CONGA completion:
// color and attribute tokens, their ECMA-48 SGR translations, file extension
// styling, and the default completion palette.
//
// # Vocabulary
//
// Style strings are space-separated token strings from a closed vocabulary (see
// docs/completion-wire-protocol.md §6):
//
//   - Attributes: "bold", "dim", "italic", "underlined", "blink", "inverse"
//   - Foreground: "black", "red", "green", "yellow", "blue", "magenta", "cyan",
//     "white", "bright-" prefixed variants, "fg-default"
//   - Background: "bg-" prefixed foreground tokens, including "bg-bright-*"
//     variants and "bg-default"
//   - Extended: "color0"–"color255", "#rrggbb", and "rgb(r,g,b)", each also
//     usable after a "bg-" prefix
//
// Tokens outside this vocabulary are ignored. SGR parameter numbers are emitted
// in a fixed order: attributes (bold 1, dim 2, italic 3, underlined 4, blink 5,
// inverse 7), then foreground, then background.
package style

import (
	"strconv"
	"strings"
)

const (
	// Default resets to standard terminal styling.
	Default = ""

	// Black standard foreground color.
	Black = "black"
	// Red standard foreground color.
	Red = "red"
	// Green standard foreground color.
	Green = "green"
	// Yellow standard foreground color.
	Yellow = "yellow"
	// Blue standard foreground color.
	Blue = "blue"
	// Magenta standard foreground color.
	Magenta = "magenta"
	// Cyan standard foreground color.
	Cyan = "cyan"
	// White standard foreground color.
	White = "white"

	// BrightBlack bright foreground color.
	BrightBlack = "bright-black"
	// BrightRed bright foreground color.
	BrightRed = "bright-red"
	// BrightGreen bright foreground color.
	BrightGreen = "bright-green"
	// BrightYellow bright foreground color.
	BrightYellow = "bright-yellow"
	// BrightBlue bright foreground color.
	BrightBlue = "bright-blue"
	// BrightMagenta bright foreground color.
	BrightMagenta = "bright-magenta"
	// BrightCyan bright foreground color.
	BrightCyan = "bright-cyan"
	// BrightWhite bright foreground color.
	BrightWhite = "bright-white"

	// BgBlack standard background color.
	BgBlack = "bg-black"
	// BgRed standard background color.
	BgRed = "bg-red"
	// BgGreen standard background color.
	BgGreen = "bg-green"
	// BgYellow standard background color.
	BgYellow = "bg-yellow"
	// BgBlue standard background color.
	BgBlue = "bg-blue"
	// BgMagenta standard background color.
	BgMagenta = "bg-magenta"
	// BgCyan standard background color.
	BgCyan = "bg-cyan"
	// BgWhite standard background color.
	BgWhite = "bg-white"

	// BgBrightBlack bright background color.
	BgBrightBlack = "bg-bright-black"
	// BgBrightRed bright background color.
	BgBrightRed = "bg-bright-red"
	// BgBrightGreen bright background color.
	BgBrightGreen = "bg-bright-green"
	// BgBrightYellow bright background color.
	BgBrightYellow = "bg-bright-yellow"
	// BgBrightBlue bright background color.
	BgBrightBlue = "bg-bright-blue"
	// BgBrightMagenta bright background color.
	BgBrightMagenta = "bg-bright-magenta"
	// BgBrightCyan bright background color.
	BgBrightCyan = "bg-bright-cyan"
	// BgBrightWhite bright background color.
	BgBrightWhite = "bg-bright-white"

	// Bold text formatting attribute.
	Bold = "bold"
	// Dim text formatting attribute.
	Dim = "dim"
	// Italic text formatting attribute.
	Italic = "italic"
	// Underlined text formatting attribute.
	Underlined = "underlined"
	// Blink text formatting attribute.
	Blink = "blink"
	// Inverse text formatting attribute.
	Inverse = "inverse"
)

// ECMA-48 SGR parameter bases and offsets.
const (
	fgStandardBase = 30
	bgStandardBase = 40
	brightDelta    = 60
	sgrDefaultFG   = 39
	sgrDefaultBG   = 49
)

// Style represents parsed text styling attributes and colors.
type Style struct {
	Foreground string
	Background string
	Bold       bool
	Dim        bool
	Italic     bool
	Underlined bool
	Blink      bool
	Inverse    bool
}

// SGR returns the semicolon-delimited ECMA-48 SGR parameter numbers for this
// style: attributes first (bold 1, dim 2, italic 3, underlined 4, blink 5,
// inverse 7), then foreground, then background. Colors that cannot be
// translated are omitted. The result carries no escape-sequence framing.
func (s Style) SGR() string {
	codes := make([]string, 0, 3)

	if s.Bold {
		codes = append(codes, "1")
	}

	if s.Dim {
		codes = append(codes, "2")
	}

	if s.Italic {
		codes = append(codes, "3")
	}

	if s.Underlined {
		codes = append(codes, "4")
	}

	if s.Blink {
		codes = append(codes, "5")
	}

	if s.Inverse {
		codes = append(codes, "7")
	}

	if s.Foreground != "" {
		if code := colorSGR(s.Foreground, false); code != "" {
			codes = append(codes, code)
		}
	}

	if s.Background != "" {
		if code := colorSGR(s.Background, true); code != "" {
			codes = append(codes, code)
		}
	}

	return strings.Join(codes, ";")
}

// Of combines multiple style tokens into a single space-delimited style string.
func Of(styles ...string) string {
	clean := make([]string, 0, len(styles))
	for _, s := range styles {
		if trimmed := strings.TrimSpace(s); trimmed != "" {
			clean = append(clean, trimmed)
		}
	}

	return strings.Join(clean, " ")
}

// Parse parses a space-separated style string into a structured Style. Tokens
// are matched case-insensitively against the closed vocabulary; unknown tokens
// are ignored.
func Parse(s string) Style {
	var style Style

	for token := range strings.FieldsSeq(s) {
		applyToken(&style, strings.ToLower(token))
	}

	return style
}

// SGR returns the ECMA-48 SGR parameter numbers for a given style string.
func SGR(s string) string {
	return Parse(s).SGR()
}

// TruecolorRGB parses a truecolor token ("#rrggbb" or "rgb(r,g,b)") into its
// RGB components, and reports whether the token is a truecolor form.
func TruecolorRGB(token string) (int, int, int, bool) {
	if hex, ok := strings.CutPrefix(token, "#"); ok && len(hex) == 6 {
		return parseHexRGB(hex)
	}

	if inner, ok := strings.CutPrefix(token, "rgb("); ok {
		return parseFuncRGB(inner)
	}

	return 0, 0, 0, false
}

func applyToken(style *Style, token string) {
	switch token {
	case Bold:
		style.Bold = true
	case Dim:
		style.Dim = true
	case Italic:
		style.Italic = true
	case Underlined:
		style.Underlined = true
	case Blink:
		style.Blink = true
	case Inverse:
		style.Inverse = true
	case "fg-default":
		style.Foreground = "default"
	default:
		if bg, ok := strings.CutPrefix(token, "bg-"); ok {
			style.Background = bg
		} else if isForegroundColor(token) {
			style.Foreground = token
		}
	}
}

func isForegroundColor(token string) bool {
	if token == "default" {
		return false
	}

	return colorSGR(token, false) != ""
}

func colorSGR(name string, background bool) string {
	if code, ok := namedColorSGR(name, background); ok {
		return code
	}

	if code, ok := indexedColorSGR(name, background); ok {
		return code
	}

	if code, ok := trueColorSGR(name, background); ok {
		return code
	}

	return ""
}

func namedColorSGR(name string, background bool) (string, bool) {
	if name == "default" {
		if background {
			return strconv.Itoa(sgrDefaultBG), true
		}

		return strconv.Itoa(sgrDefaultFG), true
	}

	base := fgStandardBase
	if background {
		base = bgStandardBase
	}

	if bright, ok := strings.CutPrefix(name, "bright-"); ok {
		if index := colorIndex(bright); index >= 0 {
			return strconv.Itoa(base + brightDelta + index), true
		}

		return "", false
	}

	if index := colorIndex(name); index >= 0 {
		return strconv.Itoa(base + index), true
	}

	return "", false
}

func colorIndex(name string) int {
	for index, color := range []string{Black, Red, Green, Yellow, Blue, Magenta, Cyan, White} {
		if name == color {
			return index
		}
	}

	return -1
}

func indexedColorSGR(name string, background bool) (string, bool) {
	index, ok := strings.CutPrefix(name, "color")
	if !ok {
		return "", false
	}

	number, err := strconv.Atoi(index)
	if err != nil || number < 0 || number > 255 {
		return "", false
	}

	if background {
		return "48;5;" + strconv.Itoa(number), true
	}

	return "38;5;" + strconv.Itoa(number), true
}

func trueColorSGR(name string, background bool) (string, bool) {
	red, green, blue, ok := TruecolorRGB(name)
	if !ok {
		return "", false
	}

	code := "38;2;"
	if background {
		code = "48;2;"
	}

	return code + strconv.Itoa(red) + ";" + strconv.Itoa(green) + ";" + strconv.Itoa(blue), true
}

func parseHexRGB(hex string) (int, int, int, bool) {
	red, errRed := strconv.ParseUint(hex[0:2], 16, 8)
	green, errGreen := strconv.ParseUint(hex[2:4], 16, 8)

	blue, errBlue := strconv.ParseUint(hex[4:6], 16, 8)
	if errRed != nil || errGreen != nil || errBlue != nil {
		return 0, 0, 0, false
	}

	return int(red), int(green), int(blue), true
}

func parseFuncRGB(name string) (int, int, int, bool) {
	inner, ok := strings.CutSuffix(name, ")")
	if !ok {
		return 0, 0, 0, false
	}

	parts := strings.Split(inner, ",")
	if len(parts) != 3 {
		return 0, 0, 0, false
	}

	red, errRed := strconv.Atoi(strings.TrimSpace(parts[0]))
	green, errGreen := strconv.Atoi(strings.TrimSpace(parts[1]))

	blue, errBlue := strconv.Atoi(strings.TrimSpace(parts[2]))
	if errRed != nil || errGreen != nil || errBlue != nil ||
		outOfRGBRange(red) || outOfRGBRange(green) || outOfRGBRange(blue) {
		return 0, 0, 0, false
	}

	return red, green, blue, true
}

func outOfRGBRange(value int) bool {
	return value < 0 || value > 255
}
