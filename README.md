# CONGA

[![check](https://github.com/conga-sh/conga/actions/workflows/check.yaml/badge.svg?branch=main&event=push)](https://github.com/conga-sh/conga/actions/workflows/check.yaml)
[![Go Reference](https://pkg.go.dev/badge/github.com/conga-sh/conga.svg)](https://pkg.go.dev/github.com/conga-sh/conga)
[![Go 1.27](https://img.shields.io/badge/Go-1.27-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![license: MIT](https://img.shields.io/badge/license-MIT-blue)](LICENSE)
[![docs](https://img.shields.io/badge/docs-docs.conga.sh-blue)](https://docs.conga.sh)

CONGA is a batteries-included CLI framework for Go. Model the command line
with Go structs, and CONGA derives the flags, environment bindings,
configuration, validation, and autocompletion.

## Features

- Declarative command schemas: flags, positional arguments, and subcommands
  defined as Go struct fields.
- Value resolution across command-line flags, environment variables,
  configuration files, and defaults, with origin tracking.
- Multi-format configuration decoding for JSON, YAML, TOML, and HCL, with
  directory search paths and cascading file merges.
- Built-in autocompletion for Bash, Zsh, Fish, PowerShell, Elvish, and NuShell.
- Rich validation constraints (`range`, `len`, `enum`, `xor`, `and`) evaluated
  at startup and parse time.
- Opt-in modular architecture keeping unreferenced subsystems pruned at link time.

## Installation

Requires Go 1.27+.

```sh
go get github.com/conga-sh/conga
```

## Quickstart

```go
package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/conga-sh/conga"
)

type CLI struct {
	Name conga.OptionalFlag[string] `default:"World" help:"Name of the person to greet" short:"n"`
	Loud conga.OptionalFlag[bool]   `help:"Print greeting in uppercase"           short:"l"`
}

func (c *CLI) Run(ctx context.Context, session *conga.Session) error {
	greeting := fmt.Sprintf("Hello, %s!", c.Name.Get())
	if c.Loud.Get() {
		greeting = strings.ToUpper(greeting)
	}

	fmt.Fprintln(session.Stdout(), greeting)

	return nil
}

func main() {
	app := conga.New[CLI]().
		Name("hello").
		Description("Minimal single-command hello world CLI").
		Version("1.0.0")

	app.FatalIfError(app.Run())
}
```

```sh
go run . --name conga --loud
```

```
HELLO, CONGA!
```

Fields become flags, and `Run` reads their resolved values with `Get()`.

## Value Precedence

Flags resolve through a deterministic 4-tier precedence ladder:

1. **Command-line flags** (`--port 9000`, `-p 9000`)
2. **Environment variables** (`PORT=9000` or `APP_PORT=9000` with `App.EnvPrefix`)
3. **Configuration files** (JSON, YAML, TOML, HCL)
4. **Struct field defaults** (`default:"8080"`)

Positional arguments bind only to command-line tokens, falling back to `default`.

## Struct Tags

Common metadata and validation tags:

| Tag | Example | Description |
| --- | --- | --- |
| `short` | `short:"p"` | Single-character flag alias (`-p`) |
| `default` | `default:"8080"` | Fallback value if no other source supplies one |
| `env` | `env:"PORT"` | Environment variable binding |
| `help` | `help:"Port to bind"` | Usage description shown in `--help` |
| `range` | `range:"1..65535"` | Inclusive bounds for numbers, durations, and byte sizes |
| `enum` | `enum:"json,yaml,text"` | Restricts accepted values to a fixed set |
| `xor` | `xor:"auth"` | Mutual exclusivity group (only one flag in group allowed) |
| `config` | `config:"listen_port"` | Configuration file key name (or `"-"` to ignore) |

For advanced constraints (`len`, `items`, `and`, `conga` directives) and per-container applicability, see the [Struct Tag Reference](https://docs.conga.sh/reference/struct-tags/).

## Subcommands

Define subcommands by nesting struct types inside `conga.Command[T]`:

```go
type RootCLI struct {
	Deploy conga.Command[DeployCmd] `summary:"Deploy artifacts to a target cluster"`
}

type DeployCmd struct {
	Target conga.RequiredFlag[string] `short:"t" help:"Deployment target"`
}

func (d *DeployCmd) Run(ctx context.Context, session *conga.Session) error {
	fmt.Fprintf(session.Stdout(), "Deploying to %s\n", d.Target.Get())
	return nil
}
```

## Built-ins

Every CONGA application includes built-in operational flags:

- `--help` (`-h`): Prints command syntax and flag descriptions.
- `--version` (`-v`): Prints version and build information.
- `--why`: Debugging flag that prints a provenance report explaining where every flag value came from (CLI, env, config, or default) and which competing sources were overridden. Enable with `app.EnableWhy()`.

For subcommand-based CLIs, CONGA provides `HelpCommand`, `VersionCommand`, and `CompletionCommand`. See the [Built-ins Reference](https://docs.conga.sh/reference/builtins/).

## Modular Architecture

CONGA isolates heavier dependencies—such as configuration decoders (`decoder/{json,yaml,toml,hcl}`) and shell completion drivers—into opt-in packages so unused features are pruned at link time. See [Binary Size & Architecture](https://docs.conga.sh/explanation/binary-size/).

## Documentation

- [Documentation & Guides](https://docs.conga.sh)
- [Struct Tag Reference](https://docs.conga.sh/reference/struct-tags/)
- [API Reference (pkg.go.dev)](https://pkg.go.dev/github.com/conga-sh/conga)
