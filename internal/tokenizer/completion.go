package tokenizer

import (
	"strings"
)

// ParseCompLine splits a command line string up to point (the cursor position) into words
// according to POSIX shell quoting and whitespace rules. If the cursor is preceded by whitespace,
// a trailing empty token is included to represent an unstarted argument at the cursor.
func ParseCompLine(line string, point int) []string {
	if point < 0 {
		point = 0
	}

	if point > len(line) {
		point = len(line)
	}

	parser := newCompLineParser(line[:point])

	return parser.parse()
}

type compLineParser struct {
	input         string
	tokens        []string
	curToken      strings.Builder
	inToken       bool
	inSingleQuote bool
	inDoubleQuote bool
	inEscape      bool
}

func newCompLineParser(input string) *compLineParser {
	return &compLineParser{
		input: input,
	}
}

func (p *compLineParser) parse() []string {
	for i := 0; i < len(p.input); i++ {
		char := p.input[i]

		if p.inEscape {
			p.curToken.WriteByte(char)
			p.inEscape = false
			p.inToken = true

			continue
		}

		if p.inSingleQuote {
			p.handleSingleQuote(char)

			continue
		}

		if p.inDoubleQuote {
			i = p.handleDoubleQuote(char, i)

			continue
		}

		p.handleNormalChar(char)
	}

	if p.inToken {
		p.tokens = append(p.tokens, p.curToken.String())
	} else {
		p.tokens = append(p.tokens, "")
	}

	return p.tokens
}

func (p *compLineParser) handleSingleQuote(char byte) {
	if char == '\'' {
		p.inSingleQuote = false
		p.inToken = true
	} else {
		p.curToken.WriteByte(char)
		p.inToken = true
	}
}

func (p *compLineParser) handleDoubleQuote(char byte, index int) int {
	switch char {
	case '\\':
		if index+1 < len(p.input) && isEscapableInDoubleQuote(p.input[index+1]) {
			index++
			p.curToken.WriteByte(p.input[index])
		} else {
			p.curToken.WriteByte(char)
		}

		p.inToken = true
	case '"':
		p.inDoubleQuote = false
		p.inToken = true
	default:
		p.curToken.WriteByte(char)
		p.inToken = true
	}

	return index
}

func (p *compLineParser) handleNormalChar(char byte) {
	switch char {
	case '\\':
		p.inEscape = true
		p.inToken = true
	case '\'':
		p.inSingleQuote = true
		p.inToken = true
	case '"':
		p.inDoubleQuote = true
		p.inToken = true
	case ' ', '\t', '\n', '\r':
		if p.inToken {
			p.tokens = append(p.tokens, p.curToken.String())
			p.curToken.Reset()
			p.inToken = false
		}
	default:
		p.curToken.WriteByte(char)
		p.inToken = true
	}
}

func isEscapableInDoubleQuote(char byte) bool {
	return char == '"' || char == '\\' || char == '$' || char == '`' || char == '\n'
}
