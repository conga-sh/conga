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
	"github.com/conga-sh/conga/internal/configdata"
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
	// default tag. Changed() reports false for defaulted values.
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
// struct tags (such as [File] with the existing tag), implement [TagAwareHelpProvider]
// instead. Implement exactly one of [HelpProvider] or [TagAwareHelpProvider], not both.
type HelpProvider interface {
	// Help returns the value syntax description displayed in help screens.
	//
	// The engine consults the provider when the owning field declares no help
	// tag; a field with a help tag shows that text instead. Shells show the
	// provider's text as the description of the field's completion candidates.
	Help() string
}

// TagAwareHelpProvider is implemented by custom domain types whose help description
// depends on custom struct tags declared on the owning field (see [TagSpecProvider]).
//
// For example, [File] describes the field as an existing file, or as a file or stdin,
// depending on the declared tags.
//
// The engine prefers TagAwareHelpProvider over [HelpProvider]. Implement exactly one
// of the pair, not both.
type TagAwareHelpProvider interface {
	// Help returns the tailored help description for the field given its declared tags.
	//
	// The engine consults the provider when the owning field declares no help
	// tag; a field with a help tag shows that text instead. Shells show the
	// provider's text as the description of the field's completion candidates.
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
// true and [App.Exit] exits with status 2. For a built-in subcommand such as
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
// the glob tag) implement [TagAwarePredictable] instead.
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
// For example, [File] returns executable completions when the executable tag is declared,
// or glob-filtered completions when the glob tag is declared.
//
// The engine evaluates Predict during schema compilation; implementations must not
// depend on runtime parsed values. Implement exactly one of [Predictable] or
// [TagAwarePredictable], not both.
type TagAwarePredictable interface {
	// Predict returns a predictor tailored to the field's declared tags.
	Predict(tags FieldTags) predict.Predictor
}

// TagSpecProvider is implemented by custom domain types that define custom struct tags
// accepted on fields of their type (e.g. the existing tag on [File] and [Dir]).
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
// The value resolves from the command line, the environment, a configuration
// file, or a default struct tag, in that order of precedence. An omitted flag
// with no default holds the zero value of T.
type OptionalFlag[T any] struct {
	value        T
	set          bool
	changed      bool
	source       ValueSource
	sourceDetail string
}

// IsSet reports whether the flag received a value from any source, including
// explicit CLI arguments, environment variables, configuration files, or a
// struct default tag.
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
// Returns false if the value was populated exclusively from a struct default
// tag or if the flag was omitted.
func (f *OptionalFlag[T]) Changed() bool {
	return f.changed
}

// Source reports the configuration tier from which the flag value was resolved.
// See [ValueSource] for the tiers and their precedence.
func (f *OptionalFlag[T]) Source() ValueSource {
	return f.source
}

// SourceDetail returns the concrete origin recorded with [OptionalFlag.Source]:
// the flag token, the environment variable name, the contributing
// configuration origin (a file path for file-loaded values, or the loader's
// label), or the default tag literal. For a merged map flag the origin lists
// every contributing configuration file in load order, separated by commas.
// It is empty when the flag is unset.
func (f *OptionalFlag[T]) SourceDetail() string {
	return f.sourceDetail
}

// String returns the string representation of the resolved value. Returns an
// empty string if the receiver is nil.
func (f *OptionalFlag[T]) String() string {
	if f == nil {
		return ""
	}

	return stringx.Stringify(f.value)
}

// RequiredFlag represents a mandatory command-line flag of type T.
//
// The requirement is satisfied if a value is supplied by a command-line flag,
// an environment variable, a configuration file, or a struct default tag.
// If no source provides a value, Run returns a usage error before any lifecycle
// stage, starting with [Validator.Validate], executes; [IsUsageError] reports true and
// [App.Exit] exits with status 2.
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
// exclusively by a struct default tag.
func (f *RequiredFlag[T]) Changed() bool {
	return f.changed
}

// Source reports the configuration tier from which the flag value was resolved.
// See [ValueSource] for the tiers and their precedence.
func (f *RequiredFlag[T]) Source() ValueSource {
	return f.source
}

// SourceDetail returns the concrete origin recorded with [RequiredFlag.Source]:
// the flag token, the environment variable name, the contributing
// configuration origin (a file path for file-loaded values, or the loader's
// label), or the default tag literal. For a merged map flag the origin lists
// every contributing configuration file in load order, separated by commas.
// It is empty when the flag is unset.
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
// populated from a struct default tag.
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

// SourceDetail returns the default tag literal when the value came from a
// default tag, and an empty string when it came from the command line or is unset.
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
// declaration order. If omitted and no struct default tag is present, Run
// returns a usage error before any lifecycle stage, starting with [Validator.Validate],
// executes; [IsUsageError] reports true and [App.Exit] exits with status 2.
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

// SourceDetail returns the default tag literal when the value came from a
// default tag, and an empty string when it came from the command line.
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

// DashArgs captures trailing arguments passed explicitly after the -- token.
//
// All arguments following -- are captured verbatim without flag parsing.
//
// A DashArgs field accepts only the name and conga tags.
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

// RawArgs captures all trailing unparsed command-line arguments without requiring --.
//
// Parsing stops at the first unrecognized flag or argument, forwarding all remaining
// tokens into the container. This container supports proxy commands and CLI wrappers.
//
// A RawArgs field accepts only the name and conga tags.
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

// Command represents a subcommand container holding struct type T.
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

// HelpCommand provides an opt-in help subcommand.
//
// Declare a HelpCommand field directly on a command struct:
//
//	type CLI struct {
//		Help conga.HelpCommand `summary:"Show help for any command"`
//	}
//
// The help subcommand is a meta-command: it bypasses the execution lifecycle, so
// ancestor [PreRunner], [PostRunner], and [Finalizer] hooks do not run for it.
//
// See https://docs.conga.sh/reference/builtins/.
type HelpCommand struct {
	path []string
}

// Path returns the subcommand path whose help was requested, or empty when
// rendering the help screen of the parent command.
func (h *HelpCommand) Path() []string {
	if h == nil {
		return nil
	}

	return append([]string(nil), h.path...)
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
	if len(h.path) == 0 {
		fmt.Fprintln(session.Stdout(), help.FormatWith(parentChain, session.helpOptions))

		return nil
	}

	root := parentChain[0]
	targetChain := []*spec.Command{root}
	current := root

	for _, seg := range h.path {
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

// VersionCommand provides an opt-in version subcommand.
//
// Declare a VersionCommand field directly on a command struct:
//
//	type CLI struct {
//		Version conga.VersionCommand `summary:"Show version and build information"`
//	}
//
// The version subcommand is a meta-command: it bypasses the execution lifecycle,
// so ancestor [PreRunner], [PostRunner], and [Finalizer] hooks do not run for it.
//
// See https://docs.conga.sh/reference/builtins/.
type VersionCommand struct {
	json bool
}

// JSON reports whether the version output should be rendered as a JSON object.
func (v *VersionCommand) JSON() bool {
	return v != nil && v.json
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

	if v != nil && v.json {
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

// CompletionCommand provides an opt-in completion subcommand.
//
// Declare a CompletionCommand field directly on a command struct:
//
//	type CLI struct {
//		Completion conga.CompletionCommand `summary:"Generate shell completion scripts"`
//	}
//
// The field requires [App.EnableCompletion]; declaring it without that call fails
// schema build with an [ErrSchema] error naming the missing opt-in.
//
// The completion subcommand is a meta-command: it bypasses the execution
// lifecycle, so ancestor [PreRunner], [PostRunner], and [Finalizer] hooks do not run
// for it.
//
// See https://docs.conga.sh/reference/builtins/.
type CompletionCommand struct {
	shell string
}

// Shell returns the target shell name.
func (c *CompletionCommand) Shell() string {
	if c == nil {
		return ""
	}

	return c.shell
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

	// Invoked without a shell (e.g. app completion), the command renders its
	// help screen instead of failing: the supported-shells list and current-
	// shell enable hints are the guidance the caller is looking for.
	if c.shell == "" {
		fmt.Fprintln(session.Stdout(), session.Help())

		return nil
	}

	parsedShell, err := completion.ParseShell(c.shell)
	if err != nil {
		return &validationError{
			msg:      err.Error(),
			cmdChain: session.cmdChain,
			cause:    errInvalidValue,
		}
	}

	return generateCompletionScript(parsedShell, session.cmdChain[0], session.Stdout())
}

// Features declares the application's feature toggles through the fields of T.
//
// Each exported field of T declares one feature: the field name, converted to
// kebab-case (or overridden with the name tag), is the feature name. A bool
// field is an on/off toggle; an integer field is a leveled toggle where 0 means
// off and any other value must be listed in the enum tag.
//
//	type Experiments struct {
//		RemoteCache      bool `help:"Experimental distributed CAS artifact cache"`
//		SandboxIsolation int  `enum:"1,2" help:"Experimental build sandbox isolation: 1=chroot, 2=rootless-container"`
//	}
//
//	type CLI struct {
//		Features conga.Features[Experiments] `flag:"feature-gates" env:"APP_FEATURES" config:"experiments"`
//	}
//
//	app := conga.New[CLI]().EnableFeatures()
//
// Declare the Features field on the root command struct only. Activating
// feature toggles requires calling [App.EnableFeatures] on the application
// builder; declaring a Features field or a feature tag without it fails schema
// build with [ErrSchema].
//
// Every feature is
// carried by a single key=value map surface: the map flag named by the flag
// tag (omitted when the tag is absent), the environment surface declared on
// the container, and the configuration section named by config. The env
// surface is one of two exclusive shapes: env names a combined map variable
// carrying name=value pairs, and envprefix derives one variable per feature
// (<PREFIX>_<UPPER_SNAKE(feature name)>), with each field's env tag
// overriding its derived variable and env:"-" excluding the field. A
// container without either tag binds no environment variable, and the app
// EnvPrefix never reaches features. A key
// may use any spelling of a declared feature name (kebab-case, snake_case, or
// camelCase); keys are canonicalized before the merge, so precedence follows
// the tier rather than the spelling. The field defaults form the base set;
// config, environment, and command line sources merge over it key by key in
// ascending precedence (config < env < CLI), each source overlaying only the
// keys it lists (Kubernetes-style sparse overrides), so a config file can
// enable one feature while the command line enables another. Read the resolved
// values through [Features.Get]:
//
//	if cli.Features.Get().GentleForce { ... }
//
// The root help screen renders an automatic Features section listing every
// toggle: its help tag text, its stage tag maturity (alpha, beta, ga, or
// deprecated — optional, but all features must declare one when any does), and
// its default. Features staged ga or deprecated are omitted from the section.
//
// Flags and subcommands gated by the feature tag are pruned from help, why, and
// completion when their feature is inactive, and invoking them is a usage error.
//
// Gating enforcement is symmetric for explicit inputs: a gated flag supplied on
// the command line is a usage error even when only help was requested, and a
// configuration key naming a gated flag whose feature is inactive is rejected
// with [ErrConfig]. Environment variables are the exception: pruning removes
// the flag from the schema before environment binding, so a variable targeting
// an inactive gated flag is ignored rather than rejected, keeping ambient
// environment state from breaking runs it does not control.
//
// See https://docs.conga.sh/how-to/feature-flags/.
type Features[T any] struct {
	value        T
	source       ValueSource
	sourceDetail string
	provided     map[string]string
	resolved     map[string]string
	keySources   map[string]ValueSource
	keyOrigins   map[string]configdata.Value
}

// Get returns a pointer to the inner feature struct with every resolved value bound.
func (f *Features[T]) Get() *T {
	return &f.value
}

// String returns the string representation of the features struct. Returns an empty
// string if the receiver is nil.
func (f *Features[T]) String() string {
	if f == nil {
		return ""
	}

	return stringx.Stringify(f.value)
}

// IsSet reports whether any feature source contributed a value.
func (f *Features[T]) IsSet() bool { return f.source != SourceNone }

// Changed reports whether a feature value came from the command line, the
// environment, or a configuration file rather than exclusively from defaults.
func (f *Features[T]) Changed() bool { return f.source != SourceNone && f.source != SourceDefault }

// Source reports the highest configuration tier that contributed a key to the
// feature map.
func (f *Features[T]) Source() ValueSource { return f.source }

// SourceDetail returns the concrete origin recorded with [Features.Source].
func (f *Features[T]) SourceDetail() string { return f.sourceDetail }

// ConfigFile represents a root-level flag for loading configuration files.
//
// By default the flag holds a single file path (last provided value wins).
// Declaring the multi tag enables repeated --config-file flags and
// comma-separated paths to accumulate in [ConfigFile.Paths] for multi-file
// cascading merges:
//
//	type ServerCLI struct {
//		Config conga.ConfigFile `multi:"true" short:"c"`
//	}
//
//	app -c base.toml -c prod.toml
//
// The invocation above loads both files in order (later files override earlier
// ones on key conflicts) and reports both through [ConfigFile.Paths].
//
// Probed candidate paths that were absent on disk are reported by
// [Session.CandidateConfigFiles].
//
// See https://docs.conga.sh/how-to/manage-config-files/.
type ConfigFile struct {
	paths        []string
	set          bool
	changed      bool
	source       ValueSource
	sourceDetail string
}

// IsSet reports whether any configuration file path resolved: from a CLI flag,
// an environment variable, or the probed search tier.
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

// Changed reports whether configuration file paths were supplied by any source:
// a CLI flag, an environment variable, or the probed search tier. It mirrors
// [ConfigFile.IsSet]; a ConfigFile field accepts no default tag, so there is no
// default-only state.
func (c *ConfigFile) Changed() bool {
	return c.changed
}

// Source reports the configuration tier from which the configuration file
// paths resolved. See [ValueSource] for the tiers and their precedence.
func (c *ConfigFile) Source() ValueSource {
	return c.source
}

// SourceDetail returns the concrete origin recorded with [ConfigFile.Source].
//
//	app --config-file prod.toml
//
// records SourceCLI with the detail "--config-file"; paths loaded from the
// search tier record SourceConfig with the comma-joined loaded paths.
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
// Each occurrence on the command line increments the count by 1. With no
// occurrences and no other source, [Counter.Get] returns 0.
//
// Counters count command-line occurrences starting at 0, so a default tag is
// rejected at schema build. An environment or configuration file binding seeds
// a base level that command-line occurrences override; use [OptionalFlag] with
// an integer type for a counted value that needs a numeric default.
type Counter struct {
	value        int
	set          bool
	changed      bool
	source       ValueSource
	sourceDetail string
}

// IsSet reports whether the counter flag received a count from any source.
func (c *Counter) IsSet() bool { return c.set }

// Get returns the resolved count: command-line occurrences take precedence, then
// an environment or configuration file binding, then 0 when no source supplied
// one. See [Counter].
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

// Changed reports whether the count was explicitly supplied by a CLI flag,
// environment variable, or configuration file. It mirrors [Counter.IsSet];
// a Counter field accepts no default tag, so there is no default-only state.
func (c *Counter) Changed() bool {
	return c.changed
}

// Source reports the configuration tier from which the count was resolved.
// See [ValueSource] for the tiers and their precedence.
func (c *Counter) Source() ValueSource {
	return c.source
}

// SourceDetail returns the concrete origin recorded with [Counter.Source]:
// the flag token, the environment variable name, or the contributing
// configuration origin (a file path for file-loaded values, or the loader's
// label). It is empty when the counter is unset.
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

// setSelected restores the command selection after an observation-only
// completion invocation.
func (c *Command[T]) setSelected(selected bool) {
	c.selected = selected
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

// containerKind classifies the feature-declaration container for the engine,
// carrying the inner feature struct pointer so the schema builder can reflect
// over its fields.
func (f *Features[T]) containerKind() container.Classification {
	return container.Classification{Kind: container.KindFeatures, Target: &f.value}
}

// getValue returns the feature map resolved by the engine cascade with
// canonical feature-name keys (nil when no source supplied a value); the
// engine uses it for key validation.
func (f *Features[T]) getValue() any { return f.provided }

// setValue stores the resolved feature map (canonical feature-name keys) and
// records its source. The owning App later materializes the typed feature
// struct by overlaying the stored map onto the field defaults.
func (f *Features[T]) setValue(value any, source ValueSource, detail string) {
	provided, ok := value.(map[string]string)
	if !ok {
		panic(fmt.Sprintf("conga: invariant violation: expected map[string]string in Features.setValue, got %T", value))
	}

	f.provided = provided
	f.source = source
	f.sourceDetail = detail
}

func (f *Features[T]) setMapProvenance(sources map[string]ValueSource, origins map[string]configdata.Value) {
	f.keySources = sources
	f.keyOrigins = origins
}

func (f *Features[T]) mapProvenance() (map[string]ValueSource, map[string]configdata.Value) {
	return f.keySources, f.keyOrigins
}

func (f *Features[T]) setResolvedValues(values map[string]string) {
	f.resolved = values
}

func (f *Features[T]) resolvedValues() map[string]string {
	return f.resolved
}

func (f *Features[T]) resolveFeatures(cmd *spec.Command, cmdChain []*spec.Command, flagValues map[*spec.Flag][]string, state *bindingState) ([]prunedFlag, error) {
	return resolveFeatureGates(f, &f.value, cmd, cmdChain, flagValues, state)
}

func (f *Features[T]) rejectPrunedConfig(pruned []prunedFlag, configData *configdata.Data) error {
	return checkFeatureConfigRestrictions(pruned, configData)
}

// pruneForCompletion prunes inactive features for a completion invocation
// through the pure completion resolver, leaving the container's bound state
// untouched and keeping the feature payload behind the state-interface fence
// for dead-code elimination.
func (f *Features[T]) pruneForCompletion(cmd *spec.Command, flagValues map[*spec.Flag][]string, state *bindingState) {
	pruneFeaturesForCompletion(cmd, flagValues, state)
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
