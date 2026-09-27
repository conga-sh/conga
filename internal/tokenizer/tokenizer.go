package tokenizer

import (
	"strconv"
	"strings"
)

// Scanner provides a forward-only, peekable cursor over a sequence of tokens.
type Scanner struct {
	tokens []Token
	cursor int
}

// NewScanner initializes a new Scanner with the provided tokens.
func NewScanner(tokens []Token) *Scanner {
	return &Scanner{
		tokens: tokens,
		cursor: 0,
	}
}

// HasNext returns true if there are more tokens remaining to be read.
func (s *Scanner) HasNext() bool {
	return s.cursor < len(s.tokens)
}

// Next consumes and returns the next token, or returns (Token{}, false) if exhausted.
func (s *Scanner) Next() (Token, bool) {
	if !s.HasNext() {
		return Token{}, false
	}

	tok := s.tokens[s.cursor]
	s.cursor++

	return tok, true
}

// Peek returns the next token without advancing the cursor, or returns (Token{}, false) if exhausted.
func (s *Scanner) Peek() (Token, bool) {
	if !s.HasNext() {
		return Token{}, false
	}

	return s.tokens[s.cursor], true
}

// Tokenize converts a slice of command-line arguments (typically os.Args[1:])
// into a slice of structured Token objects conforming to POSIX and GNU CLI conventions.
func Tokenize(args []string) []Token {
	tokens := make([]Token, 0, len(args))
	inTerminatedState := false

	for i, arg := range args {
		if inTerminatedState || arg == "" || arg == "-" {
			tokens = append(tokens, Token{
				Type:  TokenTypePositional,
				Raw:   arg,
				Index: i,
			})

			continue
		}

		if arg == "--" {
			tokens = append(tokens, Token{
				Type:  TokenTypeTerminator,
				Raw:   arg,
				Index: i,
			})
			inTerminatedState = true

			continue
		}

		if strings.HasPrefix(arg, "--") {
			tokens = append(tokens, tokenizeLongFlag(arg, i))

			continue
		}

		if strings.HasPrefix(arg, "-") {
			tokens = append(tokens, tokenizeShortFlag(arg, i))

			continue
		}

		tokens = append(tokens, Token{
			Type:  TokenTypePositional,
			Raw:   arg,
			Index: i,
		})
	}

	return tokens
}

// IsNegativeNumber checks if a token's raw representation corresponds to a numeric literal (e.g. "-10", "-3.14", "-1e-3").
func (t Token) IsNegativeNumber() bool {
	if !strings.HasPrefix(t.Raw, "-") || len(t.Raw) < 2 {
		return false
	}

	if _, err := strconv.ParseFloat(t.Raw, 64); err == nil {
		return true
	}

	if strings.HasPrefix(t.Raw, "-0x") || strings.HasPrefix(t.Raw, "-0X") ||
		strings.HasPrefix(t.Raw, "-0o") || strings.HasPrefix(t.Raw, "-0O") ||
		strings.HasPrefix(t.Raw, "-0b") || strings.HasPrefix(t.Raw, "-0B") {
		if _, err := strconv.ParseInt(t.Raw, 0, 64); err == nil {
			return true
		}
	}

	return false
}

func tokenizeLongFlag(arg string, index int) Token {
	nameAndValue := arg[2:]
	before, after, ok := strings.Cut(nameAndValue, "=")

	if ok {
		return Token{
			Type:             TokenTypeLongFlag,
			Raw:              arg,
			Name:             before,
			Value:            after,
			HasAttachedValue: true,
			Index:            index,
		}
	}

	return Token{
		Type:             TokenTypeLongFlag,
		Raw:              arg,
		Name:             nameAndValue,
		Value:            "",
		HasAttachedValue: false,
		Index:            index,
	}
}

func tokenizeShortFlag(arg string, index int) Token {
	body := arg[1:]
	before, after, ok := strings.Cut(body, "=")

	if ok {
		return Token{
			Type:             TokenTypeShortFlag,
			Raw:              arg,
			Name:             before,
			Value:            after,
			HasAttachedValue: true,
			ShortRunes:       []rune(before),
			Index:            index,
		}
	}

	runes := []rune(body)

	var firstName string
	if len(runes) > 0 {
		firstName = string(runes[0])
	}

	return Token{
		Type:             TokenTypeShortFlag,
		Raw:              arg,
		Name:             firstName,
		Value:            "",
		HasAttachedValue: false,
		ShortRunes:       runes,
		Index:            index,
	}
}
