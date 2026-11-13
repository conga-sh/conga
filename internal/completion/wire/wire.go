// Package wire defines the shell-agnostic completion wire vocabulary shared by
// the native completion drivers: candidates, directives, per-shell quoting
// helpers, the RFC 8259 string escaper, and the CONGA_* sideband environment
// readers.
//
// The types here are the fork-free contract between the completion engine
// (predict.Result) and the per-shell formatters (bash, zsh, fish, powershell,
// export). The framing each shell consumes is specified in
// docs/completion-wire-protocol.md.
package wire

import (
	"cmp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/conga-sh/conga/predict"
)

// valueControlCutset lists the control characters FromResult flattens out of
// candidate values and displays (FlattenControl) before any framing.
const valueControlCutset = "\t\n\r"

// Candidate is a single completion candidate on the wire.
type Candidate struct {
	// Value is the string inserted into the command line.
	Value string
	// Display is the menu label, resolved to Value when the engine leaves it empty.
	Display string
	// Description is the optional help text shown next to the candidate.
	Description string
	// Style is the optional style token string from the internal/style vocabulary.
	Style string
	// Tag is the optional grouping tag (e.g. "commands", "longhand flags").
	Tag string
	// Alias reports that the candidate is an alternative spelling of another
	// candidate (a subcommand/flag alias or a predictor-supplied domain alias).
	Alias bool
}

// TrimmedDescription returns the description clamped for shells with a
// single-line description channel: only the first line is kept (trimmed) and
// truncated to the configured maximum length with "..." appended. The cut is
// hard (without ellipsis) when the limit is 3 or fewer runes.
func (c Candidate) TrimmedDescription() string {
	description, _, _ := strings.Cut(c.Description, "\n")

	return ClampDescription(strings.TrimSpace(description), DescriptionLength())
}

// Directives carries the shell-agnostic instructions accompanying a candidate
// stream.
type Directives struct {
	// NoSpace holds the runes after which no space is appended while the
	// nospace directive is active ("*" matches every value).
	NoSpace SuffixSet
	// NoPrefix holds the runes the shell must not re-insert from the current
	// word when candidates are complete word replacements ("*" matches every
	// value).
	NoPrefix PrefixSet
	// KeepOrder requests that the shell preserves the engine candidate order.
	KeepOrder bool
	// Messages are diagnostics for shells with a message channel (zsh).
	Messages []string
	// Usage is the raw usage string accompanying an error result.
	Usage string
	// SuppressDescriptions reports that the application-prefixed sideband
	// disabled candidate descriptions, so framings that style a description
	// column (zsh list-colors) can omit patterns that would never match. It is
	// framing metadata, not a wire token; the zero value keeps the default
	// (descriptions shown).
	SuppressDescriptions bool
}

// NoSpaceFor reports whether inserting value must suppress the trailing space:
// either the nospace directive is active, or the value ends with "=" or "/".
func (d Directives) NoSpaceFor(value string) bool {
	if d.NoSpace.Matches(value) {
		return true
	}

	return strings.HasSuffix(value, "=") || strings.HasSuffix(value, "/")
}

// Tokens returns the directive vocabulary tokens in effect, in fixed order
// (keeporder, noprefix, nospace), for rendering into shell framing headers.
func (d Directives) Tokens() []string {
	tokens := make([]string, 0, 3)

	if d.KeepOrder {
		tokens = append(tokens, "keeporder")
	}

	if !d.NoPrefix.IsEmpty() {
		tokens = append(tokens, "noprefix")
	}

	if !d.NoSpace.IsEmpty() {
		tokens = append(tokens, "nospace")
	}

	return tokens
}

// PrefixSet is a set of runes matched against the first rune of a word, with
// "*" as the match-any wildcard.
type PrefixSet struct {
	runeSet
}

// Matches reports whether the first rune of value is in the set. The empty
// string never matches.
func (s *PrefixSet) Matches(value string) bool {
	if s.runes == "" || value == "" {
		return false
	}

	if s.runes == "*" {
		return true
	}

	firstRune, _ := utf8.DecodeRuneInString(value)

	return strings.ContainsRune(s.runes, firstRune)
}

// SuffixSet is a set of runes matched against the last rune of a word, with
// "*" as the match-any wildcard.
type SuffixSet struct {
	runeSet
}

// Matches reports whether the last rune of value is in the set. The empty
// string never matches.
func (s *SuffixSet) Matches(value string) bool {
	if s.runes == "" || value == "" {
		return false
	}

	if s.runes == "*" {
		return true
	}

	lastRune, _ := utf8.DecodeLastRuneInString(value)

	return strings.ContainsRune(s.runes, lastRune)
}

