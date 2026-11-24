package spec

import (
	"reflect"
	"strings"
)

const (
	flagDeclarationName flagDeclarationKind = iota
	flagDeclarationAlias
	flagDeclarationShort
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

// FlagOwner names the declaration of a flag name, alias, or short rune for
// shadowing diagnostics: the Go struct and field that hold it. A struct
// appears at most once in any command chain (Command[T] holds T by value), so
// struct and field identify the declaration without the command name.
type FlagOwner struct {
	Struct string
	Field  string
}

// FlagDeclaration names one command-line spelling of a flag, its long name, an
// alias, or its short rune, for duplicate and shadowing diagnostics. The
// runtime schema builder and the static analyzer build it through the
// constructors below, so both word the spelling the same way.
type FlagDeclaration struct {
	kind  flagDeclarationKind
	name  string
	short rune
}

// String renders the declaration as it appears in schema error messages.
func (d FlagDeclaration) String() string {
	switch d.kind {
	case flagDeclarationAlias:
		return "flag alias --" + d.name
	case flagDeclarationShort:
		return "short flag -" + string(d.short)
	default:
		return "flag --" + d.name
	}
}

// FlagNameDeclaration names a flag's long name, rendered "flag --name".
func FlagNameDeclaration(name string) FlagDeclaration {
	return FlagDeclaration{kind: flagDeclarationName, name: name}
}

// FlagAliasDeclaration names a flag alias, rendered "flag alias --alias".
func FlagAliasDeclaration(alias string) FlagDeclaration {
	return FlagDeclaration{kind: flagDeclarationAlias, name: alias}
}

// ShortFlagDeclaration names a flag's short rune, rendered "short flag -r".
func ShortFlagDeclaration(short rune) FlagDeclaration {
	return FlagDeclaration{kind: flagDeclarationShort, short: short}
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
		return schemaErrorf("raw args cannot be combined with command field %s", grammar.CommandField).
			withNote("declared built-in help, version, and completion command fields also count").
			withHint("use DashArgs for '--' forwarding or declare RawArgs on a leaf command")
	}

	for _, arg := range grammar.Args {
		if arg.Required && !arg.IsSlice && !arg.IsMap {
			continue
		}

		return schemaErrorf("raw args requires a deterministic positional prefix: positional argument %s must be a required scalar", arg.Name).
			withHint("declare only required scalar positional arguments before RawArgs, or use DashArgs for '--' forwarding")
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

	return schemaErrorf("positional argument %s cannot be combined with subcommand %s", grammar.PositionalField, grammar.SubcommandField).
		withHintf("move %s into a leaf subcommand or pass the value through a flag", grammar.PositionalField)
}

// ContainerAlreadyDeclaredError builds the duplicate-container error for a
// second container of a kind a command allows once. The runtime schema builder
// and the static analyzer share it as the single source of truth;
// containerName is "RawArgs", "DashArgs", or "ConfigFile" and firstFieldName
// is the struct field of the first declaration.
func ContainerAlreadyDeclaredError(containerName, firstFieldName string) error {
	return schemaErrorf("duplicate %s field", containerName).withNotef("already declared by field %s", firstFieldName)
}

// ImplicitCommandAlreadyDeclaredError builds the duplicate-command error for a
// second implicit command on one command struct. The runtime schema builder and
// the static analyzer share it as the single source of truth.
func ImplicitCommandAlreadyDeclaredError(firstFieldName string) error {
	return schemaError("only one implicit command is allowed per command").withNotef("already declared by field %s", firstFieldName)
}

// AncestorShadowError builds the error for a flag name, alias, or short rune
// that a descendant command redeclares from an ancestor. Both sides are named
// by struct and field. The runtime schema builder and the static analyzer share
// it as the single source of truth.
func AncestorShadowError(descendant FlagOwner, declaration FlagDeclaration, ancestor FlagOwner) error {
	return schemaErrorf("duplicate %s", declaration).
		withNotef("already declared by struct %s field %s on an ancestor command", ancestor.Struct, ancestor.Field).
		ownedBy(descendant.Struct, descendant.Field)
}

// DuplicateFlagError builds the error for a flag name, alias, or short rune
// that a second field of the same command declares; firstField names the field
// that declared it first. The caller adds the struct and field context. The
// runtime schema builder and the static analyzer share it as the single
// source of truth.
func DuplicateFlagError(declaration FlagDeclaration, firstField string) error {
	return schemaErrorf("duplicate %s", declaration).withNotef("already declared by field %s", firstField)
}

// NoRunnerError builds the error for a leaf command whose struct doesn't
// implement conga.Runner, naming the struct (Command.StructName, which names an
// anonymous struct by the path of its declaring field). The runtime binder and
// the static analyzer share it as the single source of truth.
func NoRunnerError(structName string) error {
	return schemaError("runner not implemented").
		ownedBy(structName, "").
		withHint("implement conga.Runner: Run(context.Context, *conga.Session) error")
}

// VersionJSONShadowError builds the shadowing error for the built-in version
// command's --json selector, which the user cannot rename, with a migration
// hint. descendant names the VersionCommand field.
func VersionJSONShadowError(descendant FlagOwner, ancestor FlagOwner) error {
	return schemaErrorf("duplicate flag --%s", VersionJSONFlagName).
		ownedBy(descendant.Struct, descendant.Field).
		withNotef("already declared by struct %s field %s on an ancestor command; the built-in version command owns --%s", ancestor.Struct, ancestor.Field, VersionJSONFlagName).
		withHint("rename the ancestor flag or remove the VersionCommand field")
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

	return schemaError(`tag "feature" cannot be combined with implicit:"true"`).
		withNote("an implicit command is entered without being named, so it cannot be gated").
		withHint("gate the parent, or make the command explicit")
}

// flagDeclarationKind selects which spelling of a flag a FlagDeclaration names.
type flagDeclarationKind uint8
