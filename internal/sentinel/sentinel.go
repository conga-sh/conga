// Package sentinel centralizes the error sentinels owned by opt-in subsystems
// (shell completion). Their owning packages must stay free of dynamic
// package-level initializers so the linker can drop them together with their
// disabled feature. Hosting the sentinels here, in an always-linked leaf, keeps
// those packages init-free while retaining idiomatic errors.New declarations.
// This package must remain dependency-free beyond errors.
//
// Only errors that code compares for control flow live here. Errors that
// nothing compares are root errors built where they occur.
package sentinel

import (
	"errors"
)

// ErrRedirectTarget reports that the completion cursor sits at a shell
// redirection target ("cloudctl deploy > <cursor>"), where the shell
// completes filenames. The bash Patch hook raises it, and the dispatch
// path honors it with the zero-candidate framing instead of the
// raw-argument fallback.
var ErrRedirectTarget = errors.New("bash completion: current position is a redirect target")
