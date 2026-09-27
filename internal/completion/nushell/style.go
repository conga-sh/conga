package nushell

import (
	"strconv"
	"strings"

	"github.com/conga-sh/conga/internal/completion/wire"
	"github.com/conga-sh/conga/internal/style"
)

// styleRecord is nushell's style object for a completion record, in the
// format nushell's `ansi --escape` accepts: nushell color names or hex codes
// for fg/bg, and single-letter attributes for attr.
type styleRecord struct {
	Foreground string
	Background string
	Attributes string
}

// appendStyle appends the optional style record for the candidate's style
// token string; the field is omitted entirely for unstyled candidates.
func appendStyle(buffer []byte, styleTokens string) []byte {
	if wire.ColorDisabled() {
		return buffer
	}

	record := convertStyle(styleTokens)
	if record == nil {
		return buffer
	}

	buffer = append(buffer, `,"style":{`...)

	if record.Foreground != "" {
		buffer = append(buffer, `"fg":`...)
		buffer = wire.AppendJSONString(buffer, record.Foreground)
	}

	if record.Background != "" {
		if record.Foreground != "" {
			buffer = append(buffer, ',')
		}

		buffer = append(buffer, `"bg":`...)
		buffer = wire.AppendJSONString(buffer, record.Background)
	}

	if record.Attributes != "" {
		if record.Foreground != "" || record.Background != "" {
			buffer = append(buffer, ',')
		}

		buffer = append(buffer, `"attr":`...)
		buffer = wire.AppendJSONString(buffer, record.Attributes)
	}

	return append(buffer, '}')
}

// convertStyle translates a CONGA style token string into nushell's style
// record: attribute tokens become single letters (blink l, bold b, dimmed d,
// italic i, reverse r, underlined u, in that fixed order), and colors are
// converted per convertColor. An empty token string yields nil (no style
// field); tokens outside the vocabulary are ignored by the parser.
func convertStyle(styleTokens string) *styleRecord {
	if styleTokens == "" {
		return nil
	}

	parsed := style.Parse(styleTokens)
	result := &styleRecord{}

	if parsed.Blink {
		result.Attributes += "l"
	}

	if parsed.Bold {
		result.Attributes += "b"
	}

	if parsed.Dim {
		result.Attributes += "d"
	}

	if parsed.Italic {
		result.Attributes += "i"
	}

	if parsed.Inverse {
		result.Attributes += "r"
	}

	if parsed.Underlined {
		result.Attributes += "u"
	}

	if parsed.Foreground != "" {
		result.Foreground = convertColor(parsed.Foreground)
	}

	if parsed.Background != "" {
		result.Background = convertColor(parsed.Background)
	}

	return result
}

// convertColor translates one CONGA color token into nushell's vocabulary:
// indexed colors (color0-color255) and truecolor tokens (#rrggbb, rgb(r,g,b))
// normalize to hex — nushell takes hex codes but not xterm palette numbers —
// and named colors map onto nushell's own names (bright-* foregrounds are
// nushell's light_* names).
func convertColor(color string) string {
	if index, err := strconv.Atoi(strings.TrimPrefix(color, "color")); err == nil && strings.HasPrefix(color, "color") {
		if hex, ok := style.IndexedHex(index); ok {
			return hex
		}
	}

	if red, green, blue, ok := style.TruecolorRGB(color); ok {
		return style.RGBHex(red, green, blue)
	}

	if named, ok := convertNamedColor(color); ok {
		return named
	}

	return color
}

// convertNamedColor maps the CONGA named-color vocabulary onto nushell's
// color names (nushell spellings per `ansi --list`: bright variants are
// light_*, bright-black is dark_gray, bright-white is white).
func convertNamedColor(color string) (string, bool) {
	switch color {
	case "black", "red", "green", "yellow", "blue", "magenta", "cyan", "white":
		return color, true
	case "bright-black":
		return "dark_gray", true
	case "bright-red":
		return "light_red", true
	case "bright-green":
		return "light_green", true
	case "bright-yellow":
		return "light_yellow", true
	case "bright-blue":
		return "light_blue", true
	case "bright-magenta":
		return "light_magenta", true
	case "bright-cyan":
		return "light_cyan", true
	case "bright-white":
		return "white", true
	default:
		return "", false
	}
}
