package conga

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"syscall"

	"github.com/conga-sh/conga/internal/appname"
	"github.com/conga-sh/conga/internal/coerce"
	"github.com/conga-sh/conga/internal/completion"
	"github.com/conga-sh/conga/internal/completion/wire"
	"github.com/conga-sh/conga/internal/cond"
	"github.com/conga-sh/conga/internal/configengine"
	"github.com/conga-sh/conga/internal/dotenv"
	"github.com/conga-sh/conga/internal/help"
	"github.com/conga-sh/conga/internal/pathx"
	"github.com/conga-sh/conga/internal/sentinel"
	"github.com/conga-sh/conga/internal/slicex"
	"github.com/conga-sh/conga/internal/spec"
	"github.com/conga-sh/conga/internal/tokenizer"
	"github.com/conga-sh/conga/predict"
)

const (
	// UsageSummary formats usage errors with a concise 1-2 line command syntax
	// guide followed by a hint to run --help. This is the default error display mode.
	UsageSummary UsageOnErrorMode = iota

	// UsageFull formats usage errors with the error message followed by the complete
	// command help screen, including flags, arguments, and subcommands.
	UsageFull

	// UsageHint formats usage errors with only the error message followed by a
	// single-line "Run '<command> --help' for more information." hint, without
	// syntax details.
	UsageHint

	// UsageSilence formats usage errors with only the bare error message, omitting all
	// usage instructions and help hints.
	UsageSilence
)

const (
	mapPairErrorInvalid  mapPairErrorKind = iota // no '=' separator
	mapPairErrorEmptyKey                         // key is empty after trimming
)

const (
	mapEntryErrorKey   mapEntryErrorKind = iota // key coercion failed
	mapEntryErrorValue                          // value coercion failed
)

// UsageOnErrorMode specifies how command usage and help hints are formatted
// on standard error when a user supplies invalid command-line input.
//
// When an input error violates a specific value constraint (such as range, length,
// enum, or type coercion), CONGA treats the failure as self-describing and renders
// only the specific error message to keep output readable, regardless of the
// configured mode. Syntax errors, unknown flags, and missing required inputs render
// contextual guidance according to the active mode.
type UsageOnErrorMode int

// App manages the configuration, parsing, and execution lifecycle of a
// command-line application rooted in struct T.
//
// The command struct T (or its selected subcommand) executes by implementing
// [Runner].
//
// An App is constructed with [New], configured through chained builder
// methods, and executed with [App.Run] (passing the default signal-canceling
// context to [Runner.Run]) or [App.RunContext] (passing the caller's context).
//
// An App executes once; calling [App.Run] or [App.RunContext] a second time
// panics. Builder methods must be called before execution begins.
//
// See https://docs.conga.sh/explanation/lifecycle/.
type App[T any] struct {
	appConfig[T]

	// rootTarget is the command struct instance the schema binds to: the
	// instance supplied through New, or the engine-allocated instance created
	// on the first schema build.
	rootTarget *T

	rootCommand *spec.Command

	// executed guards the single-execution contract: Run/RunContext may only
	// execute once per App.
	executed bool

	// initialized records that applyDefaults has installed the engine defaults
	// on the App. It makes initialization idempotent across New and the
	// execution entry points, so a zero-value App behaves like New[T]().
	initialized bool
}

// New returns an [App] builder for the root command struct T.
//
// Calling New[T]() without arguments is the standard pattern. CONGA manages
// the root struct lifecycle: it instantiates T, binds flags and arguments to its
// fields, and executes its [Runner.Run] method.
//
// Passing an existing root instance binds that instance instead of
// allocating a fresh one, so callers can inspect bound container
// values after execution completes.
//
// Panics:
//   - If root contains more than one pointer.
//   - If the supplied pointer is nil.
func New[T any](root ...*T) *App[T] {
	if len(root) > 1 {
		panic("conga: New accepts at most one root instance")
	}

	app := &App[T]{}
	app.applyDefaults()

	if len(root) == 1 {
		if root[0] == nil {
			panic("conga: New requires a non-nil root instance")
		}

		app.rootTarget = root[0]
	}

	return app
}

// Name sets the root application binary name.
//
// The configured name appears in generated usage syntax, help headers, version
// strings, and error prefixes. When omitted, CONGA falls back to the invoked binary
// name derived from os.Args[0].
func (a *App[T]) Name(name string) *App[T] {
	a.name = name

	return a
}

// Description sets the long-form application overview, rendered at the top of
// the root command's help screen below the usage header.
//
// A subcommand supplies its own long-form overview by implementing
// [DescriptionProvider], and its one-line synopsis with the summary tag.
//
// See https://docs.conga.sh/how-to/customize-help-screens/.
func (a *App[T]) Description(desc string) *App[T] {
	a.description = desc

	return a
}

// Examples sets application usage examples displayed on the root help screen.
//
// Examples appear in a dedicated "Examples:" section on --help. Indented lines
// are preserved as formatted code or command lines:
//
//	app.Examples(`  # Start server on custom port
//	  myapp --port 9000
//
//	  # Run with verbose logging
//	  myapp -v`)
//
// See https://docs.conga.sh/how-to/customize-help-screens/.
func (a *App[T]) Examples(examples string) *App[T] {
	a.examples = examples

	return a
}

// HelpFooter sets text rendered at the bottom of every help screen, after the flags,
// arguments, and subcommands. It applies to the root command and all subcommands.
//
// See https://docs.conga.sh/how-to/customize-help-screens/.
func (a *App[T]) HelpFooter(footer string) *App[T] {
	a.helpFooter = footer

	return a
}

// Deprecated marks the entire application as deprecated.
//
// Each run that reaches the command lifecycle first prints a warning containing
// the reason text to standard error. Execution continues normally; deprecation
// warnings are non-fatal. Requests that render a built-in and return early, such
// as --help, --version, and --why, print no warning.
//
// Declare the deprecation with this method, not a Deprecated method on the root
// struct, which fails schema compilation with [ErrSchema]. To deprecate an
// individual optional flag or subcommand, use the deprecated struct tag.
func (a *App[T]) Deprecated(reason string) *App[T] {
	a.deprecated = reason

	return a
}

// Version sets the application version string.
//
// The version renders when the user passes the built-in --version flag, when
// running a mounted [VersionCommand], or when calling [Session.Version]. The
// --version flag prefers the invoked binary's name over the configured
// application name.
//
// When omitted, CONGA falls back to the Go module version recorded in the
// binary, or "dev" when the module version is unavailable. An embedded VCS
// commit hash and timestamp are appended to the rendered summary when present.
//
// The summary line takes the form
//
//	<app> has version <version> built with <goversion>
//
// For example:
//
//	Version("2.1.0")  --version  =>  myapp has version 2.1.0 built with go1.24.0
func (a *App[T]) Version(version string) *App[T] {
	a.version = version

	return a
}

// EnvPrefix sets the global environment variable prefix applied to derived flag bindings.
//
// CONGA appends a trailing underscore when the prefix lacks one, so the prefix MYAPP
// becomes MYAPP_. Flags without an explicit env tag derive their variable name from the
// prefix and the uppercase flag name, with hyphens replaced by underscores. A field
// declaring an env tag uses that name verbatim and ignores the prefix:
//
//	EnvPrefix("MYAPP")  --database-url  =>  MYAPP_DATABASE_URL
//	EnvPrefix("MYAPP")  env:"PORT"      =>  PORT
//
// Environment variable bindings outrank configuration file values and defaults, but
// yield to explicit command-line flags.
//
// See https://docs.conga.sh/how-to/bind-environment-variables/.
func (a *App[T]) EnvPrefix(prefix string) *App[T] {
	if prefix != "" && !strings.HasSuffix(prefix, "_") {
		prefix += "_"
	}

	a.envPrefix = prefix

	return a
}

// Dotenv enables loading environment variables from .env files into the process environment.
//
// When called without arguments, Dotenv probes for .env followed by .env.local in the
// current working directory, loading both if present. When explicit paths are supplied,
// CONGA loads only the specified files in the given order.
//
// Files apply in order: a key defined in more than one file takes the value from the
// last file that defines it, so .env.local overrides .env. Variables already present
// in the process environment are never overwritten and outrank every file.
//
// Files must use KEY=VALUE syntax. Empty lines and lines starting with # are ignored;
// optional export prefixes and surrounding single or double quotes are stripped.
//
// Dotenv files enforce a strict 16MB file size limit to prevent unbounded memory allocation.
// Files exceeding the limit or missing explicit paths cause execution to halt with an error
// wrapping [ErrConfig].
//
// See https://docs.conga.sh/how-to/load-env-files/.
func (a *App[T]) Dotenv(paths ...string) *App[T] {
	a.dotenvPaths = paths
	a.dotenvLoader = dotenv.Load

	return a
}

