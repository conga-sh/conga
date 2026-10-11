package predict

const (
	// DirectiveError indicates an error occurred and completions should be ignored.
	DirectiveError Directive = 1 << 0

	// DirectiveNoSpace indicates the shell should not append a space after the candidate.
	DirectiveNoSpace Directive = 1 << 1

	// DirectiveNoFileCompletion indicates the shell should not fall back to file completion.
	DirectiveNoFileCompletion Directive = 1 << 2

	// DirectiveKeepOrder indicates the shell should preserve candidate ordering.
	DirectiveKeepOrder Directive = 1 << 3
)

// Directive is a bitmask representing actions the shell completion script should perform.
type Directive int

// Has reports whether the directive contains the specified bitmask flag.
func (d Directive) Has(flag Directive) bool {
	return d&flag == flag
}
