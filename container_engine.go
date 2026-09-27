package conga

import "github.com/conga-sh/conga/internal/container"

// flagState is the engine's state view of a flag-like container: the exported
// query methods plus the unexported mutators backing value binding.
type flagState interface {
	IsSet() bool
	Changed() bool
	Source() ValueSource
	SourceDetail() string
	getValue() any
	setValue(value any, source ValueSource, detail string)
}

// argState is the engine's state view of a positional argument container.
type argState interface {
	IsSet() bool
	Changed() bool
	getValue() any
	setValue(value any, source ValueSource, detail string)
}

// commandState is the engine's state view of a subcommand container.
type commandState interface {
	IsSelected() bool
	innerValue() any
	markSelected()
}

// argsState is the engine's state view of a trailing-arguments collector.
type argsState interface {
	setArgs(args []string)
}

// containerClassified is implemented by every built-in container through its
// unexported containerKind marker. Only package conga types can satisfy it,
// which keeps user-defined container types out of scope.
type containerClassified interface {
	containerKind() container.Classification
}

// classifyContainer maps a command-struct field address to its engine
// classification. Addresses that are not built-in containers are returned as
// the zero classification so the schema builder can continue with its regular
// field diagnostics.
func classifyContainer(fieldAddress any) container.Classification {
	classified, ok := fieldAddress.(containerClassified)
	if !ok {
		return container.Classification{}
	}

	return classified.containerKind()
}

// flagStateOf recovers the engine flag state stored on a classified field
// handle. Callers only invoke it on handles the schema builder classified, so
// a nil result indicates an engine bug.
func flagStateOf(handle any) flagState {
	state, ok := handle.(flagState)
	if !ok {
		return nil
	}

	return state
}

// argStateOf recovers the engine argument state stored on a classified field
// handle. Callers only invoke it on handles the schema builder classified, so
// a nil result indicates an engine bug.
func argStateOf(handle any) argState {
	state, ok := handle.(argState)
	if !ok {
		return nil
	}

	return state
}

// commandStateOf recovers the engine subcommand state stored on a classified
// field handle. Callers only invoke it on handles the schema builder
// classified, so a nil result indicates an engine bug.
func commandStateOf(handle any) commandState {
	state, ok := handle.(commandState)
	if !ok {
		return nil
	}

	return state
}

// argsStateOf recovers the engine trailing-arguments state stored on a
// classified field handle. Callers only invoke it on handles the schema
// builder classified, so a nil result indicates an engine bug.
func argsStateOf(handle any) argsState {
	state, ok := handle.(argsState)
	if !ok {
		return nil
	}

	return state
}
