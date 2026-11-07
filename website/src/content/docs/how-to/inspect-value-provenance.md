---
title: How to inspect value provenance
description: Diagnose flag resolution sources across CLI, environment, config files, and defaults with --why.
---

When flags resolve across multiple tiers (CLI flags, environment variables, config files, and defaults), determining which source provided a specific value can be difficult. Enable CONGA's built-in `--why` flag to output a diagnostic report detailing winning values, sources, and evaluated paths.

## Enable the provenance report

Call `EnableWhy()` on the application builder to install the `--why` flag:

```go title="main.go" hidden={1-8}
package main

import (
	"context"

	"github.com/conga-sh/conga"
)

type Server struct {
	Host   conga.OptionalFlag[string]            `default:"127.0.0.1" help:"Listen address" safe:"true"`
	Port   conga.OptionalFlag[int]               `default:"8080" help:"Listen port" safe:"true" short:"p"`
	Token  conga.OptionalFlag[string]            `help:"API secret token"`
	Labels conga.OptionalFlag[map[string]string] `help:"Labels" safe:"true"`
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

To enable the machine-readable `--why=json` document as well, pass `conga.WhyOptionJSON()`: `app.EnableWhy(conga.WhyOptionJSON())`. The same report is available programmatically from `session.Why()` and `session.WhyJSON()` inside lifecycle hooks. With `conga.WhyOptionJSON()` enabled, the help screen renders the flag as `--why[=FORMAT]` with an enum hint of the accepted formats ([The `--why` report](/reference/builtins/#the---why-report)).

## Protect sensitive data with value redaction

The report redacts values by default (see [The `--why` report](/reference/builtins/#the---why-report)). To display a field in plaintext, declare `safe:"true"`:

```go title="main.go"
// [!callout /safe/] Shown in plaintext in the --why report
Port conga.OptionalFlag[int] `default:"8080" safe:"true"`

// [!callout /Token/] Shown as ██████ in the --why report
Token conga.OptionalFlag[string] `help:"API secret token"`
```

The `safe` tag applies exclusively to flag containers; positional arguments and config-file paths display without redaction.

## Interpret the provenance output

Invoke the binary with `--why` alongside any other flags or environment variables:

```sh
$ SERVER_PORT=9002 SERVER_TOKEN=hunter2 server --why --port 9003
Value provenance for "server":

  FLAG       VALUE       SOURCE    ENV             CONFIG PATH
  --host     127.0.0.1   default   SERVER_HOST     host
  --port     9003        cli       SERVER_PORT     port
  --token    ██████      env       SERVER_TOKEN    token
  --labels               unset     SERVER_LABELS   labels

values are redacted — tag fields safe:"true" to display
```

The report has these parts, in this order, and leaves out any part with nothing to show:

1. Flag table: each flag's name, winning value (or `██████` for a flag without `safe:"true"`), source (`cli`, `env`, `config`, `default`, or `unset`), environment variable, and config key. A config source shows the file's base name (`config (app.yaml)`), or the shortest unique directory prefix when two loaded files share a name (`config (/etc/.../app.yaml)`). A map flag shows `(N keys)` with a tree of its entries, a multi-file `ConfigFile` shows `(N files)`, and a slice with several values shows `(N values)`.
2. Global flags: the same table for flags declared on ancestor commands.
3. Features: every declared feature toggle, its value, and its source.
4. Config decoders: the registered config formats.
5. Config paths: each place the program searched and what it found there (`loaded`, `directory does not exist`), with the files it looked for. When a bootstrap document records scopes with `SetScoped`, the table lists the origin path once, and the flag's source shows the scope.
6. Config file suggestions: where the user could create a config file, when none was loaded.
7. A redaction note, when any value is shown as `██████`.

Map flags merge key by key across tiers: the flag row displays the key count and highest contributing tier, followed by tree-structured sub-rows (`├─ `, `└─ `) detailing each key's value and source:

```sh {8-11}
$ SERVER_LABELS=region=eu-west-1 server --why --labels shared=cli,team=core
Value provenance for "server":

  FLAG       VALUE       SOURCE    ENV             CONFIG PATH
  --host     127.0.0.1   default   SERVER_HOST     host
  --port     8080        default   SERVER_PORT     port
  --token                unset     SERVER_TOKEN    token
  --labels   (3 keys)    cli       SERVER_LABELS   labels
             ├─ region=eu-west-1   env
             ├─ shared=cli         cli
             └─ team=core          cli
