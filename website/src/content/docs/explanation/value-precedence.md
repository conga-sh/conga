---
title: About value precedence
description: The 12-factor resolution ladder — CLI, environment, config files, defaults — with provenance on every field.
---

Every flag value resolves through a deterministic four-tier ladder:

```
1. CLI          explicit --flag or -f on the command line
2. Environment  $PREFIX_NAME (via EnvPrefix) or the flag's env tag
3. Config file  keys from the active configuration file(s)
4. Default      the `default:"..."` struct tag, or the type's zero value
```

The first tier that provides a value takes precedence; lower tiers apply only when higher tiers leave the value unset.

## Wiring the environment tier

`EnvPrefix` establishes the application-wide environment namespace. Prefixes are normalized to end with `_`:

```go
app := conga.New[CLI]().
	Name("cloudctl").
	EnvPrefix("CLOUDCTL") // flag --region reads $CLOUDCTL_REGION
```

Finer control:

- `env:"VERBATIM_NAME"` on a flag uses that exact variable name verbatim.
- `env:"-"` disables environment resolution for the flag.
- The `envprefix` tag on a subcommand field scopes a per-subcommand prefix.

## Wiring the config tier

A field's config key defaults to its flag name. The `config` tag overrides it, and `config:"-"` removes the field from config resolution entirely:

```go
type Serve struct {
	Addr   conga.OptionalFlag[string] `default:":8080" config:"server.address"`
	Secret conga.OptionalFlag[string] `env:"SERVE_SECRET" config:"-" help:"Loaded only from CLI or environment"`
}
```

See [About the config cascade](config-cascade/) for search paths, cascading, profiles, and decoders.

## Provenance tracking

Because every container records where its value came from, commands can report or branch on provenance:

```go
addr := cmd.Addr.Source()         // conga.SourceCLI | SourceEnv | SourceConfig | SourceDefault | SourceNone
detail := cmd.Addr.SourceDetail() // "--addr", "CLOUDCTL_ADDR", "/etc/cloudctl/app.toml", ...
if cmd.Addr.Changed() {
	// explicitly provided via CLI, env, or config
}
```

| Constant | Meaning |
|----------|---------|
| `conga.SourceCLI` | Parsed from the command line |
| `conga.SourceEnv` | Resolved from an environment variable |
| `conga.SourceConfig` | Resolved from a configuration file |
| `conga.SourceDefault` | Fell through to the `default` tag or zero value |
| `conga.SourceNone` | Value left unset across all tiers |

## The `--why` report

`app.EnableWhy()` enables `--why`, which renders this provenance for every field of the active command chain, including the resolved config file path. Values are redacted by default so secrets stay out of terminals and CI logs; fields opt in with `safe:"true"`. See [The `--why` report](../reference/builtins/#the---why-report).
