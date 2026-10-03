package conga

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/conga-sh/conga/internal/appname"
	"github.com/conga-sh/conga/internal/coerce"
	"github.com/conga-sh/conga/internal/completion"
	"github.com/conga-sh/conga/internal/container"
	"github.com/conga-sh/conga/internal/help"
	"github.com/conga-sh/conga/internal/spec"
	"github.com/conga-sh/conga/internal/stringx"
	"github.com/conga-sh/conga/predict"
)

const (
	// SourceNone indicates no value was supplied and the container holds
	// its zero value.
	SourceNone = container.SourceNone

	// SourceDefault indicates the container value originated from a struct field's
	// `default:"..."` tag. Changed() reports false for defaulted values.
	SourceDefault = container.SourceDefault

	// SourceConfig indicates the container value was resolved from a configuration file.
	SourceConfig = container.SourceConfig

	// SourceEnv indicates the container value was resolved from an environment variable.
	SourceEnv = container.SourceEnv

	// SourceCLI indicates the container value was explicitly passed on the command line
	// as a flag or positional argument.
	SourceCLI = container.SourceCLI
)

const (
	// TagKindBool declares a strict boolean struct tag.
	//
	// Absence evaluates to false. Only "", "true", and "false" are accepted values;
	// typos and invalid strings fail during schema compilation rather than silently
	// misconfiguring the application.
	TagKindBool = spec.TagKindBool

	// TagKindString declares a raw string struct tag.
	//
	// Absence leaves the tag undeclared. An all-whitespace value fails during schema
	// compilation. A custom Validate hook can be attached to [TagSpec] to enforce
	// domain-specific syntax during schema compilation.
	TagKindString = spec.TagKindString
)

// errSessionUninitialized indicates a built-in command was invoked without an execution session.
var errSessionUninitialized = errors.New("conga: session is nil or uninitialized")

// ValueSource identifies the configuration tier from which a flag or argument was resolved.
//
// CONGA resolves flags through a four-tier precedence ladder:
//  1. [SourceCLI]: Explicit command-line arguments.
//  2. [SourceEnv]: Environment variables.
//  3. [SourceConfig]: Configuration files.
//  4. [SourceDefault]: Struct tag defaults.
//
// If none of these tiers provide a value, the source is [SourceNone].
type ValueSource = container.Source

// HelpProvider is implemented by custom domain types that provide a default
// help description of their accepted value format on generated help screens.
//
// For types whose help description varies depending on the owning field's custom
// struct tags (such as [File] with `existing:"true"`), implement [TagAwareHelpProvider]
// instead. Implement exactly one of [HelpProvider] or [TagAwareHelpProvider], not both.
type HelpProvider interface {
	// Help returns the value syntax description displayed in help screens.
	Help() string
}

// TagAwareHelpProvider is implemented by custom domain types whose help description
// depends on custom struct tags declared on the owning field (see [TagSpecProvider]).
//
// For example, [File] displays "Path to an existing file" when `existing:"true"` is declared,
// or "Path to a file or '-' for stdin" when `stdin:"true"` is declared.
//
// The engine prefers TagAwareHelpProvider over [HelpProvider]. Implement exactly one
// of the pair, not both.
type TagAwareHelpProvider interface {
	// Help returns the tailored help description for the field given its declared tags.
	Help(tags FieldTags) string
}

// Runner is implemented by command structs that execute primary application logic.
//
// The engine invokes Run only on the matched leaf command after argument parsing,
// top-down [Validator.Validate], and top-down [PreRunner.PreRun] hooks succeed.
// If Run returns an error, post-run hooks are skipped and execution jumps directly
// to [Finalizer.Finally] cleanup hooks.
//
// See https://docs.conga.sh/explanation/lifecycle/.
type Runner interface {
	// Run executes command logic for the active session.
	Run(ctx context.Context, session *Session) error
}

// Validator is implemented by command structs that perform cross-field semantic validation.
//
// Validate is the first lifecycle stage. Hooks run top-down from root to leaf, after
// struct tag constraints and domain-type [SelfValidator] checks have already passed,
// and before any [PreRunner], [Runner], [PostRunner], or [Finalizer] hook executes.
// If Validate returns an error, Run returns it without running the remaining stages
// (no command has been initialized, so no Finally hook runs); [IsUsageError] reports
// true and [App.FatalIfError] exits with status 2. For a built-in subcommand such as
// [HelpCommand], only the leaf command's Validate hook runs.
//
// See https://docs.conga.sh/explanation/lifecycle/.
type Validator interface {
	// Validate checks cross-field constraints across the command and its session.
	Validate(ctx context.Context, session *Session) error
}

