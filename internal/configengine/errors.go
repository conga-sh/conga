package configengine

import (
	"fmt"

	"github.com/conga-sh/conga/internal/configdata"
)

// configError represents a failure reading or parsing configuration files.
type configError struct {
	path  string
	msg   string
	cause error
}

func newFieldConfigError(origin, scope, msgContext string) *configError {
	path := configdata.JoinOriginScope(origin, scope)

	return &configError{
		path: path,
		msg:  fmt.Sprintf("config file %q: %s", path, msgContext),
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
	return e.cause
}
