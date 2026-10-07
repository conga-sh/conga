package bash

import (
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/conga-sh/conga/internal/completion/lex"
	"github.com/conga-sh/conga/internal/completion/wire"
)

// quotedClass lists the characters that force double-quoting of a candidate
// value: whitespace and the POSIX shell metacharacters.
const quotedClass = " \t\n\r\"'\\$`[]{}()<>;|&*?#"

//nolint:gochecknoglobals // one-time lazy construction of the shared replacer below
var fieldSanitizerOnce sync.Once

//nolint:gochecknoglobals // immutable after fieldSanitizerOnce completes
var fieldSanitizerReplacer *strings.Replacer

// Format renders candidates in bash's line-oriented framing: a directive
// header line ("D"), optional message lines ("M", discarded by the snippet to
// keep the terminal clean), and one candidate line ("C") per candidate.
//
// Normal mode carries the insertion value with the COMP_WORDBREAKS prefix
// (e.g. `--env=`) stripped and POSIX double-quoting applied; successive-tab
// mode (COMP_TYPE=63 with multiple candidates) carries tab-delimited
// display/value/description triplets that the snippet renders as
// "display (description)". Multiple candidates sharing a non-flag common
// value prefix collapse to that prefix with the nospace directive so the
// first tab inserts the shared stem.
func (s *Session) Format(toComplete string, directives wire.Directives, candidates []wire.Candidate) string {
	wordbreakPrefix := s.resolveWordbreakPrefix(toComplete)
	successive := s.isSuccessiveTabs(len(candidates))

	collapsed := false

	if !successive && len(candidates) > 1 {
		if prefix := commonValuePrefix(candidates, toComplete); prefix != "" {
			candidates = []wire.Candidate{{Value: prefix, Display: prefix}}
			collapsed = true
		}
	}

	lines := make([]string, 0, len(candidates)+len(directives.Messages)+1)
	lines = append(lines, "D\t"+strings.Join(headerTokens(directives, candidates, collapsed, successive), " "))

	for _, message := range directives.Messages {
		lines = append(lines, "M\t"+fieldSanitizer().Replace(message))
	}

	for _, candidate := range candidates {
		lines = append(lines, "C\t"+candidateLine(candidate, wordbreakPrefix, successive))
	}

	return strings.Join(lines, "\n")
}

// resolveWordbreakPrefix prefers the prefix lexed by Patch from the raw
// COMP_LINE when the session ran a Patch pass; without one (direct formatter
// invocations) it derives the prefix from the word being completed itself.
func (s *Session) resolveWordbreakPrefix(toComplete string) string {
	if s.active {
		return s.wordbreakPrefix
	}

	return lex.LastWordbreakPrefix(toComplete, s.resolveCompWordbreaks())
}

// isSuccessiveTabs reports whether the listing runs in readline's
// successive-tabs mode: COMP_TYPE=63 (the session's Patch-derived value first,
// then the environment for direct formatter invocations) with multiple
// candidates.
func (s *Session) isSuccessiveTabs(candidateCount int) bool {
	compType := s.compType
	if !s.active {
		compType = s.getEnvironment(envCompType)
	}

	return candidateCount > 1 && compType == compTypeSuccessiveTabs
}

// fieldSanitizer normalizes the characters that would corrupt the
// line-oriented framing: every tab, newline, and carriage return becomes a
// single space. Construction is deferred to first use so the driver package
// stays free of package initializers and drops at link time when bash
// completion is disabled.
func fieldSanitizer() *strings.Replacer {
	fieldSanitizerOnce.Do(func() {
		fieldSanitizerReplacer = strings.NewReplacer("\t", " ", "\n", " ", "\r", " ")
	})

	return fieldSanitizerReplacer
}

// candidateLine renders one candidate: a tab-delimited display/value/
// description triplet in successive-tab mode, or the quoted insertion value
// otherwise.
func candidateLine(candidate wire.Candidate, wordbreakPrefix string, successive bool) string {
	if successive {
		return fieldSanitizer().Replace(candidate.Display) + "\t" +
			fieldSanitizer().Replace(candidate.Value) + "\t" +
			fieldSanitizer().Replace(candidate.TrimmedDescription())
	}

	return quoteValue(insertionValue(candidate, wordbreakPrefix))
}