// SelfValidator is implemented by custom flag and argument types that validate
// their resolved value after all configuration sources have been merged.
//
// The engine always passes a non-nil context derived from [App.RunContext].
// Types that accept custom field tags or engine streams capture them at decode time
// via [ContextAwareTextUnmarshaler] and validate against that captured state here.
type SelfValidator interface {
	// Validate enforces value-level constraints on the resolved domain object.
	Validate(ctx context.Context) error
}

// ContextAwareTextUnmarshaler is implemented by custom domain types that require
// the owning field's declared custom tags and/or engine standard streams during decoding.
//
// The coercion pipeline prefers ContextAwareTextUnmarshaler over [encoding.TextUnmarshaler].
// The passed [DecodeContext] contains custom tags parsed and validated at schema build
// (see [TagSpecProvider]) and the standard streams configured via [App.Stdin] and [App.Stdout].
//
// Direct invocations with a zero [DecodeContext] behave identically to standard text unmarshaling.
type ContextAwareTextUnmarshaler interface {
	// UnmarshalTextWithContext decodes raw text using ambient field tags and streams.
	UnmarshalTextWithContext(text []byte, decodeContext DecodeContext) error
}

// Predictable is implemented by custom domain types that provide shell autocompletion
// candidates.
//
// Types whose completion candidates depend on custom field tags (such as [File] with
// `glob:"*.yaml"`) implement [TagAwarePredictable] instead.
//
// The engine evaluates Predict during schema compilation; implementations must not
// depend on runtime parsed values. Implement exactly one of [Predictable] or
// [TagAwarePredictable], not both.
type Predictable interface {
	// Predict returns an autocompletion predictor for the domain type.
	Predict() predict.Predictor
}

// TagAwarePredictable is implemented by custom domain types whose autocompletion
// candidates are tailored to custom struct tags declared on the owning field.
//
// For example, [File] returns executable completions when `executable:"true"` is declared,
// or glob-filtered completions when `glob:"*.json"` is declared.
//
// The engine evaluates Predict during schema compilation; implementations must not
// depend on runtime parsed values. Implement exactly one of [Predictable] or
// [TagAwarePredictable], not both.
type TagAwarePredictable interface {
	// Predict returns a predictor tailored to the field's declared tags.
	Predict(tags FieldTags) predict.Predictor
}

// TagSpecProvider is implemented by custom domain types that define custom struct tags
// accepted on fields of their type (e.g. `existing:"true"` on [File] and [Dir]).
//
// Registered specs feed strict tag validation, schema-time value parsing, and tag
// mutual exclusion checks.
type TagSpecProvider = spec.TagSpecProvider

// TagSpec declares a single custom struct tag accepted by a domain type.
//
// Domain types register their specs through [TagSpecProvider]; the specs feed the
// strict tag-key vocabulary, schema-time value parsing, and combination exclusion
// checks for every field of that type. The fields are:
//
//   - Name: the struct tag key, without quotes (e.g. "existing").
//   - Kind: the value grammar, either [TagKindBool] or [TagKindString].
//   - Validate: an optional check of the raw value of a [TagKindString] tag, run at
//     schema build. Return a self-describing error; it is wrapped as a schema error.
//   - Excludes: tag keys that cannot be declared alongside this one. Declaring both
//     is a schema error.
//
// Implementing TagSpecs on a domain type also makes those tags available to
// [TagAwareHelpProvider] and [TagAwarePredictable] through [FieldTags].
type TagSpec = spec.TagSpec

// TagKind describes how a custom field tag's value is parsed and validated.
type TagKind = spec.TagKind

// FieldTags holds the custom tag values declared on the field owning a resolved
// value. Only tags registered by the field's domain type through [TagSpecProvider]
// appear; boolean values are canonicalized to "true" and "false", and string values
// are stored verbatim.
//
// The map is read with Has (was the tag declared at all), Bool (was it declared
// "true"), and String (its raw value, or "" when absent).
type FieldTags = spec.FieldTags

// DecodeContext carries ambient metadata for coercing a raw string into a domain type:
// the owning field's declared custom tags and the engine-configured standard streams
// ([App.Stdin] / [App.Stdout]).
//
// The zero value selects no tags and standard process streams ([os.Stdin] / [os.Stdout]).
// Unlike [context.Context], DecodeContext carries no cancellation or deadlines.
type DecodeContext = coerce.DecodeContext

// PreRunner is implemented by commands that initialize shared resources before [Runner.Run].
//
// Hooks execute top-down from root to leaf. If PreRun returns an error, execution stops
// immediately: subsequent PreRun hooks, Run, and PostRun hooks are skipped, jumping
// directly to [Finalizer.Finally] cleanup hooks.
//
// See https://docs.conga.sh/explanation/lifecycle/.
type PreRunner interface {
	// PreRun initializes command resources prior to execution.
	PreRun(ctx context.Context, session *Session) error
}

