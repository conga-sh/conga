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
	// vocabulary (e.g. "blue"). Empty leaves styling to the shell theme.
	Style string

	// Tag is the optional grouping tag (e.g. "commands", "flags") used by
	// tag-aware shells to group candidates in the menu.
	Tag string
}

// NewItem constructs a new Item candidate with the given value and description.
func NewItem(value, description string) Item {
	return Item{
		Value:       value,
		Display:     value,
		Description: description,
	}
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
