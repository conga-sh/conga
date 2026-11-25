// Package errlines reads the pieces CONGA prints below an error's first line
// and renders them: the origins ("from:"), the note and the hint. Each piece
// comes from methods an error in the chain implements structurally
// (Origins() []string, Note() string, Hint() string), so root conga and the
// internal packages that cannot import it share one lookup.
//
// The lookup is the chain rule: each piece is taken from the outermost error
// in the chain that returns a non-empty value, in errors.As traversal order.
//
// Members splits an error into the members every caller reads and renders. It
// drops nil members and members whose Error() is empty, and returns nil when
// none is left, so Origins, Note, Hint, Plain, AppendJSON and AppendText need
// no guards of their own. Render keeps a guard because it takes an unfiltered
// slice. The labels and the line renderers behind Render are unexported.
//
// The package has no package-level state, so linking it adds no initializer.
package errlines

import (
	"strings"

	"github.com/conga-sh/conga/internal/jsonwire"
)

// Labels of the lines below an error's first line.
const (
	labelOrigin = "from:"
	labelNote   = "note:"
	labelHint   = "hint:"
)

// ANSI sequences of the colored layout.
const (
	ansiReset    = "\x1b[0m"
	ansiBoldRed  = "\x1b[1;31m"
	ansiDim      = "\x1b[2m"
	ansiBoldCyan = "\x1b[1;36m"
)

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

// Members splits err into the errors that render separately. An error with
// Unwrap() []error whose Error() is exactly its members' texts joined by
// newlines (the errors.Join shape) yields its members, nested ones flattened
// and nil ones skipped. Any other error is its own single member. Members with
// an empty Error() are dropped. A nil error, or one with nothing left, has none.
func Members(err error) []error {
	if err == nil {
		return nil
	}

	var members []error

	for _, leaf := range flatten(err, nil) {
		if leaf != nil && leaf.Error() != "" {
			members = append(members, leaf)
		}
	}

	return members
}

// Render writes each member as "<prefix>error: <first line>" followed by its
// indented lines, one member after another, each line ending in a newline. It
// skips nil members and members with an empty Error(), so a caller passing an
// unfiltered slice prints only the others. With color, the labels are
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

		for _, line := range lines(member) {
			label := styled(line.label, labelStyle(line.label), color)
			builder.WriteString("  " + label + " " + line.text + "\n")
		}
	}

	return builder.String()
}

// Plain renders the members as their first lines followed by their indented
// lines, without a prefix or color and with no trailing newline. members come
// from Members. It is the form %+v prints.
func Plain(members []error) string {
	var builder strings.Builder

	for index, member := range members {
		if index > 0 {
			builder.WriteString("\n")
		}

		builder.WriteString(member.Error())

		for _, line := range lines(member) {
			builder.WriteString("\n  " + line.label + " " + line.text)
		}
	}

	return builder.String()
}

// AppendJSON appends the JSON form of members to dst: an object for the first
// member, {"error": <line 1>, "from": [...], "note": "...", "hint": "..."},
// with every key but "error" omitted when empty, and "more": [...] holding the
// objects of the other members when there are several. members come from
// Members. An empty list is written as {"error":""}.
func AppendJSON(dst []byte, members []error) []byte {
	if len(members) == 0 {
		return append(dst, `{"error":""}`...)
	}

	dst = appendMemberJSON(dst, members[0])

	if len(members) == 1 {
		return dst
	}

	dst = dst[:len(dst)-1]
	dst = append(dst, `,"more":[`...)

	for index, member := range members[1:] {
		if index > 0 {
			dst = append(dst, ',')
		}

		dst = appendMemberJSON(dst, member)
	}

	return append(dst, ']', '}')
}

// AppendText appends the one-line text form of members to dst:
// "<line 1> (from: <origin>; <origin>; note: ...; hint: ...)" with absent
// parts left out and no parentheses when nothing follows line 1. members come
// from Members, and several are joined with " | ".
func AppendText(dst []byte, members []error) []byte {
	for index, member := range members {
		if index > 0 {
			dst = append(dst, " | "...)
		}

		dst = append(dst, member.Error()...)

		var parts []string

		if origins := Origins(member); len(origins) > 0 {
			parts = append(parts, labelOrigin+" "+strings.Join(origins, "; "))
		}

		if note := Note(member); note != "" {
			parts = append(parts, labelNote+" "+note)
		}

		if hint := Hint(member); hint != "" {
			parts = append(parts, labelHint+" "+hint)
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

// line is one labeled line below an error's first line.
type line struct {
	label string
	text  string
}

func appendMemberJSON(dst []byte, member error) []byte {
	dst = append(dst, `{"error":`...)
	dst = jsonwire.AppendString(dst, member.Error())

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

// lines returns the labeled lines of err, one per origin, then the note, then
// the hint.
func lines(err error) []line {
	var result []line

	for _, origin := range Origins(err) {
		result = append(result, line{label: labelOrigin, text: origin})
	}

	if note := Note(err); note != "" {
		result = append(result, line{label: labelNote, text: note})
	}

	if hint := Hint(err); hint != "" {
		result = append(result, line{label: labelHint, text: hint})
	}

	return result
}

func labelStyle(label string) string {
	if label == labelHint {
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

		switch wrapper := err.(type) { //nolint:errorlint // walk steps one Unwrap level at a time
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