// PostRunner is implemented by commands that perform follow-up work after successful execution.
//
// Hooks execute bottom-up from leaf to root, but only if [Runner.Run] succeeded without error.
// If a PostRun hook returns an error, subsequent PostRun hooks are skipped, jumping directly
// to [Finalizer.Finally] cleanup hooks.
//
// See https://docs.conga.sh/explanation/lifecycle/.
type PostRunner interface {
	// PostRun executes post-command logic after Run succeeds.
	PostRun(ctx context.Context, session *Session) error
}

// Finalizer is implemented by commands that require guaranteed resource cleanup.
//
// Hooks execute bottom-up from leaf to root for all initialized commands, running
// unconditionally even when PreRun, Run, or PostRun return an error.
// The err parameter receives the error that halted execution, or nil if all prior stages
// succeeded.
//
// See https://docs.conga.sh/explanation/lifecycle/.
type Finalizer interface {
	// Finally releases resources, receiving the halting error or nil.
	Finally(ctx context.Context, session *Session, err error) error
}

// DescriptionProvider is implemented by subcommand structs to supply a detailed,
// multi-line overview for their generated help screen.
type DescriptionProvider interface {
	// Description returns the long-form overview for the subcommand's help screen.
	Description() string
}

// ExampleProvider is implemented by subcommand structs to supply usage examples
// for their generated help screen.
type ExampleProvider interface {
	// Examples returns usage examples formatted for the subcommand's help screen.
	Examples() string
}

// OptionalFlag represents an optional command-line flag of type T.
//
// An optional flag has three observable states:
//   - Omitted without a default: [OptionalFlag.IsSet] reports false, [OptionalFlag.Get] returns
//     the zero value of T, [OptionalFlag.GetOr] returns the caller-supplied fallback, and
//     [OptionalFlag.Source] reports [SourceNone].
//   - Omitted with a `default` struct tag: [OptionalFlag.IsSet] reports true, [OptionalFlag.Changed]
//     reports false, [OptionalFlag.Get] returns the parsed default, and [OptionalFlag.Source]
//     reports [SourceDefault].
//   - Explicitly provided via CLI, environment variable, or configuration file:
//     [OptionalFlag.IsSet] and [OptionalFlag.Changed] both report true, and [OptionalFlag.Source]
//     identifies the originating tier.
type OptionalFlag[T any] struct {
	value        T
	set          bool
	changed      bool
	source       ValueSource
	sourceDetail string
}

// IsSet reports whether the flag received a value from any source, including
// explicit CLI arguments, environment variables, configuration files, or a
// struct `default:"..."` tag.
func (f *OptionalFlag[T]) IsSet() bool { return f.set }

// Get returns the resolved flag value.
//
// If the flag was omitted and declared no struct default tag, Get returns the zero
// value of T. Use [OptionalFlag.GetOr] to provide an explicit fallback.
func (f *OptionalFlag[T]) Get() T {
	return f.value
}

// GetOr returns the resolved flag value if [OptionalFlag.IsSet] is true, or
// fallback otherwise.
func (f *OptionalFlag[T]) GetOr(fallback T) T {
	if !f.set {
		return fallback
	}

	return f.value
}

// Changed reports whether the flag value was explicitly supplied by the caller
// through a CLI flag, an environment variable, or a configuration file.
//
// Returns false if the value was populated exclusively from a struct `default:"..."`
// tag or if the flag was omitted.
func (f *OptionalFlag[T]) Changed() bool {
	return f.changed
}

// Source reports the configuration tier from which the flag value was resolved
// ([SourceCLI], [SourceEnv], [SourceConfig], [SourceDefault], or [SourceNone]).
func (f *OptionalFlag[T]) Source() ValueSource {
	return f.source
}

// SourceDetail returns contextual metadata about the resolution source.
//
// Depending on [OptionalFlag.Source], this returns the environment variable name
// (e.g. "PORT"), the configuration file path, the default tag literal, or the raw
// CLI flag token. Returns an empty string if [OptionalFlag.Source] is [SourceNone].
func (f *OptionalFlag[T]) SourceDetail() string {
	return f.sourceDetail
}

// String returns the string representation of the resolved value, formatting
// primitive types, slices, and custom Stringers. Returns an empty string if the
// receiver is nil.
func (f *OptionalFlag[T]) String() string {
	if f == nil {
		return ""
	}

	return stringx.Stringify(f.value)
}

// RequiredFlag represents a mandatory command-line flag of type T.
//
// The requirement is satisfied if a value is supplied by a command-line flag,
// an environment variable, a configuration file, or a struct `default:"..."` tag.
// If no source provides a value, Run returns a usage error before any lifecycle
// stage, starting with [Validator.Validate], executes; [IsUsageError] reports true and
// [App.FatalIfError] exits with status 2.
//
// When satisfied by explicit input, [RequiredFlag.Changed] reports true. When
// satisfied by a struct default tag, [RequiredFlag.IsSet] reports true while
// [RequiredFlag.Changed] stays false.
type RequiredFlag[T any] struct {
	value        T
	set          bool
	changed      bool
	source       ValueSource
	sourceDetail string
}

