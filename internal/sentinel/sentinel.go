// Package sentinel centralizes the error sentinels owned by opt-in subsystems
// (configuration, .env, shell completion). Their owning packages must stay
// free of dynamic package-level initializers so the linker can drop them
// together with their disabled feature; hosting the sentinels here, in an
// always-linked leaf, keeps those packages init-free while retaining idiomatic
// errors.New declarations. This package must remain dependency-free beyond
// errors.
package sentinel

import (
	"errors"
)

var (
	// ErrConfig indicates the configuration file could not be loaded.
	// Application code classifies configuration failures through the root
	// conga.ErrConfig alias, which points at this sentinel.
	ErrConfig = errors.New("config error")

	// ErrConfigRead indicates the configuration file could not be read.
	ErrConfigRead = error(&sentinelError{parent: ErrConfig, msg: "reading config file"})

	// ErrConfigParse indicates the configuration file could not be parsed.
	ErrConfigParse = error(&sentinelError{parent: ErrConfig, msg: "parsing config file"})

	// ErrConfigUnknownField indicates the configuration file declared an
	// unknown field or section.
	ErrConfigUnknownField = error(&sentinelError{parent: ErrConfig, msg: "unknown field"})

	// ErrConfigConflict indicates the configuration file could not be loaded.
	ErrConfigConflict = error(&sentinelError{parent: ErrConfig, msg: "multiple conflicting config files found"})

	// ErrDotenv indicates a .env file could not be read or parsed.
	ErrDotenv = errors.New("dotenv error")

	// ErrDotenvRead indicates a .env file could not be read.
	ErrDotenvRead = error(&sentinelError{parent: ErrDotenv, msg: "reading .env file"})

	// ErrDotenvParse indicates a .env file syntax error was encountered.
	ErrDotenvParse = error(&sentinelError{parent: ErrDotenv, msg: "parsing .env file"})

	// ErrUnknownShell indicates that an unrecognized shell identifier was requested.
	ErrUnknownShell = errors.New("unknown shell")

	// ErrRedirectTarget reports that the completion cursor sits at a shell
	// redirection target ("cloudctl deploy > <cursor>"), where completion
	// yields no candidates. The bash Patch hook raises it; the dispatch
	// path honors it with the zero-candidate framing instead of the
	// raw-argument fallback.
	ErrRedirectTarget = errors.New("bash completion: current position is a redirect target")
)

// sentinelError wraps a parent sentinel error with a static contextual message.
//
// Unlike fmt.Errorf("%w: ...", parent), sentinelError is constructed as a static
// struct literal, ensuring internal/sentinel stays free of dynamic package-level
// initializers (..inittask) and avoids pulling in the "fmt" dependency at package scope.
type sentinelError struct {
	parent error
	msg    string
}

func (e *sentinelError) Error() string {
	return e.parent.Error() + ": " + e.msg
}

func (e *sentinelError) Unwrap() error {
	return e.parent
}
