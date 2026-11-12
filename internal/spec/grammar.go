package spec

import (
	"fmt"
	"reflect"
	"strings"
)

// TrailingGrammar describes the trailing-argument grammar of one command for
// the RawArgs boundary invariant. It carries only classification metadata so
// schema build and the static analyzer share one validation source.
type TrailingGrammar struct {
	// HasRawArgs reports whether the command declares a RawArgs collector.
	HasRawArgs bool
	// CommandField is the first declared command field, including built-in
	// help, version, and completion command fields. It names the conflicting
	// field in the RawArgs diagnostic and is empty when none is declared.
	CommandField string
	// Args lists every declared positional argument in declaration order.
	Args []TrailingArgGrammar
}

// TrailingArgGrammar describes one positional argument preceding a RawArgs
// collector.
type TrailingArgGrammar struct {
	// Name is the display name used in diagnostics.
	Name string
	// Required reports whether the argument must be supplied.
	Required bool
	// IsSlice reports whether the argument collects repeated values.
	IsSlice bool
	// IsMap reports whether the argument collects key/value entries.
	IsMap bool
}

// BranchGrammar describes whether one command declares positional arguments
// and user subcommands. It carries the field names so the branch/leaf
// invariant can report both sides of the conflict.
type BranchGrammar struct {
	// PositionalField is the first declared positional argument field.
	PositionalField string
	// SubcommandField is the first declared user subcommand field. Built-in
	// help, version, and completion commands do not count as branching.
	SubcommandField string
}

// ValidateTrailingGrammar enforces a deterministic RawArgs boundary: a command
// declaring RawArgs may not also declare any command field, and every
// positional argument before the collector must be a required scalar. RawArgs
// is a leaf collector; DashArgs remains the explicit '--'-delimited alternative
// for commands that also branch.
func ValidateTrailingGrammar(grammar TrailingGrammar) error {
	if !grammar.HasRawArgs {
		return nil
	}

	if grammar.CommandField != "" {
		return fmt.Errorf("%w: raw args cannot be combined with command field %s (declared built-in help, version, and completion command fields also count); a command is either a branch (subcommands) or a leaf (positional/raw arguments): use DashArgs for '--' forwarding or declare RawArgs on a leaf command", ErrSchema, grammar.CommandField)
	}

	for _, arg := range grammar.Args {
		if arg.Required && !arg.IsSlice && !arg.IsMap {
			continue
		}

		return fmt.Errorf("%w: raw args requires a deterministic positional prefix: positional argument %s must be a required scalar; declare only required scalar positional arguments before RawArgs, or use DashArgs for '--' forwarding", ErrSchema, arg.Name)
	}

	return nil
}

// ValidateBranchGrammar enforces the branch/leaf invariant: a command is
// either a branch (user subcommands) or a leaf (positional arguments), never
// both. Mixing the two lets a positional typed before a subcommand bind the
// routed leaf's arguments, which shifts positional indexes and the RawArgs
// boundary away from what the schema declares. Kong refuses the same shape as
// "can't mix positional arguments and branching arguments".
func ValidateBranchGrammar(grammar BranchGrammar) error {
	if grammar.PositionalField == "" || grammar.SubcommandField == "" {
		return nil
	}

	return fmt.Errorf("%w: positional argument %s cannot be combined with subcommand %s; a command is either a branch (subcommands) or a leaf (positional arguments): move %s into a leaf subcommand or pass the value through a flag", ErrSchema, grammar.PositionalField, grammar.SubcommandField, grammar.PositionalField)
}

// TrailingCollectorAlreadyDeclaredError builds the duplicate-container error
// for a second trailing-argument collector of one kind on a command struct.
// The runtime schema builder and the static analyzer share it as the single
// source of truth; containerName is "RawArgs" or "DashArgs" and firstFieldName
// is the struct field of the first declaration.
func TrailingCollectorAlreadyDeclaredError(containerName, firstFieldName string) error {
	return fmt.Errorf("%w: %s (already declared by field %s)", ErrDuplicateFlag, containerName, firstFieldName)
}

// ImplicitCommandAlreadyDeclaredError builds the duplicate-command error for a
// second implicit command on one command struct. The runtime schema builder and
// the static analyzer share it as the single source of truth.
func ImplicitCommandAlreadyDeclaredError(firstFieldName string) error {
	return fmt.Errorf("%w: only one implicit command is allowed per command; implicit command already declared by field %s", ErrSchema, firstFieldName)
}

// ValidateImplicitFeature rejects a feature tag on an implicit command. An
// implicit command is entered without being named, so a gate on it would make
// the parent's operand and flag grammar depend on a feature. Gating the parent,
// or a flag of the implicit command, stays legal. The runtime schema builder
// and the static analyzer share it as the single source of truth.
func ValidateImplicitFeature(tag reflect.StructTag) error {
	if strings.TrimSpace(tag.Get("feature")) == "" {
		return nil
	}

	return fmt.Errorf("%w: tag \"feature\" cannot be combined with implicit:\"true\" (an implicit command is entered without being named, so it cannot be gated; gate the parent, or make the command explicit)", ErrSchema)
}