// IsSet reports whether the required flag received a value from any source.
func (f *RequiredFlag[T]) IsSet() bool { return f.set }

// Get returns the resolved flag value.
func (f *RequiredFlag[T]) Get() T {
	return f.value
}

// Changed reports whether the flag was explicitly provided by the caller via
// CLI, environment variable, or configuration file, returning false if satisfied
// exclusively by a struct `default:"..."` tag.
func (f *RequiredFlag[T]) Changed() bool {
	return f.changed
}

// Source reports the configuration tier from which the flag value was resolved
// ([SourceCLI], [SourceEnv], [SourceConfig], or [SourceDefault]).
func (f *RequiredFlag[T]) Source() ValueSource {
	return f.source
}

// SourceDetail returns contextual metadata about the resolution source, such as
// the environment variable name, configuration file path, or CLI flag token.
func (f *RequiredFlag[T]) SourceDetail() string {
	return f.sourceDetail
}

// String returns the string representation of the resolved value. Returns an
// empty string if the receiver is nil.
func (f *RequiredFlag[T]) String() string {
	if f == nil {
		return ""
	}

	return stringx.Stringify(f.value)
}

// OptionalArg represents an optional positional command-line argument of type T.
//
// Positional arguments resolve strictly from command-line arguments (not environment
// variables or configuration files) and are evaluated in declaration order after all
// required positional arguments.
//
// If omitted on the command line, [OptionalArg.IsSet] reports false and [OptionalArg.Get]
// returns the zero value of T.
type OptionalArg[T any] struct {
	value        T
	set          bool
	changed      bool
	source       ValueSource
	sourceDetail string
}

// IsSet reports whether a positional argument was supplied on the command line or
// populated from a struct `default:"..."` tag.
func (a *OptionalArg[T]) IsSet() bool { return a.set }

// Get returns the resolved positional argument value.
//
// If omitted without a default tag, Get returns the zero value of T. Use
// [OptionalArg.GetOr] to provide a fallback value.
func (a *OptionalArg[T]) Get() T {
	return a.value
}

// GetOr returns the resolved argument value if [OptionalArg.IsSet] is true, or
// fallback otherwise.
func (a *OptionalArg[T]) GetOr(fallback T) T {
	if !a.set {
		return fallback
	}

	return a.value
}

// Changed reports whether the argument was explicitly provided on the command line,
// returning false if populated from a default tag or omitted.
func (a *OptionalArg[T]) Changed() bool {
	return a.changed
}

// Source reports whether the argument came from [SourceCLI], [SourceDefault], or [SourceNone].
func (a *OptionalArg[T]) Source() ValueSource {
	return a.source
}

// SourceDetail returns additional context on the argument source (e.g. the raw token).
func (a *OptionalArg[T]) SourceDetail() string {
	return a.sourceDetail
}

// String returns the string representation of the argument value. Returns an empty
// string if the receiver is nil.
func (a *OptionalArg[T]) String() string {
	if a == nil {
		return ""
	}

	return stringx.Stringify(a.value)
}

// RequiredArg represents a mandatory positional command-line argument of type T.
//
// Required positional arguments must be supplied on the command line in struct
// declaration order. If omitted and no struct `default:"..."` tag is present, Run
// returns a usage error before any lifecycle stage, starting with [Validator.Validate],
// executes; [IsUsageError] reports true and [App.FatalIfError] exits with status 2.
type RequiredArg[T any] struct {
	value        T
	set          bool
	changed      bool
	source       ValueSource
	sourceDetail string
}

// IsSet reports whether the required argument received a value.
func (a *RequiredArg[T]) IsSet() bool { return a.set }

// Get returns the resolved positional argument value.
func (a *RequiredArg[T]) Get() T {
	return a.value
}

// Changed reports whether the argument was explicitly supplied on the command line
// rather than populated from a struct default tag.
func (a *RequiredArg[T]) Changed() bool {
	return a.changed
}

// Source reports whether the argument came from [SourceCLI] or [SourceDefault].
func (a *RequiredArg[T]) Source() ValueSource {
	return a.source
}

// SourceDetail returns additional context on the argument source.
func (a *RequiredArg[T]) SourceDetail() string {
	return a.sourceDetail
}

// String returns the string representation of the argument value. Returns an empty
// string if the receiver is nil.
func (a *RequiredArg[T]) String() string {
	if a == nil {
		return ""
	}

	return stringx.Stringify(a.value)
}