// ConfigOptions sets how CONGA discovers and merges config files. Options come
// from the [ConfigOption] constructors: [ConfigPaths], [ConfigExplicit], and
// [ConfigCascading], each declaring one resolution rule.
//
// Config files also require a registered decoder (see [App.RegisterDecoder]).
// See https://docs.conga.sh/how-to/manage-config-files/.
func (a *App[T]) ConfigOptions(options ...ConfigOption) *App[T] {
	a.configOptions = append(a.configOptions, options...)
	a.configSettings = foldConfigOptions(a.configOptions)
	a.installConfigEngine()

	return a
}

// RegisterDecoder registers a decoder for a configuration file extension:
//
//	app.RegisterDecoder(".yaml", yaml.Decode)
//
// The extension is normalized automatically: leading asterisks and dots are
// stripped, whitespace is trimmed, and characters are converted to lowercase.
// For example, ".yaml", "yaml", and "*.YAML" are all equivalent.
//
// The decoder callback receives the raw file bytes and unmarshals them into a
// nested map[string]any with string keys. Registering an extension that was previously
// registered overwrites the earlier registration.
//
// Decoders for JSON, YAML, TOML, and HCL ship with CONGA in the decoder/json,
// decoder/yaml, decoder/toml, and decoder/hcl packages.
//
// See https://docs.conga.sh/reference/decoders/.
func (a *App[T]) RegisterDecoder(extension string, decoder ConfigDecoder) *App[T] {
	a.installConfigEngine()
	a.configRegistry.Register(extension, decoder)

	return a
}

// StrictTagValidation controls whether struct tags are strictly validated when the
// command schema is compiled (enabled by default).
//
// When strict validation is enabled, CONGA inspects all struct tags on command fields.
// Unknown tag keys (such as misspelled hlp or deflt), invalid constraint combinations,
// or tags applied to unsupported field types cause [App.Run] or [App.RunContext] to return
// an error wrapping [ErrSchema].
//
// Common third-party tags (json, yaml, validate, db, and others listed in the package
// documentation) are permitted by default. [App.AllowTags] registers additional external
// tag keys (kong, bson, or custom annotations) without disabling strict validation.
func (a *App[T]) StrictTagValidation(strict bool) *App[T] {
	a.tagValidationDisabled = !strict

	return a
}

// AllowTags adds struct tag keys that strict tag validation accepts.
//
// Strict validation (the default) rejects unrecognized tag keys. CONGA accepts
// common third-party tags out of the box (json, yaml, validate, db, and others
// listed in the package documentation); register any others your structs carry
// (for example bson or a custom annotation) instead of disabling strict
// validation:
//
//	type ServerCLI struct {
//		Port  conga.OptionalFlag[int]    `default:"8080" help:"Listening port" short:"p"`
//		Token conga.RequiredFlag[string] `help:"API access token" audit:"mask"`
//	}
//
//	app := conga.New[ServerCLI]().AllowTags("audit")
//
// Registered keys are ignored by the schema builder on all commands, subcommands,
// and nested structs; multiple calls append to the permitted tag set. For
// narrower scope use the conga allow-unknown-tags directive, or call
// [App.StrictTagValidation] with false to disable tag checking entirely.
func (a *App[T]) AllowTags(tags ...string) *App[T] {
	a.allowedTags = append(a.allowedTags, tags...)

	return a
}

// Predictor registers a named completion predictor for shell autocompletion.
//
// Command struct fields reference registered predictors using the predictor struct tag.
//
// Panics:
//   - If name is empty.
//   - If name matches a reserved built-in predictor name or alias (file, dirs,
//     executables, symlinks, none).
//   - If a predictor with the same name has already been registered.
//
// See https://docs.conga.sh/how-to/wire-custom-completion/.
func (a *App[T]) Predictor(name string, predictor predict.Predictor) *App[T] {
	a.applyDefaults()

	if name == "" {
		panic("conga: predictor name cannot be empty")
	}

	if spec.IsReservedPredictorName(name) {
		panic(fmt.Sprintf("conga: cannot register predictor %q: reserved built-in predictor name", name))
	}

	if _, exists := a.predictors[name]; exists {
		panic(fmt.Sprintf("conga: predictor %q is already registered", name))
	}

	a.predictors[name] = predictor

	return a
}

// EnableCompletion activates shell completion: script generation for Bash, Zsh,
// Fish, PowerShell, Elvish, and Nushell, plus the hidden __complete wire
// protocol. A mounted [CompletionCommand] requires it.
//
// See https://docs.conga.sh/how-to/enable-shell-completion/.
func (a *App[T]) EnableCompletion() *App[T] {
	a.completionShells = completion.SupportedShells()
	a.installCompletionHooks()

	return a
}

// DisableHelpFlag stops CONGA from injecting the built-in --help flag, freeing
// that name for your own flags. [Session.Help], error-driven usage summaries, and
// an explicit [HelpCommand] still work.
//
// See https://docs.conga.sh/reference/builtins/.
func (a *App[T]) DisableHelpFlag() *App[T] {
	a.helpDisabled = true

	return a
}

// DisableVersionFlag stops CONGA from injecting the built-in --version flag,
// freeing that name for your own flags. [Session.Version], version metadata in
// --why reports, and an explicit [VersionCommand] still work.
//
// See https://docs.conga.sh/reference/builtins/.
func (a *App[T]) DisableVersionFlag() *App[T] {
	a.versionDisabled = true

	return a
}

// EnableWhy activates the built-in --why flag. When passed, CONGA skips
// command execution and prints a provenance report to standard output showing
// where every flag value originated (CLI, environment, configuration file, or
// struct default) and why competing sources were overridden.
//
// See https://docs.conga.sh/how-to/inspect-value-provenance/.
func (a *App[T]) EnableWhy() *App[T] {
	a.installWhyRenderer()

	return a
}

// Use mounts one or more builder bindings onto the app.
//
// Bindings are functions with signature func(*App[T]). Each runs immediately
// in the given order, registering types and predictors through the App builder
// (RegisterLocation, for example, calls [App.RegisterType] with a completion
// predictor), and nil bindings are skipped:
//
//	app.Use(network.RegisterInterface, timezone.RegisterLocation)
//
// See https://docs.conga.sh/reference/domain-types/.
func (a *App[T]) Use(bindings ...func(*App[T])) *App[T] {
	for _, binding := range bindings {
		if binding != nil {
			binding(a)
		}
	}

	return a
}

// RegisterType registers a parser and optional completion predictor
// for custom type U on the application builder.
//
// The parser callback converts raw string input from flags, arguments, or environment
// variables into an instance of type U. A returned error is wrapped in [ErrCoerce].
//
// If type U was previously registered, the new parser and predictor overwrite the
// existing registration.
//
// Panics:
//   - If the receiver App is nil.
//   - If parser is nil.
//
// See https://docs.conga.sh/how-to/add-custom-flag-types/.
func (a *App[T]) RegisterType[U any](parser func(string) (U, error), opts ...RegisterTypeOption) *App[T] {
	if a == nil {
		panic("conga: RegisterType requires a non-nil app")
	}

	if parser == nil {
		panic("conga: RegisterType requires a non-nil parser")
	}

	targetType := reflect.TypeFor[U]()

	wrapped := func(raw string) (any, error) {
		return parser(raw)
	}

	reg := typeRegistration{
		parser: wrapped,
	}

	for _, opt := range opts {
		if opt != nil {
			opt(&reg)
		}
	}

	a.registerType(targetType, reg)

	return a
}

// RegisterMap registers a parser and optional completion predictor
// for map type map[K]V, so flags and positional arguments of that type accept
// key=value pairs. K must be string or a type with underlying kind string; V
// may be any type supported by CONGA. Maps with both a string key and a string
// value bind natively and need no registration.
//
// Flags accept repeated or comma-separated pairs:
//
//	--flag k1=v1 --flag k2=v2
//	--flag k1=v1,k2=v2
//
// The sample map is read for its type only; its entries are ignored.
//
// Panics:
//   - If the receiver App is nil.
//
// See https://docs.conga.sh/how-to/add-custom-flag-types/.
func (a *App[T]) RegisterMap[M ~map[K]V, K ~string, V any](_ M, opts ...RegisterTypeOption) *App[T] {
	if a == nil {
		panic("conga: RegisterMap requires a non-nil app")
	}

	parser := func(raw string) (M, error) {
		result := make(M)

		keyString, valueString, ok := strings.Cut(raw, "=")
		if !ok {
			return nil, &mapPairError{item: raw, kind: mapPairErrorInvalid}
		}

		keyString = strings.TrimSpace(keyString)
		if keyString == "" {
			return nil, &mapPairError{item: raw, kind: mapPairErrorEmptyKey}
		}

		key, err := coerce.Value[K](keyString)
		if err != nil {
			return nil, &mapEntryError{key: keyString, kind: mapEntryErrorKey, cause: err}
		}

		value, err := coerce.Value[V](strings.TrimSpace(valueString))
		if err != nil {
			return nil, &mapEntryError{key: keyString, kind: mapEntryErrorValue, cause: err}
		}

		result[key] = value

		return result, nil
	}

	return a.RegisterType(parser, opts...)
}

