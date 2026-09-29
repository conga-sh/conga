---
title: About reading values
description: Containers hold the resolved winner of the resolution ladder, and Get returns it without a presence check.
---

A CONGA command struct describes input with typed containers, and the engine resolves every container before any lifecycle hook runs. This page explains the reader surface those containers expose and the guarantees behind `Get()`.

## Values arrive resolved

By the time `Run` is called, the engine has parsed the command line, consulted the environment when `EnvPrefix` or `env` tags are declared, merged any config files, and applied `default` tags. Each container receives one resolved value — the winner of the [resolution ladder](/explanation/value-precedence/) — together with the tier that supplied it. The command body never participates in resolution.

## Get is unconditional

`Get()` returns the resolved value as `T`:

- A required flag or positional argument that received no value fails validation before `Run`; a command body never observes a missing required value.
- An unset optional container holds the zero value of `T` (`""`, `0`, `false`, or `nil`), and `Get()` returns it like any other value.
- A `default` tag fills the container during resolution, so `Get()` returns the default when no higher tier provided a value.

A plain pointer field, or a value field plus a separate "was it set" boolean, carries the presence question into every read. A container carries it once: `Get()` is always safe, and the command body contains no nil checks or zero-value guard clauses.

## Distinguishing unset from zero

`Get()` alone cannot tell an empty string supplied on the command line from an empty string that no tier supplied. When that distinction matters, the container answers it directly:

| Method | Question |
|--------|----------|
| `IsSet()` | Did any tier, including `default`, supply a value? |
| `Changed()` | Did the command line, the environment, or a config file supply the value, rather than a default? |
| `Source()` | Which tier supplied the winner: `SourceCLI`, `SourceEnv`, `SourceConfig`, `SourceDefault`, or `SourceNone`? |
| `SourceDetail()` | The concrete origin: the environment variable name, the config file path, or the flag spelling. |

`GetOr(fallback)` builds on the same state for optional containers: it returns the resolved value when one was set, and the fallback when the container is unset. Required containers expose no `GetOr`, because resolution cannot complete without them, so a fallback has no meaning.

```go
func (s *Serve) Run(ctx context.Context, session *conga.Session) error {
	port := s.Port.GetOr(8080)
	if s.Verbose.Changed() {
		fmt.Fprintf(session.Stderr(), "verbosity came from %s\n", s.Verbose.SourceDetail())
	}

	return nil
}
```

## The reader surface per container

Every flag and positional container exposes `Get()`, `IsSet()`, `Changed()`, `Source()`, `SourceDetail()`, and `String()`; optional containers add `GetOr()`.

| Container | Value | Notes |
|-----------|-------|-------|
| `OptionalFlag[T]`, `RequiredFlag[T]` | `T` | Required flags reject a missing value with a usage error |
| `OptionalArg[T]`, `RequiredArg[T]` | `T` | Positional; resolved from the command line only |
| `Counter` | `int` | `Get()` returns the number of occurrences |
| `DashArgs`, `RawArgs` | `[]string` | `Get()`, `GetOr()`, `Len()`, `IsSet()`, `String()` |
| `Command[T]` | `*T` | `Get()` returns the selected subcommand struct; `IsSelected()` reports whether the engine routed to it |

`String()` implements `fmt.Stringer`, so a container can be printed or logged directly. The tags that feed each container are catalogued in [About flags and args](/explanation/flags-and-args/) and the [struct tag reference](/reference/struct-tags/); the precedence order itself is covered in [About value precedence](/explanation/value-precedence/).
