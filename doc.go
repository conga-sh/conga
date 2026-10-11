// Package conga builds command-line applications from Go structs.
//
// An application is a Go struct whose fields declare its flags, positional
// arguments, and subcommands. CONGA inspects the struct once at startup, then
// populates the fields from command-line arguments, environment variables, and
// configuration files.
//
// # Declaring a command
//
// Fields use container types to define the CLI interface:
//
//   - Flags: [OptionalFlag], [RequiredFlag], [Counter], [ConfigFile]
//   - Positional arguments: [OptionalArg], [RequiredArg], [DashArgs], [RawArgs]
//   - Subcommands: [Command]
//
// Struct tags declare names, help text, fallback values, and constraints such
// as range, len, enum, xor, and and. For the full tag specification,
// See https://docs.conga.sh/reference/struct-tags/.
//
// Commands execute by implementing [Runner]. Lifecycle hooks can be added
// with [Validator], [PreRunner], [PostRunner], and [Finalizer].
//
// # Example
//
//	type ServerCLI struct {
//		Host conga.OptionalFlag[string] `short:"H" default:"127.0.0.1" help:"Address to bind"`
//		Port conga.RequiredFlag[int]    `short:"p" env:"PORT" range:"1..65535" help:"Port number"`
//	}
//
//	func (s *ServerCLI) Run(ctx context.Context, session *conga.Session) error {
//		fmt.Fprintf(session.Stdout(), "Listening on %s:%d\n", s.Host.Get(), s.Port.Get())
//		return nil
//	}
//
//	func main() {
//		app := conga.New[ServerCLI]()
//
//		if err := app.Run(); err != nil {
//			app.Exit(err)
//		}
//	}
//
// # Value resolution
//
// Flags resolve through four sources in order of precedence:
//
//  1. Command-line flags
//  2. Environment variables
//  3. Configuration files
//  4. Struct field default tags
//
// Positional arguments resolve only from command-line arguments and default
// tags. [OptionalFlag.Source] reports where a resolved value originated.
//
// See https://docs.conga.sh/explanation/value-precedence/.
//
// # Built-ins
//
// CONGA provides built-in flags for standard operational tasks:
//
//   - --help: prints command syntax and flag descriptions. Enabled
//     by default; disable with [App.DisableHelpFlag].
//   - --version: prints version and build information. Enabled by
//     default; disable with [App.DisableVersionFlag].
//   - --why: prints a provenance report showing where every flag resolved
//     from (CLI flag, environment variable, configuration file, or default tag)
//     and which competing values were overridden. Useful when debugging complex
//     configurations. Enable with [App.EnableWhy].
//
// When a built-in flag is passed, CONGA prints the output to standard output
// and exits immediately without running command logic; [App.Run] returns nil.
//
// For applications that prefer subcommands over flags, CONGA also provides
// [HelpCommand], [VersionCommand], and [CompletionCommand]. Declare them
// directly as struct fields on the command. These subcommands bypass lifecycle
// hooks and render their output to standard output.
//
// See https://docs.conga.sh/reference/builtins/.
//
// # Execution and streams
//
// [App.Run] executes with a default context that cancels on SIGINT and
// SIGTERM; [App.RunContext] executes with the caller's context. An App
// executes exactly once; a second call panics. A zero-value [App] is usable
// and installs the same defaults as [New] on first use.
//
// Commands receive a [*Session] providing invocation metadata and configured
// I/O streams ([Session.Stdout], [Session.Stderr], [Session.Stdin]). Streams
// default to the standard process streams ([os.Stdout], [os.Stderr], [os.Stdin]),
// and can be redirected on the builder via [App.Stdout], [App.Stderr], or
// [App.Stdin] for testing or embedded execution.
//
// [App.Exit] reports errors and exits with status 2 for usage errors
// or status 1 for runtime failures. A nil error exits with status 0.
//
// See https://docs.conga.sh/explanation/lifecycle/.
//
// # Optional subsystems
//
// Several features are opt-in:
//
//   - Shell completion: call [App.EnableCompletion] to generate completion
//     scripts for Bash, Zsh, Fish, PowerShell, Elvish, and Nushell.
//   - Configuration decoders: import and register decoders such as
//     decoder/yaml using [App.RegisterDecoder].
//   - Dotenv files: call [App.Dotenv] to load variables from .env files.
package conga
