package configengine

import (
	"errors"
	"fmt"

	"github.com/conga-sh/conga/internal/sentinel"
)

// configError represents a failure reading or parsing configuration files.
type configError struct {
	path  string
	msg   string
	cause error
}

func newFieldConfigError(filePath, msgContext, causeContext string) *configError {
	return &configError{
		path:  filePath,
		msg:   fmt.Sprintf("config file %q: %s", filePath, msgContext),
		cause: fmt.Errorf("%w: %s", sentinel.ErrConfigUnknownField, causeContext),
	}
}

// ConfigPath returns the failing configuration file path, or "" when the
// error carries none.
func (e *configError) ConfigPath() string {
	return e.path
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

func (e *configError) Unwrap() error {
	if e.cause != nil {
		return e.cause
	}

	return sentinel.ErrConfig
}

func (e *configError) Is(target error) bool {
	if target == sentinel.ErrConfig {
		return true
	}

	if e.cause != nil {
		return errors.Is(e.cause, target)
	}

	return false
}
