---
title: About value precedence
description: The 12-factor resolution ladder — CLI, environment, config files, defaults — with provenance on every field.
---

Configuration values in CONGA adhere to Twelve-Factor principles: immediate command-line arguments override environment variables, environment variables override config files, and config files override hardcoded defaults.

This page explains how CONGA resolves input across these sources, why resolution separates file discovery from value binding, and how specialised containers diverge from the general resolution ladder. For step-by-step setup guides, see [How to bind environment variables](/how-to/bind-environment-variables/) and [How to manage config files](/how-to/manage-config-files/).

## The general flag resolution ladder

For general configuration flags (`OptionalFlag[T]`), values resolve through a deterministic four-tier ladder:

```text no-copy
1. CLI          explicit --flag or -f on the command line
2. Environment  $PREFIX_NAME (via EnvPrefix) or the flag's env tag
3. Config file  keys from the active config file(s)
4. Default      the `default:"..."` struct tag, or the type's zero value
```

The first tier that supplies a value wins. Lower tiers apply only when higher tiers leave the value unset.

CLI flags represent transient execution choices made by an operator for a single run and carry the highest priority. Environment variables configure an execution environment—such as a container runtime, orchestrator, or shell session—across runs. Config files provide structured settings shared across multiple invocations of a host or project. Default values compile a baseline into the binary or rely on Go zero values.

## Two-phase resolution

Resolution occurs before any lifecycle hook or command body executes. To feed the four-tier ladder deterministically, CONGA splits resolution into two sequential phases:

```text no-copy
Phase 1: Config File Discovery
  CLI (--config-file) --> Env ($PREFIX_CONFIG) --> Explicit --> Search Tiers (XDG/Paths)
                                                                       |
                                                                       v
                                                             Loaded Config Map
                                                                       |
Phase 2: Flag Value Binding                                            v
  CLI (--flag) ---------> Environment ($VAR) ----> Config File Key ---> Struct Default
```

### Phase 1: Config file discovery

Before binding individual flags, CONGA determines which config files exist and loads them into a flat key-value map. Discovery follows its own precedence (`--config-file`, then `<PREFIX>_CONFIG`, then `conga.ConfigExplicit(...)`, then probed search tiers), with per-tier existence semantics and merge rules described in [About the config cascade](/explanation/config-cascade/). Once discovered files load and validate against the schema, their keys form the lookup table for Phase 2.

### Phase 2: Flag value binding

With the configuration map prepared, CONGA iterates over the command hierarchy's flags, evaluating the four-tier ladder for each field. The winning tier sets the container's value and provenance. Map flags are the one exception: instead of a winning tier, every tier contributes its keys and higher tiers override per key (see [Map flags](#map-flags)).

## Why ConfigFile sits outside the four-tier ladder

Although declared as a flag container (`conga.ConfigFile`) on the root command struct, `ConfigFile` operates outside the four-tier flag ladder because it governs file discovery (Phase 1) rather than individual value binding (Phase 2).

First, a config file cannot define which config file to load, so Tier 3 cannot feed itself.

Second, the schema validator rejects `default` struct tags on `ConfigFile` fields at startup. A default path on the flag would be neither loaded nor validated through the configuration engine. Probed locations belong in search paths (`conga.ConfigPaths`), while display examples in help text use `placeholder`.

Instead, `ConfigFile` configures and records Phase 1. It discovers config files through CLI flags, environment variables, explicit targets, or probed directories. When a file loads from a search tier, `ConfigFile` records `SourceConfig`. If no config files resolve, `ConfigFile` remains unset (`SourceNone`), and flag binding proceeds to defaults for remaining fields.

## Ladder exceptions and specialised containers

Not all input containers traverse all four tiers:

### Mandatory flags

`RequiredFlag[T]` enforces presence without fallback. The schema builder rejects `default` tags on required flags. If tiers 1 through 3 leave a required flag unset, resolution halts before command execution with a usage error (exit code 2).

### Tier opt-outs

Individual flags can bypass specific tiers using struct tags. Setting `env:"-"` disables environment variable resolution, restricting the flag to CLI arguments, config files, and defaults. Setting `config:"-"` excludes the field from config file binding, ensuring values originate only from the command line, environment, or defaults.

### Decoderless applications

If an application registers no config decoders (`app.RegisterDecoder`), Tier 3 remains inactive for all flags. The engine skips file loading and falls directly from the environment tier to struct defaults.

### Positional arguments

Positional containers (`RequiredArg[T]`, `OptionalArg[T]`, `DashArgs`, and `RawArgs`) resolve exclusively from command-line tokens. `OptionalArg[T]` populates from its `default` tag when omitted; `RequiredArg[T]` requires an explicit CLI token.

### Map flags

`map[K]V` flags merge key by key across all four tiers instead of resolving a single winning tier. Each tier supplies a set of `key=value` pairs, and the effective map is the fold of `default < config < env < CLI`, with a higher tier overriding only the keys it names. A config file can therefore set one key while the command line sets another, and both apply. This matches the established convention for map-like flags (Kubernetes `--feature-gates`, Helm `--set`, `git -c`). A key a lower tier sets can be overridden from a higher tier but never removed.

### Feature maps

A `conga.Features[T]` container resolves as a map flag: every tier contributes `name=value` pairs, higher tiers override per key, and the merged result overlays the inner features' `default` tags key by key, so features no tier lists keep their declared defaults (sparse overrides). The container records the highest contributing tier once; `Features.Source()` reports it, and `--why` renders each feature's effective value against the tier that last wrote its key. See [Feature toggles](/reference/struct-tags/#feature-toggles-congafeaturest) and the [container accessors](/reference/conga/#containers).

### Environment file integration

When `.env` loading is enabled via `app.Dotenv()`, variables from `.env` files populate the process environment before flag resolution begins ([Dotenv files](/reference/dotenv/)). These values feed Tier 2 as standard environment variables.

## Provenance tracking

CONGA records the origin of every resolved value. Rather than discarding resolution history after binding, containers retain their winning tier and source detail throughout execution (map containers record the highest contributing tier):

```go
source := cmd.Addr.Source()       // conga.SourceCLI, SourceEnv, SourceConfig, etc.
detail := cmd.Addr.SourceDetail() // "--addr", "CLOUDCTL_ADDR", "/etc/cloudctl/app.toml"
if cmd.Addr.Changed() {
	// Value originated from an external source (CLI, env, or config file)
}
```

| Constant | Meaning |
|----------|---------|
| `conga.SourceCLI` | Bound from a command-line flag or argument token |
| `conga.SourceEnv` | Resolved from an environment variable |
| `conga.SourceConfig` | Extracted from a loaded config file |
| `conga.SourceDefault` | Supplied by a `default` tag or type zero value |
| `conga.SourceNone` | Container remained unset across all tiers |

The `Changed()` method reports whether an external tier (CLI, environment, or config file) supplied the value, distinguishing explicitly configured values from default fallbacks.

## Inspecting precedence with `--why`

Applications enabling `app.EnableWhy()` gain the `--why` diagnostic flag, which prints the winning tier and exact source detail for every field in the active command chain. [The `--why` report](/reference/builtins/#the---why-report) documents the output, and [How to inspect value provenance](/how-to/inspect-value-provenance/) walks through a diagnosis. Search path precedence is covered in [About the config cascade](/explanation/config-cascade/).
