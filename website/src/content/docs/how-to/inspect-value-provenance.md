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

To enable the machine-readable `--why=json` document as well, pass `conga.WhyJSON()`: `app.EnableWhy(conga.WhyJSON())`. The same report is available programmatically from `session.Why()` and `session.WhyJSON()` inside lifecycle hooks. With `conga.WhyJSON()` enabled, the help screen renders the flag as `--why[=FORMAT]` with an enum hint of the accepted formats ([The `--why` report](/reference/builtins/#the---why-report)).

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

The report divides diagnostics into four areas:

1. Flag table: shows the flag name, winning value (or `██████` if untagged), winning source tier (`cli`, `env`, `config`, or `default`), evaluated environment variable, and configuration key. Unambiguous config origins display as the file basename (`config (app.yaml)`), while colliding basenames show the shortest unique directory prefix (`config (/etc/.../app.yaml)`). Entry-bearing map rows display key counts `(N keys)` and tree branches (`├─ `, `└─ `). Multi-file `ConfigFile` rows display `(N files)` with tree sub-rows, and slice flags with multiple values render `(N values)`.
2. Config decoders: lists all format decoders registered on the application.
3. Config paths: lists evaluated candidate file paths and probe outcomes (`loaded`, `not found`). When a bootstrap document records scopes via `SetScoped`, the table lists the origin path once while the value's source column includes the scope.
4. Features: lists every declared feature toggle, its resolved value, and its contributing tier.

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

## Inspect provenance as JSON

To emit a machine-readable document for CI or external tooling, pass `conga.WhyJSON()` when enabling Why:

```go title="main.go"
app := conga.New[Server]().
	Name("server").
	EnableWhy(conga.WhyJSON())
```

Invoking the binary with `--why=json` emits a versioned JSON document to stdout:

```sh
SERVER_PORT=9002 server --why=json --port 9003
```

```json no-copy
{
  "$schema": "https://conga.sh/schemas/why-v1.json",
  "schemaVersion": 1,
  "command": "server",
  "version": "1.0.0",
  "timestamp": "2026-10-20T08:00:00Z",
  "workingDirectory": "/workspace",
  "flags": [
    {
      "name": "--host",
      "type": "string",
      "value": "127.0.0.1",
      "source": "default",
      "detail": "127.0.0.1",
      "env": "SERVER_HOST",
      "configPath": "host",
      "hidden": false,
      "redacted": false
    },
    {
      "name": "--port",
      "type": "int",
      "value": 9003,
      "source": "cli",
      "detail": "--port",
      "env": "SERVER_PORT",
      "configPath": "port",
      "hidden": false,
      "redacted": false
    },
    {
      "name": "--token",
      "type": "string",
      "value": "██████",
      "source": "env",
      "detail": "SERVER_TOKEN",
      "env": "SERVER_TOKEN",
      "configPath": "token",
      "hidden": false,
      "redacted": true
    }
  ],
  "globalFlags": [],
  "features": [],
  "config": {
    "decoders": [".json", ".yaml", ".toml"],
    "paths": [
      {
        "path": "/workspace/config.yaml",
        "status": "not found",
        "probedFiles": []
      }
    ],
    "examples": []
  },
  "redacted": true
}
```

Passing `--why=json` without `conga.WhyJSON()` on the builder fails with a usage error (exit 2).

## Query provenance in lifecycle hooks

Commands can inspect provenance programmatically during execution. Call `session.Why()` for the text table or `session.WhyJSON()` for the JSON document inside lifecycle hooks (`PreRun`, `Run`, `PostRun`):

```go title="main.go"
func (s *Server) PreRun(ctx context.Context, session *conga.Session) error {
	// Log provenance or export to telemetry
	report := session.Why()
	fmt.Fprintln(session.Stderr(), report)

	return nil
}
```

Both methods return an empty string if Why is not enabled on the application. `session.WhyJSON()` also requires `conga.WhyJSON()` on `app.EnableWhy`.

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
