// Package sentinel centralizes the error sentinels owned by opt-in subsystems
// (configuration, .env, shell completion). Their owning packages must stay
// free of dynamic package-level initializers so the linker can drop them
// together with their disabled feature; hosting the sentinels here, in an
// always-linked leaf, keeps those packages init-free while retaining idiomatic
// errors.New declarations. This package must remain dependency-free beyond
// errors and fmt.
package sentinel

import (
	"errors"
	"fmt"
)

var (
	// ErrConfig indicates the configuration file could not be loaded.
	// Application code classifies configuration failures through the root
	// conga.ErrConfig alias, which points at this sentinel.
	ErrConfig = errors.New("config error")

	// ErrConfigRead indicates the configuration file could not be read.
	ErrConfigRead = fmt.Errorf("%w: reading config file", ErrConfig)

	// ErrConfigParse indicates the configuration file could not be parsed.
	ErrConfigParse = fmt.Errorf("%w: parsing config file", ErrConfig)

	// ErrConfigUnknownField indicates the configuration file declared an
	// unknown field or section.
	ErrConfigUnknownField = fmt.Errorf("%w: unknown field", ErrConfig)

	// ErrConfigConflict indicates the configuration file could not be loaded.
	ErrConfigConflict = fmt.Errorf("%w: multiple conflicting config files found", ErrConfig)

	// ErrDotenv indicates a .env file could not be read or parsed.
	ErrDotenv = errors.New("dotenv error")

	// ErrDotenvRead indicates a .env file could not be read.
	ErrDotenvRead = fmt.Errorf("%w: reading .env file", ErrDotenv)

	// ErrDotenvParse indicates a .env file syntax error was encountered.
	ErrDotenvParse = fmt.Errorf("%w: parsing .env file", ErrDotenv)

	// ErrUnknownShell indicates that an unrecognized shell identifier was requested.
	ErrUnknownShell = errors.New("unknown shell")

	// ErrRedirectTarget reports that the completion cursor sits at a shell
	// redirection target ("cloudctl deploy > <cursor>"), where completion
	// yields no candidates. The bash Patch hook raises it; the dispatch
	// path honors it with the zero-candidate framing instead of the
	// raw-argument fallback.
	ErrRedirectTarget = errors.New("bash completion: current position is a redirect target")
)
