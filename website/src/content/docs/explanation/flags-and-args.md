---
title: About flags and args
description: Architectural design of CONGA input modeling, container types, and argument binding.
---

Command-line interfaces accept two primary forms of input: named flags and positional arguments. CONGA models both as typed container fields declared directly on command structs.

This page explains how CONGA structures command-line inputs, the engineering trade-offs of using container types instead of plain struct fields, and how flags differ from positional arguments in evaluation and scoping. For the complete container API, see [package conga](/reference/conga/#containers); for struct tag syntax, see the [struct tag reference](/reference/struct-tags/).

## Approaches to input in Go

Command-line libraries in the Go ecosystem generally approach input modeling in one of two ways:

1. **Imperative flag sets**: Standard library `flag`, `spf13/cobra`, and `urfave/cli` register flags imperatively onto flag sets. Flags bind to standalone variables through pointers or are queried by string keys such as `cmd.Flags().GetString("addr")`. This model is flexible, but separates the command line specification from data structures and defers type and name checks to runtime.
2. **Struct-field reflection**: Libraries like `alecthomas/kong` and `alexflint/go-arg` derive flag schemas directly from Go struct definitions using struct tags. Flags map to native Go types (`string`, `int`, `bool`) or pointers (`*string`). This approach keeps configuration declarations concise and feels like ordinary Go code.

CONGA follows the struct-based approach, but uses dedicated generic containers—such as `OptionalFlag[T]`, `RequiredFlag[T]`, and `RequiredArg[T]`—rather than bare primitive fields.

## The container trade-off

Using container types introduces an explicit trade-off between declaration brevity and runtime capability.

Wrapping fields in `OptionalFlag[string]` requires more typing than declaring a bare `string`. Accessing the value also requires an explicit method call: `cmd.Addr.Get()` rather than `cmd.Addr`. For straightforward utilities that only need basic flag parsing, this extra layer can feel verbose or clunky compared to plain struct fields.

CONGA accepts this syntactic overhead because containers solve several structural challenges in multi-tier CLI applications:

### Distinguishing unset values without pointers
In Go, every primitive type has a zero value. A bare `string` defaults to `""` and an integer defaults to `0`. If a command struct uses bare primitives, the application cannot distinguish whether the user explicitly passed `--port 0` or `--name ""`, or whether the flag was omitted entirely.

Libraries using plain structs often turn to pointer fields (`*int`, `*string`) to distinguish presence. Pointers solve the presence question, but they require defensive nil checks throughout command handlers. If a handler reads `*cmd.Port` without verifying it is non-nil, the process panics.

Containers solve this without pointers:
- `cmd.Port.Get()` is always safe to call and returns the resolved value or fallback.
- `cmd.Port.IsSet()` indicates whether any tier provided a value.
- The command handler never needs nil checks or defensive guard clauses.

### Tracking resolution provenance
CONGA is built around a multi-tier resolution ladder where values can originate from CLI flags, environment variables, configuration files, or hardcoded defaults. Bare fields and pointers discard that origin once populated.

Containers retain their history throughout execution:
- `cmd.Addr.Source()` identifies which tier supplied the value (`SourceCLI`, `SourceEnv`, `SourceConfig`, `SourceDefault`).
- `cmd.Addr.SourceDetail()` provides the concrete origin, such as the environment variable name or configuration file path.
- `cmd.Addr.Changed()` indicates whether an external source altered the value from its default.

This metadata powers diagnostic features like the `--why` provenance report and allows commands to adjust behavior based on how a setting was configured.

## Flags versus positional arguments

Flags and positional arguments serve different roles in a CLI, and CONGA enforces their differences through distinct container types:

### Addressing and order
Flags are addressed by name (`--output`, `-o`). They are order-independent and may appear anywhere on the command line relative to other flags.

Positional arguments (`RequiredArg[T]`, `OptionalArg[T]`) have no names on the command line. Their identity depends entirely on their position. In Go source code, their sequence matches the order of the fields declared on the command struct.

### Resolution scope
Flags participate in the complete four-tier resolution ladder. A flag value can come from the command line, an environment variable, an active configuration file, or a default tag.

Positional arguments represent immediate execution targets, such as file paths, hostnames, or artifact identifiers. They resolve strictly from command-line arguments. Positional arguments never read from environment variables or configuration files, preventing ambient system settings from altering explicit command targets.

## Deterministic positional evaluation

Positional parsing in CONGA is greedy and deterministic. The engine iterates through command-line tokens from left to right, binding them to argument containers in declaration order.

Because parsing does not backtrack, CONGA enforces a strict structural invariant: **all required arguments must appear before any optional argument**. A struct declaring an optional argument before a required one fails during startup with `ErrSchema`.

This rule eliminates grammar ambiguity. If an optional argument could precede a required argument, an invocation supplying fewer tokens than parameters would be ambiguous: the first token could satisfy either the optional parameter or the mandatory one. Requiring mandatory arguments first ensures that every token binding is predictable.

## Forwarding and raw payloads

Many command-line tools wrap existing utilities or forward arguments to compilers, test runners, or sub-processes. CONGA provides two dedicated containers for trailing tokens:

### DashArgs for delimited forwarding
`conga.DashArgs` captures tokens that appear after the standard POSIX `--` delimiter. The host application parses its own flags up to the delimiter, and places all subsequent tokens into `DashArgs`. Tools like `go test` use this pattern to separate build options from test binary flags (for example, `go test ./... -- -v`).

### RawArgs for command passthrough
`conga.RawArgs` captures trailing tokens without requiring a `--` delimiter. Once the parser encounters an argument at the `RawArgs` position, flag parsing stops immediately and all remaining arguments are captured as-is. Container tools like `docker run` use this pattern to pass arbitrary commands to container runtimes (for example, `docker run alpine echo hello`).

## Declarative constraints

Input validation frequently clutters command handlers with manual range checks, length bounds, and mutual exclusivity tests. CONGA allows these constraints to be declared directly on container fields with struct tags such as `enum`, `range`, `len`, `xor`, and `and`.

The engine evaluates declared constraints after type coercion but before invoking lifecycle hooks (`Validate`, `PreRun`, `Run`). When an input violates a constraint, the engine halts execution with a usage error (exit code 2). This separates input validation from command business logic, ensuring that command bodies receive values that already satisfy domain requirements.
