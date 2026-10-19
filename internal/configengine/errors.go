package configengine

import (
	"errors"
	"fmt"

	"github.com/conga-sh/conga/internal/configdata"
	"github.com/conga-sh/conga/internal/sentinel"
)

// configError represents a failure reading or parsing configuration files.
type configError struct {
	path  string
	msg   string
	cause error
}

func newFieldConfigError(origin, scope, msgContext, causeContext string) *configError {
	path := configdata.JoinOriginScope(origin, scope)

	return &configError{
		path:  path,
		msg:   fmt.Sprintf("config file %q: %s", path, msgContext),
		cause: fmt.Errorf("%w: %s", sentinel.ErrConfigUnknownField, causeContext),
	}
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
