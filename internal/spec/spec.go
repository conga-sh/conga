package spec

import (
	"reflect"
	"slices"
	"strings"

	"github.com/conga-sh/conga/internal/container"
	"github.com/conga-sh/conga/predict"
)

const (
	// PredictorKindCustom indicates a user-registered dynamic predictor.
	PredictorKindCustom PredictorKind = iota

	// PredictorKindFile indicates built-in file completion (with optional extension filters).
	PredictorKindFile

	// PredictorKindDir indicates built-in directory completion.
	PredictorKindDir

	// PredictorKindExec indicates built-in PATH executable completion.
	PredictorKindExec

	// PredictorKindSymlink indicates built-in symlink completion.
	PredictorKindSymlink

	// PredictorKindNone indicates completion should be suppressed (no file fallback).
	PredictorKindNone

	// PredictorKindPredictable indicates a direct predict.Predictor (e.g. from Predictable interface).
	PredictorKindPredictable
)

// Command represents the metadata and AST definition of a command node.
type Command struct {
	Name            string
	Aliases         []string
	Summary         string
	Description     string
	Examples        string
	Footer          string
	Deprecated      string
	Group           string
	Hidden          bool
	EnvPrefix       string
	Version         string
	IsHelpCmd       bool
	IsVersionCmd    bool
	IsCompletionCmd bool
	// IsCompletionFlag is true when a completion command was synthesized from the
	// --completion flag on a flat CLI, tailoring help screens to flag syntax.
	IsCompletionFlag bool
	// Shell names the completion target shell for IsCompletionCmd commands,
	// populated during binding so `completion <shell> --help` renders that
	// shell's enable instructions. Empty on all other commands.
	Shell          string
	Flags          []*Flag
	Args           []*Arg
	Subcommands    []*Command
	DashArgs       *DashArgs
	RawArgs        *RawArgs
	ConfigFileFlag *Flag
	// ConfigSettings holds the app-level configuration resolution settings
	// (app.ConfigOptions options) injected onto the root command during build.
	// Empty on subcommands.
	ConfigSettings ConfigSettings
	Target         any
	// Handle is the engine's opaque handle to the subcommand's container value.
	// It is never inspected by this package: package conga stores the field
	// address here and re-asserts its private engine interfaces at bind time.
	// Nil when the field was not classified.
	Handle            any
	DisableCompletion bool
	DisableHelp       bool
	DisableVersion    bool
	DisableWhy        bool
}

// HasUserSubcommands returns true if this command has user-declared subcommands (excluding built-in help, version, and completion).
func (c *Command) HasUserSubcommands() bool {
	for _, subcommand := range c.Subcommands {
		if !subcommand.IsCompletionCmd && !subcommand.IsHelpCmd && !subcommand.IsVersionCmd {
			return true
		}
	}

	return false
}

// HasCompletionCommand reports whether this command contains a completion subcommand.
func (c *Command) HasCompletionCommand() bool {
	for _, subcommand := range c.Subcommands {
		if subcommand.IsCompletionCmd || subcommand.Name == "completion" {
			return true
		}
	}

	return false
}

// VersionFlagVisible reports whether the built-in --version flag is offered for
// this command: version information describes the whole application, so the
// flag exists only on the root command unless explicitly disabled. It is the
// single authority used by the binder, help renderer, and completion engine so
// --version handling stays consistent.
func (c *Command) VersionFlagVisible(isRoot bool) bool {
	return isRoot && !c.DisableVersion
}

// CompletionFlagVisible reports whether the built-in --completion flag is
// offered for this command: only on the root command of a flat CLI without a
// completion subcommand. It is the single authority used by the binder, help
// renderer, and completion engine so --completion handling stays consistent.
func (c *Command) CompletionFlagVisible(isRoot bool) bool {
	return isRoot && !c.DisableCompletion && !c.HasUserSubcommands() && !c.HasCompletionCommand()
}

// WhyFlagVisible reports whether the built-in --why flag is offered for this
// command: like --help, the flag is available on every command, and the
// provenance report it renders describes the whole invocation. It is the
// single authority used by the binder, help renderer, and completion engine
// so --why handling stays consistent.
func (c *Command) WhyFlagVisible() bool {
	return !c.DisableWhy
}

// FindSubcommand returns the subcommand matching name or alias, or nil.
func (c *Command) FindSubcommand(name string) *Command {
	for _, subcommand := range c.Subcommands {
		if subcommand.Name == name || slices.Contains(subcommand.Aliases, name) {
			return subcommand
		}
	}

	return nil
}

