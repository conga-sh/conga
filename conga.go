package conga

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"os/signal"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"syscall"

	"github.com/conga-sh/conga/config"
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
	// UsageSummary displays the clean error, a 1-2 line command syntax, and a --help hint.
	UsageSummary UsageOnErrorMode = iota

	// UsageFull displays the clean error and the complete formatted command help screen.
	UsageFull

	// UsageHint displays the clean error and a single-line "Run '<cmd> --help' for usage" hint.
	UsageHint

	// UsageSilence displays only the clean error message without any usage or help text.
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

// UsageOnErrorMode specifies how usage/help information is rendered when an input error occurs.
type UsageOnErrorMode int

// App manages command schema, flag binding, and CLI execution for struct type T.
type App[T any] struct {
	appConfig[T]

	// rootTarget is the command struct instance the schema binds to: the
	// instance supplied through New, or the engine-allocated instance created
	// on the first schema build.
	rootTarget *T

	rootCommand *spec.Command

	// executed guards the single-execution contract: Run/RunContext may only
	// execute once per App. The flag lives on App, not appConfig, so Reload's
	// fresh internal App and appConfig.clone() never inherit it.
	executed bool
}

// New creates and initializes a new fluent App builder for struct type T.
//
// The command struct instance is allocated by the engine on the first schema
// build and is reachable inside lifecycle hooks through session.Root[T]().
// Passing root retains the caller's own instance so it stays accessible outside
// the lifecycle for pre-run seeding and post-run inspection:
//
//	cli := new(CLI)
//	app := conga.New(cli)
//
// At most one root instance may be supplied, and it must be non-nil.
func New[T any](root ...*T) *App[T] {
	if len(root) > 1 {
		panic("conga: New accepts at most one root instance")
	}

	app := &App[T]{
		configData:          make(map[string]string),
		predictors:          make(map[string]predict.Predictor),
		typeRegistry:        make(map[reflect.Type]typeRegistration),
		usageMode:           UsageSummary,
		stdout:              os.Stdout,
		stderr:              os.Stderr,
		stdin:               os.Stdin,
		exitFunc:            os.Exit,
		strictTagValidation: true,
		helpEnabled:         true,
		versionEnabled:      true,
	}

	if len(root) == 1 {
		if root[0] == nil {
			panic("conga: New requires a non-nil root instance")
		}

		app.rootTarget = root[0]
	}

	return app
}

// Name sets the root application binary name (defaults to os.Args[0]).
func (a *App[T]) Name(name string) *App[T] {
	a.name = name

	return a
}

// Description sets the root application description.
func (a *App[T]) Description(desc string) *App[T] {
	a.description = desc

	return a
}

// Examples sets the root application usage examples shown on --help.
func (a *App[T]) Examples(examples string) *App[T] {
	a.examples = examples

	return a
}

// Footer sets the global footer rendered on every help screen.
func (a *App[T]) Footer(footer string) *App[T] {
	a.footer = footer

	return a
}

// Deprecated retires the entire application/binary; emits a warning on all invocations.
func (a *App[T]) Deprecated(reason string) *App[T] {
	a.deprecated = reason

	return a
}

// Version sets the version string rendered by the built-in --version flag
// and by session.Version(). The flag exists on every root command; when no
// version is set, the output falls back to the module version (or "dev") plus
// the embedded build metadata (VCS commit and date, when available).
func (a *App[T]) Version(version string) *App[T] {
	a.version = version

	return a
}

// EnvPrefix sets the global environment variable prefix for all flags (e.g. "MYAPP_").
func (a *App[T]) EnvPrefix(prefix string) *App[T] {
	if prefix != "" && !strings.HasSuffix(prefix, "_") {
		prefix += "_"
	}

	a.envPrefix = prefix

	return a
}