// DashArgs captures trailing arguments passed explicitly after the '--' token.
//
// All arguments following '--' are captured verbatim without flag parsing:
//
//	type RunCLI struct {
//		Verbose conga.OptionalFlag[bool] `short:"v" help:"Verbose output"`
//		Package conga.RequiredArg[string] `name:"package" help:"Package to run"`
//		Args    conga.DashArgs            `name:"flags"`
//	}
//
// When invoked as `app run -v pkg -- -f --target test`, Args.Get() contains
// `[]string{"-f", "--target", "test"}`.
//
// A DashArgs field accepts only the `name` and `conga` tags.
type DashArgs struct {
	rawSlice
}

// String returns the captured dash arguments formatted as a space-separated string.
// Returns an empty string if the receiver is nil.
func (raw *DashArgs) String() string {
	if raw == nil {
		return ""
	}

	return raw.rawSlice.String()
}

// RawArgs captures all trailing unparsed command-line arguments without requiring '--'.
//
// Parsing stops at the first unrecognized flag or argument, forwarding all remaining
// tokens into the container. This container is designed for proxy commands and CLI
// wrappers (such as `kubectl`, `ssh`, or `docker` wrappers):
//
//	type ExecCLI struct {
//		Verbose conga.OptionalFlag[bool] `short:"v" help:"Verbose output"`
//		Command conga.RawArgs            `name:"command"`
//	}
//
// When invoked as `app exec -v sh -c "echo hello"`, Command.Get() contains
// `[]string{"sh", "-c", "echo hello"}`.
//
// A RawArgs field accepts only the `name` and `conga` tags.
type RawArgs struct {
	rawSlice
}

// String returns the captured raw arguments formatted as a space-separated string.
// Returns an empty string if the receiver is nil.
func (raw *RawArgs) String() string {
	if raw == nil {
		return ""
	}

	return raw.rawSlice.String()
}

// Command represents a strongly-typed subcommand container holding struct type T.
//
// In parent commands and lifecycle hooks, call [Command.IsSelected] to check whether
// execution was routed to this subcommand. Call [Command.Get] to access the inner
// command struct T.
//
//	type RootCLI struct {
//		Serve conga.Command[ServeCLI] `summary:"Start HTTP server"`
//		Migrate conga.Command[MigrateCLI] `summary:"Run database migrations"`
//	}
//
// See https://docs.conga.sh/how-to/declare-subcommands/.
type Command[T any] struct {
	value    T
	selected bool
}

// IsSelected reports whether the engine routed execution to this subcommand.
// Use this in parent lifecycle hooks ([PreRunner], [PostRunner], [Finalizer])
// to determine which child branch is active.
func (c *Command[T]) IsSelected() bool {
	return c.selected
}

// Get returns a pointer to the inner command struct instance of type T.
func (c *Command[T]) Get() *T {
	return &c.value
}

// HelpCommand provides an opt-in `help [command...]` subcommand.
//
// Declare a HelpCommand field directly on a command struct — do not wrap it in
// [Command]:
//
//	type CLI struct {
//		Help conga.HelpCommand `summary:"Show help for any command"`
//	}
//
// The `help` subcommand is a meta-command: it bypasses the execution lifecycle, so
// ancestor [PreRunner], [PostRunner], and [Finalizer] hooks do not run for it.
//
// See https://docs.conga.sh/reference/builtins/.
type HelpCommand struct {
	// Path holds the subcommand path whose help was requested, populated by the
	// engine from positional tokens (e.g. `["cluster", "nodes"]` for `help cluster nodes`).
	// When empty, Run renders the help screen of the parent command.
	Path []string
}

// Run resolves the requested subcommand path and renders its help screen to
// [Session.Stdout].
//
// Panics:
//   - If ctx is nil.
func (h *HelpCommand) Run(ctx context.Context, session *Session) error {
	if ctx == nil {
		panic("conga: nil Context")
	}

	if session == nil || len(session.cmdChain) == 0 {
		return nil
	}

	parentChain := session.cmdChain[:len(session.cmdChain)-1]
	if len(h.Path) == 0 {
		fmt.Fprintln(session.Stdout(), help.FormatWith(parentChain, session.helpOptions))

		return nil
	}

	root := parentChain[0]
	targetChain := []*spec.Command{root}
	current := root

	for _, seg := range h.Path {
		var matched *spec.Command

		for _, subcommand := range current.Subcommands {
			if subcommand.Name == seg || slices.Contains(subcommand.Aliases, seg) {
				matched = subcommand

				break
			}
		}

		if matched == nil {
			return &parseError{
				msg:      fmt.Sprintf("unknown command %q", seg),
				token:    seg,
				cmdChain: targetChain,
				cause:    errUnknownCommand,
			}
		}

		current = matched
		targetChain = append(targetChain, current)
	}

	fmt.Fprintln(session.Stdout(), help.FormatWith(targetChain, session.helpOptions))

	return nil
}

