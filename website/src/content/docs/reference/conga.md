---
title: package conga
description: Core API — App builder, containers, session accessors, extensibility, and errors.
---

Deep detail lives on [pkg.go.dev/github.com/conga-sh/conga](https://pkg.go.dev/github.com/conga-sh/conga). This page is the curated map.

## The App builder

```go
func New[T any](root ...*T) *App[T]
```

`App[T]` wraps your command struct. `New` defers instance allocation to execution. Pass your own instance (`conga.New(cli)`) to inspect bound container values after execution completes, as [How to read resolved values after Run](/how-to/read-values-after-run/) shows. All builder methods return `*App[T]` for chaining.

`New` is the recommended constructor. The zero value of `App[T]` is also usable, because the execution entry points install the same defaults `New` sets, so `var app conga.App[CLI]` behaves like `conga.New[CLI]()`.

### Configuration methods

| Method | Purpose |
|--------|---------|
| `Name(name)` | Application name shown in help, usage, and version output (falls back to `os.Args[0]`) |
| `Description(desc)` | Root description |
| `Examples(examples)` | Usage examples on `--help` |
| `HelpFooter(footer)` | Footer rendered on every help screen |
| `Deprecated(reason)` | Whole-app deprecation banner |
| `Version(version)` | `--version` display string (falls back to build metadata) |
| `EnvPrefix(prefix)` | Default environment variable prefix for flags (appends trailing `_` if omitted). Does not apply to `Features` containers. A subcommand's `envprefix` tag replaces it. |
| `Dotenv(paths...)` | Enable `.env` loading (default `.env` + `.env.local`; [rules](/reference/dotenv/)) |
| `ConfigOptions(options...)` | Config resolution: `conga.ConfigExplicit(path)`, `conga.ConfigPaths(paths...)`, `conga.ConfigCascading()`, `conga.ConfigKeyCase(keyCase)`; `ConfigExplicit` cannot be combined with paths or cascading. A `conga.Bootstrapper` may additionally contribute a document with `Bootstrap.SetConfig` |
| `RegisterDecoder(ext, decoder)` | Per-app config format decoder |
| `AllowTags(tags...)` | Allow additional struct tag keys (matched exactly) |
| `Predictor(name, p)` | Register named completion predictor (panics on misuse) |
| `Use(bindings...)` | Apply builder bindings (`network.RegisterInterface`, `timezone.RegisterLocation`, …) |
| `RegisterType[U](parser, opts...)` | Register a parser (and optional predictor) for type `U` |
| `RegisterMap[M](opts...)` | Register a `map[K]V` parser for a map type with string-compatible keys |
| `Args(args)` | Custom argument slice (defaults to `os.Args[1:]`) |
| `UsageOnError(mode)` | Error-time usage rendering mode |
| `Stdin(r)` | Input reader for `session.Stdin()`, `File` fields with `stdin`, and `FileContent` reading `-` (default `os.Stdin`) |
| `Stdout(w)` / `Stderr(w)` | Output writers (default `os.Stdout`/`os.Stderr`) |
| `SetenvFunc(fn)` | Writer for variables loaded by [Dotenv](/reference/dotenv/) (default `os.Setenv`) |
| `ExitFunc(handler)` | Custom exit handler for `Exit` |
| `EnableWhy(options...)` / `EnableCompletion(options...)` / `EnableFeatures()` | Opt in to the `--why` report, shell completion, and struct-declared feature toggles; pass `conga.WhyOptionJSON()` to `EnableWhy` to enable `--why=json` |
| `DisableHelpFlag()` / `DisableVersionFlag()` | Remove the `--help`/`--version` flags |
| `HelpShortFlag('h')` / `VersionShortFlag('V')` | Assign short runes to the `--help`/`--version` flags (collisions fail schema build with `ErrDuplicateFlag`) |

### Execution methods

| Method | Purpose |
|--------|---------|
| `Run()` | Parse + execute with a signal-aware context (`SIGINT`/`SIGTERM`) |
| `RunContext(ctx)` | Execute with your context; panics on a nil context |
| `Exit(err)` | Render error messages and exit (0/1/2 contract) |

### Extensibility functions

```go
func (a *App[T]) RegisterType[U any](parser TypeParser[U], opts ...RegistrationOption) *App[T]
func (a *App[T]) RegisterMap[M ~map[K]V, K ~string, V any](opts ...RegistrationOption) *App[T]
func WithPredictor(p predict.Predictor) RegistrationOption
```

`TypeParser[U]` is a `func(raw string) (U, error)`, and an error it returns classifies under `ErrCoerce` for a command-line value and `ErrConfig` for one from the environment, a config file, or a `default` tag. `RegistrationOption` customizes registrations, and `WithPredictor` attaches default completion to a registered type.

## Containers

| Container | Methods |
|-----------|----------------|
| `OptionalFlag[T]` / `RequiredFlag[T]` | `Get`, `IsSet`, `Changed`, `Source`, `SourceDetail`, `String` (`GetOr` on optional only) |
| `OptionalArg[T]` / `RequiredArg[T]` | same as flags |
| `Counter` | same as flags, value is `int` |
| `ConfigFile` | root-only config path flag; `multi:"true"` for multi-path (`Path`, `Paths`) |
| `Features[T]` | root-only feature declaration; values read through `Get`, plus `IsSet`, `Changed`, `Source`, `SourceDetail`, `String` |
| `DashArgs` / `RawArgs` | `Get []string`, `GetOr`, `Len`, `IsSet`, `String` |
| `Command[T]` | `Get() *T`, `IsSelected()` |

`HelpCommand` (with `Path() []string`), `VersionCommand` (with `JSON() bool`), and `CompletionCommand` (with `Shell() string`) are the opt-in built-in subcommand containers.

`RequiredFlag[T]` rejects `default`, `xor`, and `oneof` tags. When satisfied by input, environment, or configuration, both `Changed()` and `IsSet()` report true.

## Lifecycle interfaces

| Interface | Method | Stage |
|-----------|--------|-------|
| `Validator` | `Validate(ctx, session) error` | Cross-field command validation |
| `PreRunner` | `PreRun(ctx, session) error` | Resource initialization |
| `Runner` | `Run(ctx, session) error` | Command body (required) |
| `PostRunner` | `PostRun(ctx, session) error` | Post-success follow-up |
| `Finalizer` | `Finally(ctx, session, err) error` | Guaranteed teardown |
| `DescriptionProvider` / `ExampleProvider` | `Description()` / `Examples()` | Custom command metadata |

Value-shape interfaces (`SelfValidator`, `HelpProvider`, `TagAwareHelpProvider`, `ContextAwareTextUnmarshaler`) and the predictor hooks are cataloged in [Domain types](/reference/domain-types/#domain-type-interfaces).

## Session and hierarchy accessors

```go
func (s *Session) Root[T any]() *T
func (s *Session) Stdin() io.Reader
func (s *Session) Stdout() io.Writer
func (s *Session) Stderr() io.Writer
func (s *Session) Help() string
func (s *Session) Usage() string
func (s *Session) Version() string
func (s *Session) Why() string
func (s *Session) WhyJSON() string
func (s *Session) Path() string
func (s *Session) Name() string
func (s *Session) CandidateConfigFiles() []string
func (s *Session) LoadedConfigFiles() []string
```

`Root` returns the application struct bound to `T`, either the instance passed to
`New` or the engine-allocated instance. The type argument `T` must match the
exact root command struct type, or `Root` panics. Enclosing command structs are reached
by walking the selected `Command[T]` fields (`root.Cluster.Get()`).

`Why` and `WhyJSON` render the same value-provenance report as the built-in `--why` flag. They return an empty string when Why is not enabled, and `WhyJSON` also requires `app.EnableWhy(conga.WhyOptionJSON())`.

The App snapshots standard streams at the start of execution for the active session.

`CandidateConfigFiles`/`LoadedConfigFiles` report the configuration sources the run evaluated. These are file paths for file-based resolution, plus the distinct origin paths the application recorded when a [`conga.Bootstrapper`](/how-to/load-domain-documents/) supplies a document with `Bootstrap.SetConfig`. Document scopes recorded with `ConfigDocument.SetScoped` are not part of the reported path.

`Help`, `Usage`, and `Version` are the formatting primitives behind the built-in `help` and `version` commands. `Help` returns the active command's help screen as a string, `Usage` returns the syntax usage summary as a string, and `Version` returns the formatted version summary line as a string, falling back to the module version (or `dev`) plus the embedded build metadata when no version string was configured.

## Domain types

`File`, `FileContent`, `Dir`, `ByteSize` — see [Domain types](/reference/domain-types/).

## Value sources

`ValueSource` constants are `SourceNone`, `SourceDefault`, `SourceConfig`, `SourceEnv`, and `SourceCLI`. [About value precedence](/explanation/value-precedence/#provenance-tracking) gives their meanings.

## Usage rendering modes

`UsageOnErrorMode`: `UsageSummary` (default), `UsageFull`, `UsageHint`, `UsageSilence`.

## Errors

Base categories (`ErrSchema`, `ErrParse`, `ErrCoerce`, `ErrValidation`, `ErrConfig`) plus the `UsageError` and `CompletionError` interfaces and `IsUsageError(err)` / `IsCompletionError(err)` classifications — cataloged in [Errors and exit codes](/reference/errors/).

## Provenance options

```go
func WhyOptionJSON() WhyOption

type WhyOption interface{ /* sealed */ }
```

`WhyOption` is a sealed interface produced by option constructors such as `WhyOptionJSON()`. `WhyOptionJSON` enables machine-readable `--why=json` reports on `App.EnableWhy`. Without it, the JSON renderer is not linked into the binary.

## Completion options

```go
func CompletionEnvPrefix(prefix string) CompletionOption

type CompletionOption interface{ /* sealed */ }
```

`CompletionOption` is a sealed interface produced by option constructors such as `CompletionEnvPrefix`. Pass `CompletionEnvPrefix` to `App.EnableCompletion` to override the derived environment-variable prefix (`<APP>_`) that names the completion environment variables `<APP>_COMPLETION_DESCRIPTIONS`, `<APP>_COMPLETION_ALIASES`, and `<APP>_COMPLETION_BARE_SPACE_FLAGS`. The prefix is normalized to upper snake case, and an empty prefix keeps the application-name derivation. POSIX shells cannot assign a variable name that begins with a digit, so a prefix derived from such an application name gains a leading underscore (an application named `7z` reads `_7Z_COMPLETION_DESCRIPTIONS`), while an explicit prefix that normalizes to a leading digit is rejected at schema build.

## Configuration options

`app.ConfigOptions` takes `conga.ConfigOption` values from four constructors. `conga.ConfigPaths(paths...)` sets the search paths, `conga.ConfigExplicit(path)` names one file that must exist, and `conga.ConfigCascading()` merges every file found instead of taking the first. `ConfigExplicit` cannot be combined with `ConfigPaths` or `ConfigCascading`. `conga.ConfigKeyCase(keyCase)` sets the casing of keys derived from flag, subcommand, and feature names: `conga.KebabCase` (the default), `conga.SnakeCase`, or `conga.CamelCase`. Each key has exactly one spelling, explicit `config` tags are read as written, and an unknown `KeyCase` value panics. `conga.ConfigDecoder` is the decoder type `app.RegisterDecoder` takes, `func(data []byte) (map[string]any, error)`.

### Config file resolution

CONGA picks config files in this order, and the first that applies wins:

1. The root `ConfigFile` flag (`--config-file` unless renamed).
2. The config environment variable, `<PREFIX>_CONFIG` for the app's environment prefix.
3. `conga.ConfigExplicit(path)`.
4. The search paths: those given to `conga.ConfigPaths(...)`, in order, or else the built-in location `${XDG_CONFIG_HOME:-~/.config}/<app-name>`.

The built-in location applies when the app has at least one registered decoder and a resolvable name, and only when no `ConfigPaths` are declared. `ConfigExplicit` also bypasses it.

Each search path is probed for every registered extension. An existing directory is searched for `config.<ext>` and `<app-name>.<ext>`, an existing file is used as it is, and a path that doesn't exist is tried as `<path>.<ext>`. Without `ConfigCascading`, the first path that yields a file wins. With it, every path that yields a file is loaded and merged, and the first-listed path wins each key. Cascading requires a declared root `ConfigFile` to carry `multi:"true"`.

| Situation | Result |
|-----------|--------|
| A file named by the `ConfigFile` flag, the config environment variable, or `ConfigExplicit` doesn't exist | `ErrConfig`, exit 1 |
| A named file's extension has no registered decoder | `ErrConfig`, exit 1 |
| A search path yields no file | Skipped, and probing continues |
| One search path yields several files with registered extensions | `ErrConfig`, exit 1: `multiple conflicting config files found` |
| No file is found anywhere | The run continues on the other sources |
| A file larger than 16 MiB | `ErrConfig`, exit 1, naming the size and the limit |
| Malformed content | `ErrConfig`, exit 1 |
| An unknown key | `ErrConfig`, exit 1, with a "did you mean" hint for near misses |

With no decoder registered, the `ConfigFile` flag's help text ends in `(no configuration decoder registered)`.

### Bootstrap and config documents

```go
type Bootstrapper interface {
	Bootstrap(ctx context.Context, b *Bootstrap) error
}

func (b *Bootstrap) Path() string

func (b *Bootstrap) Name() string

func (b *Bootstrap) LookupEnv(key string) (string, bool)

func (b *Bootstrap) Dotenv(paths ...string) error

func (b *Bootstrap) DotenvExplicit(paths ...string) error

func (b *Bootstrap) SetConfig(doc *ConfigDocument)

type ConfigDocument struct{ /* ... */ }

func (d *ConfigDocument) Set(key string, value any, origin string) *ConfigDocument

func (d *ConfigDocument) SetScoped(key string, value any, origin, scope string) *ConfigDocument

func (d *ConfigDocument) SetKey(key, entryKey string, value any, origin string) *ConfigDocument

func (d *ConfigDocument) SetKeyScoped(key, entryKey string, value any, origin, scope string) *ConfigDocument
```

A command struct that implements `Bootstrapper` runs `Bootstrap` after the bootstrap pass, which binds the fields tagged `bootstrap:"true"` from the command line, environment, and defaults. The normal binding pass then binds every field, the tagged ones included, with config added. An error returned from `Bootstrap` wraps `ErrConfig` and exits 1.

| `Bootstrap` method | Effect |
|---|---|
| `Path()` | The selected invocation path, such as `cloudctl deploy worker` |
| `Name()` | The matched leaf command name |
| `LookupEnv(key)` | The environment this invocation sees, like `os.LookupEnv`, including variables loaded by earlier `Dotenv` calls |
| `Dotenv(paths...)` | Loads each existing `.env` file through `App.SetenvFunc`, skipping missing ones. With no paths, loads `.env` then `.env.local`. Never overwrites a variable already in the process environment |
| `DotenvExplicit(paths...)` | Like `Dotenv`, but a missing file is an `ErrConfig` error |
| `SetConfig(doc)` | Supplies the run's config document. A later call replaces an earlier one, and nil leaves the document inactive |

The document is the highest-precedence part of the config source and overlays file values key by key, giving `CLI > environment > document > config file > default`. Each entry's origin appears in `--why`.

| `ConfigDocument` method | Effect |
|---|---|
| `Set(key, value, origin)` | Records a whole value for `key`. A later `Set` of the same key replaces the entry, including its `SetKey` entries |
| `SetKey(key, entryKey, value, origin)` | Records one entry of the map value `key`, merging key by key with earlier writes, so a later layer overrides only the map keys it names |
| `SetScoped(key, value, origin, scope)` | `Set` with a scope within the origin. `--why` attributes the value as `origin › scope`, and the Config paths table lists the origin once |
| `SetKeyScoped(key, entryKey, value, origin, scope)` | `SetKey` with a scope, attributed the same way |

All four methods return the document for chaining, and panic on a nil receiver. An empty scope behaves like the unscoped method. See [How to load domain documents](/how-to/load-domain-documents/).
