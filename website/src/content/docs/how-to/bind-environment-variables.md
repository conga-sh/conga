---
title: How to bind environment variables
description: Map environment variables to command flags using prefixes, subcommand tags, and custom overrides.
---

CONGA automatically binds environment variables to command flags during flag resolution. This guide explains how to namespace environment variables with an application prefix, configure subcommand prefixes, override variable names per field, and disable environment binding for sensitive flags.

## Set an application prefix

Call `EnvPrefix` on the application builder to namespace all command flags under an environment variable prefix:

```go title="main.go"
type Server struct {
	Host      conga.OptionalFlag[string] `default:"127.0.0.1" help:"Listen address"`
	Port      conga.OptionalFlag[int]    `default:"8080" help:"Listen port" short:"p"`
	MaxMemory conga.OptionalFlag[string] `default:"512MB" help:"Maximum allocated memory"`
}

func main() {
	app := conga.New[Server]().
		Name("server").
		EnvPrefix("SERVER")

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
}
```

CONGA generates the expected environment variable name by converting the flag name to uppercase, replacing hyphens with underscores, and prepending the application prefix with an underscore (`<PREFIX>_<FLAG>`):

- `--host` binds to `SERVER_HOST`.
- `--port` binds to `SERVER_PORT`.
- `--max-memory` binds to `SERVER_MAX_MEMORY`.

Command-line flags take precedence over environment variables in the resolution ladder:

```sh
SERVER_PORT=9002 server --port 9003
```

In the example above, the flag resolves to `9003` from `SourceCLI`. If the `--port` flag is omitted on the command line, the flag resolves to `9002` from `SourceEnv`.

Environment variable binding applies to flags; positional arguments resolve exclusively from command-line tokens.

## Namespace subcommands with envprefix

To namespace flags on subcommands, add an `envprefix` struct tag to the command field or struct:

```go title="main.go"
type CLI struct {
	Worker conga.Command[WorkerCmd] `envprefix:"WORKER" summary:"Background worker"`
}

type WorkerCmd struct {
	Concurrency conga.OptionalFlag[int] `default:"4" help:"Worker concurrency"`
}
```

If the root application configures `EnvPrefix("APP")` and a subcommand declares `envprefix:"WORKER"`, CONGA combines the prefixes:

- `--concurrency` binds to `APP_WORKER_CONCURRENCY`.

## Override variable names per field

Use the `env` struct tag to bind a flag to a custom environment variable name. Explicit `env` tags ignore the application prefix:

```go title="main.go"
type Database struct {
	URL conga.RequiredFlag[string] `env:"DATABASE_URL" help:"Connection string"`
}
```

When `env:"DATABASE_URL"` is declared, CONGA reads `DATABASE_URL` directly, regardless of whether `app.EnvPrefix` is configured.

## Disable environment binding for sensitive flags

To prevent a flag from reading the environment when an application prefix is configured, declare `env:"-"`:

```go title="main.go"
type CLI struct {
	Token conga.OptionalFlag[string] `env:"-" help:"One-time token accepted only via CLI"`
}
```

Declaring `env:"-"` ensures the flag resolves exclusively from command-line arguments or its default value.

## Where to go next

- [How to load .env files](/how-to/load-env-files/) — feed `.env` and `.env.local` files into the environment tier.
- [How to inspect value provenance](/how-to/inspect-value-provenance/) — diagnose which tier provided each value with `--why`.
- [About value precedence](/explanation/value-precedence/) — the four-tier resolution ladder and provenance rules.
