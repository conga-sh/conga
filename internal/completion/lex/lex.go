// Package lex implements a clean-room lexer for bash COMP_LINE command lines
// (docs/completion-wire-protocol.md). It provides the quote-aware
// word extraction, redirect and pipeline classification, and COMP_WORDBREAKS
// prefix extraction that the native bash driver's Patch seam needs to reject
// redirect targets, isolate the current pipeline segment, and trim wordbreak
// prefixes from candidate values.
//
// The lexer follows the GNU Bash manual (Programmable Completion) and the
// POSIX Shell Command Language: single quotes preserve every character,
// double quotes honor backslash escapes for '"', '\', '$', '`', and newline,
// backslash escapes any character outside quotes, and unclosed quotes at the
// cursor tolerate the token through end of line (the user is mid-quote).
package lex

import (
	"slices"
	"strings"
	"unicode/utf8"
)

// Token kinds: Word carries a quotes-resolved value; Redirect, Pipe, and
// Separator carry the operator text.
const (
	// Word is a command word: a contiguous run of non-whitespace,
	// non-operator characters with quotes resolved. Raw keeps the original
	// source text including quote and wordbreak characters.
	Word Kind = iota
	// Redirect is an I/O redirection operator (<, >, >>, <>, <<<, <&, &>,
	// >&, &>>) optionally preceded by an fd number (e.g. "2>").
	Redirect
	// Pipe is a pipeline delimiter ("|", "|&").
	Pipe
	// Separator is a command list separator (";", "&", "&&", "||").
	Separator
)

// redirectForms lists the redirection operators longest-first so prefix
// forms ("<<<" before "<") match first.
//
//nolint:gochecknoglobals // immutable, longest-first redirect operator match order
var redirectForms = []string{"<<<", "&>>", "<>", "<&", "&>", ">&", ">>", "<", ">"}

// Kind classifies a lexed token.
type Kind int

// Token is one lexed element of a command line.
type Token struct {
	// Kind is the token classification.
	Kind Kind
	// Value is the quotes-resolved text for Word tokens and the operator
	// spelling for the other kinds.
	Value string
	// Raw is the original source text of the token. For words it retains
	// quote and wordbreak characters (e.g. `--env="pro`), which
	// WordbreakPrefix needs to reproduce readline's replacement semantics.
	Raw string
	// OpenQuote is the quote character ('\'' or '"') left unclosed at the end
	// of a word (the user is mid-quote), or 0 when every quote is closed.
	OpenQuote byte
}

// Line is a lexed command line: a sequence of tokens with the trailing
// empty word preserved for whitespace-terminated lines (the word being
// completed at the cursor may be empty).
type Line struct {
	Tokens []Token
}

// Words returns the quotes-resolved word values in order. Redirect operators
// and their targets are included; combine with CurrentPipeline and
// WithoutRedirects for engine-ready words.
func (l Line) Words() []string {
	words := make([]string, 0, len(l.Tokens))
	for _, token := range l.Tokens {
		if token.Kind == Word {
			words = append(words, token.Value)
		}
	}

	return words
}

// CurrentPipeline returns the tokens of the last pipeline segment: the
// tokens after the final Pipe or Separator (completion after "|", ";",
// "&&", "||" concerns only the segment at the cursor).
func (l Line) CurrentPipeline() Line {
	for i, v := range slices.Backward(l.Tokens) {
		if v.Kind == Pipe || v.Kind == Separator {
			return Line{Tokens: l.Tokens[i+1:]}
		}
	}

	return l
}

// WithoutRedirects drops every Redirect token together with the word that
// follows it as its target, so completing after a redirect (e.g.
// "corpusctl deploy > /tmp/out.log --env ") resumes at the preceding words.
func (l Line) WithoutRedirects() Line {
	kept := make([]Token, 0, len(l.Tokens))

	skipTarget := false
	for _, token := range l.Tokens {
		if skipTarget {
			skipTarget = token.Kind == Redirect

			continue
		}

		if token.Kind == Redirect {
			skipTarget = true

			continue
		}

		kept = append(kept, token)
	}

	return Line{Tokens: kept}
}

