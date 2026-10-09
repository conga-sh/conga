---
title: How to inspect value provenance
description: Diagnose flag resolution sources across CLI, environment, config files, and defaults with --why.
---

When flags resolve across multiple tiers (CLI flags, environment variables, config files, and defaults), determining which source provided a specific value can be difficult. Enable CONGA's built-in `--why` flag to output a diagnostic report detailing winning values, sources, and evaluated paths.

## Enable the provenance report

Call `EnableWhy()` on the application builder to install the `--why` flag:

```go title="main.go"
type Server struct {
	Host  conga.OptionalFlag[string] `default:"127.0.0.1" help:"Listen address" safe:"true"`
	Port  conga.OptionalFlag[int]    `default:"8080" help:"Listen port" safe:"true" short:"p"`
	Token conga.OptionalFlag[string] `help:"API secret token"`
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

Passing `--why` intercepts execution during the resolution phase. CONGA computes the provenance table, prints the report to stdout, and exits with code 0 before any lifecycle hooks (`PreRun`, `Run`, `PostRun`) execute.

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
SERVER_PORT=9002 server --why --port 9003
```

```text no-copy
Value provenance for "server":

  FLAG     VALUE       SOURCE   ENV           CONFIG PATH
  --host   127.0.0.1   default  SERVER_HOST   host
  --port   9003        cli      SERVER_PORT   port
  --token  ██████      default  SERVER_TOKEN  token

Config decoders: none

Config paths checked:
  none
```

The report contains three diagnostic sections:

- **Flag table**:
  - `FLAG`: The declared flag name.
  - `VALUE`: The winning value, or `██████` if untagged.
  - `SOURCE`: The winning tier (`cli`, `env`, `config`, or `default`).
  - `ENV`: The derived or explicit environment variable evaluated for this flag.
  - `CONFIG PATH`: The evaluated config file key.
- **Config decoders**: All file format decoders registered on the application.
- **Config paths checked**: Every file path probed during resolution, annotated with its status (`(none found)` or loaded).

## Where to go next

- [About value precedence](/explanation/value-precedence/) — the deterministic four-tier resolution ladder and provenance rules.
- [How to bind environment variables](/how-to/bind-environment-variables/) — configure environment variable prefixes and explicit overrides.
- [How to manage config files](/how-to/manage-config-files/) — register file decoders and configure search paths.
- [The `--why` report](/reference/builtins/#the---why-report) — built-in reference documentation for `--why`.