// FindFlag returns the flag matching name or alias, or nil.
func (c *Command) FindFlag(name string) *Flag {
	for _, flagSpec := range c.Flags {
		if flagSpec.Name == name || slices.Contains(flagSpec.Aliases, name) {
			return flagSpec
		}
	}

	return nil
}

// FindShortFlag returns the flag matching short rune r, or nil.
func (c *Command) FindShortFlag(r rune) *Flag {
	for _, flagSpec := range c.Flags {
		if flagSpec.Short == r {
			return flagSpec
		}
	}

	return nil
}

// Range represents parsed range bounds for numeric, length, or collection constraints.
type Range struct {
	MinStr string
	MaxStr string
	HasMin bool
	HasMax bool
	Exact  bool
}

// PredictorKind identifies the type of completion predictor.
type PredictorKind int

// PredictorSpec represents the parsed predictor configuration on a flag or positional argument.
type PredictorSpec struct {
	Kind      PredictorKind
	Name      string
	Patterns  []string
	Predictor predict.Predictor
}

// Flag represents the metadata and binder for a command-line flag.
type Flag struct {
	Name        string
	Short       rune
	Aliases     []string
	Default     string
	EnvVars     []string
	EnvExcluded bool
	ConfigKey   string
	XorGroup    string
	AndGroup    string
	Enum        []string
	Range       *Range
	Len         *Range
	Items       *Range
	// FieldTags holds the custom tag values declared on the flag that were
	// registered by its domain type (see spec.TagSpecProvider). Nil when the
	// flag's type registers no tags or declares none.
	FieldTags   FieldTags
	Help        string
	Placeholder string
	Group       string
	Hidden      bool
	// Safe reports whether the resolved value may be displayed in the --why
	// provenance report. Declared via the safe struct tag; an absent tag
	// defaults to true for boolean and counter flags (their value domain can
	// never carry credential material) and false for every other flag type.
	Safe         bool
	Deprecated   string
	Sep          string
	IsBool       bool
	IsSlice      bool
	IsMap        bool
	IsCounter    bool
	IsNegatable  bool
	IsConfigFile bool
	// EngineOwned marks flags synthesized by the engine rather than declared
	// by user struct fields. The engine binds them by identity and they never
	// resolve into user containers, so ancestor flag shadowing validation
	// exempts them: a same-named user flag on an ancestor stays reachable.
	EngineOwned bool
	// Required reports whether the flag was declared through a required
	// container (conga.RequiredFlag[T]).
	Required bool
	// Handle is the engine's opaque handle to the flag's container value. It is
	// never inspected by this package: package conga stores the field address
	// here and re-asserts its private engine interfaces at bind time. Nil when
	// the field was not classified.
	Handle     any
	Predictor  *PredictorSpec
	TargetType reflect.Type
}

// Arg represents the metadata and binder for a positional argument.
type Arg struct {
	Name    string
	Default string
	Help    string
	Enum    []string
	Range   *Range
	Len     *Range
	Items   *Range
	// FieldTags holds the custom tag values declared on the argument that
	// were registered by its domain type (see spec.TagSpecProvider). Nil
	// when the argument's type registers no tags or declares none.
	FieldTags  FieldTags
	Sep        string
	IsSlice    bool
	IsMap      bool
	Required   bool
	Predictor  *PredictorSpec
	TargetType reflect.Type
	// Handle is the engine's opaque handle to the argument's container value. It
	// is never inspected by this package: package conga stores the field address
	// here and re-asserts its private engine interfaces at bind time. Nil when
	// the field was not classified.
	Handle any
}

// DashArgs represents the binder for arguments explicitly passed after '--'.
type DashArgs struct {
	Name string
	Help string
	// Handle is the engine's opaque handle to the '--' trailing-arguments
	// container value. It is never inspected by this package and nil when not
	// classified.
	Handle any
}

// RawArgs represents the binder for unparsed trailing raw arguments.
type RawArgs struct {
	Name string
	Help string
	// Handle is the engine's opaque handle to the raw trailing-tokens
	// container value. It is never inspected by this package and nil when not
	// classified.
	Handle any
}

