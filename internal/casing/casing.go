package casing

import (
	"strings"
	"unicode"
)

// Kebab converts a CamelCase, PascalCase, snake_case, or dotted string into
// kebab-case.
func Kebab(s string) string {
	return joinWords(s, "-", unicode.ToLower)
}

// Snake converts a CamelCase, PascalCase, kebab-case, or dotted string into
// snake_case.
func Snake(s string) string {
	return joinWords(s, "_", unicode.ToLower)
}

// UpperSnake converts a CamelCase, PascalCase, kebab-case, or dotted string
// into UPPER_SNAKE_CASE.
func UpperSnake(s string) string {
	return joinWords(s, "_", unicode.ToUpper)
}

// UppercaseWords converts a CamelCase word into uppercase, dash-separated
// tokens, grouping acronyms (HTTPMethod -> HTTP-METHOD, ExistingFile ->
// EXISTING-FILE).
func UppercaseWords(s string) string {
	return joinWords(s, "-", unicode.ToUpper)
}

// joinWords splits s into lowercase-delimited word tokens, treating '-', '_',
// and '.' as separators and inserting a separator before an upper-case rune
// that follows a lower-case rune or precedes a lower-case rune (which groups
// acronyms: APIKey -> api-key). Every emitted rune is passed through transform
// in a single pass, so the case conversion is applied exactly once.
//
// Known limitations of the boundary heuristic:
//   - a trailing upper-case rune after a digit is not split (version2B ->
//     version2b);
//   - an acronym followed by a lowercase suffix is split (IPv4 -> i-pv4,
//     OAuth2Client -> o-auth2-client).
func joinWords(s string, sep string, transform func(rune) rune) string {
	var builder strings.Builder
	builder.Grow(len(s) + len(s)/2)

	pending := false
	runes := []rune(s)

	for i, r := range runes {
		if r == '-' || r == '_' || r == '.' {
			if builder.Len() > 0 {
				pending = true
			}

			continue
		}

		if unicode.IsUpper(r) {
			if i > 0 && (unicode.IsLower(runes[i-1]) || (i+1 < len(runes) && unicode.IsLower(runes[i+1]))) {
				if builder.Len() > 0 {
					pending = true
				}
			}
		}

		if pending {
			builder.WriteString(sep)

			pending = false
		}

		builder.WriteRune(transform(r))
	}

	return builder.String()
}
