// Package powershell renders completion results as the JSON payload consumed
// by the generated registration snippet and generates the
// Register-ArgumentCompleter snippets for the wire protocol
// (docs/completion-wire-protocol.md).
package powershell

import (
	"cmp"
	"strings"

	"github.com/conga-sh/conga/internal/completion/export"
	"github.com/conga-sh/conga/internal/completion/wire"
	"github.com/conga-sh/conga/internal/style"
)

// sgrEscape opens a backtick-escaped SGR sequence and sgrReset is the full
// ECMA-48 reset (SGR 0). The literal "`e[" form is resolved to a real escape
// character by the generated snippet's PowerShell replace.
const (
	sgrEscape = "`e["
	sgrReset  = "`e[0m"
)

// metacharacters lists the whitespace and PowerShell metacharacters that
// force single-quoting of a candidate value.
const (
	metacharacters = " `{}()[]*$?\\\"|<>&,;#'"
	controlCutset  = "\n\t\r"
)

// Format renders candidates and directives in the powershell framing: the
// export JSON schema plus a per-candidate "sgr" field, with the
// presentation fields pre-composed for the snippet's CompletionResult
// mapping. "value" carries the insertion text (quoted when it contains
// whitespace or metacharacters, with a trailing space unless the nospace
// rules apply); "display" carries the ListItemText content ("display
// (description)" normally, the bare display in tooltip mode); "description"
// carries the ToolTip content (the clamped description in tooltip mode, a
// single space otherwise). Styled segments are wrapped in backtick-escaped
// SGR sequences when color is enabled. Candidates with an empty value are
// dropped: they have no insertion text. toComplete is accepted for seam
// symmetry and ignored: the framing is position-independent.
func Format(toComplete string, directives wire.Directives, candidates []wire.Candidate) string {
	colorOn := !wire.ColorDisabled()
	tooltip := wire.Tooltip()

	prepared := make([]wire.Candidate, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.Value == "" {
			continue
		}

		prepared = append(prepared, composeCandidate(candidate, directives, colorOn, tooltip))
	}

	return export.Marshal(directives, prepared, sgrResolver(colorOn))
}

// composeCandidate renders one candidate's wire fields into their final
// presentation content: the quoted insertion value, the ListItemText, and
// the ToolTip.
func composeCandidate(candidate wire.Candidate, directives wire.Directives, colorOn bool, tooltip bool) wire.Candidate {
	value := sanitizeField(candidate.Value)
	noSpace := directives.NoSpaceFor(value)

	if quoteNeeded(value) {
		value = wire.QuotePowerShell(value)
	}

	if !noSpace {
		value += " "
	}

	display := sanitizeField(candidate.Display)
	description := sanitizeField(candidate.TrimmedDescription())

	listItem, toolTip := composePresentation(display, description, valueStyleSGR(candidate.Style), colorOn, tooltip)

	return wire.Candidate{
		Value:       value,
		Display:     ensureNotEmpty(listItem),
		Description: ensureNotEmpty(toolTip),
		Style:       candidate.Style,
		Tag:         candidate.Tag,
	}
}

// composePresentation resolves the ListItemText and ToolTip content for one
// candidate: the description joins the ListItemText as a "(description)"
// suffix unless tooltip mode moves it into the ToolTip; color-enabled
// renderings wrap the segments in backtick-escaped SGR sequences with a
// trailing reset.
func composePresentation(display string, description string, valueSGR string, colorOn bool, tooltip bool) (string, string) {
	descriptionSGR := style.SGR(style.Theme.Description)

	switch {
	case !colorOn && tooltip:
		return display, description
	case !colorOn:
		if description != "" {
			return display + " (" + description + ")", " "
		}

		return display, " "
	case tooltip:
		return styledText(valueSGR, display), styledText(descriptionSGR, description)
	default:
		var builder strings.Builder

		builder.WriteString(escape(valueSGR, display))

		if description != "" {
			builder.WriteString(escape(descriptionSGR, " ("+description+")"))
		}

		if valueSGR != "" || (description != "" && descriptionSGR != "") {
			builder.WriteString(sgrReset)
		}

		return builder.String(), " "
	}
}

// valueStyleSGR resolves the SGR parameter numbers for a candidate's value
// styling: the candidate's own style tokens, falling back to the theme's
// default value style.
func valueStyleSGR(candidateStyle string) string {
	return style.SGR(cmp.Or(candidateStyle, style.Theme.Value))
}

// sgrResolver renders the per-candidate "sgr" field: the ECMA-48 SGR
// parameter numbers of the raw style tokens, present when the candidate
// carries a style and color is enabled.
func sgrResolver(colorOn bool) export.StyleResolver {
	return func(candidate wire.Candidate) string {
		if !colorOn || candidate.Style == "" {
			return ""
		}

		return style.SGR(candidate.Style)
	}
}

// escape opens a styled segment: the backtick-escaped SGR sequence followed
// by the text, or the bare text when the style translates to no parameters.
func escape(sgr string, text string) string {
	if sgr == "" {
		return text
	}

	return sgrEscape + sgr + "m" + text
}

// styledText wraps text in a closed styled segment: the backtick-escaped SGR
// sequence, the text, and a full reset.
func styledText(sgr string, text string) string {
	if sgr == "" {
		return text
	}

	return sgrEscape + sgr + "m" + text + sgrReset
}

// quoteNeeded reports whether the value contains whitespace or PowerShell
// metacharacters requiring single-quoting.
func quoteNeeded(value string) bool {
	return strings.ContainsAny(value, metacharacters)
}

// ensureNotEmpty substitutes a single space: CompletionResult rejects empty
// strings, and an empty ToolTip makes PowerShell discard the completion
// entirely.
func ensureNotEmpty(s string) string {
	if s == "" {
		return " "
	}

	return s
}

func sanitizeField(s string) string {
	return wire.StripControl(s, controlCutset)
}