// RunContext executes the application lifecycle using the provided context.
//
// Resolution and binding run first: dotenv loading (if configured via
// [App.Dotenv]), tokenization, configuration file resolution, then flag and
// argument coercion from the CLI, environment, and configuration sources.
// Constraints are then enforced in this order: struct tag constraints, the leaf
// command's [Runner] contract, and domain-type [SelfValidator] checks.
//
// The lifecycle runs in five stages on the matched leaf command:
//
//  1. Validate: [Validator.Validate] hooks run top-down. A failure returns an
//     error immediately; no PreRun, Run, PostRun, or Finally hook executes,
//     because no command has been initialized yet.
//  2. PreRun: [PreRunner.PreRun] hooks run top-down to initialize resources.
//  3. Run: [Runner.Run] executes on the leaf.
//  4. PostRun: [PostRunner.PostRun] hooks run bottom-up, only after Run succeeds.
//  5. Finally: [Finalizer.Finally] hooks run bottom-up unconditionally for every
//     initialized command, receiving the halting error or nil.
//
// An error from any stage between PreRun and PostRun skips the remaining
// stages and jumps straight to Finally. An error from a Finally hook is joined
// with the halting error and returned alongside it.
//
// RunContext propagates the provided context to every lifecycle hook but does not
// itself poll for cancellation; a hook that observes cancellation can return
// ctx.Err(), which then flows through Finally cleanup like any other error. Standard
// streams are snapshotted when execution begins; subsequent builder mutations do not
// affect the running session.
//
// The built-in subcommands ([HelpCommand], [VersionCommand],
// [CompletionCommand]) bypass the lifecycle, so ancestor hooks never
// execute for them.
//
// Returns:
//   - nil on successful execution or when handling built-in flags (--help, --version, --why).
//   - An error wrapping [ErrSchema] if the struct tags or command schema are invalid.
//   - An error wrapping [ErrParse], [ErrCoerce], or [ErrValidation] if user input is invalid.
//   - An error wrapping [ErrConfig] if configuration or .env loading fails.
//   - The error returned by [Runner.Run] or lifecycle hooks, or ctx.Err() on cancellation.
//
// RunContext never terminates the process; it returns every failure to the
// caller. [App.Exit] maps a returned error to an exit code.
//
// Panics:
//   - If ctx is nil.
//   - If the App has already been executed: an App instance runs exactly once.
func (a *App[T]) RunContext(ctx context.Context) error {
	a.applyDefaults()

	a.initDefaults()

	if a.handleCompletionInvocation != nil && a.isCompletionInvocation() {
		return a.handleCompletionInvocation(a)
	}

	if a.executed {
		panic("conga: App already executed")
	}

	a.executed = true

	if ctx == nil {
		panic("conga: nil Context")
	}

	// 1. Load Dotenv if enabled
	if a.dotenvLoader != nil {
		if err := a.dotenvLoader(a.dotenvPaths...); err != nil {
			return fmt.Errorf("%w: %w", ErrConfig, err)
		}
	}

	session, handled, err := a.prepare(ctx)
	if err != nil {
		return err
	}

	if handled {
		return nil
	}

	for _, notice := range a.deprecationNotices {
		fmt.Fprintln(session.Stderr(), notice)
	}

	return executeLifecycle(ctx, session)
}

// Run executes the application, passing a default context to the command's
// [Runner.Run] method and lifecycle hooks. The default context cancels when an
// [os.Interrupt] (SIGINT) or [syscall.SIGTERM] signal is received.
//
// For callers supplying their own context, use [App.RunContext].
//
// Panics:
//   - If the App has already been executed: an App instance runs exactly once.
//
// See https://docs.conga.sh/explanation/lifecycle/.
func (a *App[T]) Run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return a.RunContext(ctx)
}

// Args overrides the command-line argument slice parsed by the application.
// Defaults to os.Args[1:].
//
//	app.Args([]string{"--port", "9000", "server"})
//
// See https://docs.conga.sh/how-to/test-a-command/.
func (a *App[T]) Args(args []string) *App[T] {
	a.args = args
	a.argsExplicitlySet = true

	return a
}

// UsageOnError configures how command syntax and help guidance are formatted on
// standard error when command-line input parsing or validation fails.
//
// By default, CONGA uses [UsageSummary]. See [UsageOnErrorMode] for details on
// available presentation modes.
//
// See https://docs.conga.sh/explanation/error-reporting/.
func (a *App[T]) UsageOnError(mode UsageOnErrorMode) *App[T] {
	a.usageMode = mode

	return a
}

// Stdin redirects the standard input stream reader for the application.
//
// The stream defaults to [os.Stdin]. It is passed to commands via [Session.Stdin] and
// feeds [File] containers declaring the stdin tag and [FileContent] containers reading
// from the - path.
//
// The stream reader is snapshotted when [App.Run] or [App.RunContext] begins; mutations
// made after execution starts do not affect the active session. Passing a nil reader
// is a no-op.
func (a *App[T]) Stdin(reader io.Reader) *App[T] {
	if reader != nil {
		a.stdin = reader
	}

	return a
}

// Stdout redirects the standard output stream writer for the application.
//
// The stream defaults to [os.Stdout]. It is passed to commands via [Session.Stdout] and
// is used by built-in help, version, and --why renderers, as well as [File] containers
// declaring the stdout tag and writing to the - path.
//
// The stream writer is snapshotted when [App.Run] or [App.RunContext] begins; mutations
// made after execution starts do not affect the active session. Passing a nil writer
// is a no-op.
func (a *App[T]) Stdout(writer io.Writer) *App[T] {
	if writer != nil {
		a.stdout = writer
	}

	return a
}

// Stderr redirects the standard error stream writer for the application.
//
// The stream defaults to [os.Stderr]. It is passed to commands via [Session.Stderr] and
// is used for diagnostics, deprecation notices, syntax usage summaries, and [App.Exit].
//
// The stream writer is snapshotted when [App.Run] or [App.RunContext] begins; mutations
// made after execution starts do not affect the active session. Passing a nil writer
// is a no-op.
func (a *App[T]) Stderr(writer io.Writer) *App[T] {
	if writer != nil {
		a.stderr = writer
	}

	return a
}

// ExitFunc overrides the function [App.Exit] calls to terminate the
// process. Defaults to [os.Exit]. Passing a nil handler is a no-op.
//
//	var exitCode int
//	app.ExitFunc(func(code int) { exitCode = code })
func (a *App[T]) ExitFunc(exitHandler func(code int)) *App[T] {
	if exitHandler != nil {
		a.exitFunc = exitHandler
	}

	return a
}

// Exit terminates the process with an exit status mapped from err, using the
// configured exit function (see [App.ExitFunc], defaults to [os.Exit]).
//
// Exit accepts the error returned by [App.Run] or [App.RunContext].
//
// All post-run cleanup ([PostRunner.PostRun] and [Finalizer.Finally] hooks)
// completes before [App.Run] returns, so Exit runs only after teardown
// finishes.
//
// Exit code conventions:
//   - A nil err exits with status 0 and no output.
//   - Exit code 1: runtime errors from command execution ([Runner.Run] or lifecycle
//     hooks), developer schema defects ([ErrSchema]), and configuration failures
//     ([ErrConfig], including .env loading).
//   - Exit code 2: usage errors ([IsUsageError] reports true for parse, coercion,
//     and validation failures) and internal completion failures.
//
// Usage error rendering follows [UsageOnErrorMode], except for self-describing
// value errors (coercion failures and value-constraint violations), which always
// print the error line alone.
//
// See https://docs.conga.sh/explanation/error-reporting/.
func (a *App[T]) Exit(err error) {
	a.applyDefaults()

	if err == nil {
		a.exit(0)

		return
	}

	if IsCompletionError(err) {
		fmt.Fprintln(a.stderr, err)
		a.exit(2)

		return
	}

	name := cond.SetOrDefaultFunc(a.name, getAppName)
	if invoked, ok := appname.Binary(); ok {
		name = invoked
	}

	if IsUsageError(err) {
		a.handleUsageFatal(a.stderr, err, name)

		return
	}

	// Runtime error from Runner.Run()
	fmt.Fprintf(a.stderr, "%s: error: %v\n", name, err)
	a.exit(1)
}

