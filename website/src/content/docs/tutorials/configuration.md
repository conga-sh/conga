---
title: Configuration
description: Build server, a CLI whose values resolve from flags, environment variables, and a TOML config file — and report where each value came from with --why.
---

In this tutorial, we will build `server`, a CLI that prints its listen address. We will resolve host and port from defaults, then environment variables, then a TOML config file, and we will report the source of each value with `--why`.

## Set up the module

```sh
mkdir server && cd server
go mod init example.com/server
go get github.com/conga-sh/conga
```

## Define the command

Create `main.go`:

```go title="main.go"
package main

import (
	"context"
	"fmt"

	"github.com/conga-sh/conga"
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
		Description("A server that reads its address from flags, environment, or a config file").
		Version("1.0.0")

	app.FatalIfError(app.Run())
}
```

We run it with the defaults, then override the port:

```sh
$ go run .
listening on 127.0.0.1:8080
$ go run . --port 9001
listening on 127.0.0.1:9001
```

`safe:"true"` controls display in the `--why` report: values are redacted by default, and tagged fields display theirs.

## Fall back to the environment

Environment binding is opt-in per application. We add `.EnvPrefix("SERVER")` to the builder chain:

```go title="main.go" ins={6}
func main() {
	app := conga.New[Server]().
		Name("server").
		Description("A server that reads its address from flags, environment, or a config file").
		Version("1.0.0").
		EnvPrefix("SERVER")

	app.FatalIfError(app.Run())
}
```

The prefix namespaces every flag: CONGA uppercases the flag name and joins it to the prefix, so `--host` reads `SERVER_HOST` and `--port` reads `SERVER_PORT`. We run the three tiers:

```sh
$ go run .
listening on 127.0.0.1:8080
$ SERVER_PORT=9002 go run .
listening on 127.0.0.1:9002
$ SERVER_PORT=9002 go run . --port 9003
listening on 127.0.0.1:9003
```

The last run shows the order: an explicit flag outranks the environment variable, which outranks the default. Config files occupy the tier between the environment and the default. The help screen now advertises the variables:

```sh
$ go run . --help
A server that reads its address from flags, environment, or a config file

Usage:
  server [flags]

Flags:
      --host="127.0.0.1"  Address to listen on [env: SERVER_HOST]
  -p, --port=8080         Port to listen on [env: SERVER_PORT]
      --help              Show help
      --version           Show version
```

The full four-tier ladder is explained in [About value precedence](../explanation/value-precedence/).

## Load a config file

Config files are opt-in: register a decoder for each format to read, then declare where to look. The TOML decoder lives in its own package, so fetch it first:

```sh
go get github.com/conga-sh/conga/decoder/toml
```

We import the decoder and the config options package, register the format, and declare a search path:

```go title="main.go" ins={6-7}
import (
	"context"
	"fmt"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/config"
	"github.com/conga-sh/conga/decoder/toml"
)
```

```go title="main.go" ins={7-8}
func main() {
	app := conga.New[Server]().
		Name("server").
		Description("A server that reads its address from flags, environment, or a config file").
		Version("1.0.0").
		EnvPrefix("SERVER").
		RegisterDecoder(".toml", toml.RawDecoder).
		ConfigOptions(config.Paths("."))

	app.FatalIfError(app.Run())
}
```

`config.Paths(".")` declares the current directory as the search tier. There, CONGA probes `config.toml` and `server.toml` (the second stem is the app name from `Name("server")`) and uses the first file that exists. Each config key is the flag name, so we create `server.toml`:

```toml title="server.toml"
host = "0.0.0.0"
port = 9004
```

```sh
$ go run .
listening on 0.0.0.0:9004
$ SERVER_PORT=9002 go run .
listening on 0.0.0.0:9002
```

The first run reads both values from the file. In the second run the environment tier still wins for `port`, so `host` keeps the file's `0.0.0.0` while `port` comes from `SERVER_PORT`. See [How to manage config files](../how-to/manage-config-files/) for explicit paths, cascading, and profiles, and [About the config cascade](../explanation/config-cascade/) for the resolution rules.

## See where each value came from

CONGA records provenance for every field. We add `.EnableWhy()` to switch on the `--why` report:

```go title="main.go" ins={6}
func main() {
	app := conga.New[Server]().
		Name("server").
		Description("A server that reads its address from flags, environment, or a config file").
		Version("1.0.0").
		EnableWhy().
		EnvPrefix("SERVER").
		RegisterDecoder(".toml", toml.RawDecoder).
		ConfigOptions(config.Paths("."))

	app.FatalIfError(app.Run())
}
```

```sh
$ go run . --why
Value provenance for "server":

  FLAG     VALUE     SOURCE   ENV           CONFIG PATH
  --host   0.0.0.0   config   SERVER_HOST   host
  --port   9004      config   SERVER_PORT   port

Config decoders: .toml

Config paths checked:
  .   loaded: server.toml
```

Each row is one flag: the winning value, the tier that supplied it, and the environment variable and config key that can supply it. We run the same command through a different tier to change the `SOURCE` column:

```sh
$ go run . --why --port 9003
Value provenance for "server":

  FLAG     VALUE     SOURCE   ENV           CONFIG PATH
  --host   0.0.0.0   config   SERVER_HOST   host
  --port   9003      cli      SERVER_PORT   port

Config decoders: .toml

Config paths checked:
  .   loaded: server.toml
```

`--why` answers the invocation by printing the report and exiting before `Run` executes. The two fields display their values because they carry `safe:"true"`; untagged fields render as `██████`, which keeps secrets out of terminals and CI logs. See [The `--why` report](../reference/builtins/#the---why-report) for the full report reference.

## The complete program

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
		Description("A server that reads its address from flags, environment, or a config file").
		Version("1.0.0").
		EnableWhy().
		EnvPrefix("SERVER").
		RegisterDecoder(".toml", toml.RawDecoder).
		ConfigOptions(config.Paths("."))

	app.FatalIfError(app.Run())
}
```

## Where to go next

- [How to load .env files](../how-to/load-env-files/) — layer `.env` and `.env.local` under the environment tier.
- [How to hot-reload configuration](../how-to/hot-reload-config/) — watch the file and swap values at runtime.
- [About the config cascade](../explanation/config-cascade/) — the built-in XDG tier, cascading, and existence semantics.
- [How to run the examples](../how-to/examples-cookbook/) — example 13 for the full precedence ladder, example 14 for multiple formats and `.env`.
