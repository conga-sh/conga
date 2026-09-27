package style

// Theme is the active completion palette. Drivers read styles from it when
// rendering candidates; its fields may be overridden to customize colors.
//
//nolint:gochecknoglobals // single active palette instance overridable for palette customization
var Theme = Palette{
	Value:       Default,
	Description: Dim,
	Error:       "bold red",
	Usage:       Dim,

	Highlights: [12]string{
		Blue,
		Yellow,
		Magenta,
		Cyan,
		Green,
		"dim blue",
		"dim yellow",
		"dim magenta",
		"dim cyan",
		"dim green",
		Bold,
		"dim bold",
	},

	FlagArg:      Blue,
	FlagMultiArg: Magenta,
	FlagNoArg:    Default,
	FlagOptArg:   Yellow,
}

// Palette defines the color palette for completion rendering. All fields hold
// style token strings from the package vocabulary.
type Palette struct {
	// Value is the default style for values.
	Value string
	// Description is the default style for descriptions.
	Description string
	// Error is the default style for errors.
	Error string
	// Usage is the default style for usage messages.
	Usage string

	// Highlights styles grouped candidate categories by depth (index 0-11).
	Highlights [12]string

	// FlagArg styles flags with an argument.
	FlagArg string
	// FlagMultiArg styles flags with multiple arguments.
	FlagMultiArg string
	// FlagNoArg styles flags without an argument.
	FlagNoArg string
	// FlagOptArg styles flags with an optional argument.
	FlagOptArg string
}

// Highlight returns the configured style for a given highlight depth level
// (0–11). Out-of-range levels reset to standard terminal styling.
func (p Palette) Highlight(level int) string {
	if level < 0 || level >= len(p.Highlights) {
		return Default
	}

	return p.Highlights[level]
}
