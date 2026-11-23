package spec

import (
	"errors"
	"fmt"
	"strings"
)

// Runtime feature value validation sentinels.
var (
	errFeatureMustBeInteger            = errors.New("must be an integer")
	errFeatureMustBeBoolean            = errors.New("must be true or false (or 1 or 0)")
	errFeatureInvalidEnum              = errors.New("must be one of")
	errFeatureMustBeNonNegativeInteger = errors.New("must be a non-negative integer")
	errFeatureIntegerOutOfRange        = errors.New("is out of range for the feature's integer type")
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

// Error renders the message followed by the labelled note and hint, as in
// `message (note: ...; hint: ...)`.
func (e *SchemaError) Error() string {
	var builder strings.Builder

	builder.WriteString(e.Message())

	switch {
	case e.note != "" && e.hint != "":
		builder.WriteString(" (note: " + e.note + "; hint: " + e.hint + ")")
	case e.note != "":
		builder.WriteString(" (note: " + e.note + ")")
	case e.hint != "":
		builder.WriteString(" (hint: " + e.hint + ")")
	}

	return builder.String()
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

// Note returns the detail on the defect, or "" when there is none.
func (e *SchemaError) Note() string {
	return e.note
}

// Hint returns the action that fixes the defect, or "" when there is none.
func (e *SchemaError) Hint() string {
	return e.hint
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
