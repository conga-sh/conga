// Package conga is a batteries-included CLI framework for Go with
// linker-friendly opt-in subsystems.
//
// Commands are declared as plain structs. Typed containers such as
// [OptionalFlag], [RequiredFlag], [OptionalArg], and [RequiredArg] mark fields
// as flags and positional arguments, while [Command] introduces subcommands.
// Struct tags configure identification, help text, 12-factor binding,
// validation constraints, and completion predictors — see the complete
// reference in docs/struct-tags.md.
//
// # Configuration cascade
//
// Every flag value resolves through a deterministic precedence ladder:
// CLI > environment > configuration file > default. Positional arguments
// resolve from the command line alone. Each resolved value reports its origin
// through the Source and SourceDetail accessors, using the [ValueSource]
// constants [SourceNone], [SourceDefault], [SourceConfig], [SourceEnv], and
// [SourceCLI]. Configuration files load from explicit paths, declared search
// paths, or the built-in XDG tier, and .env files feed the environment tier.
// JSON, TOML, YAML, and HCL decoders live in their own opt-in packages
// (decoder/json, decoder/toml, decoder/yaml, decoder/hcl), each exporting a
// RawDecoder wired in explicitly via [App.RegisterDecoder].
//
// # Lifecycle
//
// A command implements the [Runner] interface (Run). The optional hooks
// [PreRunner], [Validator], [PostRunner], and [Finalizer] wrap execution, with
// [Finalizer] guaranteed to run for teardown. Build an application with [New],
// configure it with the fluent builder methods on [App], then invoke Run or
// RunContext.
//
// # Execution session
//
// Subcommands and lifecycle hooks access execution state through [Session].
// [Session.Root] returns the root application struct, and enclosing commands
// are reached by walking Command[T] fields from there. [Session.Path] and
// [Session.Name] report the invocation path, [Session.CandidateConfigFiles],
// [Session.LoadedConfigFiles], and [Session.Reload] expose evaluated and
// reloadable configuration, and [Session.Stdin], [Session.Stdout], and
// [Session.Stderr] return the configured standard streams, captured when the
// App starts executing. Domain types implementing [ContextAwareTextUnmarshaler]
// receive the field's declared tags and these streams at decode time, so "-"
// stdio conventions follow App.Stdin/App.Stdout instead of the process globals.
//
// # Linker-friendly subsystems
//
// Completion ([App.EnableCompletion]), configuration ([App.RegisterDecoder]
// and [App.ConfigOptions]), .env loading ([App.Dotenv]), and the provenance
// report ([App.EnableWhy]) link only when an application enables them. Unused
// payloads are dead-code eliminated, so a minimal binary carries minimal
// framework weight.
package conga
