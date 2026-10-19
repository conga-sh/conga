---
title: How to inspect value provenance
description: Diagnose flag resolution sources across CLI, environment, config files, and defaults with --why.
---

When flags resolve across multiple tiers (CLI flags, environment variables, config files, and defaults), determining which source provided a specific value can be difficult. Enable CONGA's built-in `--why` flag to output a diagnostic report detailing winning values, sources, and evaluated paths.

## Enable the provenance report

Call `EnableWhy()` on the application builder to install the `--why` flag:

```go title="main.go"
package main

import (
	"context"

	"github.com/conga-sh/conga"
)

type Server struct {
	Host  conga.OptionalFlag[string] `default:"127.0.0.1" help:"Listen address" safe:"true"`
	Port  conga.OptionalFlag[int]    `default:"8080" help:"Listen port" safe:"true" short:"p"`
	Token conga.OptionalFlag[string] `help:"API secret token"`
}

func (s *Server) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

func main() {
	app := conga.New[Server]().
		Name("server").
		EnableWhy().
		EnvPrefix("SERVER")

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
}
```

Passing `--why` intercepts execution during the resolution phase. CONGA binds every flag and configuration source with the regular resolution pipeline, prints the report to stdout, and exits with code 0 before any lifecycle hooks (`PreRun`, `Run`, `PostRun`) execute. Invalid configuration or flag values fail with their normal errors instead of rendering partial diagnostic rows.

To enable the machine-readable `--why=json` document as well, pass `conga.WhyJSON()`: `app.EnableWhy(conga.WhyJSON())`. The same report is available programmatically from `session.Why()` and `session.WhyJSON()` inside lifecycle hooks.

## Protect sensitive data with value redaction

The report redacts values by default (see [The `--why` report](/reference/builtins/#the---why-report)). To display a field in plaintext, declare `safe:"true"`:

```go title="main.go"
// Renders plaintext in --why report
Port conga.OptionalFlag[int] `default:"8080" safe:"true"`

// Redacted as ██████ in --why report
Token conga.OptionalFlag[string] `help:"API secret token"`
```

The `safe` tag applies exclusively to flag containers; positional arguments and config-file paths display without redaction.

## Interpret the provenance output

Invoke the binary with `--why` alongside any other flags or environment variables:

```sh
SERVER_PORT=9002 SERVER_TOKEN=hunter2 server --why --port 9003
```

```text no-copy
Value provenance for "server":

  FLAG      VALUE       SOURCE    ENV            CONFIG PATH
  --host    127.0.0.1   default   SERVER_HOST    host
  --port    9003        cli       SERVER_PORT    port
  --token   ██████      env       SERVER_TOKEN   token

values are redacted — tag fields safe:"true" to display
```

The report contains these diagnostic sections (the feature table appears only when the root command declares `conga.Features[T]`):

- **Flag table**:
  - `FLAG`: The declared flag name.
  - `VALUE`: The winning value, or `██████` if untagged. Entry-bearing map rows display the key count `(N keys)`. Multi-file `ConfigFile` rows display `(N files)` and list each path as a sub-row. Other slice flags with more than one item display `(N values)` and list each item as a sub-row.
  - `SOURCE`: The winning tier (`cli`, `env`, `config`, or `default`). A config-sourced value names the contributing file, using its basename when unambiguous and the shortest unique directory prefix followed by `...` when basenames collide (`config (app.yaml)`, `config (/etc/.../app.yaml)`). The JSON report keeps the full resolved paths.
  - `ENV`: The derived or explicit environment variable evaluated for this flag.
  - `CONFIG PATH`: The evaluated config file key.
- **Config decoders**: All file format decoders registered on the application.
- **Config paths**: Evaluated file paths and their probe outcome (`loaded`, `not found`), reported as resolved without index footnotes. A candidate that resolved to a different file, such as a search directory, names that file (`loaded: config.yaml`). For a programmatic `conga.ConfigLoader` document, the table lists each distinct origin path once; a scope recorded with `conga.ConfigDocument.SetScoped` appears only in the value's SOURCE annotation (`config (pipeline.local.yaml › pipeline deploy)`).
- **Features**: Every declared feature, its resolved value, and its source.

The same report is available programmatically: `session.Why()` returns the text table and `session.WhyJSON()` returns the JSON document (empty unless Why and the JSON option are enabled, respectively).

Map flags merge key by key across tiers: the flag row displays the key count and highest contributing tier, followed by tree-structured sub-rows (`├─ `, `└─ `) detailing each key's value and source:

```sh
SERVER_LABELS=region=eu-west-1 server --why --labels shared=cli,team=core
```

```text no-copy
  --labels   (3 keys)              cli   SERVER_LABELS   labels
             ├─ region=eu-west-1   env
             ├─ shared=cli         cli
             └─ team=core          cli
```

## Inspect feature provenance

When the root command declares a `conga.Features[T]` container (see [Feature toggles](/reference/struct-tags/#feature-toggles-congafeaturest)), the report ends with a `Features:` table. Each feature reports the tier that last wrote its key; keys no tier set fall back to the field default (`default`) or render `unset`:

```sh
serve --why
```

```text no-copy
Features:

  FEATURE      VALUE   SOURCE
  http3        false   unset
  admin-api    false   unset
  strict-tls   true    default
  metrics      true    default
```

## Where to go next

- [About feature toggles](/explanation/feature-toggles/) — design rationale, sparse gate merging, and member pruning.
- [About value precedence](/explanation/value-precedence/) — the deterministic four-tier resolution ladder and provenance rules.
- [How to bind environment variables](/how-to/bind-environment-variables/) — configure environment variable prefixes and explicit overrides.
- [How to manage config files](/how-to/manage-config-files/) — register file decoders and configure search paths.
- [The `--why` report](/reference/builtins/#the---why-report) — built-in reference documentation for `--why`.
