package bash

import (
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/conga-sh/conga/internal/completion/lex"
	"github.com/conga-sh/conga/internal/completion/wire"
)

// escapedClass lists the characters backslash-escaped in a value typed
// outside quotes: whitespace, the POSIX shell metacharacters, and '!' for
// history expansion.
const (
	escapedClass  = " \t\n\r\"'\\$`[]{}()<>;|&*?#!"
	controlCutset = "\t\n\r"
)

// Format renders candidates in bash's line-oriented framing: a directive
// header line ("D"), optional message lines ("M", discarded by the snippet to
// keep the terminal clean), and one candidate line ("C") per candidate.
//
// Normal mode carries the insertion value with the COMP_WORDBREAKS prefix
// (e.g. `--env=`) stripped and quoted for the word's state: backslash-escaped
// when unquoted, continued (and closed when complete) inside a quote the user
// opened; successive-tab mode (COMP_TYPE=63 with multiple candidates) carries
// tab-delimited display/value/description triplets that the snippet renders
// as "display (description)". Multiple candidates sharing a non-flag common
// value prefix collapse to that prefix with the nospace directive so the
// first tab inserts the shared stem.
func (s *Session) Format(toComplete string, directives wire.Directives, candidates []wire.Candidate) string {
	candidates = sanitizedCandidates(candidates)
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
		lines = append(lines, "M\t"+sanitizeField(message))
	}

	for _, candidate := range candidates {
		closeQuote := !collapsed && !directives.NoSpaceFor(candidate.Value)
		lines = append(lines, "C\t"+s.candidateLine(candidate, wordbreakPrefix, successive, closeQuote))
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
		compType = s.getEnvironment(wire.EnvCompType)
	}

	return candidateCount > 1 && compType == compTypeSuccessiveTabs
}

func sanitizeField(s string) string {
	return wire.FlattenControl(s, controlCutset)
}

// sanitizedCandidates returns a copy of candidates with each value and display
// cleaned (sanitizeField) and the candidates left without a value dropped, so
// the shared-prefix collapse, the nospace rule, and the prefix-trim check all
// read the text bash will insert.
func sanitizedCandidates(candidates []wire.Candidate) []wire.Candidate {
	sanitized := make([]wire.Candidate, 0, len(candidates))

	for _, candidate := range candidates {
		candidate.Value = sanitizeField(candidate.Value)
		if candidate.Value == "" {
			continue
		}

		candidate.Display = sanitizeField(candidate.Display)
		sanitized = append(sanitized, candidate)
	}

	return sanitized
}

// candidateLine renders one candidate: a tab-delimited display/value/
// description triplet in successive-tab mode, or the quoted insertion value
// otherwise. closeQuote reports whether a quote left open in the word is
// closed after the value (a complete value that takes a trailing space).
func (s *Session) candidateLine(candidate wire.Candidate, wordbreakPrefix string, successive bool, closeQuote bool) string {
	if successive {
		return sanitizeField(candidate.Display) + "\t" +
			sanitizeField(candidate.Value) + "\t" +
			sanitizeField(candidate.TrimmedDescription())
	}

	value := sanitizeField(insertionValue(candidate, wordbreakPrefix))

	switch s.openQuote {
	case '"':
		return continueDoubleQuote(value, closeQuote)
	case '\'':
		return continueSingleQuote(value, closeQuote)
	default:
		return escapeValue(value)
	}
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

// escapeValue renders a value typed outside quotes for readline's
// `-o noquote` mode: shell metacharacters, whitespace, and '!' are
// backslash-escaped, as bash's own filename completion and printf %q do.
// The result can be extended by typing on (`My\ ` for a shared prefix), and
// a leading '~' stays unescaped so tilde expansion survives.
func escapeValue(value string) string {
	var builder strings.Builder

	for i := range len(value) {
		if strings.IndexByte(escapedClass, value[i]) >= 0 {
			builder.WriteByte('\\')
		}

		builder.WriteByte(value[i])
	}

	return builder.String()
}

// continueDoubleQuote renders a value inside a double quote the user opened:
// '"', '\', '$', and '`' are backslash-escaped, and '!' (live for history
// expansion inside double quotes) is written as `"\!"`, closing and reopening
// the quote around it. closeQuote appends the closing '"'.
func continueDoubleQuote(value string, closeQuote bool) string {
	var builder strings.Builder

	for i := range len(value) {
		switch char := value[i]; char {
		case '"', '\\', '$', '`':
			builder.WriteByte('\\')
			builder.WriteByte(char)
		case '!':
			builder.WriteString(`"\!"`)
		default:
			builder.WriteByte(char)
		}
	}

	if closeQuote {
		builder.WriteByte('"')
	}

	return builder.String()
}

// continueSingleQuote renders a value inside a single quote the user opened:
// every character is literal except '\”, written as `'\”`. closeQuote
// appends the closing '\”.
func continueSingleQuote(value string, closeQuote bool) string {
	value = strings.ReplaceAll(value, "'", `'\''`)

	if closeQuote {
		value += "'"
	}

	return value
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
	return slices.ContainsFunc(candidates, func(candidate wire.Candidate) bool {
		return candidate.Display != "" && candidate.Display != candidate.Value
	})
}

// noSpaceForAny reports whether the nospace rule applies to the stream: the
// engine's nospace set, or any candidate ending with "=" or "/".
func noSpaceForAny(directives wire.Directives, candidates []wire.Candidate) bool {
	return slices.ContainsFunc(candidates, func(candidate wire.Candidate) bool {
		return directives.NoSpaceFor(candidate.Value)
	})
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
