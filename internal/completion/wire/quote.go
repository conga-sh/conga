package wire

import (
	"os"
	"path/filepath"
	"strings"
)

// PowerShellSingleQuotes lists the characters PowerShell treats as a single
// quote: ASCII ' and U+2018–U+201B.
const PowerShellSingleQuotes = "'\u2018\u2019\u201a\u201b"

// posixSafePunctuation lists the non-alphanumeric characters that need no
// quoting in a POSIX shell word.
const posixSafePunctuation = "-_/.:,@"

// QuotePOSIX quotes s for safe evaluation in POSIX-compatible shells (bash,
// zsh): the string is enclosed in single quotes, in which every character is
// literal, with embedded single quotes escaped.
func QuotePOSIX(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// QuoteFish quotes s for safe evaluation in fish: single quotes preserve every
// character except backslashes and single quotes, which are escaped with a backslash.
func QuoteFish(s string) string {
	escaped := strings.ReplaceAll(s, `\`, `\\`)

	return "'" + strings.ReplaceAll(escaped, "'", `\'`) + "'"
}

// QuotePowerShell quotes s as a PowerShell single-quoted string literal, in
// which every character PowerShell reads as a single quote is doubled: the
// ASCII ' and the Unicode quotes U+2018–U+201B (‘ ’ ‚ ‛), any of which would
// otherwise end the literal.
func QuotePowerShell(s string) string {
	if !strings.ContainsAny(s, PowerShellSingleQuotes) {
		return "'" + s + "'"
	}

	var builder strings.Builder

	builder.WriteByte('\'')

	for _, r := range s {
		if strings.ContainsRune(PowerShellSingleQuotes, r) {
			builder.WriteRune(r)
		}

		builder.WriteRune(r)
	}

	builder.WriteByte('\'')

	return builder.String()
}

// QuoteElvish quotes s for safe evaluation in elvish: single quotes preserve
// every character except the single quote itself, which is doubled.
func QuoteElvish(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// ExecTarget returns name when set, and otherwise resolves the base name of
// the running executable so generated snippets can reference the binary.
func ExecTarget(name string) string {
	if name != "" {
		return name
	}

	if exe, err := os.Executable(); err == nil {
		return filepath.Base(exe)
	}

	if len(os.Args) > 0 && os.Args[0] != "" {
		return filepath.Base(os.Args[0])
	}

	return "app"
}

// SanitizeIdentifier maps a command name onto a conservative identifier for
// generated shell scripts (function, variable, and `let` names): alphanumeric
// and underscore runes are kept, everything else becomes an underscore, and
// an empty result falls back to a single underscore.
func SanitizeIdentifier(s string) string {
	var builder strings.Builder

	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_':
			builder.WriteRune(r)
		default:
			builder.WriteRune('_')
		}
	}

	if builder.Len() == 0 {
		return "_"
	}

	return builder.String()
}

// QuoteTargetPOSIX quotes target for use as a word in a POSIX shell script
// unless it consists only of characters that are literal in an unquoted word
// (alphanumerics and the common path/punctuation set "-_/.:,@"), where it is
// returned unchanged. Quoting is otherwise unconditional: POSIX single quotes
// make every character literal.
func QuoteTargetPOSIX(target string) string {
	if isPOSIXSafe(target) {
		return target
	}

	return QuotePOSIX(target)
}

// QuoteIfUnsafe quotes s through quote when s contains any rune in unsafe,
// and returns it unchanged otherwise. It is the shared quote-only-when-needed
// skeleton for shell bareword emission.
func QuoteIfUnsafe(s string, unsafe string, quote func(string) string) string {
	if strings.ContainsAny(s, unsafe) {
		return quote(s)
	}

	return s
}

// isPOSIXSafe reports whether every rune of s is literal in an unquoted POSIX
// shell word.
func isPOSIXSafe(s string) bool {
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case strings.ContainsRune(posixSafePunctuation, r):
		default:
			return false
		}
	}

	return true
}