// RedirectAtEnd reports whether the completion position sits at a redirect:
// either the trailing token is a redirect operator ("cloudctl deploy >"),
// or the token before the trailing word is a redirect operator ("cloudctl
// deploy > <cursor>"), in which case no candidates must be offered.
func (l Line) RedirectAtEnd() bool {
	n := len(l.Tokens)
	if n == 0 {
		return false
	}

	return l.Tokens[n-1].Kind == Redirect || (n >= 2 && l.Tokens[n-2].Kind == Redirect)
}

// WordbreakPrefix returns the quotes-resolved part of the last word that
// readline leaves in place when it inserts a completion: the text up to and
// including the last wordbreak rune outside quotes and not escaped, or up to
// a quote left open at the end of the word, whichever comes later (`--env=`
// for `--env=pro`, `--env=` for `--env="pro`, "" for `"My Dir/in` and
// `My\ Dir/in`). wordbreaks is the rune set, normally the COMP_WORDBREAKS
// value. The prefix is empty when the line ends with an operator or no such
// boundary exists.
func (l Line) WordbreakPrefix(wordbreaks string) string {
	if len(l.Tokens) == 0 || l.Tokens[len(l.Tokens)-1].Kind != Word {
		return ""
	}

	return replacementPrefix(l.Tokens[len(l.Tokens)-1].Raw, wordbreaks)
}

// Parse lexes the cursor-truncated command line into tokens. Unclosed quotes
// are tolerated (the token runs to end of line). A line that is empty or ends
// with whitespace separating words yields a trailing empty Word token.
// Whitespace inside quotes or escaped with a backslash belongs to the word
// (`foo\ ` is the one word "foo "). An unquoted '#' at the start of a word
// begins a comment that consumes the rest of the line.
func Parse(line string) Line {
	tokens, trailingSeparator := lexLine(line)

	if line == "" || trailingSeparator {
		tokens = append(tokens, Token{Kind: Word})
	}

	return Line{Tokens: tokens}
}

// WordsUpTo lexes line truncated at point and returns the quotes-resolved words.
func WordsUpTo(line string, point int) []string {
	point = max(0, min(point, len(line)))

	return Parse(line[:point]).Words()
}

// LastWordbreakPrefix returns the prefix of word up to and including its
// last occurrence of any rune in wordbreaks, or "" when none occurs.
func LastWordbreakPrefix(word string, wordbreaks string) string {
	prefix := ""

	for index, r := range word {
		if strings.ContainsRune(wordbreaks, r) {
			prefix = word[:index+utf8.RuneLen(r)]
		}
	}

	return prefix
}

// replacementPrefix resolves raw (one word's source text) and returns the
// resolved value up to readline's replacement start: after the last
// wordbreak rune outside quotes and escapes, or after a quote still open at
// the end of raw. Quote and escape rules match the lexer's.
func replacementPrefix(raw string, wordbreaks string) string { //nolint:cyclop,gocyclo // one scan whose switch is the quote/escape state table; splitting it would hide the state transitions
	var value strings.Builder

	keep := 0
	quote := byte(0)
	quoteStart := 0

	for i := 0; i < len(raw); i++ {
		char := raw[i]

		switch {
		case quote == '\'' && char == '\'', quote == '"' && char == '"':
			quote = 0
		case quote == '\'':
			value.WriteByte(char)
		case quote == '"' && char == '\\' && i+1 < len(raw) && isEscapableInDoubleQuote(raw[i+1]):
			i++
			writeEscaped(&value, raw[i])
		case quote == '"':
			value.WriteByte(char)
		case char == '\'' || char == '"':
			quote = char
			quoteStart = value.Len()
		case char == '\\' && i+1 < len(raw):
			i++
			writeEscaped(&value, raw[i])
		default:
			value.WriteByte(char)

			if strings.IndexByte(wordbreaks, char) >= 0 {
				keep = value.Len()
			}
		}
	}

	if quote != 0 {
		keep = max(keep, quoteStart)
	}

	return value.String()[:keep]
}

