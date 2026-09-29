---
title: How to inspect value provenance
description: Diagnose flag resolution sources across CLI, environment, config files, and defaults with --why.
---

This guide demonstrates how to enable the built-in `--why` flag to inspect where each flag value resolved from across the four-tier resolution ladder.

## Enable the provenance report

Call `EnableWhy()` on the application builder to install the `--why` flag:

```go title="main.go"
package main

import (
	"context"
	"fmt"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/config"
	"github.com/conga-sh/conga/decoder/toml"
)

type Server struct {
	Host conga.OptionalFlag[string] `default:"127.0.0.1" help:"Address to listen on" safe:"true"`
	Port conga.OptionalFlag[int]    `default:"8080" help:"Port to listen on" safe:"true" short:"p"`
}

func (s *Server) Run(ctx context.Context, session *conga.Session) error {
	fmt.Fprintf(session.Stdout(), "listening on %s:%d\n", s.Host.Get(), s.Port.Get())

	return nil
}

func main() {
	app := conga.New[Server]().
		Name("server").
		EnableWhy().
		EnvPrefix("SERVER").
		RegisterDecoder(".toml", toml.RawDecoder).
		ConfigOptions(config.Paths("."))

	app.FatalIfError(app.Run())
}
```

The `safe:"true"` tag opts a field into value rendering. Untagged fields redact their resolved values as `██████`, protecting credentials and tokens from display in terminal logs.

## Run the provenance report

Pass `--why` on the command line alongside other arguments:

```sh
$ go run . --why
Value provenance for "server":

  FLAG     VALUE       SOURCE   ENV           CONFIG PATH
  --host   127.0.0.1   default  SERVER_HOST   host
  --port   8080        default  SERVER_PORT   port

Config decoders: .toml

Config paths checked:
  .   (none found)
```

The report displays every declared flag, its winning value, the winning resolution tier (`cli`, `env`, `config`, or `default`), the associated environment variable, and the config file key.

Override a value through the command line or environment to observe the source change:

```sh
$ SERVER_PORT=9002 go run . --why --port 9003
Value provenance for "server":

  FLAG     VALUE       SOURCE   ENV           CONFIG PATH
  --host   127.0.0.1   default  SERVER_HOST   host
  --port   9003        cli      SERVER_PORT   port

Config decoders: .toml

Config paths checked:
  .   (none found)
```

The report exits with code 0 before any lifecycle hooks (`PreRun`, `Run`, `PostRun`) execute.

## Where to go next

- [About value precedence](/explanation/value-precedence/) — the deterministic resolution ladder and provenance guarantees.
- [How to bind environment variables](/how-to/bind-environment-variables/) — map environment variables to flags.
- [How to manage config files](/how-to/manage-config-files/) — register file decoders and search paths.
- [The `--why` report](/reference/builtins/#the---why-report) — built-in reference documentation for `--why`.