// ClampDescription truncates description to maxLength runes, appending "..."
// when text was dropped. The cut is hard (without ellipsis) when maxLength is
// 3 or fewer runes.
func ClampDescription(description string, maxLength int) string {
	runes := []rune(description)
	if len(runes) <= maxLength {
		return description
	}

	if maxLength <= 3 {
		return string(runes[:maxLength])
	}

	return string(runes[:maxLength-3]) + "..."
}

// FromResult translates an engine result into the wire vocabulary: candidates
// in engine order with the display defaulting to the value, and directives
// derived from the engine's directive bits. envPrefix is the application's
// normalized completion environment prefix (TASK_ for an app named task):
// alias candidates are dropped when the prefixed alias variable disables them,
// and descriptions are blanked when the prefixed description variable disables
// them, so every framing observes the same suppressed stream. Tabs, newlines,
// and carriage returns are flattened out of each value and display
// (FlattenControl), so every framing sees the same cleaned value; descriptions
// stay raw because fish and elvish keep only their first line. Candidates whose
// value is then empty are dropped: they have no insertion text, and a blank
// candidate next to a real one would keep bash and zsh from inserting the only
// real match. The result's file-completion fallback list is engine-internal
// and is not carried onto the wire.
func FromResult(envPrefix string, result predict.Result) ([]Candidate, Directives) {
	aliases := AliasesEnabled(envPrefix)
	descriptions := DescriptionsEnabled(envPrefix)

	candidates := make([]Candidate, 0, len(result.Items))

	for _, item := range result.Items {
		if item.Alias && !aliases {
			continue
		}

		value := FlattenControl(item.Value, valueControlCutset)
		if value == "" {
			continue
		}

		candidate := Candidate{
			Value:   value,
			Display: cmp.Or(FlattenControl(item.Display, valueControlCutset), value),
			Style:   item.Style,
			Tag:     item.Tag,
			Alias:   item.Alias,
		}

		if descriptions {
			candidate.Description = item.Description
		}

		candidates = append(candidates, candidate)
	}

	directives := Directives{
		KeepOrder:            result.Directive.Has(predict.DirectiveKeepOrder),
		Usage:                result.Usage,
		SuppressDescriptions: !descriptions,
	}

	if result.Directive.Has(predict.DirectiveNoSpace) {
		directives.NoSpace.Add('*')
	}

	if result.Directive.Has(predict.DirectiveError) {
		directives.Messages = append(directives.Messages, cmp.Or(result.Usage, "ERROR"))
	}

	return candidates, directives
}

// FlattenControl flattens the characters in cutset out of input: those at the
// start and end are removed (a value read from CRLF lines keeps no trailing
// "\r"), and each remaining one becomes an ASCII space. A CRLF pair counts as
// one line break and becomes a single space when both characters are in
// cutset. It returns input unchanged if input contains no cutset characters.
func FlattenControl(input, cutset string) string {
	if !strings.ContainsAny(input, cutset) {
		return input
	}

	input = strings.Trim(input, cutset)

	if strings.ContainsRune(cutset, '\r') && strings.ContainsRune(cutset, '\n') {
		input = strings.ReplaceAll(input, "\r\n", "\n")
	}

	return strings.Map(func(r rune) rune {
		if strings.ContainsRune(cutset, r) {
			return ' '
		}

		return r
	}, input)
}

// runeSet holds a sorted, deduplicated set of runes or "*" as a wildcard.
type runeSet struct {
	runes string
}

// Add inserts runes into the set, deduplicating and sorting them; adding "*"
// collapses the set to the wildcard.
func (s *runeSet) Add(runes ...rune) {
	s.runes = addRunes(s.runes, runes...)
}

// IsEmpty reports whether the set holds no runes.
func (s *runeSet) IsEmpty() bool {
	return s.runes == ""
}

// String returns the set's runes as a sorted string ("*" for the wildcard).
func (s *runeSet) String() string {
	return s.runes
}

// addRunes merges runes into an existing sorted, deduplicated set; adding "*"
// (or adding to a set already holding "*") collapses the set to the wildcard.
func addRunes(existing string, add ...rune) string {
	if strings.ContainsRune(existing, '*') || slices.Contains(add, '*') {
		return "*"
	}

	unique := []rune(existing)
	for _, r := range add {
		if !slices.Contains(unique, r) {
			unique = append(unique, r)
		}
	}

	slices.Sort(unique)

	return string(unique)
}