// RegisterTypeOption configures a custom type registration on [App.RegisterType]
// and [App.RegisterMap]. [WithPredictor] is currently the only constructor.
type RegisterTypeOption func(*typeRegistration)

// WithPredictor attaches a shell autocompletion predictor to a registered custom type.
//
// Flags and positional arguments using this type automatically inherit the predictor
// for shell autocompletion, unless overridden by an explicit predictor struct tag.
func WithPredictor(p predict.Predictor) RegisterTypeOption {
	return func(r *typeRegistration) {
		r.hasPredictor = true
		r.predictor = p
	}
}

// mapPairErrorKind identifies the KEY=VALUE syntax failure mode.
type mapPairErrorKind uint8

// mapPairError duplicates coerce.go's carrier: the RegisterMap parser
// reimplements pair splitting for typed K/V across the package boundary;
// the two stay deliberately in sync.
type mapPairError struct {
	item string
	kind mapPairErrorKind
}

func (e *mapPairError) Error() string {
	switch e.kind {
	case mapPairErrorInvalid:
		return fmt.Sprintf("invalid key-value pair %q, expected KEY=VALUE", e.item)
	case mapPairErrorEmptyKey:
		return fmt.Sprintf("empty key in key-value pair %q", e.item)
	default:
		return fmt.Sprintf("invalid key-value pair %q", e.item)
	}
}

func (e *mapPairError) Unwrap() error { return ErrCoerce }

// mapEntryErrorKind identifies which side of a KEY=VALUE entry failed coercion.
type mapEntryErrorKind uint8

// mapEntryError is the root cause for typed map entry coercion failures.
type mapEntryError struct {
	key   string
	kind  mapEntryErrorKind
	cause error // inner coercion failure from coerce.Value[K] / coerce.Value[V]
}

func (e *mapEntryError) Error() string {
	if e.cause == nil {
		panic("conga: invariant violation: mapEntryError requires a non-nil cause")
	}

	switch e.kind {
	case mapEntryErrorKey:
		return fmt.Sprintf("key %q: %s", e.key, e.cause)
	case mapEntryErrorValue:
		return fmt.Sprintf("value for key %q: %s", e.key, e.cause)
	default:
		return fmt.Sprintf("invalid map entry for key %q: %s", e.key, e.cause)
	}
}

func (e *mapEntryError) Unwrap() error {
	if e.cause != nil {
		return e.cause
	}

	return ErrCoerce
}

func (e *mapEntryError) Is(target error) bool { return target == ErrCoerce }

type typeRegistration struct {
	parser       coerce.TypeParser
	hasPredictor bool
	predictor    predict.Predictor
}

// configResolution memoizes the outcome of the run's single configuration
// resolution: the loaded files and their winning source, the extraction and load
// errors, and the candidate paths probed.
type configResolution struct {
	loadedPaths  []string
	loadedSource ValueSource
	extractErr   error
	loadErr      error
	candidates   []string
}

func (r *configResolution) execErr() error {
	if r.extractErr != nil {
		return r.extractErr
	}

	return r.loadErr
}

func (r *configResolution) whyErr() error {
	if r.loadErr != nil {
		return r.loadErr
	}

	return r.extractErr
}

// appConfig holds every configuration field for an App. It is the single source
// of truth for configuration state: New populates it with defaults and the
// builder methods mutate it.
type appConfig[T any] struct {
	name                  string
	description           string
	examples              string
	helpFooter            string
	deprecated            string
	deprecationNotices    []string
	version               string
	envPrefix             string
	configOptions         []ConfigOption
	configSettings        spec.ConfigSettings
	configRegistry        *configengine.Registry
	configEngine          configEngineFactory
	dotenvPaths           []string
	dotenvLoader          func(paths ...string) error
	configData            map[string]string
	configResolution      *configResolution
	args                  []string
	argsExplicitlySet     bool
	usageMode             UsageOnErrorMode
	stdout                io.Writer
	stderr                io.Writer
	stdin                 io.Reader
	exitFunc              func(code int)
	tagValidationDisabled bool
	allowedTags           []string
	predictors            map[string]predict.Predictor
	typeRegistry          map[reflect.Type]typeRegistration
	helpDisabled          bool
	versionDisabled       bool
	helpOptions           help.Options
	completionShells      []string
	// inferPredictor performs type-based predictor inference during schema
	// build. Completion installs it; nil keeps the inference machinery
	// (including its filesystem probes) unreferenced at link time.
	inferPredictor func(targetType reflect.Type, tags spec.FieldTags) *spec.PredictorSpec
	// featurePredictor builds the completion predictor for feature map keys.
	// Completion installs it; nil keeps the predictor package unreferenced when
	// completion is disabled.
	featurePredictor func(name string, values map[string][]string) *spec.PredictorSpec
	// The following feature hooks are method expressions: they receive the
	// executing App as their first argument. Nil means the subsystem is
	// disabled and unreferenced, so its packages drop at link time.
	whyRenderer                func(app *App[T], cmdChain []*spec.Command, flagValues map[*spec.Flag][]string)
	defaultPredictors          func(app *App[T])
	mountCompletion            func(app *App[T], cmd *spec.Command)
	handleCompletionInvocation func(app *App[T]) error
	completionHelp             func(app *App[T], cmdChain []*spec.Command, completionValue string, withHelp bool) (bool, error)
	renderCompletionScript     func(app *App[T], cmdChain []*spec.Command, shell string) (bool, error)
}

// configView is the root package's view over the configuration resolution
// engine: the subset of the engine the always-reachable pipeline consumes. It
// deliberately mentions no configengine type so the engine package is reachable
// only through the factory installed by installConfigEngine.
type configView interface {
	Resolve(explicitTargets []string, cmd *spec.Command) ([]string, map[string]string, ValueSource, error)
	CandidatePaths(explicitPaths []string, cmd *spec.Command) []string
	EffectiveSearchPaths() []string
	ProbedFiles(candidate string, extensions []string) []string
	ConfigExamples(candidates []string, tiers []string, extensions []string) []string
}

// configEngineFactory builds the per-command engine view. The App's live fields
// are passed on every call so a factory installed before Name or ConfigOptions
// always resolves with the final values.
type configEngineFactory func(name string, envPrefix string, registry *configengine.Registry, cmd *spec.Command) configView

// emptyConfigView is the engine view for applications that never installed a
// configuration engine: it rejects named configuration targets with a
// decoder-required error and surfaces the environment targets, but loads and
// probes nothing.
type emptyConfigView struct {
	envPrefix string
}

func (view emptyConfigView) Resolve(explicitTargets []string, cmd *spec.Command) ([]string, map[string]string, ValueSource, error) {
	if len(explicitTargets) > 0 {
		return nil, nil, SourceNone, newDecoderRequiredError("flag --"+configFlagName(cmd), explicitTargets[len(explicitTargets)-1])
	}

	for _, envKey := range configengine.EnvKeys(view.envPrefix, cmd) {
		if envValue := os.Getenv(envKey); envValue != "" {
			return nil, nil, SourceNone, newDecoderRequiredError("environment variable "+envKey, envValue)
		}
	}

	return nil, nil, SourceNone, nil
}

func (view emptyConfigView) CandidatePaths(explicitPaths []string, cmd *spec.Command) []string {
	if len(explicitPaths) > 0 {
		return slicex.Map(explicitPaths, pathx.Expand)
	}

	if envPaths := configengine.EnvCandidates(view.envPrefix, cmd); len(envPaths) > 0 {
		return envPaths
	}

	return nil
}

func (emptyConfigView) EffectiveSearchPaths() []string { return nil }

func (emptyConfigView) ProbedFiles(_ string, _ []string) []string { return nil }

func (emptyConfigView) ConfigExamples(_ []string, _ []string, _ []string) []string { return nil }

// decoderRequiredError is the config-incapable rejection of a named
// configuration target. It carries the offending path so the tolerant --why
// report marks the matching candidate row as failed.
type decoderRequiredError struct {
	cause error
	path  string
}

func newDecoderRequiredError(source string, path string) *decoderRequiredError {
	return &decoderRequiredError{
		cause: fmt.Errorf("%w: %s requires a registered config decoder: call App.RegisterDecoder with a format decoder (e.g. decoder/json)", ErrConfig, source),
		path:  path,
	}
}

