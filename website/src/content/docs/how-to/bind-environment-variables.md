---
title: How to bind environment variables
description: Map environment variables to command flags using prefixes and struct tags.
---

This guide demonstrates how to configure automatic environment variable binding for command flags, override variable names per field, and exclude individual flags from environment resolution.

## Set an application prefix

Call `EnvPrefix` on the application builder to namespace all command flags under an environment variable prefix:

```go title="main.go"
package main

import (
	"context"
	"fmt"

	"github.com/conga-sh/conga"
)

type Server struct {
	Host conga.OptionalFlag[string] `default:"127.0.0.1" help:"Listen address"`
	Port conga.OptionalFlag[int]    `default:"8080" help:"Listen port" short:"p"`
}

func (s *Server) Run(ctx context.Context, session *conga.Session) error {
	fmt.Fprintf(session.Stdout(), "listening on %s:%d\n", s.Host.Get(), s.Port.Get())

	return nil
}

func main() {
	app := conga.New[Server]().
		Name("server").
		EnvPrefix("SERVER")

	app.FatalIfError(app.Run())
}
```

CONGA joins the prefix to the uppercased flag name using an underscore:
- `--host` binds to `SERVER_HOST`.
- `--port` binds to `SERVER_PORT`.

Run the command with environment variables:

```sh
$ SERVER_PORT=9002 go run .
listening on 127.0.0.1:9002
```

Command-line flags take precedence over environment variables:

```sh
$ SERVER_PORT=9002 go run . --port 9003
listening on 127.0.0.1:9003
```

## Override variable names per field

Use the `env` tag to bind a flag to a custom environment variable name that ignores the application prefix:

```go title="main.go"
type Database struct {
	URL conga.RequiredFlag[string] `env:"DATABASE_URL" help:"Connection string"`
}
```

When an explicit `env` tag is present, CONGA reads that exact variable name without prefixing.

## Disable environment binding for specific flags

To prevent a flag from reading the environment when an application prefix is configured, declare `env:"-"`:

```go title="main.go"
type CLI struct {
	Token conga.OptionalFlag[string] `env:"-" help:"One-time token accepted only via CLI"`
}
```

The flag resolves exclusively from the command line or its declared default.

## Where to go next

- [How to load .env files](/how-to/load-env-files/) — feed `.env` and `.env.local` files into the environment tier.
- [How to inspect value provenance](/how-to/inspect-value-provenance/) — diagnose which tier provided each value with `--why`.
- [About value precedence](/explanation/value-precedence/) — the four-tier resolution ladder and provenance rules.
