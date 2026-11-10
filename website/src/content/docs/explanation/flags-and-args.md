---
title: About flags and args
description: Why CONGA declares flags and positional arguments as typed container fields, and how that shapes the grammar it accepts.
---

A CONGA command declares its input as fields on its struct. Each field's container type, such as `OptionalFlag[T]` or `RequiredArg[T]`, makes the field a flag or a positional argument. Before any lifecycle hook runs, CONGA fills every container from the command line, the environment, config files, and `default` tags, so a command body only reads values. [package conga](/reference/conga/#containers) lists the container methods, and the [struct tag reference](/reference/struct-tags/) lists every tag and rule.

## Approaches to input in Go

Go command-line libraries model input in one of two ways.

The standard library's `flag`, `spf13/cobra`, and `urfave/cli` register flags on a flag set at run time. Each flag binds to a separate variable through a pointer, or is looked up by name with calls such as `cmd.Flags().GetString("addr")`. The shape of the command line lives apart from the program's data, and a wrong name or type shows up as an error at run time.

`alecthomas/kong` and `alexflint/go-arg` read the shape from a struct instead. Each field is an ordinary Go value, and its tags say what it is on the command line. Declarations stay short and read like ordinary Go.

CONGA reads the shape from a struct too, but wraps each field in a generic container such as `OptionalFlag[T]`, `RequiredFlag[T]`, or `RequiredArg[T]`.

## What the container costs and buys

Containers give up some declaration brevity. Defining a field's behaviour through a wrapper type can feel unusual at first, when `Addr conga.OptionalFlag[string]` stands where `Addr string` would do. Every read then goes through a getter, `cmd.Addr.Get()` rather than `cmd.Addr`, and in a command that reads many values those calls add up.

CONGA accepts that cost for what the type gives back. The field's type says what the field is for and how it behaves. With bare fields, tags carry that. One tag marks a field as a positional argument, another makes it required, another makes it a subcommand, and a reader has to scan every tag to learn the field's role. In CONGA the type carries it. `RequiredFlag[string]`, `OptionalArg[string]`, and `Command[DeployCommand]` say flag or argument or subcommand, and required or optional, before any tag. The tags that remain describe the value, such as its name, help text, default, or bounds.

### Presence apart from value

A bare field can't tell an omitted flag from an explicit zero. A pointer field can, but the program then spells out the nil check and the fallback at every read. A container provides both as methods. `cmd.Port.IsSet()` tells an explicit `--port 0` from an omitted flag, and `cmd.Port.GetOr(8080)` supplies a fallback in one expression.

### Where each value came from

Because every value passes through a container, the container can also record which source supplied it:

- `IsSet()` reports whether any source, a `default` tag included, gave the field a value.
- `Changed()` reports whether the command line, the environment, or a config file gave it one.
- `Source()` names that source (`SourceCLI`, `SourceEnv`, `SourceConfig`, `SourceDefault`).
- `SourceDetail()` names the exact origin, such as the flag as typed, the variable name, or the config file path.

The `--why` report is built from this record.

## Schema defaults versus fallbacks in code

An optional input can get a fallback in two places.

A `default:"..."` tag is part of the schema. When no other source sets the input, CONGA fills the container from the tag, records `SourceDefault`, and shows the default on the help screen.

`GetOr(fallback)` runs in the command body and applies only when the container is still unset (`SourceNone`). It suits a fallback that depends on the running system, such as a directory found at run time, or on another flag. Required containers have no `GetOr`, since they always hold a value by the time the body runs.

## Flags versus positional arguments

Positional arguments take their order on the command line from the order of their fields on the struct. Flags read from all four sources. Positional arguments read only from the command line, plus an optional argument's `default`.

## Why the positional grammar is strict

CONGA binds positional words left to right, in declaration order, and never backtracks. That makes two grammars ambiguous, so schema build rejects them with `ErrSchema`.

The first is an optional argument declared before a required one. Given one word, the parser couldn't tell whether it fills the optional slot or the required one, so required arguments come first.

The second is a command that declares both positional arguments and `Command[T]` subcommands. With positionals only on leaf commands, every positional word belongs to one command, so the argument indexes and the `RawArgs` boundary follow from the schema rather than from the words typed before a subcommand name. The parser and shell completion count arguments the same way for the same reason. Context a branch needs, such as `-C <path>` or `--context`, goes in a flag, which its descendants inherit.

## Forwarding words to another program

CONGA has two containers for words that a command hands on to another program.

`DashArgs` collects every word after `--`, the convention `cargo run --release -- --port 8080` follows. The `--` is an explicit boundary that the user types, so `DashArgs` is safe on a command with subcommands. CONGA reads no word after it as a subcommand name or as one of the tool's own flags.

`RawArgs` collects the rest of the line from the first word the declared positionals don't take, without a `--`, as in `docker run alpine echo hello`. No delimiter marks where forwarding starts, so the boundary must follow from the schema alone. `RawArgs` therefore sits only on a leaf command, after required scalar arguments. An optional or repeated argument before it, or a subcommand beside it, would make the boundary ambiguous. With required scalars only, the boundary always falls after the same number of positional words, so parsing and completion stay predictable. [Trailing arguments](/reference/struct-tags/#trailing-arguments-dashargs-rawargs) has the full rules.

## Constraints declared on the field

Range checks, length bounds, and conflicts between flags are declared on the fields, with the tags `enum`, `range`, `len`, and the group tags `xor`, `oneof`, and `and`. CONGA checks them after parsing the values and before any lifecycle hook, and a violation is a usage error (exit 2). This keeps input validation out of the command body, which receives only values that satisfy the declared rules.

A group counts a member as supplied when its `Changed()` is true, whatever its value. `--json --text=false` names both output formats, and an `xor` of the two reports the conflict rather than reading `--text=false` as absent. This is also why a `oneof` member takes no `default`. A default would give the flag a value without making its `Changed()` true, so the flag would hold a value and still not count toward the group.

A group name belongs to the command that declares it. A parent's `xor:"output"` and a child's `xor:"output"` are two unrelated groups, so a shared name can't couple commands by accident. [Flags](/reference/struct-tags/#flags-optionalflagt-requiredflagt-counter) lists the group rules.

## Values fixed for the run

By the time the lifecycle hooks run, every container holds its final value. A handler can read a container but can't change it or ask CONGA to resolve it again, and config files are read once per run. A changed config file takes effect on the next run.
