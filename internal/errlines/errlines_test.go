package errlines_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/errlines"
)

// pieceError implements the three structural methods.
type pieceError struct {
	message string
	origins []string
	note    string
	hint    string
	cause   error
}

func (e *pieceError) Error() string     { return e.message }
func (e *pieceError) Origins() []string { return e.origins }
func (e *pieceError) Note() string      { return e.note }
func (e *pieceError) Hint() string      { return e.hint }
func (e *pieceError) Unwrap() error     { return e.cause }

// joinError is a third-party join shape whose members may be nil.
type joinError struct{ members []error }

func (e *joinError) Error() string {
	text := ""

	for _, member := range e.members {
		if member == nil {
			continue
		}

		if text != "" {
			text += "\n"
		}

		text += member.Error()
	}

	return text
}

func (e *joinError) Unwrap() []error { return e.members }

func TestLookup_TakesTheOutermostNonEmptyValue(t *testing.T) {
	t.Parallel()

	inner := &pieceError{message: "inner", origins: []string{"inner origin"}, note: "inner note", hint: "inner hint"}
	outer := &pieceError{message: "outer", hint: "outer hint", cause: inner}

	assert.Equal(t, []string{"inner origin"}, errlines.Origins(outer))
	assert.Equal(t, "inner note", errlines.Note(outer))
	assert.Equal(t, "outer hint", errlines.Hint(outer))
}

func TestLookup_SkipsImplementersThatReturnEmpty(t *testing.T) {
	t.Parallel()

	err := fmt.Errorf("wrapped: %w", &pieceError{message: "empty", cause: &pieceError{message: "inner", note: "n"}})

	assert.Equal(t, "n", errlines.Note(err))
	assert.Equal(t, "", errlines.Hint(err))
	assert.Zero(t, errlines.Origins(err))
}

func TestLookup_WalksJoinMembersInOrder(t *testing.T) {
	t.Parallel()

	err := errors.Join(errors.New("plain"), &pieceError{message: "first", hint: "first hint"}, &pieceError{message: "second", hint: "second hint"})

	assert.Equal(t, "first hint", errlines.Hint(err))
}

func TestLookup_NilHasNoPieces(t *testing.T) {
	t.Parallel()

	assert.Zero(t, errlines.Origins(nil))
	assert.Equal(t, "", errlines.Note(nil))
	assert.Equal(t, "", errlines.Hint(nil))
}

func TestMembers(t *testing.T) {
	t.Parallel()

	first, second, third := errors.New("first"), errors.New("second"), errors.New("third")

	t.Run("a single error is its own member", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, []error{first}, errlines.Members(first))
	})

	t.Run("a join yields its members and flattens nested joins", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, []error{first, second, third}, errlines.Members(errors.Join(errors.Join(first, second), third)))
	})

	t.Run("a wrapped join is one error", func(t *testing.T) {
		t.Parallel()

		wrapped := fmt.Errorf("outer: %w", errors.Join(first, second))

		assert.Equal(t, []error{wrapped}, errlines.Members(wrapped))
	})

	t.Run("fmt.Errorf with two %w is one error", func(t *testing.T) {
		t.Parallel()

		both := fmt.Errorf("%w and %w", first, second)

		assert.Equal(t, []error{both}, errlines.Members(both))
	})

	t.Run("nil and empty-message members are dropped", func(t *testing.T) {
		t.Parallel()

		real := errors.New("real")

		assert.Equal(t, []error{real}, errlines.Members(errors.Join(nil, &pieceError{message: "", hint: "h"}, real)))
	})

	t.Run("an empty-message error has no members", func(t *testing.T) {
		t.Parallel()

		assert.Zero(t, errlines.Members(&pieceError{message: "", hint: "h"}))
		assert.Zero(t, errlines.Members(errors.Join(errors.New(""), &pieceError{message: ""})))
		assert.Zero(t, errlines.Members(nil))
	})

	t.Run("a nil member is skipped", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, []error{first, second}, errlines.Members(&joinError{members: []error{first, nil, second}}))
	})

	t.Run("a join whose text is not its members' texts is one error", func(t *testing.T) {
		t.Parallel()

		odd := &oddJoin{members: []error{first, second}}

		assert.Equal(t, []error{odd}, errlines.Members(odd))
	})
}

// oddJoin has Unwrap() []error but a text that is not its members' joined.
type oddJoin struct{ members []error }

func (e *oddJoin) Error() string   { return "something else" }
func (e *oddJoin) Unwrap() []error { return e.members }

