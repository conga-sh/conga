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
	// SourceNone indicates the value was not provided from any source.
	SourceNone = container.SourceNone

	// SourceDefault indicates the value came from the struct tag's `default` definition.
	SourceDefault = container.SourceDefault

	// SourceConfig indicates the value was resolved from a configuration file.
	SourceConfig = container.SourceConfig

	// SourceEnv indicates the value was resolved from an environment variable.
	SourceEnv = container.SourceEnv

	// SourceCLI indicates the value was explicitly passed on the command line.
	SourceCLI = container.SourceCLI
)

const (
	// TagKindBool declares a strict boolean tag value: absence means false,
	// while "", "true", and "false" are the only accepted values. Typos fail
	// at schema build instead of silently inverting the tag's intent.
	TagKindBool = spec.TagKindBool

	// TagKindString declares a raw string tag value: absence leaves the tag
	// undeclared, an empty (all-whitespace) value fails at schema build, and
	// the spec's optional Validate hook (see TagSpec) validates the raw
	// value at schema build.
	TagKindString = spec.TagKindString
)

// errSessionUninitialized indicates a built-in command was invoked without an execution session.
var errSessionUninitialized = errors.New("conga: session is nil or uninitialized")

// ValueSource identifies the origin of a resolved flag or argument value.
type ValueSource = container.Source

// HelpProvider is implemented by custom domain types that provide a default
// help description for their accepted value format. Types that need the
// owning field's declared custom tags implement TagAwareHelpProvider
// instead; the engine consults the tag-aware form first and falls back to
// this parameterless form. Implement exactly one form of the pair, not both.
type HelpProvider interface {
	Help() string
}

// TagAwareHelpProvider is implemented by custom domain types whose default
// help description is tailored to the custom field tags declared on the
// owning field (see TagSpecProvider; e.g. File with existing:"true"). tags is
// empty for fields that declare no registered tags. The engine prefers this
// form over the plain HelpProvider hook. Implement exactly one form of the
// pair, not both.
type TagAwareHelpProvider interface {
	Help(tags FieldTags) string
}

// Runner is implemented by commands that execute application logic.
type Runner interface {
	Run(ctx context.Context, session *Session) error
}

// Validator is implemented by commands that perform side-effect-free constraint validation.
type Validator interface {
	Validate(ctx context.Context, session *Session) error
}

// SelfValidator is implemented by custom flag/arg types that validate their
// resolved value during the validation phase, after the full configuration
// cascade has resolved the final value. The engine always passes a non-nil
// context derived from RunContext; implementations never need to nil-check
// it. Types that accept custom field tags or engine streams capture them at
// decode time via ContextAwareTextUnmarshaler and read the captured state here.
type SelfValidator interface {
	Validate(ctx context.Context) error
}

// ContextAwareTextUnmarshaler is implemented by domain types that need the
// owning field's declared tags and/or the engine's ambient environment during
// value decoding. The coercion pipeline prefers it over plain
// encoding.TextUnmarshaler, passing a DecodeContext with the tags declared on
// the owning field, parsed and validated at schema build (see
// TagSpecProvider), and the streams configured via App.Stdin/App.Stdout. The
// zero DecodeContext (no tags, process streams) makes direct calls behave
// identically to UnmarshalText. Plain encoding.TextUnmarshaler implementations
// of the same type parse an untagged, unconstrained value.
type ContextAwareTextUnmarshaler interface {
	UnmarshalTextWithContext(text []byte, decodeContext DecodeContext) error
}

// Predictable is implemented by custom domain types that define their own
// autocompletion candidates. Types that need the owning field's declared
// custom tags implement TagAwarePredictable instead; the engine consults the
// tag-aware form first and falls back to this parameterless form. The engine
// consults Predict at schema build, so implementations must not depend on
// parsed values. Implement exactly one form of the pair, not both.
type Predictable interface {
	Predict() predict.Predictor
}

// TagAwarePredictable is implemented by custom domain types whose completion
// candidates are tailored to the custom field tags declared on the owning
// field (see TagSpecProvider; e.g. File with executable:"true"). tags is
// empty for fields that declare no registered tags. The engine prefers this
// form over the plain Predictable hook and consults it at schema build, so
// implementations must not depend on parsed values. Implement exactly one
// form of the pair, not both.
type TagAwarePredictable interface {
	Predict(tags FieldTags) predict.Predictor
}

