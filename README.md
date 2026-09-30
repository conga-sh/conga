# CONGA

[![check](https://github.com/conga-sh/conga/actions/workflows/check.yaml/badge.svg?branch=main&event=push)](https://github.com/conga-sh/conga/actions/workflows/check.yaml)
[![Go Reference](https://pkg.go.dev/badge/github.com/conga-sh/conga.svg)](https://pkg.go.dev/github.com/conga-sh/conga)
[![Go 1.27](https://img.shields.io/badge/Go-1.27-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![license: MIT](https://img.shields.io/badge/license-MIT-blue)](LICENSE)
[![docs](https://img.shields.io/website?url=https%3A%2F%2Fdocs.conga.sh)](https://docs.conga.sh)

CONGA is a batteries-included CLI framework for Go with linker-friendly
opt-in subsystems. Model the command line with Go structs, and CONGA derives
the flag schema, 12-factor configuration cascade, validation, help screens,
and shell completion from them.

## What you get

- **Declarative schema.** Flags (`OptionalFlag[T]`, `RequiredFlag[T]`, `Counter`),
  positional arguments, subcommands, and constraints (`xor`, `and`, `enum`,
  `range`, `len`, `items`), all derived from struct fields and tags.
  See the [Struct Tag Reference](https://docs.conga.sh/reference/struct-tags/).
- **12-factor configuration with provenance.** Every value resolves through a
  deterministic precedence ladder — CLI > environment > config file > default —
  and `Source()` / `SourceDetail()` report where it came from. See
  [About the config cascade](https://docs.conga.sh/explanation/config-cascade/).
- **Lifecycle with guaranteed teardown.** `Validate` → `PreRun` → `Run` →
  `PostRun` → `Finally`, with hooks receiving `ctx` and `session`. See
  [About the lifecycle](https://docs.conga.sh/explanation/lifecycle/).
- **6-shell autocompletion.** Bash, zsh, fish, powershell, elvish, and nushell,
  with self-completing domain types and a decoupled predictor SDK. See
  [How to enable shell completion](https://docs.conga.sh/how-to/enable-shell-completion/).
- **Extensibility.** Register third-party types, mount plugins, or add decoders
  for new configuration formats. See
  [How to add custom flag types](https://docs.conga.sh/how-to/add-custom-flag-types/).
- **Static schema linting.** `conga-lint` enforces leaf `Runner` contracts,
  struct tags, and ancestor flag shadowing before the binary runs. See
  [How to lint CONGA schemas](https://docs.conga.sh/how-to/lint-schemas/).
- **Opt-in subsystems, linker friendly.** Completion, configuration, and
  `.env` loading link only when enabled; omitting one drops its code and
  dependencies from the binary.

## Installation

Requires Go 1.27+. Add CONGA to your module; optional decoders, predictors,
and plugins live in satellite packages:

```sh
go get github.com/conga-sh/conga
```

See the [Quickstart](https://docs.conga.sh/tutorials/quickstart/) to build
your first CLI.

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

Fields become flags, and `Run` reads their resolved values with `Get()`. Walk
through the full program in the
[Quickstart tutorial](https://docs.conga.sh/tutorials/quickstart/),
or copy from the [Examples Cookbook](https://docs.conga.sh/how-to/examples-cookbook/).

## Documentation

- [Website](https://docs.conga.sh/) — tutorials, how-to guides,
  reference, and explanations.
- [API reference](https://pkg.go.dev/github.com/conga-sh/conga) on pkg.go.dev.
- [Struct Tag Reference](https://docs.conga.sh/reference/struct-tags/) —
  every supported struct tag and directive.
- [Built-ins & Deprecation](https://docs.conga.sh/reference/builtins/) —
  help, `--version`/`--json`, `--why`, completion styles, and deprecation notices.
- [Examples Cookbook](https://docs.conga.sh/how-to/examples-cookbook/) — 28
  runnable programs, mirrored in [`_examples/`](_examples/README.md).
- Contributor docs: [testing guide](docs/TESTING.md) and
  [completion wire protocol](docs/completion-wire-protocol.md).
