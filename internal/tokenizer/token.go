package tokenizer

import "fmt"

const (
	// TokenTypeUnknown represents an unclassified token.
	TokenTypeUnknown TokenType = iota

	// TokenTypePositional represents a positional argument / operand (e.g., "build", "file.txt", or "-" for stdin).
	TokenTypePositional

	// TokenTypeLongFlag represents a long option starting with "--" (e.g., "--verbose" or "--port=8080").
	TokenTypeLongFlag

	// TokenTypeShortFlag represents one or more short options starting with "-" (e.g., "-v", "-xzvf", or "-p8080").
	TokenTypeShortFlag

	// TokenTypeTerminator represents the explicit "--" option terminator.
	TokenTypeTerminator
)

// TokenType represents the syntactic category of a command-line token.
type TokenType int

// String returns a human-readable name for the token type.
func (t TokenType) String() string {
	switch t {
	case TokenTypePositional:
		return "Positional"
	case TokenTypeLongFlag:
		return "LongFlag"
	case TokenTypeShortFlag:
		return "ShortFlag"
	case TokenTypeTerminator:
		return "Terminator"
	default:
		return "Unknown"
	}
}

// Token represents a single parsed syntactic unit from os.Args.
type Token struct {
	// Type is the classification of this token.
	Type TokenType

	// Raw is the original, unmodified argument string as provided in os.Args.
	Raw string

	// Name is the parsed name for long flags (e.g. "port" for "--port=8080").
	// For single short flags, it is the string representation of the rune (e.g. "v" for "-v").
	Name string

	// Value contains the attached value if provided with "=" or attached to a short flag.
	// For example, in "--port=8080" Value is "8080". In "-p8080" Value is "8080".
	Value string

	// HasAttachedValue indicates if a value was directly attached via "=" or short flag suffix.
	HasAttachedValue bool

	// ShortRunes contains all runes in a short flag cluster (e.g. ['x', 'z', 'v', 'f'] for "-xzvf").
	ShortRunes []rune

	// Index is the zero-based index of this token in the input args slice.
	Index int
}

// String provides a formatted representation of the token for debugging.
func (t Token) String() string {
	switch t.Type {
	case TokenTypeLongFlag:
		if t.HasAttachedValue {
			return fmt.Sprintf("LongFlag(--%s=%q)", t.Name, t.Value)
		}

		return fmt.Sprintf("LongFlag(--%s)", t.Name)
	case TokenTypeShortFlag:
		if len(t.ShortRunes) > 1 {
			return fmt.Sprintf("ShortBundle(-%s)", string(t.ShortRunes))
		}

		if t.HasAttachedValue {
			return fmt.Sprintf("ShortFlag(-%s=%q)", t.Name, t.Value)
		}

		return fmt.Sprintf("ShortFlag(-%s)", t.Name)
	case TokenTypePositional:
		return fmt.Sprintf("Positional(%q)", t.Raw)
	case TokenTypeTerminator:
		return "Terminator(--)"
	default:
		return fmt.Sprintf("Unknown(%q)", t.Raw)
	}
}
