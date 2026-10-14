// Package nushell renders completion results as a bare JSON array of records
// for nushell's external completer and generates the nushell registration
// snippet for the wire protocol
// (docs/completion-wire-protocol.md).
package nushell

import (
	"strings"

	"github.com/conga-sh/conga/internal/completion/wire"
)

// metacharacters lists the characters that force double-quoting of an
// insertion value: nushell inserts the record's value verbatim into the
// command line, so every shell metacharacter must be quoted for direct
// insertion.
const (
	metacharacters = " {}()[]<>$&\"'|;#\\`"
	controlCutset  = "\n\r"
)

// Format renders candidates and directives in the nushell framing: a bare
// JSON array of records (no trailing newline) in engine order. The record's
// value is pre-quoted for direct insertion (nushell inserts it verbatim),
// with the nospace rule expressed as a trailing space baked into the value.
// Nushell's external-completer contract has no directive, message, or usage
// channel: noprefix, keeporder, messages, and usage are silently dropped.
// The framing is position-independent: toComplete is accepted for seam
// symmetry and ignored.
func Format(toComplete string, directives wire.Directives, candidates []wire.Candidate) string {
	buffer := make([]byte, 0, 96*len(candidates)+16)

	buffer = append(buffer, '[')

	for i, candidate := range candidates {
		if i > 0 {
			buffer = append(buffer, ',')
		}

		buffer = appendCandidate(buffer, directives, candidate)
	}

	return string(append(buffer, ']'))
}

// fieldSanitizer strips the characters nushell's record JSON cannot carry:
// newlines and carriage returns (tabs are kept). Construction is deferred to
// first use so the driver package stays free of package initializers and
// drops at link time when nushell completion is disabled.
// appendCandidate appends one record: the pre-quoted insertion value with the
// trailing-space rule, the raw display, the optional clamped description, and
// the optional style record.
func appendCandidate(buffer []byte, directives wire.Directives, candidate wire.Candidate) []byte {
	value := quoteInsertionValue(sanitizeField(candidate.Value))
	if !directives.NoSpaceFor(candidate.Value) {
		value += " "
	}

	display := sanitizeField(candidate.Display)
	if display == "" {
		display = sanitizeField(candidate.Value)
	}

	buffer = append(buffer, `{"value":`...)
	buffer = wire.AppendJSONString(buffer, value)
	buffer = append(buffer, `,"display":`...)
	buffer = wire.AppendJSONString(buffer, display)

	if description := clampedDescription(candidate.Description); description != "" {
		buffer = append(buffer, `,"description":`...)
		buffer = wire.AppendJSONString(buffer, description)
	}

	if !wire.ColorDisabled() {
		buffer = appendStyle(buffer, candidate.Style)
	}

	return append(buffer, '}')
}

// quoteInsertionValue wraps the value in double quotes with backslash
// escaping when it carries any nushell metacharacter; a leading tilde stays
// outside the quotes so home-directory abbreviation survives.
func quoteInsertionValue(value string) string {
	if !strings.ContainsAny(value, metacharacters) {
		return value
	}

	if rest, ok := strings.CutPrefix(value, "~"); ok {
		return `~"` + escapeDoubleQuoted(rest) + `"`
	}

	return `"` + escapeDoubleQuoted(value) + `"`
}

// escapeDoubleQuoted escapes backslashes and double quotes for a nushell
// double-quoted string literal.
func escapeDoubleQuoted(value string) string {
	escaped := strings.ReplaceAll(value, `\`, `\\`)

	return strings.ReplaceAll(escaped, `"`, `\"`)
}

// clampedDescription applies the nushell description normalization: the
// sanitizer strips newlines and carriage returns first — flattening
// multi-line descriptions into one line (the fork's behavior, unlike fish's
// and elvish's first-line cut) — and the flattened text is then trimmed and
// clamped to the configured maximum length with a "..." suffix (hard cut
// when the limit is 3 or fewer runes).
func clampedDescription(description string) string {
	return wire.ClampDescription(strings.TrimSpace(sanitizeField(description)), wire.DescriptionLength())
}

func sanitizeField(s string) string {
	return wire.StripControl(s, controlCutset)
}
