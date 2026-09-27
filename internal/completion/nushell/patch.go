package nushell

import (
	"strings"

	"github.com/conga-sh/conga/internal/completion/lex"
)

// Patch dequotes nushell's raw completion spans: nushell passes completed
// positions still quoted, while the engine expects resolved tokens.
// Double- and single-quoted spans are
// re-lexed to their first quote-resolved word (only when the leading quote
// is closed — an unterminated span is the word being completed and passes
// through raw); backtick-wrapped spans lose their backticks; every other
// word passes through unchanged.
func Patch(args []string) []string {
	out := make([]string, len(args))
	copy(out, args)

	for index, arg := range out {
		if arg == "" {
			continue
		}

		switch arg[0] {
		case '"', '\'':
			if !strings.Contains(arg[1:], string(arg[0])) {
				continue
			}

			if words := lex.Parse(arg).Words(); len(words) > 0 {
				out[index] = words[0]
			}
		case '`':
			out[index] = strings.Trim(arg, "`")
		}
	}

	return out
}
