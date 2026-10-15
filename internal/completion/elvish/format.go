// Package elvish renders completion results as a single-line JSON object for
// elvish's edit:complex-candidate consumption and generates the elvish
// registration snippet for the wire protocol
// (docs/completion-wire-protocol.md).
package elvish

import (
	"strconv"
	"strings"

	"github.com/conga-sh/conga/internal/completion/wire"
	"github.com/conga-sh/conga/internal/style"
)

const controlCutset = "\n\r\t"

// Format renders candidates and directives in the elvish framing: a
// single-line JSON object (no trailing newline) with candidates in engine
// order. Every top-level field (usage, messages, descriptionStyle) and every
// candidate field (value, display, description, codeSuffix, style) is always
// present. The usage string is cleared when any candidates exist: it is only
// sent for the zero-candidate error case, where messages carry it. The
// framing is position-independent: toComplete is accepted for seam symmetry
// and ignored.
func Format(toComplete string, directives wire.Directives, candidates []wire.Candidate) string {
	usage := ""
	if len(candidates) == 0 {
		usage = directives.Usage
	}

	buffer := make([]byte, 0, 96*len(candidates)+128)

	buffer = append(buffer, `{"usage":`...)
	buffer = wire.AppendJSONString(buffer, usage)
	buffer = append(buffer, `,"messages":[`...)

	for i, message := range directives.Messages {
		if i > 0 {
			buffer = append(buffer, ',')
		}

		buffer = wire.AppendJSONString(buffer, message)
	}

	descriptionStyle := "default"
	if !wire.ColorDisabled() {
		descriptionStyle = normalizeStyleToken(style.Theme.Description)
	}

	buffer = append(buffer, `],"descriptionStyle":`...)
	buffer = wire.AppendJSONString(buffer, descriptionStyle)
	buffer = append(buffer, `,"candidates":[`...)

	for i, candidate := range candidates {
		if i > 0 {
			buffer = append(buffer, ',')
		}

		buffer = appendCandidate(buffer, directives, candidate)
	}

	buffer = append(buffer, `]}`...)

	return string(buffer)
}

// appendCandidate appends one candidate object: the sanitized value, display,
// and clamped description; the codeSuffix carrying the nospace rule; and the
// style in elvish vocabulary.
func appendCandidate(buffer []byte, directives wire.Directives, candidate wire.Candidate) []byte {
	display := sanitizeField(candidate.Display)
	if display == "" {
		display = sanitizeField(candidate.Value)
	}

	codeSuffix := " "
	if directives.NoSpaceFor(candidate.Value) {
		codeSuffix = ""
	}

	buffer = append(buffer, `{"value":`...)
	buffer = wire.AppendJSONString(buffer, sanitizeField(candidate.Value))
	buffer = append(buffer, `,"display":`...)
	buffer = wire.AppendJSONString(buffer, display)
	buffer = append(buffer, `,"description":`...)
	buffer = wire.AppendJSONString(buffer, sanitizeField(candidate.TrimmedDescription()))
	buffer = append(buffer, `,"codeSuffix":`...)
	buffer = wire.AppendJSONString(buffer, codeSuffix)
	buffer = append(buffer, `,"style":`...)
	buffer = wire.AppendJSONString(buffer, normalizeStyle(candidate.Style))

	return append(buffer, '}')
}

func sanitizeField(s string) string {
	return wire.StripControl(s, controlCutset)
}

// normalizeStyle translates a style token string to elvish's `styled`
// vocabulary: the attribute, named-color, bright-*, bg-*, and colorN tokens
// pass through verbatim (elvish's styling parser accepts them), while
// truecolor tokens (#rrggbb, rgb(r,g,b) — including bg- prefixed forms)
// normalize to the nearest indexed color, which elvish renders as
// colorN/bg-colorN. An empty style renders as "default".
func normalizeStyle(s string) string {
	if wire.ColorDisabled() || strings.TrimSpace(s) == "" {
		return "default"
	}

	tokens := strings.Fields(s)

	normalized := make([]string, 0, len(tokens))
	for _, token := range tokens {
		normalized = append(normalized, normalizeStyleToken(token))
	}

	return strings.Join(normalized, " ")
}

// normalizeStyleToken translates one style token to elvish vocabulary.
func normalizeStyleToken(token string) string {
	bgPrefix := ""
	body := token

	if rest, ok := strings.CutPrefix(token, "bg-"); ok {
		bgPrefix = "bg-"
		body = rest
	}

	red, green, blue, ok := style.TruecolorRGB(body)
	if !ok {
		return token
	}

	return bgPrefix + "color" + strconv.Itoa(style.NearestIndexed(red, green, blue))
}
