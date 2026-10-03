// Package conga binds command-line arguments, environment variables, and
// configuration files into Go struct fields.
//
// CONGA uses typed container structs to declare CLI flags, positional
// arguments, and subcommands directly on application structs:
//
//   - Flags: [OptionalFlag], [RequiredFlag], [Counter], [ConfigFile]
//   - Positional arguments: [OptionalArg], [RequiredArg], [DashArgs], [RawArgs]
//   - Subcommands: [Command]
//
// Command schemas, validation rules, and completion hooks are inspected once at
// startup. Value binding at execution time assigns through the container's own
// state accessors, so no per-token reflection occurs on the binding path.
//
// # Quick Start
//
// Define an application struct with typed containers and struct tags, implement
// [Runner], and invoke [App.Run]:
//
//	package main
//
//	import (
//		"context"
//		"fmt"
//
//		"github.com/conga-sh/conga"
//	)
//
//	type ServerCLI struct {
//		Host conga.OptionalFlag[string] `short:"H" default:"127.0.0.1" help:"Host address to bind"`
//		Port conga.RequiredFlag[int]    `short:"p" env:"PORT" help:"Port number (1-65535)" range:"1..65535"`
//	}
//
//	func (c *ServerCLI) Run(ctx context.Context, session *conga.Session) error {
//		fmt.Fprintf(session.Stdout(), "Listening on %s:%d\n", c.Host.Get(), c.Port.Get())
//		return nil
//	}
//
//	func main() {
//		app := conga.New[ServerCLI]()
//		app.FatalIfError(app.Run())
//	}
//
// # Value Resolution Precedence
//
// Flags resolve through a deterministic four-tier precedence ladder:
//
//  1. Command-line flags (e.g. --port 9000, -p 9000)
//  2. Environment variables (e.g. PORT=9000 or APP_PORT=9000 with [App.EnvPrefix])
//  3. Configuration files (.json, .yaml, .toml, .hcl)
//  4. Struct tag defaults (`default:"8080"`)
//
// Positional arguments resolve from command-line arguments only, falling back to
// a `default` struct tag. They never bind environment variables or configuration
// file keys.
//
// Containers record the origin of their resolved value: call [OptionalFlag.Source]
// and [OptionalFlag.SourceDetail] to inspect where a value originated, or enable
// [App.EnableWhy] to generate a full provenance report via `--why`.
//
// See https://docs.conga.sh/explanation/value-precedence/.
//
// # Execution Lifecycle
//
// Before the lifecycle runs, CONGA resolves configuration, binds and coerces
// every flag and positional argument, then enforces constraints in this order:
// struct tag constraints (`range`, `len`, `items`, `enum`, `xor`, `and`,
// `negatable`), the leaf command's [Runner] contract, domain-type [SelfValidator]
// checks, and finally [Validator] hooks.
//
// Command execution then progresses through a five-stage lifecycle:
//
//  1. Validate: [Validator.Validate] runs top-down (root to leaf). An error
//     aborts the run and is returned to the caller; [App.FatalIfError] reports
//     it as a usage error with exit status 2.
//  2. PreRun: [PreRunner.PreRun] runs top-down to initialize shared resources.
//  3. Run: [Runner.Run] executes on the selected leaf command.
//  4. PostRun: [PostRunner.PostRun] runs bottom-up on success.
//  5. Finally: [Finalizer.Finally] runs bottom-up unconditionally for every
//     initialized command, receiving the halting error or nil.
//
// If any stage between PreRun and PostRun returns an error, intermediate steps
// are skipped and execution jumps directly to Finally for cleanup.
//
// The built-in subcommands ([HelpCommand], [VersionCommand],
// [CompletionCommand]) are meta-commands: they bypass the lifecycle entirely, so
// ancestor PreRun, PostRun, and Finally hooks never execute for them.
//
// See https://docs.conga.sh/explanation/lifecycle/.
//
// # Execution Session and Streams
//
// Commands receive a [*Session] providing invocation metadata ([Session.Path],
// [Session.Name], [Session.Root]) and configured I/O streams ([Session.Stdout],
// [Session.Stderr], [Session.Stdin]). Streams default to the standard process
// streams ([os.Stdout], [os.Stderr], [os.Stdin]), and can be redirected on the
// builder via [App.Stdout], [App.Stderr], or [App.Stdin] for testing and embedded
// CLI execution.
//
// Standard streams are snapshotted when execution begins; builder modifications
// made after execution starts do not affect the active session.
//
// # Struct Tag Reference
//
// Struct tags declare metadata, validation constraints, and binding behaviors.
// The authoritative reference, including bounds syntax, help variables, and
// `conga` directives, is published at
// https://docs.conga.sh/reference/struct-tags/.
//
// A tag is only valid on the constructs listed below. Declaring it elsewhere
// fails schema compilation with [ErrSchema].
//
// Flags ([OptionalFlag], [RequiredFlag], [Counter]) accept:
//
//   - `short:"x"`: Single-rune short flag alias (e.g. `-p`).
//   - `name:"flag"`: Flag name override (default: kebab-case field name).
//   - `aliases:"a,b"`: Comma-separated long-name aliases.
//   - `help:"text"`: Help text rendered on `--help`.
//   - `default:"val"`: Fallback value when no other source supplies one.
//   - `env:"VAR"`: Comma-separated environment variable names. `env:"-"` disables
//     environment binding; explicit names ignore [App.EnvPrefix].
//   - `config:"key"`: Configuration file key (default: the flag name). `config:"-"`
//     excludes the field from configuration files.
//   - `range:"min..max"`: Inclusive bounds for numeric types, [time.Duration], and
//     text-unmarshaled types exposing an ordered projection.
//   - `len:"min..max"`: String length or slice/map length constraint.
//   - `items:"min..max"`: Slice or map element count constraint.
//   - `enum:"a,b,c"`: Restricts accepted values to a comma-separated set.
//   - `xor:"group"`: Enforces mutual exclusivity among flags sharing the group name.
//   - `and:"group"`: Requires all flags in the group to be provided together.
//   - `sep:"ch"`: Slice element separator (default `,`). `sep:"none"` disables
//     splitting and preserves the unparsed string.
//   - `negatable:"true"`: Boolean flags only; enables `--[no-]flag` syntax.
//   - `predictor:"name"`: Selects a completion predictor registered via [App.Predictor].
//   - `placeholder:"NAME"`: Value placeholder displayed in help syntax.
//   - `group:"Heading"`: Help-screen grouping header.
//   - `hidden:"true"`: Suppresses the flag from help screens and completion.
//   - `safe:"true"`: Controls whether the value is displayed in `--why` reports.
//   - `deprecated:"msg"`: Emits a deprecation warning on standard error when used.
//   - `conga:"directive"`: Engine directives; see the reference above.
//
// Positional arguments ([OptionalArg], [RequiredArg]) accept a narrower set:
// `name` (rendered as the usage placeholder), `help`, `default`, `enum`, `range`,
// `len`, `items`, `sep`, `predictor`, and `conga`. Positional arguments never bind
// environment variables or configuration file keys.
//
// Subcommands ([Command]) accept `name`, `summary` (one-line synopsis),
// `aliases`, `group`, `hidden`, `deprecated`, `envprefix` (an environment prefix
// scoped to this subcommand and its descendants), and `conga`. Long-form
// subcommand descriptions come from [DescriptionProvider], not a tag.
//
// Configuration-file flags ([ConfigFile]) accept `name`, `short`, `help`,
// `aliases`, `placeholder`, `group`, `hidden`, `deprecated`, `env`, `sep`,
// `conga`, and `multi`. On this construct `config` takes only the value `"-"`,
// which opts the flag out of configuration binding; any other value is a schema
// error. `multi:"true"` is likewise exclusive to [ConfigFile]: it switches the
// flag to repeated-argument and slice arity so cascaded files accumulate in
// [ConfigFile.Paths].
//
// Trailing-argument collectors ([DashArgs], [RawArgs]) accept `name` and
// `conga`.
//
// Domain types declare capability tags of their own. [File] accepts `existing`,
// `stdin`, `stdout`, `executable`, `symlink`, and `glob`; the combinations that
// conflict are listed on [File]. [Dir] accepts `existing`. [FileContent] accepts
// none and always resolves `-` to standard input.
//
// # Struct Tag Validation and Foreign Tags
//
// CONGA strictly validates struct tags at startup by default. Unrecognized tag keys,
// typographical errors, and tags applied to incompatible constructs cause schema
// compilation to fail with an error wrapping [ErrSchema].
//
// Common third-party tags are recognized out of the box and accepted silently:
//
//	json, yaml, yml, toml, hcl, xml, mapstructure, envconfig,
//	validate, binding, gorm, db, doc, example, description
//
// Additional external or domain-specific tag keys (e.g. `kong`, `bson`, or custom
// annotations) can be whitelisted schema-wide using [App.AllowTags]. Alternatively,
// `conga:"allow-unknown-tags"` permits arbitrary tags on a specific field or command,
// and [App.StrictTagValidation] disables strict tag checking entirely.
//
// # Opt-in Subsystems
//
// CONGA isolates optional subsystems so that unreferenced features are pruned
// at link time, keeping compiled binaries lean:
//
//   - Shell completion: call [App.EnableCompletion] to register the drivers and the
//     hidden `__complete` wire protocol. A mounted [CompletionCommand] requires it;
//     without it the subcommand is dropped from the schema without warning.
//   - Configuration decoders: import decoder/json, decoder/yaml, decoder/toml, or
//     decoder/hcl and call [App.RegisterDecoder]. Without a registered decoder the
//     configuration tiers are skipped.
//   - Dotenv loading: call [App.Dotenv].
//   - Value provenance: call [App.EnableWhy].
//
// See https://docs.conga.sh/explanation/binary-size/.
//
// # Execution Invariants
//
// An [App] instance executes exactly once. Calling [App.Run] or [App.RunContext]
// a second time panics.
package conga
