// Package zsh renders completion results as the record stream consumed by
// the generated registration snippet and generates the #compdef registration
// snippets for the wire protocol (docs/completion-wire-protocol.md).
package zsh

import (
	"cmp"
	"slices"
	"strings"

	"github.com/conga-sh/conga/internal/completion/wire"
	"github.com/conga-sh/conga/internal/slicex"
	"github.com/conga-sh/conga/internal/style"
)

// Record kinds of the framing: a directives header (always first, carrying
// the directive tokens and the raw usage), an optional list-colors
// specification, message lines, and one candidate record per candidate.
const (
	recordDirectives = "D"
	recordListColors = "L"
	recordMessage    = "M"
	recordCandidate  = "C"
	fieldSep         = "\x1f"
	controlCutset    = "\t\n\r\x1f"

	longhandFlagsTag   = "longhand flags"
	shorthandFlagsTag  = "shorthand flags"
	mergedFlagsTag     = "flags"
	valueEscapeClass   = " \"'\\$`|&;<>()[]{}*?!^#:"
	patternEscapeClass = "\\[]*?()|^~#<>:"
)

// Format renders candidates and directives in the zsh framing: a directives
// record ("D", always first), an optional list-colors record ("L"), message
// records ("M"), and one candidate record ("C") per candidate.
//
// Flag tag merging applies before framing: the "longhand flags" and
// "shorthand flags" tags merge into "flags" unless CONGA_MERGE_FLAGS is
// explicitly disabled. The noprefix directive is implied when the word being
// completed is empty: candidates are complete word replacements and zsh must
// not auto-insert into the buffer (compstate[insert]=""). The usage string
// folds into the message stream — deduplicated against the engine's own
// messages — so ActiveHelp hints and usage lines share the single
// _message -r channel. Candidate records are grouped by tag (first
// appearance, untagged candidates last) so the snippet emits exactly one
// _describe call per tag. toComplete only drives the noprefix rule and is
// otherwise ignored: the framing is position-independent.
func Format(toComplete string, directives wire.Directives, candidates []wire.Candidate) string {
	colorOn := !wire.ColorDisabled()

	candidates = mergeFlagTags(candidates)
	candidates = groupByTag(candidates)

	if toComplete == "" && len(candidates) > 0 {
		directives.NoPrefix.Add('*')
	}

	lines := make([]string, 0, 2+len(directives.Messages)+len(candidates))
	lines = append(lines, directivesRecord(directives))

	if colorOn {
		lines = append(lines, recordListColors+fieldSep+listColorsSpec(candidates, wire.DescriptionsEnabled()))
	}

	lines = append(lines, messageRecords(directives, colorOn)...)

	for _, candidate := range candidates {
		lines = append(lines, candidateRecord(candidate, directives, colorOn))
	}

	return strings.Join(lines, "\n")
}

// sanitizeField normalizes the characters that would corrupt the
// line-oriented record stream: every tab, newline, carriage return, and unit
// separator becomes a single space.
func sanitizeField(s string) string {
	return wire.ReplaceControlWithSpace(s, controlCutset)
}

// mergeFlagTags re-tags the engine's "longhand flags" and "shorthand flags"
// candidates into the merged "flags" tag unless CONGA_MERGE_FLAGS is
// explicitly set to a parseable false. Merging is default-on for zsh.
func mergeFlagTags(candidates []wire.Candidate) []wire.Candidate {
	merge, explicit := wire.MergeFlags()
	if explicit && !merge {
		return candidates
	}

	for i, candidate := range candidates {
		if candidate.Tag == longhandFlagsTag || candidate.Tag == shorthandFlagsTag {
			candidates[i].Tag = mergedFlagsTag
		}
	}

	return candidates
}

// groupByTag stably partitions the candidates by tag in order of first
// appearance, with untagged candidates last, so the snippet's consecutive-tag
// flush emits exactly one _describe call per tag.
func groupByTag(candidates []wire.Candidate) []wire.Candidate {
	grouped := make([]wire.Candidate, 0, len(candidates))
	tags := make([]string, 0, len(candidates))

	for _, candidate := range candidates {
		if candidate.Tag != "" && !slices.Contains(tags, candidate.Tag) {
			tags = append(tags, candidate.Tag)
		}
	}

	for _, tag := range append(tags, "") {
		for _, candidate := range candidates {
			if candidate.Tag == tag {
				grouped = append(grouped, candidate)
			}
		}
	}

	return grouped
}