func (e *decoderRequiredError) Error() string { return e.cause.Error() }

func (e *decoderRequiredError) Unwrap() error { return e.cause }

func (e *decoderRequiredError) ConfigPath() string { return e.path }

// installConfigEngine lazily creates the decoder registry and installs the
// resolution engine factory. It is called exclusively from the config builder
// methods, so minimal applications never reference the engine package.
func (a *App[T]) installConfigEngine() {
	if a.configRegistry == nil {
		a.configRegistry = configengine.NewRegistry()
	}

	if a.configEngine == nil {
		a.configEngine = engineConfigResolver
	}
}

// configurationView returns the installed engine view, or the zero view when
// the application is not config-capable.
func (a *App[T]) configurationView(cmd *spec.Command) configView {
	if a.configEngine == nil {
		return emptyConfigView{envPrefix: a.envPrefix}
	}

	return a.configEngine(a.name, a.envPrefix, a.configRegistry, cmd)
}

func (a *App[T]) completeForShell(targetShell string, args []string) (predict.Result, string) {
	return a.completeForShellWithEnv(targetShell, args, nil)
}

func (a *App[T]) completeForShellWithEnv(targetShell string, args []string, lookup completion.EnvLookup) (predict.Result, string) {
	a.initDefaults()

	cmd, err := a.buildSpec()
	if err != nil {
		return predict.None().Invoke(predict.Context{}), ""
	}

	targetShell = cond.SetOrDefaultFunc(targetShell, detectShell)

	if featureBinding := featureStateOfCommand(cmd); featureBinding != nil {
		cliValues := cliFlagValues(cmd, args, cmd.FeatureFlag, cmd.ConfigFileFlag)

		featureBinding.pruneForCompletion(cmd, cliValues, &bindingState{
			EnvPrefix:             a.envPrefix,
			ConfigData:            a.loadConfigDataForCompletion(cmd, cliValues),
			TypeParsers:           a.collectTypeParsers(),
			EnvLookup:             lookup,
			TolerateFeatureErrors: true,
		})
	}

	opts := spec.CompleteOptions{
		RegisteredPredictors: a.predictors,
		TargetShell:          targetShell,
	}

	completionSession := completion.NewSessionWithEnv(targetShell, lookup)

	patchedArgs, err := completionSession.Patch(args)
	if errors.Is(err, sentinel.ErrRedirectTarget) {
		// A redirect target ("cloudctl deploy > <cursor>") never completes:
		// falling back to the raw arguments would leak subcommand
		// candidates, so short-circuit with the zero-candidate framing
		// (docs/completion-wire-protocol.md §4.1). Other patch errors keep
		// the pass-through behavior below.
		invoked := predict.None().Invoke(predict.Context{})

		return invoked, completionSession.Format("", invoked)
	}

	normArgs := cond.If(err == nil && len(patchedArgs) > 0, patchedArgs, args)
	if len(normArgs) == 0 {
		normArgs = []string{""}
	}

	invoked := spec.Complete(cmd, normArgs, opts)
	toComplete := normArgs[len(normArgs)-1]

	formatted := completionSession.Format(toComplete, invoked)

	return invoked, formatted
}

func (a *App[T]) completeCompLineForShell(targetShell string, compLine string, compPoint int) (predict.Result, string) {
	words := tokenizer.ParseCompLine(compLine, compPoint)
	if len(words) > 0 {
		words = words[1:]
	}

	lookup := func(key string) (string, bool) {
		switch key {
		case "COMP_LINE":
			return compLine, true
		case "COMP_POINT":
			return strconv.Itoa(compPoint), true
		default:
			return os.LookupEnv(key)
		}
	}

	return a.completeForShellWithEnv(targetShell, words, lookup)
}

func (a *App[T]) loadConfigDataForCompletion(cmd *spec.Command, flagValues map[*spec.Flag][]string) map[string]string {
	if a.configEngine == nil {
		return nil
	}

	explicitTargets, err := a.extractExplicitConfigTargets(cmd, []*spec.Command{cmd}, flagValues)
	if err != nil {
		return nil
	}

	_, values, _, err := a.configurationView(cmd).Resolve(explicitTargets, cmd)
	if err != nil {
		return nil
	}

	return values
}

// applyDefaults installs the engine defaults a fresh App starts from: the
// standard process streams, the process exit function, and the internal
// registries. It is idempotent; New calls it eagerly and the execution entry
// points call it again, so a zero-value App behaves like New[T]().
func (a *App[T]) applyDefaults() {
	if a.initialized {
		return
	}

	a.initialized = true

	if a.stdout == nil {
		a.stdout = os.Stdout
	}

	if a.stderr == nil {
		a.stderr = os.Stderr
	}

	if a.stdin == nil {
		a.stdin = os.Stdin
	}

	if a.exitFunc == nil {
		a.exitFunc = os.Exit
	}

	if a.predictors == nil {
		a.predictors = make(map[string]predict.Predictor)
	}

	if a.configData == nil {
		a.configData = make(map[string]string)
	}

	if a.typeRegistry == nil {
		a.typeRegistry = make(map[reflect.Type]typeRegistration)
	}
}

func (a *App[T]) initDefaults() {
	a.name = cond.SetOrDefaultFunc(a.name, getAppName)

	if !a.argsExplicitlySet {
		if len(os.Args) > 1 {
			a.args = os.Args[1:]
		} else {
			a.args = []string{}
		}
	}
}

func (a *App[T]) initDefaultPredictors() {
	if _, ok := a.predictors["shell"]; !ok {
		items := slicex.Map(completion.ShellDescriptions(), func(info completion.ShellInfo) predict.Item {
			return predict.NewItem(info.Name, info.Description)
		})

		a.predictors["shell"] = predict.Items(items...)
	}
}

func (a *App[T]) registerType(targetType reflect.Type, reg typeRegistration) {
	if a.typeRegistry == nil {
		a.typeRegistry = make(map[reflect.Type]typeRegistration)
	}

	a.typeRegistry[targetType] = reg
}

func (a *App[T]) collectTypeParsers() coerce.Registry {
	if len(a.typeRegistry) == 0 {
		return nil
	}

	parsers := make(coerce.Registry, len(a.typeRegistry))
	for registeredType, registration := range a.typeRegistry {
		parsers[registeredType] = registration.parser
	}

	return parsers
}

func (a *App[T]) collectTypePredictors() map[reflect.Type]predict.Predictor {
	if len(a.typeRegistry) == 0 {
		return nil
	}

	predictors := make(map[reflect.Type]predict.Predictor)

	for registeredType, registration := range a.typeRegistry {
		if registration.hasPredictor {
			predictors[registeredType] = registration.predictor
		}
	}

	if len(predictors) == 0 {
		return nil
	}

	return predictors
}

func (a *App[T]) buildSpec() (*spec.Command, error) {
	if a.defaultPredictors != nil {
		a.defaultPredictors(a)
	}

	opts := spec.Options{
		StrictTagValidation:  !a.tagValidationDisabled,
		AllowedTags:          a.allowedTags,
		RegisteredPredictors: a.predictors,
		TypePredictors:       a.collectTypePredictors(),
		ClassifyContainer:    classifyContainer,
		ConfigFilePatterns: slicex.Map(a.configRegistry.Extensions(), func(extension string) string {
			return "*" + extension
		}),
		ConfigSettings:         a.configSettings,
		ConfigCapable:          a.configEngine != nil,
		DisableHelp:            a.helpDisabled,
		DisableVersion:         a.versionDisabled,
		DisableWhy:             a.whyRenderer == nil,
		DisableCompletion:      a.mountCompletion == nil,
		CompletionShells:       a.completionShells,
		InferPredictor:         a.inferPredictor,
		FeaturePredictor:       a.featurePredictor,
		ValidateTextRangeBound: validateTextRangeBound,
		EnvPrefix:              a.envPrefix,
	}

	if a.rootTarget == nil {
		a.rootTarget = new(T)
	}

	cmd, err := spec.Build(a.rootTarget, a.name, opts)
	if err != nil {
		return nil, err
	}

	if a.mountCompletion != nil {
		a.mountCompletion(a, cmd)
	}

	if a.description != "" {
		cmd.Description = strings.TrimSpace(a.description)
	}

	if a.examples != "" {
		cmd.Examples = strings.TrimSpace(a.examples)
	}

	if a.helpFooter != "" {
		cmd.HelpFooter = a.helpFooter
	}

	if a.deprecated != "" {
		cmd.Deprecated = a.deprecated
	}

	if a.version != "" {
		cmd.Version = a.version
	}

	if a.envPrefix != "" && cmd.EnvPrefix == "" {
		cmd.EnvPrefix = a.envPrefix
	}

	a.rootCommand = cmd

	return cmd, nil
}