// VersionCommand provides an opt-in `version [--json]` subcommand.
//
// Declare a VersionCommand field directly on a command struct — do not wrap it in
// [Command]:
//
//	type CLI struct {
//		Version conga.VersionCommand `summary:"Show version and build information"`
//	}
//
// The `version` subcommand is a meta-command: it bypasses the execution lifecycle,
// so ancestor [PreRunner], [PostRunner], and [Finalizer] hooks do not run for it.
//
// See https://docs.conga.sh/reference/builtins/.
type VersionCommand struct {
	// JSON indicates whether the version output should be rendered as a flat
	// machine-readable JSON object instead of the human-readable text block.
	JSON bool
}

// Run renders detailed application version information to [Session.Stdout].
//
// If [VersionCommand.JSON] is true, Run renders a machine-readable JSON payload.
//
// Panics:
//   - If ctx is nil.
func (v *VersionCommand) Run(ctx context.Context, session *Session) error {
	if ctx == nil {
		panic("conga: nil Context")
	}

	if session == nil || session.stdout == nil {
		return nil
	}

	if v != nil && v.JSON {
		fmt.Fprintln(session.Stdout(), help.VersionJSON(session.appVersion))

		return nil
	}

	name := session.appName
	if invoked, ok := appname.Binary(); ok {
		name = invoked
	}

	fmt.Fprintln(session.Stdout(), help.VersionDetailed(name, session.appVersion))

	return nil
}

// CompletionCommand provides an opt-in `completion <shell>` subcommand.
//
// Declare a CompletionCommand field directly on a command struct — do not wrap it
// in [Command]:
//
//	type CLI struct {
//		Completion conga.CompletionCommand `summary:"Generate shell completion scripts"`
//	}
//
// The field requires [App.EnableCompletion]; without that call the subcommand is
// dropped from the schema with no warning.
//
// The `completion` subcommand is a meta-command: it bypasses the execution
// lifecycle, so ancestor [PreRunner], [PostRunner], and [Finalizer] hooks do not run
// for it.
//
// See https://docs.conga.sh/reference/builtins/.
type CompletionCommand struct {
	// Shell holds the target shell name ("bash", "zsh", "fish", "powershell",
	// "elvish", "nushell"). When empty, Run renders usage guidance listing supported shells.
	Shell string
}

// Run generates and outputs the shell completion script for the requested shell
// to [Session.Stdout].
//
// Panics:
//   - If ctx is nil.
func (c *CompletionCommand) Run(ctx context.Context, session *Session) error {
	if ctx == nil {
		panic("conga: nil Context")
	}

	if session == nil || len(session.cmdChain) == 0 {
		return errSessionUninitialized
	}

	// Invoked without a shell (e.g. `app completion`), the command renders its
	// help screen instead of failing: the supported-shells list and current-
	// shell enable hints are the guidance the caller is looking for.
	if c.Shell == "" {
		fmt.Fprintln(session.Stdout(), help.FormatWith(session.cmdChain, session.helpOptions))

		return nil
	}

	parsedShell, err := completion.ParseShell(c.Shell)
	if err != nil {
		return &validationError{
			msg:      err.Error(),
			cmdChain: session.cmdChain,
			cause:    errInvalidValue,
		}
	}

	return generateCompletionScript(parsedShell, session.cmdChain[0], session.Stdout())
}

// ConfigFile represents a root-level flag for loading configuration files.
//
// By default, the flag holds a single file path (last provided value wins). Declaring
// `multi:"true"` enables repeated `--config-file` flags and comma-separated paths
// to accumulate in [ConfigFile.Paths] for multi-file cascading merges.
//
// Probed candidate paths that were absent on disk are reported by [Session.CandidateConfigFiles].
//
// See https://docs.conga.sh/how-to/manage-config-files/.
type ConfigFile struct {
	paths        []string
	set          bool
	changed      bool
	source       ValueSource
	sourceDetail string
}

// IsSet reports whether configuration file paths were supplied by any source
// (CLI, environment, or default discovery).
func (c *ConfigFile) IsSet() bool { return c.set }

// Path returns the effective configuration file path: the highest-precedence loaded
// file, or an empty string if no configuration file was loaded.
func (c *ConfigFile) Path() string {
	if len(c.paths) == 0 {
		return ""
	}

	return c.paths[len(c.paths)-1]
}

// Paths returns a copy of the configuration files actually loaded for this flag,
// ordered from lowest to highest precedence.
//
// To inspect probed candidate paths that were absent on disk, call
// [Session.CandidateConfigFiles].
func (c *ConfigFile) Paths() []string {
	return append([]string(nil), c.paths...)
}

// Changed reports whether configuration file paths were explicitly passed via
// CLI or environment variable (not automatically discovered by the engine).
func (c *ConfigFile) Changed() bool {
	return c.changed
}

// Source reports where the configuration file paths originated.
func (c *ConfigFile) Source() ValueSource {
	return c.source
}