// lexer holds the scan state for one line.
type lexer struct {
	source string
	pos    int
	tokens []Token
}

// scanWord consumes one word: contiguous non-whitespace characters including
// quotes and wordbreak runes, terminated by whitespace, an operator, a
// comment, or end of line. Redirection, pipeline, and list operators
// terminate the word even without surrounding whitespace; an fd number
// immediately followed by a redirection operator folds into the redirect
// token.
func (l *lexer) scanWord() {
	word := newWordBuilder()

	for l.pos < len(l.source) {
		char := l.source[l.pos]

		if isWhitespace(char) {
			break
		}

		if isOperator(char) {
			fdNumber := word.rawText()
			if fdNumber != "" && !allDigits(fdNumber) {
				l.tokens = append(l.tokens, word.token())

				fdNumber = ""
			}

			l.scanOperator(fdNumber)

			return
		}

		if char == '#' && word.rawEmpty() {
			l.pos = len(l.source)

			return
		}

		switch char {
		case '\'':
			word.scanSingleQuote(l)
		case '"':
			word.scanDoubleQuote(l)
		case '\\':
			word.scanEscape(l)
		default:
			word.push(char, char)

			l.pos++
		}
	}

	l.tokens = append(l.tokens, word.token())
}

// scanOperator consumes one operator at the cursor: a redirection (folding
// the given fd-number prefix, e.g. "2"), a pipeline delimiter, or a list
// separator.
func (l *lexer) scanOperator(fdNumber string) {
	remaining := l.source[l.pos:]

	var (
		kind Kind
		text string
	)

	switch {
	case strings.HasPrefix(remaining, "&&"):
		kind, text = Separator, "&&"
	case strings.HasPrefix(remaining, "|&"):
		kind, text = Pipe, "|&"
	case strings.HasPrefix(remaining, "||"):
		kind, text = Separator, "||"
	default:
		for _, form := range redirectForms {
			if strings.HasPrefix(remaining, form) {
				kind, text = Redirect, form

				break
			}
		}
	}

	if text == "" {
		text = remaining[:1]

		switch text[0] {
		case '|':
			kind = Pipe
		case ';', '&':
			kind = Separator
		}
	}

	raw := fdNumber + text
	l.tokens = append(l.tokens, Token{Kind: kind, Value: text, Raw: raw})
	l.pos += len(text)
}

// wordBuilder accumulates one word's quotes-resolved value and raw source
// text.
type wordBuilder struct {
	value strings.Builder
	raw   strings.Builder
	// openQuote is the quote character of a region left unclosed at end of
	// line, or 0.
	openQuote byte
}

func newWordBuilder() *wordBuilder {
	return &wordBuilder{}
}

// push appends one character to both the value and the raw text.
func (w *wordBuilder) push(value byte, raw byte) {
	w.value.WriteByte(value)
	w.raw.WriteByte(raw)
}

// pushRaw appends a character to the raw text only (quote delimiters).
func (w *wordBuilder) pushRaw(raw byte) {
	w.raw.WriteByte(raw)
}

// pushValue appends a character to the value text only (escaped characters).
func (w *wordBuilder) pushValue(value byte) {
	w.value.WriteByte(value)
}

// rawEmpty reports whether no source characters were consumed yet.
func (w *wordBuilder) rawEmpty() bool {
	return w.raw.Len() == 0
}

// rawText returns the raw source text consumed so far.
func (w *wordBuilder) rawText() string {
	return w.raw.String()
}

// token materializes the word token.
func (w *wordBuilder) token() Token {
	return Token{Kind: Word, Value: w.value.String(), Raw: w.raw.String(), OpenQuote: w.openQuote}
}

