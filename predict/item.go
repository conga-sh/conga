package predict

// Item represents a single completion candidate with optional description, display label, and styling.
type Item struct {
	// Value is the string inserted into the command line.
	Value string

	// Display is the menu label. Leaving it empty is supported: Items
	// normalizes it to Value at construction, and the wire layer resolves an
	// empty Display to Value when serializing candidates.
	Display string

	// Description is the optional help text shown next to the candidate.
	Description string

	// Style is the optional style token string from the internal style
	// vocabulary (e.g. blue). Empty leaves styling to the shell theme.
	Style string

	// Tag is the optional grouping tag (e.g. commands, flags) used by
	// tag-aware shells to group candidates in the menu.
	Tag string

	// Alias marks the candidate as an alternative spelling of another
	// candidate: a subcommand or flag alias declared on the command schema, or
	// a domain alias (such as a taskfile task alias) supplied by a predictor.
	// Aliases are offered by default and can be suppressed by the user with
	// the application-prefixed completion aliases sideband (<APP>_COMPLETION_ALIASES).
	Alias bool
}

// NewItem constructs a new Item candidate with the given value and description.
func NewItem(value, description string) Item {
	return Item{
		Value:       value,
		Display:     value,
		Description: description,
	}
}

// WithAlias returns a copy of the Item marked as an alias of another
// candidate. Predictors completing domain aliases (for example taskfile task
// aliases) use this so the user can suppress them with the completion aliases
// sideband.
func (i Item) WithAlias() Item {
	i.Alias = true

	return i
}

// WithDescription returns a copy of the Item with the specified description.
func (i Item) WithDescription(description string) Item {
	i.Description = description

	return i
}

// WithDisplay returns a copy of the Item with the specified display label.
func (i Item) WithDisplay(display string) Item {
	i.Display = display

	return i
}

// WithStyle returns a copy of the Item with the specified style applied.
func (i Item) WithStyle(style string) Item {
	i.Style = style

	return i
}

// WithTag returns a copy of the Item with the specified group tag.
func (i Item) WithTag(tag string) Item {
	i.Tag = tag

	return i
}
