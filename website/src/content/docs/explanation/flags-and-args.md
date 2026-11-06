---
title: About flags and args
description: Architectural design of CONGA input modelling, container types, and argument binding.
---

Command-line interfaces accept two primary forms of input: named flags and positional arguments. CONGA models both as typed container fields declared directly on command structs.

Before any lifecycle hook executes, the engine resolves every container across command-line arguments, environment variables, config files, and default tags. Command bodies read from these resolved containers rather than interacting with the resolution pipeline. For the complete container API, see [package conga](/reference/conga/#containers); for struct tag syntax, see the [struct tag reference](/reference/struct-tags/).

## Approaches to input in Go

Go command-line libraries model input in one of two ways.

The standard library's `flag`, `spf13/cobra`, and `urfave/cli` register flags on a flag set at run time. Each flag binds to a separate variable through a pointer, or is looked up by name with calls such as `cmd.Flags().GetString("addr")`. The shape of the command line lives apart from the program's data, and a wrong name or type shows up as an error at run time.

`alecthomas/kong` and `alexflint/go-arg` read the shape from a struct instead: each field, with its tags, is a flag of a plain Go type (`string`, `int`, `bool`) or a pointer (`*string`). Declarations stay short and read like ordinary Go.

CONGA follows the struct-based approach with dedicated generic containers: `OptionalFlag[T]`, `RequiredFlag[T]`, and `RequiredArg[T]`.

## The container trade-off

Using container types introduces an explicit trade-off between declaration brevity and runtime capability.

Wrapping fields in `OptionalFlag[string]` requires more typing than declaring a bare `string`. Accessing the value also requires an explicit method call: `cmd.Addr.Get()` rather than `cmd.Addr`. For straightforward utilities that only need basic flag parsing, this extra layer can feel verbose compared to plain struct fields.

CONGA accepts this syntactic overhead because containers solve several structural challenges in multi-tier CLI applications:

### Separating value access from presence checks

Bare fields and pointers conflate presence with value: an omitted flag and an explicit zero look the same, and a pointer field must be nil-checked before use. Containers keep the two apart. `cmd.Port.Get()` returns the value as `T`, with no pointer to check. A required flag or argument without a value fails before any lifecycle hook (`Validate`, `PreRun`, `Run`) runs, so handlers never see an empty required container. When the difference matters, `cmd.Port.IsSet()` tells an explicit `--port 0` from an omitted flag, and `cmd.Port.GetOr(8080)` supplies a fallback in one expression.

### Decoupling presence and provenance from values
CONGA is built around a multi-tier resolution ladder where values can originate from CLI flags, environment variables, config files, or hardcoded defaults. Bare fields and pointers discard that origin once populated.

Containers retain their resolution history throughout execution:
- `cmd.Addr.IsSet()` indicates whether any tier (including a default tag) provided a value.
- `cmd.Addr.Changed()` indicates whether an external source (CLI, env, or config) altered the value from its default.
- `cmd.Addr.Source()` identifies which tier supplied the value (`SourceCLI`, `SourceEnv`, `SourceConfig`, `SourceDefault`).
- `cmd.Addr.SourceDetail()` provides the concrete origin, such as the environment variable name or config file path.

This metadata powers diagnostic features like the `--why` provenance report and allows commands to adjust behaviour based on how a setting was configured.

## Schema defaults versus dynamic fallbacks

CONGA offers two ways to give an optional input a fallback.

A `default:"..."` tag is part of the schema. When no other source sets the input, binding fills the container from the tag (`IsSet()` is true, `Changed()` is false, `Source()` is `SourceDefault`), and help screens show the default.

`GetOr(fallback)` runs in the command body and applies only when the container is unset (`IsSet()` is false, `Source()` is `SourceNone`). It suits fallbacks that depend on the running system, such as a directory found at run time, or on other flags.

`GetOr` is available on optional containers; required containers guarantee a validated value prior to execution.

## Flags versus positional arguments

Flags and positional arguments serve different roles in a CLI, and CONGA enforces their differences through distinct container types:

### Addressing and order
Flags are addressed by name (`--output`, `-o`). They are order-independent and may appear anywhere on the command line relative to other flags.

Positional arguments (`RequiredArg[T]`, `OptionalArg[T]`) have no names on the command line. Their identity depends entirely on their position. In Go source code, their sequence matches the order of the fields declared on the command struct.

### Resolution scope
Flags participate in the complete four-tier resolution ladder. A flag value can come from the command line, an environment variable, an active config file, or a default tag.

Positional arguments represent immediate execution targets, such as file paths, hostnames, or artifact identifiers. They resolve strictly from command-line tokens, isolating execution targets from ambient environment or configuration settings.

## Deterministic positional evaluation

Positional parsing in CONGA is greedy and deterministic. The engine iterates through command-line tokens from left to right, binding them to argument containers in declaration order.

Because parsing does not backtrack, CONGA enforces a strict structural invariant: all required arguments must appear before any optional argument. A struct declaring an optional argument before a required one fails during startup with `ErrSchema`.

This rule eliminates grammar ambiguity. If an optional argument could precede a required argument, an invocation supplying fewer tokens than parameters would be ambiguous: the first token could satisfy either the optional parameter or the mandatory one. Requiring mandatory arguments first ensures that every token binding is predictable.

Positional arguments belong to leaf commands. A command that declares `Command[T]` subcommands routes to those branches and cannot also declare positional arguments; schema build rejects the mix with `ErrSchema`. This keeps every positional token attributable to a single command level: argument indexes and the `RawArgs` boundary follow from the schema, not from tokens typed before a subcommand name. Branch commands inherit flags from their ancestors, so shared context such as `-C <path>` or `--context` belongs in a flag.

## Forwarding and raw payloads

Many command-line tools wrap existing utilities or forward arguments to compilers, test runners, or sub-processes. CONGA provides two dedicated containers for trailing tokens:

### DashArgs for delimited forwarding
`conga.DashArgs` captures tokens that appear after the standard POSIX `--` delimiter. The host application parses its own flags up to the delimiter, and places all subsequent tokens into `DashArgs`. Tools like `go test` use this pattern to separate build options from test binary flags (for example, `go test ./... -- -v`).

`--` also ends subcommand routing for the rest of the invocation: no token after the delimiter is matched against a `Command[T]` name, even when the active command declares no trailing collector. Without a `DashArgs` field, post-terminator tokens are positional input, and a branch command (which cannot also declare positional arguments) rejects them; `myctl -- deploy` therefore fails instead of selecting the `deploy` subcommand. Shell completion follows the same boundary: it offers no subcommand candidates after `--`, and no candidates at all when the active command has neither a positional slot nor a trailing collector for the word.

### RawArgs for command passthrough
`conga.RawArgs` captures trailing tokens without requiring a `--` delimiter. It is a leaf-only collector: the command cannot declare subcommands, and the positional arguments before it must be required scalars; use `conga.DashArgs` when the command also branches. Once the parser encounters an argument at the `RawArgs` position, flag parsing stops immediately and all remaining arguments are captured as-is. Tools like `docker run` use this pattern to pass arbitrary commands to container runtimes (for example, `docker run alpine echo hello`).

Both trailing containers expose `Get() []string`, `GetOr()`, `Len()`, and `IsSet()`.

## Declarative constraints

Input validation frequently clutters command handlers with manual range checks, length bounds, and mutual exclusivity tests. CONGA allows these constraints to be declared directly on container fields with struct tags such as `enum`, `range`, `len`, `xor`, `oneof`, and `and`.

The engine evaluates declared constraints after type coercion but before invoking lifecycle hooks (`Validate`, `PreRun`, `Run`). When an input violates a constraint, the engine halts execution with a usage error (exit code 2). This separates input validation from command business logic, ensuring that command bodies receive values that already satisfy domain requirements.

### Flag grouping semantics

Flags often interact with one another: certain options conflict, others require companion flags, and some require callers to make an explicit choice among alternatives. CONGA models these relationships through three grouping tags:

| Tag | Users may supply | Otherwise |
|---|---|---|
| `xor:"group"` | None of the flags, or one | Usage error (exit 2) |
| `oneof:"group"` | Exactly one of the flags | Usage error (exit 2) |
| `and:"group"` | None of the flags, or all of them | Usage error (exit 2) |

The usage synopsis shows a `oneof` group as alternatives, such as `(--input-file=PATH | --input-url=URL)`, so the choice is visible in help. A `default` on a `oneof` member is rejected: it would give the flag a value without counting as supplied, since the rule counts flags whose `Changed()` is true.

Group names are scoped to the command that declares them. A group name used on an ancestor or descendant command remains completely independent, preventing accidental coupling across the command hierarchy. Every `xor` or `and` group needs at least two members; a one-member group is a no-op and fails schema construction. A `oneof` group needs at least two members unless its sole member is a `Counter`, where it means "require at least one occurrence" (use `RequiredFlag` for a single mandatory flag instead). Two members of the same `oneof` or `xor` group may not also share an `and` group: supplying either member forces the other, so the exclusive rule could never hold.

Group membership is provenance-based: `--flag=false`, `--flag=`, `--count=0`, and `--no-flag` all count as supplying a member because `Changed()` is true. Shell completion suppresses sibling members only after a member is present on the command line; a group satisfied by an environment variable or config file still offers siblings, and selecting one then fails at runtime. Help-screen headings use the `section` tag; the `xor`, `and`, and `oneof` tags govern validation only.

## Immutability during execution

Once execution reaches the lifecycle hooks, container state is immutable. Command handlers read resolved values; they cannot re-run the resolution ladder or modify container values in place. Configuration is resolved once at startup; restarting the process applies file changes.
