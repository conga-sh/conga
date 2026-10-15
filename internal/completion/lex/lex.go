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

// WordbreakPrefix returns the prefix of the last word's raw text up to and
// including its last wordbreak rune (e.g. `--env=` for the current word
// `--env=`, `--env="` for `--env="pro`). wordbreaks is the rune set,
// normally the COMP_WORDBREAKS value. The prefix is empty when the line ends
// with an operator or the last word contains no wordbreak rune.
func (l Line) WordbreakPrefix(wordbreaks string) string {
	if len(l.Tokens) == 0 || l.Tokens[len(l.Tokens)-1].Kind != Word {
		return ""
	}

	raw := l.Tokens[len(l.Tokens)-1].Raw

	return LastWordbreakPrefix(raw, wordbreaks)
}

// Parse lexes the cursor-truncated command line into tokens. Unclosed quotes
// are tolerated (the token runs to end of line); a line that ends with
// whitespace (or is empty) yields a trailing empty Word token. An unquoted
// '#' at the start of a word begins a comment that consumes the rest of the
// line.
func Parse(line string) Line {
	tokens := lexLine(line)

	if endsInWhitespace(line) {
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
	return Token{Kind: Word, Value: w.value.String(), Raw: w.raw.String()}
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

// endsInWhitespace reports whether the line is empty or ends with a space,
// tab, carriage return, or newline.
func endsInWhitespace(line string) bool {
	if line == "" {
		return true
	}

	switch line[len(line)-1] {
	case ' ', '\t', '\r', '\n':
		return true
	default:
		return false
	}
}

// lexLine scans the full line into tokens.
func lexLine(line string) []Token {
	scan := &lexer{source: line}

	for scan.pos < len(scan.source) {
		switch r := scan.source[scan.pos]; {
		case isWhitespace(r):
			scan.pos++
		case r == '#':
			scan.pos = len(scan.source)
		default:
			scan.scanWord()
		}
	}

	return scan.tokens
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
