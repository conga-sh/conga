// Package errlines reads the pieces CONGA prints below an error's first line
// and renders them: the origins ("from:"), the note and the hint. Each piece
// comes from methods an error in the chain implements structurally
// (Origins() []string, Note() string, Hint() string), so root conga and the
// internal packages that cannot import it share one lookup.
//
// The lookup is the chain rule: each piece is taken from the outermost error
// in the chain that returns a non-empty value, in errors.As traversal order.
// The package has no package-level state, so linking it adds no initializer.
package errlines

import (
	"strings"

	"github.com/conga-sh/conga/internal/jsonwire"
)

// Labels of the lines below an error's first line.
const (
	LabelOrigin = "from:"
	LabelNote   = "note:"
	LabelHint   = "hint:"
)

// ANSI sequences of the colored layout.
const (
	ansiReset    = "\x1b[0m"
	ansiBoldRed  = "\x1b[1;31m"
	ansiDim      = "\x1b[2m"
	ansiBoldCyan = "\x1b[1;36m"
)

// Line is one labeled line below an error's first line.
type Line struct {
	// Label is one of the Label constants.
	Label string
	// Text is the text after the label.
	Text string
}

// Origins returns the outermost non-empty Origins() in err's chain, or nil.
func Origins(err error) []string {
	var found []string

	walk(err, func(node error) bool {
		if provider, ok := node.(originer); ok {
			if origins := provider.Origins(); len(origins) > 0 {
				found = origins

				return true
			}
		}

		return false
	})

	return found
}

// Note returns the outermost non-empty Note() in err's chain, or "".
func Note(err error) string {
	var found string

	walk(err, func(node error) bool {
		if provider, ok := node.(noter); ok {
			if note := provider.Note(); note != "" {
				found = note

				return true
			}
		}

		return false
	})

	return found
}

// Hint returns the outermost non-empty Hint() in err's chain, or "".
func Hint(err error) string {
	var found string

	walk(err, func(node error) bool {
		if provider, ok := node.(hinter); ok {
			if hint := provider.Hint(); hint != "" {
				found = hint

				return true
			}
		}

		return false
	})

	return found
}

// Lines returns the labeled lines of err: one per origin, then the note, then
// the hint. An error whose Error() is empty has no lines.
func Lines(err error) []Line {
	if err == nil || err.Error() == "" {
		return nil
	}

	var lines []Line

	for _, origin := range Origins(err) {
		lines = append(lines, Line{Label: LabelOrigin, Text: origin})
	}

	if note := Note(err); note != "" {
		lines = append(lines, Line{Label: LabelNote, Text: note})
	}

	if hint := Hint(err); hint != "" {
		lines = append(lines, Line{Label: LabelHint, Text: hint})
	}

	return lines
}

// Members splits err into the errors that render separately. An error with
// Unwrap() []error whose Error() is exactly its members' texts joined by
// newlines (the errors.Join shape) yields its members, nested ones flattened
// and nil ones skipped. Any other error is its own single member. A nil error
// has none.
func Members(err error) []error {
	if err == nil {
		return nil
	}

	members := flatten(err, nil)
	if len(members) == 0 {
		return []error{err}
	}

	return members
}

// Render writes each member as "<prefix>error: <first line>" followed by its
// indented lines, one member after another, each line ending in a newline. A
// member with an empty Error() prints nothing. With color, the labels are
// styled: error bold red, from and note dim, hint bold cyan.
func Render(members []error, prefix string, color bool) string {
	var builder strings.Builder

	for _, member := range members {
		if member == nil || member.Error() == "" {
			continue
		}

		builder.WriteString(prefix)
		builder.WriteString(styled("error:", ansiBoldRed, color))
		builder.WriteString(" " + member.Error() + "\n")

		for _, line := range Lines(member) {
			label := styled(line.Label, labelStyle(line.Label), color)
			builder.WriteString("  " + label + " " + line.Text + "\n")
		}
	}

	return builder.String()
}

// Plain renders the members as their first lines followed by their indented
// lines, without a prefix or color and with no trailing newline. Members with an
// empty Error() are skipped. It is the form %+v prints.
func Plain(members []error) string {
	var (
		builder strings.Builder
		written bool
	)

	for _, member := range members {
		if member == nil || member.Error() == "" {
			continue
		}

		if written {
			builder.WriteString("\n")
		}

		written = true

		builder.WriteString(member.Error())

		for _, line := range Lines(member) {
			builder.WriteString("\n  " + line.Label + " " + line.Text)
		}
	}

	return builder.String()
}