// Dotenv enables zero-dependency 12-factor .env loading into the process environment.
// When called with no arguments, it automatically loads ".env" followed by ".env.local" from CWD.
// When explicit paths are provided, it loads the specified files in order.
func (a *App[T]) Dotenv(paths ...string) *App[T] {
	a.dotenvPaths = paths
	a.dotenvLoader = dotenv.Load
	a.dotenvOverloader = dotenv.Overload

	return a
}

// ConfigOptions configures how the application resolves configuration files.
// Options compose and are independent of call order:
//
//   - config.Explicit(path) names one concrete file with fatal-if-missing
//     semantics; it outranks the configuration environment variable and the
//     search tier.
//   - config.Paths(paths...) declares the author search tier, probed in
//     precedence order; the first existing file wins unless
//     config.Cascading is also declared. Declaring paths replaces the
//     built-in conventional tier, which otherwise probes
//     ${XDG_CONFIG_HOME:-~/.config}/<app-name> (files config.<ext> and
//     <app-name>.<ext> for every registered decoder extension) whenever the
//     application is config-capable.
//   - config.Cascading() merges every existing tier file in precedence order
//     (lowest to highest) instead of stopping at the first existing file. A
//     declared root ConfigFile flag must accept multiple values
//     (multi:"true") so it can report every merged file.
//
// Repeated calls accumulate rather than replace: options declared by earlier
// calls stay in effect, and a later declaration of the same slot overrides the
// previous one.
func (a *App[T]) ConfigOptions(options ...config.Option) *App[T] {
	a.configOptions = append(a.configOptions, options...)
	a.configSettings = configSettingsFrom(config.Apply(a.configOptions...))
	a.installConfigEngine()

	return a
}

// RegisterDecoder registers a custom configuration format decoder for the
// given file extension. The extension is normalized (trimmed, lowercased, an
// optional "*" prefix and a missing leading "." are both accepted), so
// ".json", "json", and "*.JSON" are equivalent. Registering an extension that
// already has a decoder replaces it (last registration wins).
func (a *App[T]) RegisterDecoder(extension string, decoder config.Decoder) *App[T] {
	a.installConfigEngine()
	a.configRegistry.Register(extension, decoder)

	return a
}

// StrictTagValidation configures whether strict struct tag linting is enabled at startup (default: true).
// When enabled, unrecognized struct tags, tag typos, or tags on incompatible field types will cause
// schema building to fail with an error wrapping ErrSchema.
func (a *App[T]) StrictTagValidation(strict bool) *App[T] {
	a.strictTagValidation = strict

	return a
}

// AllowTags registers additional custom struct tags to be permitted during strict struct tag validation.
func (a *App[T]) AllowTags(tags ...string) *App[T] {
	a.allowedTags = append(a.allowedTags, tags...)

	return a
}

