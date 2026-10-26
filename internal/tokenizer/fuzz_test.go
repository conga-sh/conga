package tokenizer_test

import (
	"strings"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/completion/lex"
	"github.com/conga-sh/conga/internal/tokenizer"
)

func FuzzTokenizer(f *testing.F) {
	// Seed corpus with POSIX/GNU patterns and edge cases. Inputs are
	// NUL-separated argument lists, so individual arguments may contain
	// whitespace, quotes, and unicode.
	f.Add("-xzvf --port=8080 arg1 -- -f")
	f.Add("--host=localhost:9090 -v --dry-run")
	f.Add("-")
	f.Add("--")
	f.Add("---")
	f.Add("--=")
	f.Add("-p=8080")
	f.Add("-10 -3.14 -- -5")
	f.Add("--emoji=🚀 -🔥 简体中文")
	f.Add("")
	f.Add("a b c d e f g")
	f.Add(`it's --x='a b' "unterminated back\slash`)
	f.Add("multi\nline\ttab  spaces")
	f.Add("a\x00b c\x00'dq'\x00\"dq2\"")

	f.Fuzz(func(t *testing.T, input string) {
		args := strings.Split(input, "\x00")

		// Scanner traversal: every argument must survive Tokenize exactly
		// once, in order, with consistent indices, and token formatting or
		// classification must never panic.
		scanner := tokenizer.NewScanner(tokenizer.Tokenize(args))
		count := 0

		for scanner.HasNext() {
			tok, ok := scanner.Next()
			assert.True(t, ok)
			assert.Equal(t, count, tok.Index)
			assert.Equal(t, args[count], tok.Raw)

			_ = tok.String()
			_ = tok.IsNegativeNumber()

			count++
		}

		assert.Equal(t, len(args), count)

		// Round-trip: POSIX-quoting every argument must survive
		// WordsUpTo byte-for-byte, whatever the argument contents.
		line := quotedCompLine(args)

		assert.Equal(t, args, lex.WordsUpTo(line, len(line)))
	})
}

// quotedCompLine renders args as a single POSIX shell command line in which
// every argument is single-quoted with embedded single quotes escaped.
func quotedCompLine(args []string) string {
	var builder strings.Builder

	for i, arg := range args {
		if i > 0 {
			builder.WriteByte(' ')
		}

		builder.WriteByte('\'')
		builder.WriteString(strings.ReplaceAll(arg, "'", `'\''`))
		builder.WriteByte('\'')
	}

	return builder.String()
}
