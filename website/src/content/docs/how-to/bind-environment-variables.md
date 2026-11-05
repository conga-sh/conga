---
title: How to bind environment variables
description: Map environment variables to command flags using prefixes, subcommand tags, and custom overrides.
---

CONGA binds environment variables to command flags during flag resolution. This guide shows how to set application and subcommand prefixes, override variable names per field, and disable environment binding for sensitive flags.

## Set an application prefix

Call `EnvPrefix` on the application builder to set a prefix for flag environment variables:

```go title="main.go"
package main

import (
	"context"

	"github.com/conga-sh/conga"
)

type Server struct {
	Host      conga.OptionalFlag[string] `default:"127.0.0.1" help:"Listen address"`
	Port      conga.OptionalFlag[int]    `default:"8080" help:"Listen port" short:"p"`
	MaxMemory conga.OptionalFlag[string] `default:"512MB" help:"Maximum allocated memory"`
}

func (s *Server) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

func main() {
	app := conga.New[Server]().
		EnvPrefix("SERVER")

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
}
```

Flag names are converted to uppercase, with hyphens replaced by underscores (`<PREFIX>_<FLAG>`):

- `--host` binds to `SERVER_HOST`.
- `--port` binds to `SERVER_PORT`.
- `--max-memory` binds to `SERVER_MAX_MEMORY`.

Flags passed on the command line override environment variables:

```sh
SERVER_PORT=9002 server --port 9003
```

In the example above, `--port` resolves to `9003`. If `--port` is omitted on the command line, it resolves to `9002` from the environment.

Positional arguments do not read environment variables; they resolve only from command-line arguments.

## Namespace subcommands with envprefix

To scope a subcommand's flags under its own prefix, add an `envprefix` tag to the `Command[T]` field:

```go title="main.go"
type CLI struct {
	Worker conga.Command[WorkerCmd] `envprefix:"APP_WORKER" summary:"Background worker"`
}

type WorkerCmd struct {
	Concurrency conga.OptionalFlag[int] `default:"4" help:"Worker concurrency"`
}
```

An `envprefix` tag replaces any inherited prefix; it does not append. To keep the subcommand inside the application namespace, include the parent prefix in the tag (`APP_WORKER`).

- `envprefix:"APP_WORKER"` binds `--concurrency` to `APP_WORKER_CONCURRENCY`.
- `envprefix:"WORKER"` replaces `APP_` entirely, binding `--concurrency` to `WORKER_CONCURRENCY`.
- Omitting the tag inherits the parent prefix, binding `--concurrency` to `APP_CONCURRENCY`.

## Override variable names per field

Use the `env` struct tag to bind a flag to an explicit variable name. Explicit `env` tags ignore all prefixes:

```go title="main.go"
type Database struct {
	URL conga.RequiredFlag[string] `env:"DATABASE_URL" help:"Connection string"`
}
```

CONGA reads `DATABASE_URL` directly, even when an application or subcommand prefix is configured.

## Disable environment binding for sensitive flags

To prevent a flag from reading the environment when an application prefix is configured, declare `env:"-"`:

```go title="main.go"
type CLI struct {
	Token conga.OptionalFlag[string] `env:"-" help:"One-time token accepted only via CLI"`
}
```

Declaring `env:"-"` ensures the flag resolves exclusively from command-line arguments or its default value.

## Treat empty environment variables as unset

CONGA treats empty environment variables (`FOO=""` or `export FOO=`) as unset. An empty variable will not override configuration files or default values.

To pass an empty string explicitly, provide `--flag ""` on the command line:

```sh
# APP_HOST is empty: default tag or config file supplies the host
APP_HOST= server

# Explicit empty string on CLI: overrides default and config with ""
server --host ""
```

## Where to go next

- [How to load .env files](/how-to/load-env-files/) — feed `.env` and `.env.local` files into the environment tier.
- [How to inspect value provenance](/how-to/inspect-value-provenance/) — diagnose which tier provided each value with `--why`.
- [About value precedence](/explanation/value-precedence/) — the four-tier resolution ladder and provenance rules.