// ConfigSettings carries the app-level configuration-file resolution settings
// declared through the app.ConfigOptions(...) builder. They ride through schema build
// as this typed struct on the root command so resolution, validation, and
// static analysis share one source of truth. The "~"-prefixed struct tag
// namespace (~explicit, ~searchpaths, ~cascading) is a pure reservation:
// source-declared "~"-prefixed struct tags are rejected so the vocabulary
// stays engine-owned, and no diagnostic names these keys.
type ConfigSettings struct {
	// Explicit names one concrete configuration file with fatal-if-missing
	// semantics. It outranks the configuration environment variable and the
	// search tier.
	Explicit string

	// Tier lists the author-declared search paths probed in precedence order.
	// Declaring a tier replaces the built-in conventional XDG tier.
	Tier []string

	// Cascading merges every existing tier file in precedence order (lowest to
	// highest) instead of stopping at the first existing file.
	Cascading bool
}

// Options configure schema reflection, validation, and predictors.
type Options struct {
	StrictTagValidation     bool
	AllowUnknownTags        bool
	AllowUnknownTagsRecurse bool
	AllowedTags             []string
	RegisteredPredictors    map[string]predict.Predictor
	TypePredictors          map[reflect.Type]predict.Predictor
	ConfigFilePatterns      []string
	// ClassifyContainer maps a command-struct field address to its engine
	// classification (container kind, required flag, command target). Nil
	// leaves every field unclassified, so no field is treated as a container.
	ClassifyContainer func(any) container.Classification
	// ConfigSettings carries the app-level configuration resolution settings
	// (app.ConfigOptions options) onto the root command.
	ConfigSettings ConfigSettings
	ConfigCapable  bool
	DisableHelp    bool
	DisableVersion bool
	DisableWhy     bool
	// DisableCompletion excludes every completion entry point from the schema:
	// no `completion` subcommand is mounted and no `--completion` flag is offered.
	DisableCompletion bool
	// CompletionShells enumerates the shells the completion subsystem supports;
	// it is supplied by the caller only when completion is enabled.
	CompletionShells []string
	// InferPredictor returns the predictor spec inferred from a field's Go type
	// (Predictable, TagAwarePredictable, FileFilterable, DirFilterable,
	// PathFilterable) when the field declares no explicit predictor tag. It is
	// supplied by the caller only when completion is enabled; nil disables
	// type-based inference, and inferred predictors are consumed exclusively by
	// completion.
	InferPredictor func(targetType reflect.Type, tags FieldTags) *PredictorSpec
	EnvPrefix      string
}

// FindDeclaredFlagInChain searches backwards through chain for a declared flag
// matching name or alias, without the built-in --help/--version fallback.
func FindDeclaredFlagInChain(chain []*Command, name string) *Flag {
	for _, cmd := range slices.Backward(chain) {
		if flag := cmd.FindFlag(name); flag != nil {
			return flag
		}
	}

	return nil
}

// FindFlagInChain searches backwards through chain for a flag matching name or alias,
// falling back to the built-in --help and --version flags.
func FindFlagInChain(chain []*Command, name string) *Flag {
	if flag := FindDeclaredFlagInChain(chain, name); flag != nil {
		return flag
	}

	if len(chain) == 0 {
		return nil
	}

	leaf := chain[len(chain)-1]

	if name == "help" && !leaf.DisableHelp {
		return BuiltinHelpFlag
	}

	if name == "version" && leaf.VersionFlagVisible(len(chain) == 1) {
		return BuiltinVersionFlag
	}

	if name == "completion" && chain[0].CompletionFlagVisible(len(chain) == 1) {
		return BuiltinCompletionFlag
	}

	if name == "why" && chain[0].WhyFlagVisible() {
		return BuiltinWhyFlag
	}

	return nil
}

// FindShortFlagInChain searches backwards through chain for a flag matching short rune r.
func FindShortFlagInChain(chain []*Command, r rune) *Flag {
	for _, cmd := range slices.Backward(chain) {
		if flag := cmd.FindShortFlag(r); flag != nil {
			return flag
		}
	}

	return nil
}

// CanNegate reports whether flagSpec may be re-bound by a `no-`-prefixed
// token: only boolean negatable flags qualify. Every `no-` retry path (binder
// token parsing, flag-index lookup, and completion consumption) funnels
// through this guard so a `no-` token can never consume an unrelated flag.
func CanNegate(flagSpec *Flag) bool {
	return flagSpec != nil && flagSpec.IsBool && flagSpec.IsNegatable
}

// FindNegatableFlagInChain resolves the positive flag behind a `no-`-prefixed
// long flag token (e.g. --no-color resolving --color), enforcing the
// spec.CanNegate guard.
func FindNegatableFlagInChain(chain []*Command, name string) *Flag {
	posName, cut := strings.CutPrefix(name, "no-")
	if !cut {
		return nil
	}

	flag := FindFlagInChain(chain, posName)
	if !CanNegate(flag) {
		return nil
	}

	return flag
}