// AppendJSON appends the JSON form of members to dst: an object for the first
// non-empty member, {"error": <line 1>, "from": [...], "note": "...", "hint":
// "..."}, with every key but "error" omitted when empty, and "more": [...]
// holding the objects of the other non-empty members when there are several.
func AppendJSON(dst []byte, members []error) []byte {
	firstIndex := -1

	for index, member := range members {
		if member != nil && member.Error() != "" {
			firstIndex = index

			break
		}
	}

	if firstIndex < 0 {
		return append(dst, `{"error":""}`...)
	}

	dst = appendMemberJSON(dst, members[firstIndex])

	hasMore := false

	for _, member := range members[firstIndex+1:] {
		if member == nil || member.Error() == "" {
			continue
		}

		if !hasMore {
			dst = dst[:len(dst)-1]
			dst = append(dst, `,"more":[`...)
			hasMore = true
		} else {
			dst = append(dst, ',')
		}

		dst = appendMemberJSON(dst, member)
	}

	if hasMore {
		return append(dst, ']', '}')
	}

	return dst
}

// AppendText appends the one-line text form of members to dst:
// "<line 1> (from: <origin>; <origin>; note: ...; hint: ...)" with absent
// parts left out and no parentheses when nothing follows line 1. Members with
// an empty Error() are skipped, and several members are joined with " | ".
func AppendText(dst []byte, members []error) []byte {
	written := false

	for _, member := range members {
		if member == nil || member.Error() == "" {
			continue
		}

		if written {
			dst = append(dst, " | "...)
		}

		written = true

		dst = append(dst, member.Error()...)

		var parts []string

		if origins := Origins(member); len(origins) > 0 {
			parts = append(parts, LabelOrigin+" "+strings.Join(origins, "; "))
		}

		if note := Note(member); note != "" {
			parts = append(parts, LabelNote+" "+note)
		}

		if hint := Hint(member); hint != "" {
			parts = append(parts, LabelHint+" "+hint)
		}

		if len(parts) == 0 {
			continue
		}

		dst = append(dst, " ("...)
		dst = append(dst, strings.Join(parts, "; ")...)
		dst = append(dst, ')')
	}

	return dst
}

type originer interface{ Origins() []string }

type noter interface{ Note() string }

type hinter interface{ Hint() string }

func appendMemberJSON(dst []byte, member error) []byte {
	text := member.Error()

	dst = append(dst, `{"error":`...)
	dst = jsonwire.AppendString(dst, text)

	if text == "" {
		return append(dst, '}')
	}

	if origins := Origins(member); len(origins) > 0 {
		dst = append(dst, `,"from":[`...)

		for index, origin := range origins {
			if index > 0 {
				dst = append(dst, ',')
			}

			dst = jsonwire.AppendString(dst, origin)
		}

		dst = append(dst, ']')
	}

	if note := Note(member); note != "" {
		dst = append(dst, `,"note":`...)
		dst = jsonwire.AppendString(dst, note)
	}

	if hint := Hint(member); hint != "" {
		dst = append(dst, `,"hint":`...)
		dst = jsonwire.AppendString(dst, hint)
	}

	return append(dst, '}')
}

func labelStyle(label string) string {
	if label == LabelHint {
		return ansiBoldCyan
	}

	return ansiDim
}

func styled(text, escape string, color bool) string {
	if !color {
		return text
	}

	return escape + text + ansiReset
}

// flatten appends the members of err to members.
func flatten(err error, members []error) []error {
	joined, ok := err.(interface{ Unwrap() []error })
	if !ok {
		return append(members, err)
	}

	var (
		children []error
		texts    []string
	)

	for _, child := range joined.Unwrap() {
		if child == nil {
			continue
		}

		children = append(children, child)
		texts = append(texts, child.Error())
	}

	if len(children) == 0 || strings.Join(texts, "\n") != err.Error() {
		return append(members, err)
	}

	for _, child := range children {
		members = flatten(child, members)
	}

	return members
}

// walk visits err and the errors it wraps depth first, in errors.As order,
// until visit returns true.
func walk(err error, visit func(error) bool) bool {
	for err != nil {
		if visit(err) {
			return true
		}

		switch wrapper := err.(type) {
		case interface{ Unwrap() error }:
			err = wrapper.Unwrap()
		case interface{ Unwrap() []error }:
			for _, child := range wrapper.Unwrap() {
				if walk(child, visit) {
					return true
				}
			}

			return false
		default:
			return false
		}
	}

	return false
}