// scanSingleQuote consumes a single-quoted region: every character is
// literal until the closing quote; an unclosed quote runs to end of line.
func (w *wordBuilder) scanSingleQuote(scan *lexer) {
	w.pushRaw('\'')

	scan.pos++

	for scan.pos < len(scan.source) {
		if scan.source[scan.pos] == '\'' {
			w.pushRaw('\'')

			scan.pos++

			return
		}

		w.push(scan.source[scan.pos], scan.source[scan.pos])
		scan.pos++
	}

	w.openQuote = '\''
}

// scanDoubleQuote consumes a double-quoted region: backslash escapes '"',
// '\', '$', '`', and newline (line continuation) inside; an unclosed quote
// runs to end of line.
func (w *wordBuilder) scanDoubleQuote(scan *lexer) {
	w.pushRaw('"')

	scan.pos++

	for scan.pos < len(scan.source) {
		switch char := scan.source[scan.pos]; char {
		case '"':
			w.pushRaw('"')

			scan.pos++

			return
		case '\\':
			if scan.pos+1 < len(scan.source) && isEscapableInDoubleQuote(scan.source[scan.pos+1]) {
				escaped := scan.source[scan.pos+1]
				if escaped != '\n' {
					w.pushValue(escaped)
				}

				w.pushRaw('\\')
				w.pushRaw(escaped)

				scan.pos += 2

				continue
			}

			w.push(char, char)

			scan.pos++
		default:
			w.push(char, char)

			scan.pos++
		}
	}

	w.openQuote = '"'
}

// scanEscape consumes a backslash escape outside quotes: the next character
// joins the word literally; a backslash-newline is a line continuation.
func (w *wordBuilder) scanEscape(scan *lexer) {
	if scan.pos+1 >= len(scan.source) {
		w.push('\\', '\\')

		scan.pos++

		return
	}

	next := scan.source[scan.pos+1]
	if next != '\n' {
		w.pushValue(next)
	}

	w.pushRaw('\\')
	w.pushRaw(next)

	scan.pos += 2
}

// lexLine scans the full line into tokens and reports whether the line ends
// with separating whitespace: whitespace outside any word, so neither quoted
// nor escaped. A comment keeps the previous rule of looking at the last
// character, since its text is not lexed.
func lexLine(line string) ([]Token, bool) {
	scan := &lexer{source: line}
	trailingSeparator := false

	for scan.pos < len(scan.source) {
		switch r := scan.source[scan.pos]; {
		case isWhitespace(r):
			scan.pos++
			trailingSeparator = true
		case r == '#':
			scan.pos = len(scan.source)
			trailingSeparator = isWhitespace(line[len(line)-1])
		default:
			scan.scanWord()

			trailingSeparator = false
		}
	}

	return scan.tokens, trailingSeparator
}

// isOperator reports whether r starts a redirection, pipeline, or list
// operator.
func isOperator(r byte) bool {
	switch r {
	case '<', '>', '|', ';', '&':
		return true
	default:
		return false
	}
}

// allDigits reports whether text consists solely of ASCII digits.
func allDigits(text string) bool {
	if text == "" {
		return false
	}

	for i := range len(text) {
		if text[i] < '0' || text[i] > '9' {
			return false
		}
	}

	return true
}

// isWhitespace reports whether r is a shell whitespace character.
func isWhitespace(r byte) bool {
	return r == ' ' || r == '\t' || r == '\r' || r == '\n'
}

// isEscapableInDoubleQuote reports whether backslash escapes c inside double
// quotes (POSIX: '"', '\', '$', '`', newline).
func isEscapableInDoubleQuote(c byte) bool {
	switch c {
	case '"', '\\', '$', '`', '\n':
		return true
	default:
		return false
	}
}

// writeEscaped appends the character a backslash escaped to value, dropping a
// newline: a backslash-newline is a line continuation, as in the lexer's
// scanDoubleQuote and scanEscape.
func writeEscaped(value *strings.Builder, char byte) {
	if char != '\n' {
		value.WriteByte(char)
	}
}
