// Package powershell renders completion results as the JSON payload consumed
// by the generated registration snippet and generates the
// Register-ArgumentCompleter snippets for the wire protocol
// (docs/completion-wire-protocol.md).
package powershell

import (
	"cmp"
	"strings"
	"sync"

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
const metacharacters = " `{}()[]*$?\\\"|<>&,;#'"

//nolint:gochecknoglobals // one-time lazy construction of the shared replacer below
var sanitizerOnce sync.Once

//nolint:gochecknoglobals // immutable after sanitizerOnce completes
var sanitizerReplacer *strings.Replacer

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

// Snippet renders the PowerShell registration script for the given command
// name and aliases (docs/completion-wire-protocol.md): the
// single-command form defines one completer function and registers it for
// the command name (plus its .exe variant, active on Windows and commented
// out elsewhere); the multi-command form assigns one completer scriptblock
// (invoking the binary under the name the user typed, .exe suffix stripped)
// shared by one registration line per name and alias. The completer collects
// the cursor-truncated command elements (quotes stripped, PowerShell's
// tokenization), drops the program name, and feeds the words to the binary's
// __complete entry point; the JSON payload is piped through
// ConvertFrom-Json into CompletionResult objects. An empty result set
// returns "" so PowerShell's default file completion is suppressed.
func Snippet(name string, aliases []string) string {
	if len(aliases) > 0 {
		return multiSnippet(name, aliases)
	}

	return singleSnippet(name)
}

// sanitizer strips the characters PowerShell cannot carry through a
// completion result: newlines, tabs, and carriage returns are removed.
// Construction is deferred to first use so the driver package stays free of
// package initializers and drops at link time when powershell completion is
// disabled.
func sanitizer() *strings.Replacer {
	sanitizerOnce.Do(func() {
		sanitizerReplacer = strings.NewReplacer("\n", "", "\t", "", "\r", "")
	})

	return sanitizerReplacer
}

// composeCandidate renders one candidate's wire fields into their final
// presentation content: the quoted insertion value, the ListItemText, and
// the ToolTip.
func composeCandidate(candidate wire.Candidate, directives wire.Directives, colorOn bool, tooltip bool) wire.Candidate {
	value := sanitizer().Replace(candidate.Value)
	noSpace := directives.NoSpaceFor(value)

	if quoteNeeded(value) {
		value = wire.QuotePowerShell(value)
	}

	if !noSpace {
		value += " "
	}

	display := sanitizer().Replace(candidate.Display)
	description := sanitizer().Replace(candidate.TrimmedDescription())

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