// Predictor registers a named dynamic completion predictor for flags and positional arguments.
// It panics if name is empty, is a reserved built-in predictor name, or has already been registered.
func (a *App[T]) Predictor(name string, predictor predict.Predictor) *App[T] {
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

// EnableCompletion enables shell autocompletion support: script generation, the
// `completion <shell>` entry points, and the hidden `__complete` protocol.
// Applications that never call EnableCompletion do not link the completion
// drivers.
func (a *App[T]) EnableCompletion() *App[T] {
	a.completionShells = completion.SupportedShells()
	a.installCompletionHooks()

	return a
}

// DisableHelpFlag removes the automatically injected --help flag. Help
// screens rendered by session.Help(), a declared HelpCommand, and error/usage
// handling remain available.
func (a *App[T]) DisableHelpFlag() *App[T] {
	a.helpEnabled = false

	return a
}

// DisableVersionFlag removes the automatically injected --version flag and
// its renderer. The version string set by Version() remains available to
// session.Version(), a declared VersionCommand, and the --why report. Calling
// Version after DisableVersionFlag still records the string but does not
// re-enable the flag.
func (a *App[T]) DisableVersionFlag() *App[T] {
	a.versionEnabled = false

	return a
}

// EnableWhy enables the built-in --why flag and its value-provenance report,
// which explains where every field value came from and why other sources lost.
// Applications that never call EnableWhy do not link the reporting machinery.
func (a *App[T]) EnableWhy() *App[T] {
	a.installWhyRenderer()

	return a
}

// Use applies one or more configuration plugins or type loaders to the app builder.
func (a *App[T]) Use(plugins ...func(*App[T])) *App[T] {
	for _, plugin := range plugins {
		if plugin != nil {
			plugin(a)
		}
	}

	return a
}

// RegisterType registers a strongly-typed parser and optional completion predictor
// for type U on the application builder. If type U has already been registered
// (e.g. by an imported plugin or prior RegisterType call), the new parser and
// predictor overwrite the existing entry (last-write-wins), allowing callers to
// customize or override default type behaviors.
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

// RegisterMap registers a typed map[K]V parser and optional completion predictor
// on the application builder. Passing a sample map (e.g. map[string]int{} or
// map[string]int(nil)) allows Go to infer all type parameters automatically.
// Only string map keys are supported; non-string keys panic.
func (a *App[T]) RegisterMap[M ~map[K]V, K comparable, V any](_ M, opts ...RegisterTypeOption) *App[T] {
	if a == nil {
		panic("conga: RegisterMap requires a non-nil app")
	}

	if reflect.TypeFor[K]().Kind() != reflect.String {
		panic(fmt.Sprintf("conga: RegisterMap requires string map keys, got %s", reflect.TypeFor[K]()))
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

// RunContext executes the application pipeline with the provided context:
// parses arguments, validates constraints, and runs the matched command.
// Panics if ctx is nil, and panics if the App has already executed: an App
// runs exactly once. The standard streams are captured at execution start;
// changes made afterwards do not affect the active session. Use Reload for
// re-execution with fresh configuration.
func (a *App[T]) RunContext(ctx context.Context) error {
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

// Run executes the application pipeline with a default context that listens for interruption signals (SIGINT / Ctrl+C, SIGTERM).
// An App runs exactly once: calling Run or RunContext again panics.
func (a *App[T]) Run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return a.RunContext(ctx)
}

// Args sets custom command-line arguments to parse (defaults to os.Args[1:]).
func (a *App[T]) Args(args []string) *App[T] {
	a.args = args
	a.argsExplicitlySet = true

	return a
}

// UsageOnError configures how help/usage is displayed when an input error occurs.
func (a *App[T]) UsageOnError(mode UsageOnErrorMode) *App[T] {
	a.usageMode = mode

	return a
}

// Stdin sets the standard input reader for commands that read input (defaults to os.Stdin).
// The stream is captured when Run/RunContext starts: changes made afterwards do
// not affect the active session; use Reload for re-execution with new streams.
func (a *App[T]) Stdin(reader io.Reader) *App[T] {
	if reader != nil {
		a.stdin = reader
	}

	return a
}

// Stdout sets the standard output writer for help and version messages (defaults to os.Stdout).
// The stream is captured when Run/RunContext starts: changes made afterwards do
// not affect the active session; use Reload for re-execution with new streams.
func (a *App[T]) Stdout(writer io.Writer) *App[T] {
	if writer != nil {
		a.stdout = writer
	}

	return a
}

// Stderr sets the standard error writer for error and usage messages (defaults to os.Stderr).
// The stream is captured when Run/RunContext starts: changes made afterwards do
// not affect the active session; use Reload for re-execution with new streams.
func (a *App[T]) Stderr(writer io.Writer) *App[T] {
	if writer != nil {
		a.stderr = writer
	}

	return a
}

// ExitFunc sets a custom exit handler for FatalIfError (defaults to os.Exit).
func (a *App[T]) ExitFunc(exitHandler func(code int)) *App[T] {
	if exitHandler != nil {
		a.exitFunc = exitHandler
	}

	return a
}

// FatalIfError handles an execution error by printing a user-friendly error message,
// rendering contextual usage instructions according to the configured UsageOnErrorMode,
// and terminating the application with an appropriate exit status code.
//
// Exit codes:
// - Exit code 1 for runtime errors returned by command execution (Runner.Run)
// - Exit code 2 for CLI usage errors (unknown flags, missing required arguments, constraint violations).
// - Exit code 2 for internal completion failures (recovered panics, unwritable stdout), with a diagnostic on stderr.
func (a *App[T]) FatalIfError(err error) {
	if err == nil {
		return
	}

	if compErr, ok := errors.AsType[*completionError](err); ok {
		fmt.Fprintln(a.stderr, compErr)
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

// RegisterTypeOption configures a custom type parser registration.
type RegisterTypeOption func(*typeRegistration)

// WithPredictor attaches a shell completion predictor to the registered custom type.
// Flags and arguments of this type will automatically autocomplete with this predictor
// unless overridden by an explicit struct tag.
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

// appConfig holds every configuration field for an App. It is the single source
// of truth for configuration state: New populates it with defaults and the
// reloader clones it so a reloaded pipeline never drifts from the original.
type appConfig[T any] struct {
	name                string
	description         string
	examples            string
	footer              string
	deprecated          string
	deprecationNotices  []string
	version             string
	envPrefix           string
	configOptions       []config.Option
	configSettings      spec.ConfigSettings
	configRegistry      *configengine.Registry
	configEngine        configEngineFactory
	dotenvPaths         []string
	dotenvLoader        func(paths ...string) error
	dotenvOverloader    func(paths ...string) error
	configData          map[string]string
	args                []string
	argsExplicitlySet   bool
	usageMode           UsageOnErrorMode
	stdout              io.Writer
	stderr              io.Writer
	stdin               io.Reader
	exitFunc            func(code int)
	strictTagValidation bool
	allowedTags         []string
	predictors          map[string]predict.Predictor
	typeRegistry        map[reflect.Type]typeRegistration
	helpEnabled         bool
	versionEnabled      bool
	helpOptions         help.Options
	completionShells    []string
	// inferPredictor performs type-based predictor inference during schema
	// build. Completion installs it; nil keeps the inference machinery
	// (including its filesystem probes) unreferenced at link time.
	inferPredictor func(targetType reflect.Type, tags spec.FieldTags) *spec.PredictorSpec
	// The following feature hooks are method expressions: they receive the
	// executing App as their first argument, so a reloaded App's cloned hooks
	// always dispatch to the fresh receiver. Nil means the subsystem is
	// disabled and unreferenced, so its packages drop at link time.
	whyRenderer                func(app *App[T], cmdChain []*spec.Command, flagValues map[*spec.Flag][]string)
	defaultPredictors          func(app *App[T])
	mountCompletion            func(app *App[T], cmd *spec.Command)
	handleCompletionInvocation func(app *App[T]) error
	completionHelp             func(app *App[T], cmdChain []*spec.Command, completionValue string, withHelp bool) (bool, error)
	renderCompletionScript     func(app *App[T], cmdChain []*spec.Command, shell string) (bool, error)
}

// clone returns a deep copy of the configuration, safe for building a reloaded App.
func (c appConfig[T]) clone() appConfig[T] {
	configSettings := c.configSettings
	configSettings.Tier = slices.Clone(configSettings.Tier)
	c.configSettings = configSettings
	c.configOptions = slices.Clone(c.configOptions)
	c.configRegistry = c.configRegistry.Clone()
	c.dotenvPaths = slices.Clone(c.dotenvPaths)
	c.completionShells = slices.Clone(c.completionShells)
	c.configData = maps.Clone(c.configData)
	c.args = slices.Clone(c.args)
	c.deprecationNotices = nil
	c.allowedTags = slices.Clone(c.allowedTags)
	c.predictors = maps.Clone(c.predictors)
	c.typeRegistry = maps.Clone(c.typeRegistry)

	return c
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
// decoder-required error and surfaces display-only candidates (the environment
// targets and the config-file flag's default), but loads and probes nothing.
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

	return configengine.DefaultConfigCandidates(cmd)
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
	a.initDefaults()

	cmd, err := a.buildSpec()
	if err != nil {
		return predict.None().Invoke(predict.Context{}), ""
	}

	targetShell = cond.SetOrDefaultFunc(targetShell, detectShell)

	opts := spec.CompleteOptions{
		RegisteredPredictors: a.predictors,
		TargetShell:          targetShell,
	}

	patchedArgs, err := completion.Patch(targetShell, args)
	if errors.Is(err, sentinel.ErrRedirectTarget) {
		// A redirect target ("cloudctl deploy > <cursor>") never completes:
		// falling back to the raw arguments would leak subcommand
		// candidates, so short-circuit with the zero-candidate framing
		// (docs/completion-wire-protocol.md §4.1). Other patch errors keep
		// the pass-through behavior below.
		invoked := predict.None().Invoke(predict.Context{})

		return invoked, completion.Format(targetShell, "", invoked)
	}

	normArgs := cond.If(err == nil && len(patchedArgs) > 0, patchedArgs, args)
	if len(normArgs) == 0 {
		normArgs = []string{""}
	}

	invoked := spec.Complete(cmd, normArgs, opts)
	toComplete := normArgs[len(normArgs)-1]

	formatted := completion.Format(targetShell, toComplete, invoked)

	return invoked, formatted
}

func (a *App[T]) completeCompLineForShell(targetShell string, compLine string, compPoint int) (predict.Result, string) {
	words := tokenizer.ParseCompLine(compLine, compPoint)
	if len(words) > 0 {
		words = words[1:]
	}

	return a.completeForShell(targetShell, words)
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
		StrictTagValidation:  a.strictTagValidation,
		AllowedTags:          a.allowedTags,
		RegisteredPredictors: a.predictors,
		TypePredictors:       a.collectTypePredictors(),
		ClassifyContainer:    classifyContainer,
		ConfigFilePatterns: slicex.Map(a.configRegistry.Extensions(), func(extension string) string {
			return "*" + extension
		}),
		ConfigSettings:    a.configSettings,
		ConfigCapable:     a.configEngine != nil,
		DisableHelp:       !a.helpEnabled,
		DisableVersion:    !a.versionEnabled,
		DisableWhy:        a.whyRenderer == nil,
		DisableCompletion: a.mountCompletion == nil,
		CompletionShells:  a.completionShells,
		InferPredictor:    a.inferPredictor,
		EnvPrefix:         a.envPrefix,
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

	if a.footer != "" {
		cmd.Footer = a.footer
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

// resolveConfigAndFlags resolves the configuration cascade and binds flag
// values, recording the evaluated candidate paths and the actually loaded
// config files on the session.
func (a *App[T]) resolveConfigAndFlags(session *Session, cmd *spec.Command, cmdChain []*spec.Command, flagValues map[*spec.Flag][]string) error {
	explicitTargets, err := a.extractExplicitConfigTargets(cmd, cmdChain, flagValues)
	if err != nil {
		return err
	}

	a.configData = make(map[string]string)

	loadedPaths, loadedSource, err := a.resolveAndLoadConfig(cmd, explicitTargets)
	if err != nil {
		return err
	}

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

	session.candidateFiles = a.configurationView(cmd).CandidatePaths(explicitTargets, cmd)
	session.loadedFiles = loadedPaths

	return nil
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
	// (config resolution, flag and argument coercion) reads engine state —
	// the standard streams — from one consistent run context.
	session := &Session{
		root:        getCmdTarget(cmdChain[0]),
		stdout:      a.stdout,
		stderr:      a.stderr,
		stdin:       a.stdin,
		cmdChain:    cmdChain,
		rawArgs:     rawTokens,
		reloadFunc:  a.buildReloadFunc(),
		appName:     a.name,
		appVersion:  a.version,
		helpOptions: a.helpOptions,
	}

	// Hoist builtin detection ahead of config resolution so help/version/why
	// never fail on resolution errors (e.g. a missing env-sourced config path).
	if handled, err := a.handleBuiltinFlags(cmdChain, flagValues); handled || err != nil {
		return nil, handled, err
	}

	if !isMetaCommand(leaf) {
		if err := a.resolveConfigAndFlags(session, cmd, cmdChain, flagValues); err != nil {
			return nil, false, err
		}
	}

	a.collectDeprecationNotices(cmdChain)

	if !isMetaCommand(leaf) {
		if err := validateFlagConstraints(cmdChain); err != nil {
			return nil, false, err
		}
	}

	if err := resolveArgs(leaf, cmdChain, posTokens, a.collectTypeParsers(), coerce.DecodeContext{Stdin: session.stdin, Stdout: session.stdout}); err != nil {
		return nil, false, err
	}

	bindSpecialArgs(leaf, dashArgs, rawTokens)

	if err := validateRunnableContract(leaf, cmdChain); err != nil {
		return nil, false, err
	}

	if !isMetaCommand(leaf) {
		if err := validateCustomValues(ctx, cmdChain); err != nil {
			return nil, false, err
		}
	}

	if err := runValidators(ctx, session, cmdChain); err != nil {
		return nil, false, err
	}

	return session, false, nil
}

func (a *App[T]) handleBuiltinFlags(cmdChain []*spec.Command, flagValues map[*spec.Flag][]string) (bool, error) {
	request := scanBuiltinRequest(flagValues)

	// 1. Completion flag + --help (e.g. `app --completion zsh --help` or `app --completion --help`)
	if request.hasCompletion && request.hasHelp && a.completionHelp != nil {
		return a.completionHelp(a, cmdChain, request.completionValue, true)
	}

	// 2. Bare completion flag (e.g. `app --completion`)
	if request.hasCompletion && request.completionValue == "" && a.completionHelp != nil {
		return a.completionHelp(a, cmdChain, "", false)
	}

	// 3. Standard --help
	if request.hasHelp {
		fmt.Fprintln(a.stdout, help.FormatWith(cmdChain, a.helpOptions))

		return true, nil
	}

	// 4. Standard --version
	if request.hasVersion {
		a.renderVersionBuiltin()

		return true, nil
	}

	// 5. Value-provenance report (e.g. `app --why`)
	if request.hasWhy && a.whyRenderer != nil {
		a.whyRenderer(a, cmdChain, flagValues)

		return true, nil
	}

	// 6. Completion script generation (e.g. `app --completion zsh`)
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
// screen for `--completion <shell> --help`, or the generic screen for a bare
// `--completion`.
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
	// here: completeForShell's Patch pass consumes it, and only that pass
	// sees the redirect-rejection sentinel this dispatch must honor.
	if envWords, ok := resolveCompLineEnv(); ok && len(wordsToComplete) <= 1 {
		wordsToComplete = envWords
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

func (a *App[T]) buildReloadFunc() func(ctx context.Context) (any, error) {
	return func(ctx context.Context) (any, error) {
		newRoot := new(T)

		freshApp := &App[T]{
			rootTarget: newRoot,
			appConfig:  a.clone(),
		}
		freshApp.configData = make(map[string]string)

		if freshApp.dotenvOverloader != nil {
			if err := freshApp.dotenvOverloader(freshApp.dotenvPaths...); err != nil {
				return nil, fmt.Errorf("%w: %w", ErrConfig, err)
			}
		}

		if _, _, err := freshApp.prepare(ctx); err != nil {
			return nil, err
		}

		return newRoot, nil
	}
}

func (a *App[T]) exit(code int) {
	if a.exitFunc != nil {
		a.exitFunc(code)
	}
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
// value of a single-valued one. Supplying the flag without naming a path — an
// empty or blank value — is rejected as a usage error instead of silently
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

// configSettingsFrom converts the public authoring-surface settings snapshot
// into the engine's own configuration settings representation.
func configSettingsFrom(settings config.Settings) spec.ConfigSettings {
	return spec.ConfigSettings{
		Explicit:  settings.Explicit,
		Tier:      settings.Tier,
		Cascading: settings.Cascading,
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
