---
title: How to gate features
description: Declare feature toggles on the root command and gate flags and subcommands behind them.
---

Feature toggles activate optional flags and whole subcommands at runtime. A feature is declared once as a field of a `conga.Features[T]` container on the root command struct, and gated members carry a `feature:"..."` expression that must hold for them to exist. Values arrive through the standard resolution tiers: a `--feature-gates` map flag, an environment variable, a config section, or a struct default.

## Declare the feature toggles

Add a `conga.Features[T]` field to the root command struct and give `T` one exported field per feature. A `bool` field is an on/off toggle; an integer field is a leveled toggle whose non-zero levels are listed in the `enum` tag.

```go title="main.go"
type Experiments struct {
	HTTP3   bool `help:"Experimental HTTP/3 (QUIC) listener"`
	Metrics bool `default:"true" help:"Expose the metrics endpoint"`
}

type Server struct {
	Features conga.Features[Experiments] `flag:"feature-gates" env:"SERVER_FEATURES" config:"features"`

	Listen conga.OptionalFlag[string] `default:":8080" help:"Listen address"`
	HTTP3  conga.OptionalFlag[bool]   `feature:"http3" help:"Enable the QUIC/HTTP3 listener"`
}
```

Each `feature` tag holds a comma-separated conjunction of feature names; prefix a name with `!` to require it off, as in `feature:"!strict-tls"`. The tag is accepted on `OptionalFlag`, `Counter`, `Command[T]`, and `ConfigFile` fields; `RequiredFlag` rejects it because a required input cannot be absent.

## Enable feature resolution

Call `EnableFeatures()` on the application builder. Without it, schema build rejects the `conga.Features` container and every `feature` tag with `ErrSchema`:

```go title="main.go"
func main() {
	app := conga.New[Server]().
		Name("server").
		EnableFeatures().
		EnableWhy()

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
}
```

`EnableFeatures()` installs the feature resolver only when called, so an application that never declares features does not link the feature machinery.

## Activate features

Every tier contributes `name=value` pairs to one synthesized map, which merges key by key (CLI over environment over config over default). A key accepts any spelling of a declared name (kebab-case, snake_case, or camelCase), so `http3` and `http-3` resolve to the same feature.

Activate a feature from the command line:

```sh
server --feature-gates http3=true
```

Activate a feature from the environment:

```sh
SERVER_FEATURES=metrics=false server
```

Activate a feature from a config section. The section name comes from the container's `config` tag, and the application must register a decoder for the file format (see [How to manage config files](/how-to/manage-config-files/)):

```yaml title="config.yaml"
features:
  http3: true
```

## Gate members

Inactive gated flags and subcommands are pruned from `--help`, `--why`, and shell completion. Supplying one on the command line is a usage error (exit 2):

```sh
server --http3
```

```text no-copy
server: error: flag '--http3' requires feature "http3"

Usage:
  server [flags]

Run 'server --help' for more information.
```

A config key that names an inactive gated flag fails with `ErrConfig` (exit 1). An environment variable that names one is ignored, so ambient environment state never breaks a run it does not control.

## Read feature values

`Features.Get()` returns the resolved `*T`, and `Features.Source()` reports the highest contributing tier:

```go
if s.Features.Get().HTTP3 {
	// ...
}
```

Pass `--why` to render a `Features:` table with per-key provenance. Each feature reports the tier that last wrote its key; a key no tier sets falls back to its field default or renders `unset`:

```sh
server --why
```

```text no-copy
Features:

  FEATURE   VALUE   SOURCE
  http3     false   unset
  metrics   true    default
```

## Where to go next

- [Struct tag reference](/reference/struct-tags/#feature-toggles-congafeaturest) — the full container and feature-field tag vocabulary.
- [About value precedence](/explanation/value-precedence/) — how the four tiers merge, including sparse feature maps.
- [How to inspect value provenance](/how-to/inspect-value-provenance/) — the `--why` report.
- [Feature toggles example](https://github.com/conga-sh/conga/tree/main/_examples/29-feature-toggles) — a runnable program with CLI, environment, and config activation.