// TagSpecProvider is implemented by domain types that accept custom struct
// tags on the fields that use them. The registered specs feed the strict
// tag-key vocabulary, schema-time value parsing, and combination exclusion
// checks for every field of the type.
type TagSpecProvider = spec.TagSpecProvider

// TagSpec declares a single custom struct tag accepted by a domain type.
type TagSpec = spec.TagSpec

// TagKind describes how a custom field tag's value is parsed and validated.
type TagKind = spec.TagKind

// FieldTags holds the custom tag values declared on the field owning a
// resolved value. Only tags registered by the field's domain type appear;
// boolean values are canonicalized to "true" and "false".
type FieldTags = spec.FieldTags

// DecodeContext carries the ambient input for coercing one raw value into
// its target type: the owning field's declared custom tags and the
// engine-configured standard streams (App.Stdin / App.Stdout). The zero
// value selects no tags and process defaults (os.Stdin/os.Stdout), so
// values decoded outside an App run behave identically to direct
// UnmarshalText calls. It is a plain value struct: unlike context.Context
// it carries no cancellation or deadlines.
type DecodeContext = coerce.DecodeContext

// PreRunner is implemented by commands that initialize resources before execution.
type PreRunner interface {
	PreRun(ctx context.Context, session *Session) error
}

// PostRunner is implemented by commands that execute follow-up logic only after a successful Run.
type PostRunner interface {
	PostRun(ctx context.Context, session *Session) error
}

// Finalizer is implemented by commands that perform guaranteed resource cleanup/teardown.
type Finalizer interface {
	Finally(ctx context.Context, session *Session, err error) error
}

// DescriptionProvider is implemented by subcommand structs to provide a
// detailed multi-line overview for their own help screen.
type DescriptionProvider interface {
	Description() string
}

// ExampleProvider is implemented by subcommand structs that provide usage examples.
type ExampleProvider interface {
	Examples() string
}

// OptionalFlag represents an optional command-line flag of type T.
//
// Supported struct tags: name, short, help, default, env, aliases, group,
// hidden, xor, and, enum, range, len, items, sep, negatable,
// placeholder, config, predictor, and conga directives. See
// docs/struct-tags.md for the complete reference.
type OptionalFlag[T any] struct {
	value        T
	set          bool
	changed      bool
	source       ValueSource
	sourceDetail string
}

// IsSet reports whether a value has been supplied by any source.
func (f *OptionalFlag[T]) IsSet() bool { return f.set }

// Get returns the resolved value.
func (f *OptionalFlag[T]) Get() T {
	return f.value
}

// GetOr returns the value if set, or fallback if omitted.
func (f *OptionalFlag[T]) GetOr(fallback T) T {
	if !f.set {
		return fallback
	}

	return f.value
}

// Changed returns true if a flag value was explicitly passed via CLI, env, or config (not defaults).
func (f *OptionalFlag[T]) Changed() bool {
	return f.changed
}

// Source returns where the value originated.
func (f *OptionalFlag[T]) Source() ValueSource {
	return f.source
}

// SourceDetail returns additional context on the source (e.g. the env var name, the config file path, or the explicit flag value).
func (f *OptionalFlag[T]) SourceDetail() string {
	return f.sourceDetail
}

// String implements fmt.Stringer.
func (f *OptionalFlag[T]) String() string {
	if f == nil {
		return ""
	}

	return stringx.Stringify(f.value)
}

// RequiredFlag represents a mandatory flag container on a CLI command struct.
//
// Supported struct tags: name, short, help, default, env, aliases, placeholder,
// group, hidden, sep, xor, and, enum, len, range, items, config, and conga directives.
// See docs/struct-tags.md for the complete reference.
type RequiredFlag[T any] struct {
	value        T
	set          bool
	changed      bool
	source       ValueSource
	sourceDetail string
}

// IsSet reports whether a value has been supplied (CLI, env, or config). The
// binder uses it at parse time to detect a still-missing required value.
func (f *RequiredFlag[T]) IsSet() bool { return f.set }

// Get returns the resolved value.
func (f *RequiredFlag[T]) Get() T {
	return f.value
}

// Changed returns true if the value was explicitly provided (CLI, env, or
// config), not from a default. A default satisfies the requirement just like
// explicit input does, so IsSet reports true for a defaulted value while
// Changed stays false.
func (f *RequiredFlag[T]) Changed() bool {
	return f.changed
}

// Source returns where the value originated.
func (f *RequiredFlag[T]) Source() ValueSource {
	return f.source
}

