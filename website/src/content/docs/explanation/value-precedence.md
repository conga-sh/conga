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

Unlike scalar and slice flags where a single winning tier replaces all lower tiers, `map[K]V` flags (and registered custom map types) merge key by key across all four tiers. Each tier contributes a set of `key=value` pairs, and the effective map folds in ascending order: `default < config < env < CLI`.

A higher tier overrides only the specific keys it names. Keys supplied by lower tiers persist into the final map unless explicitly overridden:

| Tier | Contributed keys | Effective map after tier |
|---|---|---|
| **1. Default tag** | `region=us-east-1`, `timeout=30s` | `region=us-east-1`, `timeout=30s` |
| **2. Config file** | `timeout=60s`, `retries=3` | `region=us-east-1`, `timeout=60s`, `retries=3` |
| **3. Environment** | `retries=5`, `debug=true` | `region=us-east-1`, `timeout=60s`, `retries=5`, `debug=true` |
| **4. CLI flag** | `debug=false`, `profile=dev` | `region=us-east-1`, `timeout=60s`, `retries=5`, `debug=false`, `profile=dev` |

This folding model matches conventions established by tools such as Kubernetes (`--feature-gates`), Helm (`--set`), and Git (`git -c`). A key set by a lower tier can be updated from a higher tier, but cannot be removed entirely.

#### Lower-tier validation and attribution

Because every tier contributes to the merged map, CONGA parses and validates map entries across all active tiers. A malformed entry or syntax error in a configuration file or environment variable produces an error even if an operator also specifies the flag on the command line. Errors attribute to the specific tier that supplied the malformed token:
- Malformed configuration entries fail during file decoding with `ErrConfig` (exit code 1).
- Malformed environment entries fail during parsing with `ErrParse` (exit code 2).

### Feature maps

A `conga.Features[T]` container resolves through the same key-by-key map merge. Every tier contributes `name=value` pairs, higher tiers override matching keys, and the resulting map overlays the inner feature struct's `default` tags key by key. Features not listed in any tier retain their field defaults (sparse overrides).

Before merging, CONGA canonicalizes all feature keys to the declared feature name. Whether a configuration file specifies `gentle_force`, an environment variable sets `GENTLE_FORCE`, or the command line sets `gentle-force`, all three match the `GentleForce` toggle. Precedence follows the resolution tier, never the spelling variant.

The container records the highest contributing tier as its `Source()`, and `--why` renders each individual feature against the tier that last wrote its key. See [About feature toggles](/explanation/feature-toggles/) and [How to inspect value provenance](/how-to/inspect-value-provenance/).

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
