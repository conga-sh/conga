package configengine

import (
	"fmt"

	"github.com/conga-sh/conga/internal/errlines"
)

// configError represents a failure reading or parsing configuration files, or
// a strict-validation failure inside one. Its Error is the first line alone.
// Only strict-validation errors carry an origin (the file or document the
// offending entry came from) and may carry a hint; the errors from finding and
// loading files name their file or directory in the message instead.
type configError struct {
	path  string
	msg   string
	cause error

	// origin is the text of the "from:" line, or "" for none.
	origin string

	// hint is the action that fixes the error, or "".
	hint string
}

func newFieldConfigError(origin, msg string) *configError {
	return &configError{msg: msg, origin: origin}
}

// withHint sets the action that fixes the error.
func (e *configError) withHint(hint string) *configError {
	e.hint = hint

	return e
}

func (e *configError) Error() string {
	if e.msg != "" {
		return e.msg
	}

	if e.path != "" && e.cause != nil {
		return fmt.Sprintf("config file %q: %v", e.path, e.cause)
	}

	if e.path != "" {
		return fmt.Sprintf("config file %q error", e.path)
	}

	if e.cause != nil {
		return e.cause.Error()
	}

	return "config error"
}

// Origins returns the "from:" text of a strict-validation error. It is the
// config file or the document label the offending entry came from. When the
// error has no origin of its own it returns its cause's, by the chain rule. It
// is nil when the cause has none either, as for an error from finding or
// loading files.
func (e *configError) Origins() []string {
	if e.origin == "" {
		return errlines.Origins(e.cause)
	}

	return []string{e.origin}
}

// Hint returns the action that fixes the error, such as a did-you-mean
// suggestion. When the error has none it returns its cause's, by the chain
// rule, or "".
func (e *configError) Hint() string {
	if e.hint != "" {
		return e.hint
	}

	return errlines.Hint(e.cause)
}

func (e *configError) Unwrap() error {
	return e.cause
}
