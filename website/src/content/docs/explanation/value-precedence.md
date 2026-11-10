---
title: About value precedence
description: The order CONGA resolves values in (command line, environment, config files, defaults), how map flags merge across those sources, and how provenance is recorded.
---

Configuration values in CONGA follow Twelve-Factor principles. Command-line arguments override environment variables, environment variables override config files, and config files override defaults.

This page explains how CONGA resolves input across these sources, why resolution separates file discovery from value binding, and where specialised containers depart from the general order. [How to bind environment variables](/how-to/bind-environment-variables/) and [How to manage config files](/how-to/manage-config-files/) set the sources up.

## The precedence order

A flag resolves through four sources, highest first:

```text no-copy
1. CLI          explicit --flag or -f on the command line
2. Environment  $<PREFIX>_<FLAG> (via EnvPrefix or envprefix), or an explicit env tag
3. Config       keys from loaded config files, or values from Bootstrap.SetConfig
4. Default      the `default:"..."` struct tag
```

The first source that supplies a value wins. With no source, a flag holds its type's zero value and reports `SourceNone`.

## Two-phase resolution

Resolution completes before any lifecycle hook runs, in two sequential phases:

```text no-copy
Phase 1: Config File Discovery
  CLI (--config-file) --> ConfigExplicit --> Env ($PREFIX_CONFIG) --> Search paths (XDG/Paths)
                                                                       |
                                                                       v
                                                             Loaded Config Map
                                                                       |
Phase 2: Flag Value Binding                                            v
  CLI (--flag) ---------> Environment ($VAR) ----> Config File Key ---> Struct Default
```

### Phase 1: Config file discovery

Before binding any flag, CONGA determines which config files to load and decodes them into one key-value map. Discovery has its own precedence, with `--config-file` first, then `conga.ConfigExplicit(...)`, then `<PREFIX>_CONFIG`, then the search paths. [About the config cascade](/explanation/config-cascade/) covers the existence rules and merging. Once the files load and validate against the schema, their keys are the lookup table for Phase 2.

A [`conga.Bootstrapper`](/explanation/lifecycle/#the-bootstrap-phase) runs before Phase 1 and can contribute a config document with `Bootstrap.SetConfig`. The document's values join the config source, overlaying file values key by key.

### Phase 2: Flag value binding

With the config map loaded, CONGA evaluates the precedence order for each flag on the selected command chain. The winning source sets the container's value and provenance. Map flags are the exception. Every source contributes its keys, and higher sources override per key (see [Map flags](#map-flags)).

## Why ConfigFile sits outside the precedence order

`conga.ConfigFile` is declared like a flag on the root command, but it governs file discovery (Phase 1) rather than value binding (Phase 2).

A config file cannot name the config file to load, so the config source cannot feed itself.

For the same reason, schema build rejects a `default` tag on a `ConfigFile` field. A default path would be neither loaded nor validated by the config engine. Probed locations belong in search paths (`conga.ConfigPaths`), and an example path shown in help belongs in `placeholder`.

`ConfigFile` instead records Phase 1. It names the files that discovery loaded, whether from the command line, the environment, an explicit target, or a search path. When no file loads, it stays unset (`SourceNone`).

## Departures from the general order

Not every container reads all four sources.

### Required flags

`RequiredFlag[T]` has no fallback. If the command line, environment, and config files all leave it unset, resolution stops before the command runs with a usage error (exit code 2).

### Per-flag opt-outs

`env:"-"` and `config:"-"` remove a flag from the environment or config source, so only the remaining sources can set it.

### Decoderless applications

CONGA skips file loading if an application does not register a config decoder, and values fall from the environment straight to defaults. A `Bootstrapper` can still supply the config source in code with `Bootstrap.SetConfig`.

### Positional arguments

`RequiredArg[T]`, `OptionalArg[T]`, `DashArgs`, and `RawArgs` read only command-line tokens. `OptionalArg[T]` falls back to its `default` tag.

### Map flags

Scalar and slice flags take the value of the single winning source. `map[K]V` flags, including registered map types, merge key by key across all four sources instead. Each source contributes `key=value` pairs, applied from lowest to highest, so a higher source overrides only the keys it names. Within the config source, [cascading](/explanation/config-cascade/) applies the same rule across files, and the nearest file wins each key.

| Source, lowest first | Contributed keys | Map after this source |
|---|---|---|
| Default tag | `region=us-east-1`, `timeout=30s` | `region=us-east-1`, `timeout=30s` |
| Config file | `timeout=60s`, `retries=3` | `region=us-east-1`, `timeout=60s`, `retries=3` |
| Environment | `retries=5`, `debug=true` | `region=us-east-1`, `timeout=60s`, `retries=5`, `debug=true` |
| Command line | `debug=false`, `profile=dev` | `region=us-east-1`, `timeout=60s`, `retries=5`, `debug=false`, `profile=dev` |

This matches the convention of Kubernetes `--feature-gates`, Helm `--set`, and `git -c`. A higher source can change a key set by a lower one, but cannot remove it.

Because every source contributes, CONGA parses map entries from every active source, even when the command line also sets the flag. A malformed entry in a config file or environment variable fails the run with a usage error (exit code 2), and the message names the source, such as `(from environment variable TOOL_LABELS)` or `(from config key "labels")`.

### Feature maps

A `conga.Features[T]` container resolves through the same key-by-key merge, then overlays its fields' `default` tags, so features no source names keep their defaults. CONGA canonicalizes feature keys before merging, so precedence follows the source, never the spelling. The application's `EnvPrefix` never applies to features. [About feature toggles](/explanation/feature-toggles/) explains the merge and its environment variables.

### `.env` files

With `app.Dotenv()`, variables from `.env` files enter the process environment before resolution and bind as ordinary environment variables ([Dotenv files](/reference/dotenv/)).

### Empty environment variables

An empty variable (`FOO=`) counts as unset for every flag type, string flags included, so lower sources apply. To set a string flag to the empty string, pass `--flag ""` on the command line.

## Provenance tracking

Every container records the source that won and the exact origin within it. For a value from the config source, the origin is the file path as resolved, or the label a bootstrap document recorded with `Set`, falling back to the config key when the document gives no label. A scalar or slice names its single winning file, and a merged map lists every contributing file in load order:

```go
source := cmd.Addr.Source()       // conga.SourceCLI, SourceEnv, SourceConfig, etc.
detail := cmd.Addr.SourceDetail() // "--addr", "CLOUDCTL_ADDR", "/etc/cloudctl/app.toml"
if cmd.Addr.Changed() {
	// Value came from the command line, the environment, or a config file
}
```

| Constant | Meaning |
|----------|---------|
| `conga.SourceCLI` | Bound from a command-line flag or argument token |
| `conga.SourceEnv` | Resolved from an environment variable |
| `conga.SourceConfig` | Read from a loaded config file or a bootstrap document |
| `conga.SourceDefault` | Supplied by a `default` tag |
| `conga.SourceNone` | Set by no source, holding the type's zero value |

`Changed()` reports whether the command line, the environment, or a config file supplied the value, separating explicitly configured values from defaults.

## Inspecting precedence with `--why`

With `app.EnableWhy()`, the `--why` flag prints the winning source and origin of every flag on the selected command chain. [The `--why` report](/reference/builtins/#the---why-report) documents the output, and [How to inspect value provenance](/how-to/inspect-value-provenance/) walks through a diagnosis.
