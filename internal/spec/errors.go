package spec

import (
	"errors"
	"fmt"

	"github.com/conga-sh/conga/internal/errlines"
)

// SchemaError reports a command schema defect discovered during schema build
// or static analysis. Its text has three roles: the message states the
// defect, the note gives more detail on it, and the hint is the action that
// fixes it.
type SchemaError struct {
	structName string
	fieldName  string
	msg        string
	note       string
	hint       string
	cause      error
}

// Error returns the message alone. The note and hint are separate pieces,
// read through Note and Hint, and App.Exit prints them on their own lines.
func (e *SchemaError) Error() string {
	return e.Message()
}

// Message returns the defect statement with its owner prefix
// ("struct <S> field <F>: " or "struct <S>: "), without the note or hint.
func (e *SchemaError) Message() string {
	if e.structName == "" {
		return e.msg
	}

	owner := "struct " + e.structName
	if e.fieldName != "" {
		owner += " field " + e.fieldName
	}

	return owner + ": " + e.msg
}

// Note returns the detail on the defect. When the error has none it returns its
// cause's, by the chain rule, or "".
func (e *SchemaError) Note() string {
	if e.note != "" {
		return e.note
	}

	return errlines.Note(e.cause)
}

// Hint returns the action that fixes the defect. When the error has none it
// returns its cause's, by the chain rule, or "".
func (e *SchemaError) Hint() string {
	if e.hint != "" {
		return e.hint
	}

	return errlines.Hint(e.cause)
}

// Unwrap returns the underlying cause when the schema error wraps one.
func (e *SchemaError) Unwrap() error {
	return e.cause
}

// IsSchemaError reports whether err is a *SchemaError.
func IsSchemaError(err error) bool {
	_, ok := errors.AsType[*SchemaError](err)

	return ok
}

// WithOwner attributes err to structName and fieldName when err does not already
// record an owner.
func WithOwner(err error, structName, fieldName string) error {
	if err == nil {
		return nil
	}

	if schemaErr, ok := errors.AsType[*SchemaError](err); ok {
		if schemaErr.structName != "" || schemaErr.fieldName != "" {
			return err
		}

		owned := *schemaErr
		owned.structName = structName
		owned.fieldName = fieldName

		return &owned
	}

	return &SchemaError{
		structName: structName,
		fieldName:  fieldName,
		msg:        err.Error(),
		cause:      err,
	}
}

// CompletionEnvPrefixError reports a completion environment prefix whose leading
// digit violates shell variable naming rules.
func CompletionEnvPrefixError(prefix string) error {
	return schemaErrorf("completion environment prefix %q begins with a digit", prefix).
		withNote("shell variable names cannot begin with a digit").
		withHint("pass conga.CompletionEnvPrefix with a prefix beginning with a letter")
}

// UnsupportedValueTypeError reports a field value type with no registered or
// built-in parser.
func UnsupportedValueTypeError(role, targetType, hint string) error {
	return schemaErrorf("unsupported %s %s", role, targetType).withHint(hint)
}

// RangeUnsupportedTypeError reports a range tag declared on an unsupported
// value type.
func RangeUnsupportedTypeError(typeName string) error {
	return schemaErrorf("range tag is not supported on type %s", typeName)
}

// withNote sets the detail on the defect; an empty note renders nothing.
func (e *SchemaError) withNote(note string) *SchemaError {
	e.note = note

	return e
}

// withNotef sets the detail on the defect from a format string.
func (e *SchemaError) withNotef(format string, args ...any) *SchemaError {
	e.note = fmt.Sprintf(format, args...)

	return e
}

// withHint sets the action that fixes the defect; an empty hint renders
// nothing.
func (e *SchemaError) withHint(hint string) *SchemaError {
	e.hint = hint

	return e
}

// withHintf sets the action that fixes the defect from a format string.
func (e *SchemaError) withHintf(format string, args ...any) *SchemaError {
	e.hint = fmt.Sprintf(format, args...)

	return e
}

// withCause records the underlying error that Unwrap returns.
func (e *SchemaError) withCause(cause error) *SchemaError {
	e.cause = cause

	return e
}

// ownedBy attributes the error to structName and fieldName; either may be
// empty.
func (e *SchemaError) ownedBy(structName, fieldName string) *SchemaError {
	e.structName = structName
	e.fieldName = fieldName

	return e
}

// schemaError builds a schema error with a constant message.
func schemaError(msg string) *SchemaError {
	return &SchemaError{msg: msg}
}

// schemaErrorf builds a schema error with a formatted message.
func schemaErrorf(format string, args ...any) *SchemaError {
	return &SchemaError{msg: fmt.Sprintf(format, args...)}
}