// insertionValue resolves the text bash inserts for one candidate: the value
// with the wordbreak prefix stripped (the shell replaces only the text after
// the last wordbreak rune), falling back to the engine display — the tail
// after the attached separator — when the prefix does not match the value
// (e.g. the mid-quote word `--env="pro`).
func insertionValue(candidate wire.Candidate, wordbreakPrefix string) string {
	value := candidate.Value

	if wordbreakPrefix != "" {
		if tail, found := strings.CutPrefix(value, wordbreakPrefix); found {
			return tail
		}

		if candidate.Display != "" && candidate.Display != candidate.Value {
			return candidate.Display
		}
	}

	return value
}

// quoteValue renders the insertion value for readline's `-o noquote` mode:
// tilde-leading values keep their tilde expansion and only backslash-escape
// metacharacters; values containing whitespace or shell metacharacters are
// double-quoted with '"', '\', '$', and '`' escaped; everything else is raw.
func quoteValue(value string) string {
	value = fieldSanitizer().Replace(value)

	switch {
	case strings.HasPrefix(value, "~"):
		return escapeTildeValue(value)
	case strings.ContainsAny(value, quotedClass):
		return quoteDouble(value)
	default:
		return value
	}
}

// escapeTildeValue backslash-escapes the metacharacters of a tilde-leading
// value so readline preserves tilde expansion.
func escapeTildeValue(value string) string {
	var builder strings.Builder

	for i := range len(value) {
		switch r := value[i]; r {
		case ' ', '\t', '"', '\'', '\\', '$', '`', '&', ';', '<', '>', '(', ')', '[', ']', '{', '}', '|', '*', '?', '#':
			builder.WriteByte('\\')
			builder.WriteByte(r)
		default:
			builder.WriteByte(r)
		}
	}

	return builder.String()
}

// quoteDouble wraps the value in double quotes with the POSIX double-quote
// escape set ('"', '\', '$', '`').
func quoteDouble(value string) string {
	var builder strings.Builder

	builder.WriteByte('"')

	for i := range len(value) {
		switch r := value[i]; r {
		case '"', '\\', '$', '`':
			builder.WriteByte('\\')
			builder.WriteByte(r)
		default:
			builder.WriteByte(r)
		}
	}

	builder.WriteByte('"')

	return builder.String()
}

// headerTokens resolves the directive vocabulary tokens for the D line:
// keeporder from the engine, noprefix when a wordbreak prefix was stripped,
// and nospace when any candidate suppresses the trailing space, the stream
// collapsed, or the listing is successive.
func headerTokens(directives wire.Directives, candidates []wire.Candidate, collapsed bool, successive bool) []string {
	tokens := []string{}

	if directives.KeepOrder {
		tokens = append(tokens, "keeporder")
	}

	if trimmedAny(candidates) {
		tokens = append(tokens, "noprefix")
	}

	if collapsed || successive || !directives.NoSpace.IsEmpty() || noSpaceForAny(directives, candidates) {
		tokens = append(tokens, "nospace")
	}

	return tokens
}

// trimmedAny reports whether any candidate had a wordbreak prefix stripped
// during rendering (candidates are then tail replacements).
func trimmedAny(candidates []wire.Candidate) bool {
	for _, candidate := range candidates {
		if candidate.Display != "" && candidate.Display != candidate.Value {
			return true
		}
	}

	return false
}

// noSpaceForAny reports whether the nospace rule applies to the stream: the
// engine's nospace set, or any candidate ending with "=" or "/".
func noSpaceForAny(directives wire.Directives, candidates []wire.Candidate) bool {
	for _, candidate := range candidates {
		if directives.NoSpaceFor(candidate.Value) {
			return true
		}
	}

	return false
}

// commonValuePrefix returns the longest common prefix of every candidate's
// value when it extends the word being completed: the first-tab stem
// insertion ("prod-" for prod-us-east-1/prod-eu-west-1). Flag values
// (leading "-") never collapse — the user needs the flag list, and the typed
// dashes already bound the word.
func commonValuePrefix(candidates []wire.Candidate, toComplete string) string {
	if len(candidates) == 0 {
		return ""
	}

	prefix := candidates[0].Value
	for _, candidate := range candidates[1:] {
		for !strings.HasPrefix(candidate.Value, prefix) {
			prefix = shrinkRune(prefix)

			if prefix == "" || prefix == toComplete {
				return ""
			}
		}
	}

	if prefix == "" || len(prefix) <= len(toComplete) || strings.HasPrefix(prefix, "-") {
		return ""
	}

	return prefix
}

// shrinkRune removes the trailing rune from prefix. If prefix is empty or contains
// an invalid trailing byte sequence, it strips at least one byte to make progress.
func shrinkRune(prefix string) string {
	_, size := utf8.DecodeLastRuneInString(prefix)
	if size == 0 {
		return ""
	}

	return prefix[:len(prefix)-size]
}