func (a *App[T]) mountCompletionSubcommand(cmd *spec.Command) {
	if cmd.HasCompletionCommand() {
		return
	}

	if cmd.HasUserSubcommands() {
		compCmd := spec.NewCompletionCommand("completion", spec.DefaultCompletionCommandHelp, &CompletionCommand{}, nil, false, a.completionShells)
		cmd.Subcommands = append(cmd.Subcommands, compCmd)
	}
}

func (a *App[T]) bindConfigAndFlags(session *Session, cmd *spec.Command, cmdChain []*spec.Command, flagValues map[*spec.Flag][]string, loadedPaths []string, loadedSource ValueSource) error {
	state := &bindingState{
		EnvPrefix:   a.envPrefix,
		ConfigData:  a.configData,
		TypeParsers: a.collectTypeParsers(),
		Stdin:       session.stdin,
		Stdout:      session.stdout,
	}

	if err := resolveFlags(cmdChain, flagValues, state); err != nil {
		return err
	}

	if cmd.ConfigFileFlag != nil && len(loadedPaths) > 0 && !flagStateOf(cmd.ConfigFileFlag.Handle).IsSet() {
		flagStateOf(cmd.ConfigFileFlag.Handle).setValue(loadedPaths, loadedSource, strings.Join(loadedPaths, ", "))
	}

	return nil
}

// prepareFeatures resolves feature gating before builtin rendering: it resolves
// configuration (unless the request is a lone --version), binds and materializes
// the feature map, prunes inactive flags and subcommands, and rejects a gated
// flag supplied on the command line (a usage error even under --help, symmetric
// with a gated subcommand). It returns the pruned flags.
func (a *App[T]) prepareFeatures(session *Session, featureBinding featureState, cmd *spec.Command, cmdChain []*spec.Command, flagValues map[*spec.Flag][]string, request builtinRequest) ([]prunedFlag, error) {
	if !request.versionOnly() {
		resolution := a.resolveSessionConfig(session, cmd, cmdChain, flagValues)
		if !request.toleratesFeatureErrors() {
			if err := resolution.execErr(); err != nil {
				return nil, err
			}
		}
	}

	featureBindingState := &bindingState{
		EnvPrefix:             a.envPrefix,
		ConfigData:            a.configData,
		TypeParsers:           a.collectTypeParsers(),
		TolerateFeatureErrors: request.toleratesFeatureErrors(),
	}

	return featureBinding.resolveFeatures(cmd, cmdChain, flagValues, featureBindingState)
}

func (a *App[T]) extractExplicitConfigTargets(cmd *spec.Command, cmdChain []*spec.Command, flagValues map[*spec.Flag][]string) ([]string, error) {
	targets, err := flagConfigTargets(cmd, cmdChain, flagValues)
	if err != nil || len(targets) > 0 {
		return targets, err
	}

	if a.configSettings.Explicit != "" {
		return []string{a.configSettings.Explicit}, nil
	}

	return nil, nil
}

func (a *App[T]) validateExecution(ctx context.Context, session *Session, leaf *spec.Command, cmdChain []*spec.Command) error {
	if err := validateRunnableContract(leaf, cmdChain); err != nil {
		return err
	}

	if !isMetaCommand(leaf) {
		if err := validateCustomValues(ctx, cmdChain); err != nil {
			return err
		}
	}

	return runValidators(ctx, session, cmdChain)
}

func (a *App[T]) prepareNonMetaExecution(session *Session, cmd *spec.Command, cmdChain []*spec.Command, flagValues map[*spec.Flag][]string, featureBinding featureState, prunedFlags []prunedFlag) error {
	resolution := a.resolveSessionConfig(session, cmd, cmdChain, flagValues)
	if err := resolution.execErr(); err != nil {
		return err
	}

	if featureBinding != nil {
		if err := featureBinding.rejectPrunedConfig(prunedFlags, a.configData); err != nil {
			return err
		}
	}

	if err := a.bindConfigAndFlags(session, cmd, cmdChain, flagValues, resolution.loadedPaths, resolution.loadedSource); err != nil {
		return err
	}

	a.collectDeprecationNotices(cmdChain)

	return validateFlagConstraints(cmdChain)
}

func (a *App[T]) prepare(ctx context.Context) (*Session, bool, error) {
	cmd, err := a.buildSpec()
	if err != nil {
		return nil, false, err
	}

	tokens := tokenizer.Tokenize(a.args)

	cmdChain, remainingTokens, index, err := routeSubcommands(cmd, tokens)
	if err != nil {
		return nil, false, err
	}

	leaf := cmdChain[len(cmdChain)-1]

	flagValues, posTokens, dashArgs, rawTokens, err := parseTokens(cmdChain, remainingTokens, index)
	if err != nil {
		return nil, false, err
	}

	// The session is born before value binding so the whole decode pipeline
	// (config resolution, flag and argument coercion) reads engine state
	// (the standard streams) from one consistent run context.
	session := &Session{
		root:        getCmdTarget(cmdChain[0]),
		stdout:      a.stdout,
		stderr:      a.stderr,
		stdin:       a.stdin,
		cmdChain:    cmdChain,
		appName:     a.name,
		appVersion:  a.version,
		helpOptions: a.helpOptions,
	}

	featureBinding := featureStateOfCommand(cmd)
	request := scanBuiltinRequest(flagValues)

	var prunedFlags []prunedFlag

	// Feature gating sources gates from the configuration tier, so
	// feature-capable applications resolve configuration before builtin
	// rendering and pruning. A lone --version renders from app metadata alone
	// and skips configuration resolution entirely.
	if featureBinding != nil {
		pruned, err := a.prepareFeatures(session, featureBinding, cmd, cmdChain, flagValues, request)
		if err != nil {
			return nil, false, err
		}

		prunedFlags = pruned
	}

	// Featureless applications defer configuration resolution below builtin
	// detection, so help/version/why never pay configuration I/O or fail on
	// resolution errors (e.g. a missing env-sourced config path).
	if handled, err := a.handleBuiltinFlags(session, flagValues, request); handled || err != nil {
		return nil, handled, err
	}

	if !isMetaCommand(leaf) {
		if err := a.prepareNonMetaExecution(session, cmd, cmdChain, flagValues, featureBinding, prunedFlags); err != nil {
			return nil, false, err
		}
	}

	if err := resolveArgs(leaf, cmdChain, posTokens, a.collectTypeParsers(), coerce.DecodeContext{Stdin: session.stdin, Stdout: session.stdout}); err != nil {
		return nil, false, err
	}

	bindSpecialArgs(leaf, dashArgs, rawTokens)

	if err := a.validateExecution(ctx, session, leaf, cmdChain); err != nil {
		return nil, false, err
	}

	return session, false, nil
}

func (a *App[T]) handleBuiltinFlags(session *Session, flagValues map[*spec.Flag][]string, request builtinRequest) (bool, error) {
	cmdChain := session.cmdChain

	// 1. Completion flag + --help (e.g. app --completion zsh --help or app --completion --help)
	if request.hasCompletion && request.hasHelp && a.completionHelp != nil {
		return a.completionHelp(a, cmdChain, request.completionValue, true)
	}

	// 2. Bare completion flag (e.g. app --completion)
	if request.hasCompletion && request.completionValue == "" && a.completionHelp != nil {
		return a.completionHelp(a, cmdChain, "", false)
	}

	// 3. Standard --help
	if request.hasHelp {
		fmt.Fprintln(a.stdout, session.Help())

		return true, nil
	}

	// 4. Standard --version
	if request.hasVersion {
		a.renderVersionBuiltin()

		return true, nil
	}

	// 5. Value-provenance report (e.g. app --why)
	if request.hasWhy && a.whyRenderer != nil {
		a.whyRenderer(a, cmdChain, flagValues)

		return true, nil
	}

	// 6. Completion script generation (e.g. app --completion zsh)
	if request.hasCompletion && request.completionValue != "" && a.renderCompletionScript != nil {
		return a.renderCompletionScript(a, cmdChain, request.completionValue)
	}

	return false, nil
}

// renderVersionBuiltin prints the version summary to stdout, preferring the
// invoked binary name over the configured application name.
func (a *App[T]) renderVersionBuiltin() {
	appName := cond.SetOrDefaultFunc(a.name, getAppName)
	if invoked, ok := appname.Binary(); ok {
		appName = invoked
	}

	fmt.Fprintln(a.stdout, help.VersionSummary(appName, a.version))
}

