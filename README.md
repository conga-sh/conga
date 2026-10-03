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
