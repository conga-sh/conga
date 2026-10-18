package conga

import (
	"github.com/conga-sh/conga/internal/configdata"
	"github.com/conga-sh/conga/internal/container"
	"github.com/conga-sh/conga/internal/spec"
)

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

// featureState is the engine's state view of the feature-declaration container:
// the flag state plus feature resolution, config restriction, and completion pruning.
type featureState interface {
	flagState
	resolveFeatures(cmd *spec.Command, cmdChain []*spec.Command, flagValues map[*spec.Flag][]string, state *bindingState) ([]prunedFlag, error)
	rejectPrunedConfig(pruned []prunedFlag, configData *configdata.Data) error
	pruneForCompletion(cmd *spec.Command, flagValues map[*spec.Flag][]string, state *bindingState)
	setMapProvenance(sources map[string]ValueSource, origins map[string]string)
	mapProvenance() (map[string]ValueSource, map[string]string)
	setResolvedValues(values map[string]string)
	resolvedValues() map[string]string
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

// featureStateOf recovers the engine feature state stored on a classified field
// handle. Callers only invoke it on handles the schema builder classified, so
// a nil result indicates an engine bug.
func featureStateOf(handle any) featureState {
	state, ok := handle.(featureState)
	if !ok {
		return nil
	}

	return state
}

// featureStateOfCommand recovers the engine feature state stored on a command's
// feature flag handle.
func featureStateOfCommand(cmd *spec.Command) featureState {
	if cmd == nil || cmd.FeatureFlag == nil {
		return nil
	}

	return featureStateOf(cmd.FeatureFlag.Handle)
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