// renderCompletionHelp renders the completion help screen: the per-shell
// screen for --completion <shell> --help, or the generic screen for a bare
// --completion.
func (a *App[T]) renderCompletionHelp(cmdChain []*spec.Command, completionValue string, withHelp bool) (bool, error) {
	var compCmd *spec.Command

	if withHelp {
		builtCmd, err := buildCompletionHelpCommand(cmdChain, completionValue, a.completionShells)
		if err != nil {
			return false, err
		}

		compCmd = builtCmd
	} else {
		compCmd = buildBareCompletionHelpCommand(a.completionShells)
	}

	fmt.Fprintln(a.stdout, help.FormatWith(append(slices.Clone(cmdChain), compCmd), a.helpOptions))

	return true, nil
}

// renderCompletionScriptBuiltin generates the shell completion script for the
// requested shell, mapping unknown shell names to a validation error.
func (a *App[T]) renderCompletionScriptBuiltin(cmdChain []*spec.Command, shell string) (bool, error) {
	parsedShell, err := completion.ParseShell(shell)
	if err != nil {
		return false, &validationError{
			msg:      err.Error(),
			cmdChain: cmdChain,
			cause:    errInvalidValue,
		}
	}

	if err := generateCompletionScript(parsedShell, cmdChain[0], a.stdout); err != nil {
		return false, err
	}

	return true, nil
}

// collectDeprecationNotices gathers provenance-aware deprecation warnings for
// the resolved command chain. Application and subcommand notices are emitted
// unconditionally; flag notices only when the flag was explicitly provided via
// CLI, environment, or configuration (defaults are never warned about).
func (a *App[T]) collectDeprecationNotices(cmdChain []*spec.Command) {
	a.deprecationNotices = nil

	if len(cmdChain) == 0 {
		return
	}

	a.collectAppDeprecationNotice(cmdChain)
	a.collectSubcommandDeprecationNotices(cmdChain)
	a.collectFlagDeprecationNotices(cmdChain)
}

func (a *App[T]) collectAppDeprecationNotice(cmdChain []*spec.Command) {
	root := cmdChain[0]
	if root.Deprecated == "" {
		return
	}

	a.deprecationNotices = append(a.deprecationNotices,
		fmt.Sprintf("WARNING: %s is deprecated: %s", root.Name, root.Deprecated))
}

func (a *App[T]) collectSubcommandDeprecationNotices(cmdChain []*spec.Command) {
	for _, cmd := range cmdChain[1:] {
		if cmd.Deprecated == "" {
			continue
		}

		a.deprecationNotices = append(a.deprecationNotices,
			fmt.Sprintf("WARNING: command '%s' is deprecated: %s", commandPath(cmdChain, cmd), cmd.Deprecated))
	}
}

func (a *App[T]) collectFlagDeprecationNotices(cmdChain []*spec.Command) {
	seen := make(map[*spec.Flag]bool)

	for _, cmd := range cmdChain {
		for _, flagSpec := range cmd.Flags {
			if flagSpec.Deprecated == "" || seen[flagSpec] || flagSpec.Handle == nil {
				continue
			}

			if !flagStateOf(flagSpec.Handle).Changed() {
				continue
			}

			a.appendFlagDeprecationNotice(flagSpec)
			seen[flagSpec] = true
		}
	}
}

func (a *App[T]) appendFlagDeprecationNotice(flagSpec *spec.Flag) {
	detail := flagSpec.Name
	if state := flagStateOf(flagSpec.Handle); state != nil {
		if d := state.SourceDetail(); d != "" {
			detail = d
		}
	}

	switch flagStateOf(flagSpec.Handle).Source() {
	case SourceCLI:
		a.deprecationNotices = append(a.deprecationNotices,
			fmt.Sprintf("WARNING: flag '%s' is deprecated: %s", detail, flagSpec.Deprecated))
	case SourceEnv:
		a.deprecationNotices = append(a.deprecationNotices,
			fmt.Sprintf("WARNING: environment variable '%s' is deprecated: %s", detail, flagSpec.Deprecated))
	case SourceConfig:
		a.deprecationNotices = append(a.deprecationNotices,
			fmt.Sprintf("WARNING: config key '%s' is deprecated: %s", detail, flagSpec.Deprecated))
	case SourceDefault, SourceNone:
		// Defaulted or unset values are never warned about.
	}
}

func (a *App[T]) isCompletionInvocation() bool {
	return len(a.args) > 0 && a.args[0] == "__complete"
}

func (a *App[T]) handleComplete() (err error) {
	defer func() {
		// A panic mid-completion previously tore through the caller and
		// corrupted the user's terminal line; recover into the documented
		// internal-error contract (docs/completion-wire-protocol.md):
		// diagnostic on stderr, exit 2, no stdout payload.
		if recovered := recover(); recovered != nil {
			err = &completionError{cause: fmt.Sprintf("%v", recovered)}
		}
	}()

	stdout := a.stdout

	var targetShell string

	wordsToComplete := []string{""}

	if len(a.args) > 1 {
		targetShell = a.args[1]
	}

	if len(a.args) > 2 {
		wordsToComplete = a.args[2:]
	}

	// The bash-native COMP_LINE environment is intentionally left intact
	// here: completeForShell's Patch pass re-lexes it, and only that pass
	// sees the redirect-rejection sentinel this dispatch must honor.
	if targetShell == "bash" {
		if envWords, ok := resolveCompLineEnv(); ok && len(wordsToComplete) <= 1 {
			wordsToComplete = envWords
		}
	}

	_, formatted := a.completeForShell(targetShell, wordsToComplete)

	if _, err := fmt.Fprint(stdout, formatted); err != nil {
		return &completionError{cause: err.Error()}
	}

	return nil
}

func (a *App[T]) handleSubcommandRequiredFatal(writer io.Writer, err error, appName string, cmdChain []*spec.Command) {
	fmt.Fprintf(writer, "%s: error: %v\n", appName, err)

	switch a.usageMode {
	case UsageSummary, UsageFull:
		if len(cmdChain) > 0 {
			fmt.Fprintf(writer, "\n%s\n", help.FormatWith(cmdChain, a.helpOptions))
		}
	case UsageHint:
		if len(cmdChain) > 0 {
			fmt.Fprintf(writer, "\n%s\n", help.Hint(cmdChain))
		}
	case UsageSilence:
		// No usage printed
	}

	a.exit(2)
}

func (a *App[T]) handleUsageFatal(writer io.Writer, err error, appName string) {
	cmdChain := extractCmdChain(err, a.rootCommand)

	if errors.Is(err, errSubcommandRequired) {
		a.handleSubcommandRequiredFatal(writer, err, appName, cmdChain)

		return
	}

	// Self-describing value errors (coercion failures and value-constraint
	// violations) name the exact flag/argument and offending value, so
	// rendering usage/help text adds noise without context.
	if sd, ok := errors.AsType[selfDescribingError](err); ok && sd.selfDescribing() {
		fmt.Fprintf(writer, "%s: error: %v\n", appName, err)
		a.exit(2)

		return
	}

	fmt.Fprintf(writer, "%s: error: %v\n", appName, err)

	switch a.usageMode {
	case UsageSummary:
		if len(cmdChain) > 0 {
			fmt.Fprintf(writer, "\n%s\n\n%s\n", help.Usage(cmdChain), help.Hint(cmdChain))
		}
	case UsageFull:
		if len(cmdChain) > 0 {
			fmt.Fprintf(writer, "\n%s\n", help.FormatWith(cmdChain, a.helpOptions))
		}
	case UsageHint:
		if len(cmdChain) > 0 {
			fmt.Fprintf(writer, "\n%s\n", help.Hint(cmdChain))
		}
	case UsageSilence:
		// No usage output printed
	}

	a.exit(2)
}

// resolveConfig resolves and loads the run's configuration exactly once,
// memoizing the outcome on the App. Feature-capable applications resolve
// configuration before builtin rendering (gating needs the config tier) and the
// --why report reuses the memo, so configuration files are probed, decoded, and
// validated a single time per run.
func (a *App[T]) resolveConfig(cmd *spec.Command, cmdChain []*spec.Command, flagValues map[*spec.Flag][]string) *configResolution {
	if a.configResolution != nil {
		return a.configResolution
	}

	explicitTargets, extractErr := a.extractExplicitConfigTargets(cmd, cmdChain, flagValues)

	a.configData = make(map[string]string)

	var (
		loadedPaths  []string
		loadedSource ValueSource
		loadErr      error
	)

	if extractErr == nil {
		loadedPaths, loadedSource, loadErr = a.resolveAndLoadConfig(cmd, explicitTargets)
	}

	a.configResolution = &configResolution{
		loadedPaths:  loadedPaths,
		loadedSource: loadedSource,
		extractErr:   extractErr,
		loadErr:      loadErr,
		candidates:   a.configurationView(cmd).CandidatePaths(explicitTargets, cmd),
	}

	return a.configResolution
}