func TestRender(t *testing.T) {
	t.Parallel()

	members := []error{
		&pieceError{message: "first", origins: []string{"origin"}, note: "note", hint: "hint"},
		&pieceError{message: ""},
		errors.New("second"),
	}

	t.Run("plain", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "app: error: first\n  from: origin\n  note: note\n  hint: hint\napp: error: second\n", errlines.Render(members, "app: ", false))
	})

	t.Run("colored", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "app: \x1b[1;31merror:\x1b[0m first\n"+
			"  \x1b[2mfrom:\x1b[0m origin\n"+
			"  \x1b[2mnote:\x1b[0m note\n"+
			"  \x1b[1;36mhint:\x1b[0m hint\n"+
			"app: \x1b[1;31merror:\x1b[0m second\n", errlines.Render(members, "app: ", true))
	})
}

func TestRender_LinesOrderAndFilter(t *testing.T) {
	t.Parallel()

	t.Run("origins, then the note, then the hint", func(t *testing.T) {
		t.Parallel()

		err := &pieceError{message: "failed", origins: []string{"a", "b"}, note: "n", hint: "h"}

		assert.Equal(t, "app: error: failed\n  from: a\n  from: b\n  note: n\n  hint: h\n", errlines.Render([]error{err}, "app: ", false))
	})

	t.Run("nil and empty-message members print nothing", func(t *testing.T) {
		t.Parallel()

		members := []error{nil, &pieceError{message: "", hint: "ignored"}, errors.New("real")}

		assert.Equal(t, "app: error: real\n", errlines.Render(members, "app: ", false))
		assert.Equal(t, "", errlines.Render([]error{nil, &pieceError{message: ""}}, "app: ", false))
		assert.Equal(t, "", errlines.Render(nil, "app: ", false))
	})
}

func TestPlain(t *testing.T) {
	t.Parallel()

	members := []error{&pieceError{message: "first", note: "n"}, errors.New("second")}

	assert.Equal(t, "first\n  note: n\nsecond", errlines.Plain(members))
	assert.Equal(t, "", errlines.Plain(nil))

	joined := errors.Join(nil, &pieceError{message: ""}, &pieceError{message: "real", note: "n"})
	filtered := errlines.Members(joined)

	assert.Equal(t, "real\n  note: n", errlines.Plain(filtered))
}

func TestAppendJSON(t *testing.T) {
	t.Parallel()

	t.Run("a single member", func(t *testing.T) {
		t.Parallel()

		got := errlines.AppendJSON(nil, []error{&pieceError{message: `say "hi"`, origins: []string{"a"}, note: "n", hint: "h"}})

		assert.Equal(t, `{"error":"say \"hi\"","from":["a"],"note":"n","hint":"h"}`, string(got))
	})

	t.Run("several members", func(t *testing.T) {
		t.Parallel()

		got := errlines.AppendJSON(nil, []error{errors.New("first"), &pieceError{message: "second", hint: "h"}, errors.New("third")})

		assert.Equal(t, `{"error":"first","more":[{"error":"second","hint":"h"},{"error":"third"}]}`, string(got))
	})

	t.Run("no members is an empty message", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, `{"error":""}`, string(errlines.AppendJSON(nil, nil)))
		assert.Equal(t, `{"error":""}`, string(errlines.AppendJSON(nil, errlines.Members(&pieceError{message: "", hint: "h"}))))
	})

	t.Run("members from a join with nil and empty entries", func(t *testing.T) {
		t.Parallel()

		got := errlines.AppendJSON(nil, errlines.Members(errors.Join(
			&pieceError{message: "", hint: "ignored"},
			&pieceError{message: "first", note: "n"},
			errors.New("second"),
		)))

		assert.Equal(t, `{"error":"first","note":"n","more":[{"error":"second"}]}`, string(got))
	})
}

func TestAppendText(t *testing.T) {
	t.Parallel()

	t.Run("joins parts and members", func(t *testing.T) {
		t.Parallel()

		got := errlines.AppendText(nil, []error{
			&pieceError{message: "first", origins: []string{"a", "b"}, note: "n", hint: "h"},
			errors.New("second"),
		})

		assert.Equal(t, "first (from: a; b; note: n; hint: h) | second", string(got))
	})

	t.Run("members from a join with an empty entry", func(t *testing.T) {
		t.Parallel()

		got := errlines.AppendText(nil, errlines.Members(errors.Join(
			&pieceError{message: "", hint: "ignored"},
			&pieceError{message: "real", origins: []string{"a"}},
		)))

		assert.Equal(t, "real (from: a)", string(got))
		assert.Equal(t, "", string(errlines.AppendText(nil, nil)))
	})
}