// SourceDetail returns additional context on the source (e.g. env var name or config key).
func (f *RequiredFlag[T]) SourceDetail() string {
	return f.sourceDetail
}

// String implements fmt.Stringer.
func (f *RequiredFlag[T]) String() string {
	if f == nil {
		return ""
	}

	return stringx.Stringify(f.value)
}

// OptionalArg represents an optional positional command-line argument of type T.
//
// Supported struct tags: name, help, default, enum, range, len, items, sep,
// predictor, and conga directives. See docs/struct-tags.md for the complete
// reference.
type OptionalArg[T any] struct {
	value        T
	set          bool
	changed      bool
	source       ValueSource
	sourceDetail string
}

// IsSet reports whether a value has been supplied by any source.
func (a *OptionalArg[T]) IsSet() bool { return a.set }

// Get returns the resolved value.
func (a *OptionalArg[T]) Get() T {
	return a.value
}

// GetOr returns the value if set, or fallback if omitted.
func (a *OptionalArg[T]) GetOr(fallback T) T {
	if !a.set {
		return fallback
	}

	return a.value
}

// Changed returns true if the value was explicitly provided (CLI), not from a default.
func (a *OptionalArg[T]) Changed() bool {
	return a.changed
}

// Source returns where the value originated.
func (a *OptionalArg[T]) Source() ValueSource {
	return a.source
}

// SourceDetail returns additional context on the source.
func (a *OptionalArg[T]) SourceDetail() string {
	return a.sourceDetail
}

// String implements fmt.Stringer.
func (a *OptionalArg[T]) String() string {
	if a == nil {
		return ""
	}

	return stringx.Stringify(a.value)
}

// RequiredArg represents a mandatory positional command-line argument of type T.
//
// Supported struct tags: name, help, default, enum, range, len, items, sep,
// predictor, and conga directives. See docs/struct-tags.md for the complete
// reference.
type RequiredArg[T any] struct {
	value        T
	set          bool
	changed      bool
	source       ValueSource
	sourceDetail string
}

// IsSet reports whether a value has been supplied. The binder uses it at
// parse time to detect a still-missing required value.
func (a *RequiredArg[T]) IsSet() bool { return a.set }

// Get returns the resolved value.
func (a *RequiredArg[T]) Get() T {
	return a.value
}

// Changed returns true if the value was explicitly provided (CLI), not from a
// default. A default satisfies the requirement just like explicit input does,
// so IsSet reports true for a defaulted value while Changed stays false.
func (a *RequiredArg[T]) Changed() bool {
	return a.changed
}

// Source returns where the value originated.
func (a *RequiredArg[T]) Source() ValueSource {
	return a.source
}

// SourceDetail returns additional context on the source.
func (a *RequiredArg[T]) SourceDetail() string {
	return a.sourceDetail
}

// String implements fmt.Stringer.
func (a *RequiredArg[T]) String() string {
	if a == nil {
		return ""
	}

	return stringx.Stringify(a.value)
}

// DashArgs captures trailing arguments passed explicitly after '--'.
//
// Supported struct tags: name, help, and conga directives. See
// docs/struct-tags.md for the complete reference.
type DashArgs struct {
	rawSlice
}

// String implements fmt.Stringer.
func (raw *DashArgs) String() string {
	if raw == nil {
		return ""
	}

	return raw.rawSlice.String()
}

// RawArgs captures all trailing unparsed tokens (flags and arguments) without requiring '--'.
//
// Supported struct tags: name, help, and conga directives. See
// docs/struct-tags.md for the complete reference.
type RawArgs struct {
	rawSlice
}

// String implements fmt.Stringer.
func (raw *RawArgs) String() string {
	if raw == nil {
		return ""
	}

	return raw.rawSlice.String()
}

// Command represents a strongly-typed subcommand container of type T.
//
// Supported struct tags: name, help, aliases, group, hidden, envprefix, and
// conga directives. See docs/struct-tags.md for the complete reference.
type Command[T any] struct {
	value    T
	selected bool
}

// IsSelected reports whether the engine routed execution to this command; use
// it in lifecycle hooks to detect which subcommand is active.
func (c *Command[T]) IsSelected() bool {
	return c.selected
}

// Get returns the pointer to the inner command struct T.
func (c *Command[T]) Get() *T {
	return &c.value
}