// SourceDetail returns additional context on the source (e.g. the CLI flag token or environment variable name).
func (c *ConfigFile) SourceDetail() string {
	return c.sourceDetail
}

// String returns the comma-separated list of loaded configuration file paths.
// Returns an empty string if the receiver is nil.
func (c *ConfigFile) String() string {
	if c == nil {
		return ""
	}

	return strings.Join(c.paths, ", ")
}

// Counter represents a repeatable boolean flag that counts its occurrences.
//
// Each occurrence on the command line increments the count by 1:
//
//	type CLI struct {
//		Verbose conga.Counter `short:"v" help:"Increase verbosity level (-v, -vvv)"`
//	}
//
// When invoked as `app -vvv`, Verbose.Get() returns 3. With no occurrences and no
// `default` tag, Verbose.Get() returns 0.
//
// A `default:"N"` tag seeds the count when the flag is omitted, and
// [Counter.Changed] reports false for a defaulted count. An explicit occurrence
// replaces that seed rather than adding to it: with `default:"2"`, `app -v`
// resolves to 1, not 3.
type Counter struct {
	value        int
	set          bool
	changed      bool
	source       ValueSource
	sourceDetail string
}

// IsSet reports whether the counter flag received a count from any source,
// including a `default:"..."` tag.
func (c *Counter) IsSet() bool { return c.set }

// Get returns the resolved occurrence count: the number of command-line
// occurrences, the `default` value when the flag is omitted, or 0 when neither
// applies. An explicit occurrence replaces a defaulted count; see [Counter].
func (c *Counter) Get() int {
	return c.value
}

// GetOr returns the occurrence count if [Counter.IsSet] is true, or fallback
// otherwise.
func (c *Counter) GetOr(fallback int) int {
	if !c.set {
		return fallback
	}

	return c.value
}

// Changed reports whether the count came from an explicit command-line occurrence
// rather than a `default:"..."` tag.
func (c *Counter) Changed() bool {
	return c.changed
}

// Source reports where the counter value originated.
func (c *Counter) Source() ValueSource {
	return c.source
}

// SourceDetail returns additional context on the source.
func (c *Counter) SourceDetail() string {
	return c.sourceDetail
}

// String returns the count formatted as a decimal string. Returns "0" if the receiver is nil.
func (c *Counter) String() string {
	if c == nil {
		return "0"
	}

	return strconv.Itoa(c.value)
}

type rawSlice struct {
	args []string
	set  bool
}

// IsSet reports whether trailing arguments were captured for the container.
func (raw *rawSlice) IsSet() bool {
	return raw.set
}

// Get returns a slice of the captured trailing arguments.
func (raw *rawSlice) Get() []string {
	return raw.args
}

// GetOr returns the captured arguments if set and non-empty, or fallback if empty.
func (raw *rawSlice) GetOr(fallback []string) []string {
	if !raw.set || len(raw.args) == 0 {
		return fallback
	}

	return raw.args
}

// Len returns the number of captured arguments.
func (raw *rawSlice) Len() int {
	return len(raw.args)
}

// String returns the captured arguments formatted as a space-separated string.
// Returns an empty string if the receiver is nil.
func (raw *rawSlice) String() string {
	if raw == nil {
		return ""
	}

	return strings.Join(raw.args, " ")
}

// setArgs stores the trailing arguments captured by the engine.
func (raw *rawSlice) setArgs(args []string) {
	raw.args = args
	raw.set = true
}

// getValue returns the resolved value for the engine.
func (f *OptionalFlag[T]) getValue() any { return f.value }

// setValue binds a value resolved by the engine cascade and records its
// source; it panics on an unexpected value type.
func (f *OptionalFlag[T]) setValue(value any, source ValueSource, detail string) {
	v, ok := value.(T)
	if !ok {
		panic(fmt.Sprintf("conga: invariant violation: expected %T in OptionalFlag.setValue, got %T", *new(T), value))
	}

	f.value = v
	f.set = true
	f.changed = source != SourceDefault
	f.source = source
	f.sourceDetail = detail
}

// containerKind classifies the optional flag for the engine.
func (f *OptionalFlag[T]) containerKind() container.Classification {
	return container.Classification{Kind: container.KindFlag}
}

// getValue returns the resolved value for the engine.
func (f *RequiredFlag[T]) getValue() any { return f.value }

// setValue binds a value resolved by the engine cascade and records its
// source; it panics on an unexpected value type.
func (f *RequiredFlag[T]) setValue(value any, source ValueSource, detail string) {
	v, ok := value.(T)
	if !ok {
		panic(fmt.Sprintf("conga: invariant violation: expected %T in RequiredFlag.setValue, got %T", *new(T), value))
	}

	f.value = v
	f.set = true
	f.changed = source != SourceDefault
	f.source = source
	f.sourceDetail = detail
}