// directivesRecord renders the leading "D" record: the comma-joined directive
// tokens and the raw usage string (metadata; the user-visible usage reaches
// the shell through the message stream).
func directivesRecord(directives wire.Directives) string {
	return recordDirectives + fieldSep + strings.Join(directives.Tokens(), ",") +
		fieldSep + sanitizeField(directives.Usage)
}

// messageRecords renders the "M" records: the engine's messages plus the
// folded usage string, each wrapped in the theme's usage style when color is
// enabled (observed parity: dim foreground, closed by the default-foreground
// reset).
func messageRecords(directives wire.Directives, colorOn bool) []string {
	messages := make([]string, 0, len(directives.Messages)+1)

	for _, message := range directives.Messages {
		messages = append(messages, styledMessage(message, colorOn))
	}

	if usage := directives.Usage; usage != "" && !slices.Contains(directives.Messages, usage) {
		messages = append(messages, styledMessage(usage, colorOn))
	}

	records := slicex.Map(messages, func(message string) string {
		return recordMessage + fieldSep + message
	})

	return records
}

// styledMessage sanitizes a message and wraps it in the theme's usage style
// when color is enabled.
func styledMessage(message string, colorOn bool) string {
	message = sanitizeField(message)
	sgr := style.SGR(style.Theme.Usage)

	if !colorOn || sgr == "" {
		return message
	}

	return "\x1b[" + sgr + "m" + message + "\x1b[39m"
}

// candidateRecord renders one "C" record: tag, display, description, the
// escaped insertion value, and the candidate's SGR parameter numbers. The
// trailing space is embedded unless the nospace rules apply (directive set,
// or the value ends with "=" or "/").
func candidateRecord(candidate wire.Candidate, directives wire.Directives, colorOn bool) string {
	value := escapeValue(sanitizeField(candidate.Value))
	if !directives.NoSpaceFor(candidate.Value) {
		value += " "
	}

	var sgr string
	if colorOn && candidate.Style != "" {
		sgr = style.SGR(candidate.Style)
	}

	fields := []string{
		sanitizeField(candidate.Tag),
		sanitizeField(candidate.Display),
		sanitizeField(candidate.Description),
		value,
		sgr,
	}

	return recordCandidate + fieldSep + strings.Join(fields, fieldSep)
}

// escapeChars backslash-escapes characters in input that are present in the class.
func escapeChars(input, class string) string {
	if !strings.ContainsAny(input, class) {
		return input
	}

	var builder strings.Builder
	builder.Grow(len(input) + 4)

	for _, r := range input {
		if strings.ContainsRune(class, r) {
			builder.WriteByte('\\')
		}

		builder.WriteRune(r)
	}

	return builder.String()
}

// escapeValue backslash-escapes the zsh metacharacters of an insertion value
// so _describe -Q can insert it verbatim. Colon is included because _describe
// uses it as the value/description separator.
func escapeValue(value string) string {
	return escapeChars(value, valueEscapeClass)
}

// listColorsSpec builds the zsh list-colors specification: one entry pair per
// candidate (a two-tone variant matching "display -- description" renderings
// and a display-only variant, with an empty value color for unstyled
// candidates so user-configured list-colors defaults do not leak onto the
// menu rows), closed by a description-column catch-all. When descriptions are
// suppressed only the display-only entries are emitted (the two-tone and
// catch-all patterns could never match). Entry count is capped by
// CONGA_ZSH_STYLE_LIMIT; a negative limit is unlimited.
func listColorsSpec(candidates []wire.Candidate, descriptions bool) string {
	limit := wire.ZshStyleLimit()
	descriptionSGR := style.SGR(style.Theme.Description)

	entries := make([]string, 0, 2*len(candidates)+1)

	for _, candidate := range candidates {
		if limit == 0 {
			break
		}

		if limit > 0 {
			limit--
		}

		display := patternEscape(sanitizeField(candidate.Display))
		valueSGR := style.SGR(cmp.Or(candidate.Style, style.Theme.Value))

		if descriptions {
			entries = append(entries, "=(#b)("+display+")([ ]## -- *)=0="+valueSGR+"="+descriptionSGR)
		}

		entries = append(entries, "=(#b)("+display+")=0="+valueSGR)
	}

	if descriptions {
		entries = append(entries, "=(#b)(-- *)=0="+descriptionSGR)
	}

	return strings.Join(entries, ":")
}

// patternEscape backslash-escapes the glob metacharacters of a display string
// inside a list-colors pattern.
func patternEscape(display string) string {
	return escapeChars(display, patternEscapeClass)
}