func (a *App[T]) resolveSessionConfig(session *Session, cmd *spec.Command, cmdChain []*spec.Command, flagValues map[*spec.Flag][]string) *configResolution {
	resolution := a.resolveConfig(cmd, cmdChain, flagValues)
	if session != nil {
		session.candidateFiles = resolution.candidates
		session.loadedFiles = resolution.loadedPaths
	}

	return resolution
}

func (a *App[T]) resolveAndLoadConfig(root *spec.Command, explicitTargets []string) ([]string, ValueSource, error) {
	loadedPaths, values, source, err := a.configurationView(root).Resolve(explicitTargets, root)
	if values != nil {
		a.configData = values
	}

	if err != nil {
		return nil, SourceNone, err
	}

	return loadedPaths, source, nil
}

// installCompletionHooks binds the completion hook fields and help renderers
// to a. It is reached exclusively through Completion, so applications that
// never enable completion keep the subsystem unreferenced.
func (a *App[T]) installCompletionHooks() {
	a.inferPredictor = spec.InferPredictorFromType
	a.featurePredictor = func(name string, values map[string][]string) *spec.PredictorSpec {
		return &spec.PredictorSpec{Kind: spec.PredictorKindPredictable, Name: name, Predictor: predict.KeyValue(values)}
	}
	a.defaultPredictors = (*App[T]).initDefaultPredictors
	a.mountCompletion = (*App[T]).mountCompletionSubcommand
	a.handleCompletionInvocation = (*App[T]).handleComplete
	a.completionHelp = (*App[T]).renderCompletionHelp
	a.renderCompletionScript = (*App[T]).renderCompletionScriptBuiltin
	a.helpOptions.CompletionInstructions = help.RenderCompletionInstructions
	a.helpOptions.CompletionFooter = func(cmdChain []*spec.Command, useFlag bool) string {
		return help.RenderCompletionFooter(cmdChain, useFlag, completion.DetectCurrentShell)
	}
}

// installWhyRenderer binds the value-provenance renderer to a. It is reached
// exclusively through EnableWhy, so applications that never enable the report
// keep the why machinery unreferenced.
func (a *App[T]) installWhyRenderer() {
	a.whyRenderer = (*App[T]).renderWhy
}

func (a *App[T]) exit(code int) {
	if a.exitFunc != nil {
		a.exitFunc(code)
	}
}

// validateTextRangeBound parses a range bound with the owning field's declared
// tags for domain types that implement [ContextAwareTextUnmarshaler]. It backs
// spec.Options.ValidateTextRangeBound, so schema build rejects range bounds that
// are invalid under the field's tags (for example a decimal bound on a
// units:"binary" [ByteSize] field). Types that ignore tags fall through to the
// default UnmarshalText-bound parser.
func validateTextRangeBound(targetType reflect.Type, tags FieldTags, raw string) error {
	unmarshaler, ok := reflect.TypeAssert[ContextAwareTextUnmarshaler](reflect.New(targetType))
	if !ok {
		return nil
	}

	return unmarshaler.UnmarshalTextWithContext([]byte(raw), DecodeContext{Tags: tags})
}

// configFlagName returns the root configuration file flag name, defaulting to
// the canonical "config-file" when the schema declares no such flag.
func configFlagName(cmd *spec.Command) string {
	if cmd != nil && cmd.ConfigFileFlag != nil {
		return cmd.ConfigFileFlag.Name
	}

	return "config-file"
}

// engineConfigResolver adapts configengine.Resolver to configView. It is
// referenced only from installConfigEngine, so it is dropped together with the
// uncalled builder methods that install it.
func engineConfigResolver(name string, envPrefix string, registry *configengine.Registry, cmd *spec.Command) configView {
	return configengine.Resolver{
		AppName:   name,
		EnvPrefix: envPrefix,
		Settings:  rootConfigSettings(cmd),
		Registry:  registry,
	}
}

// flagConfigTargets extracts the CLI-supplied configuration file paths for the
// root config-file flag: the split values of a multi-valued flag, or the last
// value of a single-valued one. Supplying the flag without naming a path (an
// empty or blank value) is rejected as a usage error instead of silently
// falling through to the environment variable or the search tier, which would
// load a different configuration than the one the user named.
func flagConfigTargets(cmd *spec.Command, cmdChain []*spec.Command, flagValues map[*spec.Flag][]string) ([]string, error) {
	if cmd == nil || cmd.ConfigFileFlag == nil {
		return nil, nil
	}

	values, ok := flagValues[cmd.ConfigFileFlag]
	if !ok || len(values) == 0 {
		return nil, nil
	}

	var targets []string

	if cmd.ConfigFileFlag.IsSlice {
		targets = extractMultiFlagValues(values, cmd.ConfigFileFlag.Sep)
	} else {
		targets = []string{values[len(values)-1]}
	}

	for _, target := range targets {
		if strings.TrimSpace(target) == "" {
			return nil, errEmptyConfigTarget(cmd, cmdChain)
		}
	}

	if len(targets) == 0 {
		return nil, errEmptyConfigTarget(cmd, cmdChain)
	}

	return targets, nil
}

// errEmptyConfigTarget builds the usage error for a config-file flag that was
// supplied without naming a path.
func errEmptyConfigTarget(cmd *spec.Command, cmdChain []*spec.Command) error {
	return &validationError{
		msg:         "path cannot be empty for flag --" + cmd.ConfigFileFlag.Name,
		cmdChain:    cmdChain,
		cause:       errInvalidValue,
		valueShaped: true,
	}
}

// rootConfigSettings returns the app-level configuration settings carried by
// the root command, tolerating a nil command for defensive callers.
func rootConfigSettings(cmd *spec.Command) spec.ConfigSettings {
	if cmd == nil {
		return spec.ConfigSettings{}
	}

	return cmd.ConfigSettings
}

func resolveCompLineEnv() ([]string, bool) {
	if compLine := os.Getenv("COMP_LINE"); compLine != "" {
		compPoint := len(compLine)

		if compPointStr := os.Getenv("COMP_POINT"); compPointStr != "" {
			if parsed, err := strconv.Atoi(compPointStr); err == nil {
				compPoint = parsed
			}
		}

		words := tokenizer.ParseCompLine(compLine, compPoint)
		if len(words) > 0 {
			words = words[1:]
		}

		return words, true
	}

	return nil, false
}

func extractMultiFlagValues(values []string, sep string) []string {
	paths := make([]string, 0, len(values))

	for _, value := range values {
		paths = append(paths, configengine.SplitList(value, sep)...)
	}

	return paths
}

func getAppName() string {
	if name, ok := appname.Binary(); ok {
		return name
	}

	return "app"
}

// generateCompletionScript renders the completion driver script for shell and
// the root command of a command chain, writing it to output. The invoked
// binary name overrides the root command name when the binary was installed
// or invoked under a different name.
func generateCompletionScript(shell completion.Shell, rootCmd *spec.Command, output io.Writer) error {
	cmdName := rootCmd.Name
	if invoked, ok := appname.Binary(); ok {
		cmdName = invoked
	}

	script, err := completion.Generate(shell, completion.DriverOptions{
		CommandName: cmdName,
		Aliases:     rootCmd.Aliases,
	})
	if err != nil {
		return err
	}

	_, err = io.WriteString(output, script)

	return err
}

func commandPath(cmdChain []*spec.Command, cmd *spec.Command) string {
	var names []string

	for _, c := range cmdChain {
		if c.Name != "" {
			names = append(names, c.Name)
		}

		if c == cmd {
			break
		}
	}

	return strings.Join(names, " ")
}

func extractCmdChain(err error, fallback *spec.Command) []*spec.Command {
	if provider, ok := errors.AsType[commandChainProvider](err); ok {
		if chain := provider.commandChain(); len(chain) > 0 {
			return chain
		}
	}

	if fallback != nil {
		return []*spec.Command{fallback}
	}

	return nil
}

func detectShell() string {
	if shell := os.Getenv(wire.EnvShell); shell != "" {
		return shell
	}

	if cur, ok := completion.DetectCurrentShell(); ok {
		return string(cur.Shell)
	}

	return ""
}