// containerKind classifies the required flag for the engine.
func (f *RequiredFlag[T]) containerKind() container.Classification {
	return container.Classification{Kind: container.KindFlag, Required: true}
}

// getValue returns the resolved value for the engine.
func (a *OptionalArg[T]) getValue() any { return a.value }

// setValue binds a value resolved by the engine cascade and records its
// source; it panics on an unexpected value type.
func (a *OptionalArg[T]) setValue(value any, source ValueSource, detail string) {
	v, ok := value.(T)
	if !ok {
		panic(fmt.Sprintf("conga: invariant violation: expected %T in OptionalArg.setValue, got %T", *new(T), value))
	}

	a.value = v
	a.set = true
	a.changed = source != SourceDefault
	a.source = source
	a.sourceDetail = detail
}

// containerKind classifies the optional argument for the engine.
func (a *OptionalArg[T]) containerKind() container.Classification {
	return container.Classification{Kind: container.KindArg}
}

// getValue returns the resolved value for the engine.
func (a *RequiredArg[T]) getValue() any { return a.value }

// setValue binds a value resolved by the engine cascade and records its
// source; it panics on an unexpected value type.
func (a *RequiredArg[T]) setValue(value any, source ValueSource, detail string) {
	v, ok := value.(T)
	if !ok {
		panic(fmt.Sprintf("conga: invariant violation: expected %T in RequiredArg.setValue, got %T", *new(T), value))
	}

	a.value = v
	a.set = true
	a.changed = source != SourceDefault
	a.source = source
	a.sourceDetail = detail
}

// containerKind classifies the required argument for the engine.
func (a *RequiredArg[T]) containerKind() container.Classification {
	return container.Classification{Kind: container.KindArg, Required: true}
}

// containerKind classifies the '--' trailing-arguments collector for the engine.
func (raw *DashArgs) containerKind() container.Classification {
	return container.Classification{Kind: container.KindDashArgs}
}

// containerKind classifies the raw trailing-tokens collector for the engine.
func (raw *RawArgs) containerKind() container.Classification {
	return container.Classification{Kind: container.KindRawArgs}
}

// innerValue returns a pointer to the inner command struct for the engine;
// user code receives the same pointer via Get.
func (c *Command[T]) innerValue() any {
	return &c.value
}

// markSelected records that the engine routed execution to this command.
func (c *Command[T]) markSelected() {
	c.selected = true
}

// containerKind classifies the subcommand container for the engine, carrying
// the inner command struct pointer so the schema builder can reflect over it.
func (c *Command[T]) containerKind() container.Classification {
	return container.Classification{Kind: container.KindCommand, Target: c.innerValue()}
}

// containerKind classifies the opt-in help subcommand for the engine.
func (h *HelpCommand) containerKind() container.Classification {
	return container.Classification{Kind: container.KindHelpCommand, Target: h}
}

// containerKind classifies the opt-in version subcommand for the engine.
func (v *VersionCommand) containerKind() container.Classification {
	return container.Classification{Kind: container.KindVersionCommand, Target: v}
}

// containerKind classifies the opt-in completion subcommand for the engine.
func (c *CompletionCommand) containerKind() container.Classification {
	return container.Classification{Kind: container.KindCompletionCommand, Target: c}
}

// getValue returns the loaded configuration file paths for the engine.
func (c *ConfigFile) getValue() any { return c.paths }

// setValue binds configuration file paths resolved by the engine cascade and
// records their source; it accepts a scalar string for single-arity flags and
// a []string for multi-arity flags or engine-loaded resolution, and panics on
// any other value type.
func (c *ConfigFile) setValue(value any, source ValueSource, detail string) {
	switch typedValue := value.(type) {
	case string:
		c.paths = []string{typedValue}
	case []string:
		c.paths = typedValue
	default:
		panic(fmt.Sprintf("conga: invariant violation: expected string or []string in ConfigFile.setValue, got %T", value))
	}

	c.set = true
	c.changed = source != SourceDefault
	c.source = source
	c.sourceDetail = detail
}

// containerKind classifies the configuration-file flag for the engine.
func (c *ConfigFile) containerKind() container.Classification {
	return container.Classification{Kind: container.KindConfigFile}
}

// getValue returns the resolved count for the engine.
func (c *Counter) getValue() any { return c.value }

// setValue binds a count resolved by the engine cascade and records its
// source; it panics on an unexpected value type.
func (c *Counter) setValue(value any, source ValueSource, detail string) {
	v, ok := value.(int)
	if !ok {
		panic(fmt.Sprintf("conga: invariant violation: expected int in Counter.setValue, got %T", value))
	}

	c.value = v
	c.set = true
	c.changed = source != SourceDefault
	c.source = source
	c.sourceDetail = detail
}

// containerKind classifies the counter flag for the engine.
func (c *Counter) containerKind() container.Classification {
	return container.Classification{Kind: container.KindCounter}
}
