---
title: Installation
description: Add CONGA to your Go module and pick optional format decoders.
---

CONGA is a single Go module with opt-in satellite packages: batteries included, opt-in subsystems, linker friendly. It targets **Go 1.27+** and has a small dependency tree.

## Core framework

```sh
go get github.com/conga-sh/conga
```

This pulls in the root `conga` package: the `App[T]` builder, flag and argument containers, subcommand routing, and help rendering. The configuration cascade, `.env` loading, and the six-shell completion engine are opt-in at link time (`app.RegisterDecoder`, `app.Dotenv()`, `app.EnableCompletion()`), so apps that skip them do not link their drivers.

## Optional format decoders

Configuration file decoders live in separate packages, so only the formats you register are linked into your binary:

```sh
go get github.com/conga-sh/conga/decoder/json
go get github.com/conga-sh/conga/decoder/yaml
go get github.com/conga-sh/conga/decoder/toml
go get github.com/conga-sh/conga/decoder/hcl
```

Register whichever you need with `app.RegisterDecoder`. See [package decoder](../reference/decoders/) and [How to manage config files](../how-to/manage-config-files/).

## Optional completion extras

- `github.com/conga-sh/conga/predict` — the predictor SDK for custom completion logic.
- `github.com/conga-sh/conga/predict/network`, `.../predict/system`, `.../predict/timezone` — domain predictors.
- `github.com/conga-sh/conga/plugin/network`, `.../plugin/system`, `.../plugin/timezone` — builder plugins that mount standard library types.
- `github.com/conga-sh/conga/watch` — live config file watching and hot reload.

## Verify

```go
package main

import (
	"fmt"

	"github.com/conga-sh/conga"
)

func main() {
	app := conga.New[struct {
		Ready conga.OptionalFlag[bool] `help:"Just checking" short:"r"`
	}]().Name("check")

	fmt.Println("module resolves fine")
	_ = app
}
```

```sh
go build ./...
```

If the build succeeds, you are ready for the [Quickstart](../quickstart/).