// HelpCommand provides an opt-in `help [command...]` subcommand.
type HelpCommand struct {
	// Path holds the subcommand path whose help was requested, populated by
	// the engine from the positional arguments (e.g. ["deploy", "worker"] for
	// `help deploy worker`). It is empty when no subcommand path was given, in
	// which case Run renders the help of the chain containing help itself.
	Path []string
}

// Run resolves the requested subcommand path and renders its help screen.
// Panics if ctx is nil.
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

// VersionCommand provides an opt-in `version` subcommand.
type VersionCommand struct {
	// JSON renders the flat machine-readable object instead of the
	// docker-style block, selected with --json.
	JSON bool
}

// Run renders detailed version information to Stdout. With JSON set, it
// renders the machine-readable object instead.
// Panics if ctx is nil.
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
type CompletionCommand struct {
	// Shell holds the shell name requested as the first positional argument,
	// populated by the engine (e.g. "bash", "zsh"). It is empty when the
	// command was invoked without a shell, in which case Run renders the
	// completion command's own help screen with the supported-shells guidance
	// instead of failing.
	Shell string
}

// Run generates and outputs the shell completion script for the requested shell.
// Panics if ctx is nil.
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

// ConfigFile represents an opt-in root-level flag for supplying configuration
// file paths. By default the flag holds a single path (last value wins);
// declaring multi:"true" makes it multi-valued — repeated --config-file and
// sep-separated values accumulate, enabling base+profile merging. The flag
// reports the configuration files actually loaded; probed-but-absent
// candidates are surfaced by session.CandidateConfigFiles() instead.
//
// Supported struct tags: name, short, help, env, aliases, placeholder,
// group, hidden, multi, sep, and conga directives. See docs/struct-tags.md for
// the complete reference.
type ConfigFile struct {
	paths        []string
	set          bool
	changed      bool
	source       ValueSource
	sourceDetail string
}

// IsSet reports whether configuration file paths were supplied by any source.
func (c *ConfigFile) IsSet() bool { return c.set }

// Path returns the effective configuration file path: the highest-precedence
// loaded file, or "" when none was resolved.
func (c *ConfigFile) Path() string {
	if len(c.paths) == 0 {
		return ""
	}

	return c.paths[len(c.paths)-1]
}

// Paths returns a copy of the configuration files actually loaded for this
// flag, in load order (lowest to highest precedence). Probed-but-absent
// candidate paths are reported by session.CandidateConfigFiles() instead.
func (c *ConfigFile) Paths() []string {
	return append([]string(nil), c.paths...)
}

// Changed returns true if configuration file paths were explicitly provided
// via CLI or env (not engine-loaded).
func (c *ConfigFile) Changed() bool {
	return c.changed
}

// Source returns where the configuration file paths originated.
func (c *ConfigFile) Source() ValueSource {
	return c.source
}

// SourceDetail returns additional context on the source.
func (c *ConfigFile) SourceDetail() string {
	return c.sourceDetail
}

// String implements fmt.Stringer.
func (c *ConfigFile) String() string {
	if c == nil {
		return ""
	}

	return strings.Join(c.paths, ", ")
}

// Counter represents an opt-in flag for tracking repeatable occurrences of a flag (e.g. -v, -vv, -vvv).
//
// Supported struct tags: name, short, help, default, env, aliases, group,
// hidden, xor, and, enum, range, config, and conga directives.
// See docs/struct-tags.md for the complete reference.
type Counter struct {
	value        int
	set          bool
	changed      bool
	source       ValueSource
	sourceDetail string
}

// IsSet reports whether the counter flag was supplied by any source.
func (c *Counter) IsSet() bool { return c.set }

// Get returns the resolved count value.
func (c *Counter) Get() int {
	return c.value
}

// GetOr returns the count if set, or fallback if omitted.
func (c *Counter) GetOr(fallback int) int {
	if !c.set {
		return fallback
	}

	return c.value
}

// Changed returns true if the counter flag was explicitly passed.
func (c *Counter) Changed() bool {
	return c.changed
}

// Source returns where the counter value originated.
func (c *Counter) Source() ValueSource {
	return c.source
}

// SourceDetail returns additional context on the source.
func (c *Counter) SourceDetail() string {
	return c.sourceDetail
}

// String implements fmt.Stringer.
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

// Get returns the captured arguments slice.
func (raw *rawSlice) Get() []string {
	return raw.args
}

// GetOr returns the captured arguments if set, or fallback if empty.
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

// String implements fmt.Stringer.
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