```

## Inspect provenance as JSON

To emit a machine-readable document for CI or external tooling, pass `conga.WhyOptionJSON()` when enabling Why:

```go title="main.go"
app := conga.New[Server]().
	Name("server").
	EnableWhy(conga.WhyOptionJSON())
```

Invoking the binary with `--why=json` writes a versioned JSON document to stdout on one line. Here it is piped through `jq` for reading:

```sh
$ SERVER_PORT=9002 SERVER_TOKEN=hunter2 server --why=json --port 9003 | jq
{
  "$schema": "https://conga.sh/schemas/why/v1.json",
  "schemaVersion": 1,
  "command": "server",
  "timestamp": "2026-11-06T23:44:06Z",
  "workingDirectory": "/tmp",
  "redacted": true,
  "flags": [
    {
      "name": "host",
      "type": "string",
      "value": "127.0.0.1",
      "source": "default",
      "env": "SERVER_HOST",
      "configPath": "host"
    },
    {
      "name": "port",
      "type": "int",
      "value": 9003,
      "source": "cli",
      "env": "SERVER_PORT",
      "configPath": "port"
    },
    {
      "name": "token",
      "type": "string",
      "redacted": true,
      "value": null,
      "source": "env",
      "env": "SERVER_TOKEN",
      "configPath": "token"
    },
    {
      "name": "labels",
      "type": "map",
      "value": null,
      "source": "unset",
      "env": "SERVER_LABELS",
      "configPath": "labels"
    }
  ],
  "config": {
    "decoders": [],
    "paths": []
  }
}
```

A redacted value is `null`, with `"redacted": true` on the flag and on the document. Keys with nothing to report are left out, such as `version` when the app sets none, and `globalFlags` and `features` when they're empty.

Passing `--why=json` without `conga.WhyOptionJSON()` on the builder fails with a usage error (exit 2).

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

Both methods return an empty string if Why is not enabled on the application. `session.WhyJSON()` also requires `conga.WhyOptionJSON()` on `app.EnableWhy`.

## Inspect feature provenance

When the root command declares a `conga.Features[T]` container (see [Feature toggles](/reference/struct-tags/#feature-toggles-congafeaturest)), the report lists features after the flag tables. Each feature shows the source that last set it; a feature no source sets shows `default` when its field has a `default` tag, and `unset` otherwise. This run of the [feature toggles example](https://github.com/conga-sh/conga/tree/main/_examples/29-feature-toggles) has the flag table and config sections hidden:

```sh hidden={3-9,16-30}
$ feature-toggles --why
Value provenance for "feature-toggles":

  FLAG              VALUE   SOURCE    ENV            CONFIG PATH
  --config-file             unset     API_CONFIG     -
  --feature-gates           unset     API_FEATURES   features
  --listen          :8080   default   API_LISTEN     listen
  --cache           128     default   API_CACHE      cache

Features:
  FEATURE      VALUE   SOURCE
  http3        false   unset
  admin-api    false   unset
  strict-tls   true    default
  metrics      true    default

Config decoders: .yaml, .yml

Config paths:
  /home/alice/.config/feature-toggles   directory does not exist
    looked for: /home/alice/.config/feature-toggles.yaml
    looked for: /home/alice/.config/feature-toggles.yml

Create a config file at one of:
  /home/alice/.config/feature-toggles/config.yaml
  /home/alice/.config/feature-toggles/config.yml
  /home/alice/.config/feature-toggles/feature-toggles.yaml
  /home/alice/.config/feature-toggles/feature-toggles.yml
  /home/alice/.config/feature-toggles.yaml
  /home/alice/.config/feature-toggles.yml
```

## Where to go next

- [About feature toggles](/explanation/feature-toggles/) — design rationale, sparse gate merging, and member pruning.
- [About value precedence](/explanation/value-precedence/) — the deterministic four-tier resolution ladder and provenance rules.
- [How to bind environment variables](/how-to/bind-environment-variables/) — configure environment variable prefixes and explicit overrides.
- [How to manage config files](/how-to/manage-config-files/) — register file decoders and configure search paths.
- [The `--why` report](/reference/builtins/#the---why-report) — built-in reference documentation for `--why`.
