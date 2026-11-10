---
title: About value precedence
description: The order CONGA resolves values in (command line, environment, config files, defaults), how map flags merge across those sources, and how provenance is recorded.
---

Configuration values in CONGA follow Twelve-Factor principles. Command-line arguments override environment variables, environment variables override config files, and config files override defaults.

This page explains how CONGA resolves input across these sources, why resolution separates file discovery from value binding, and where specialized containers depart from the general order. [How to bind environment variables](/how-to/bind-environment-variables/) and [How to manage config files](/how-to/manage-config-files/) set the sources up.

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

<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 960 378" role="img" aria-labelledby="precedence-title" class="precedence-diagram">
  <title id="precedence-title">Resolution runs in two phases. Phase 1 picks the config files. It tries the ConfigFile flag, then ConfigExplicit, then the config environment variable, then the search paths, and the first that applies wins. The files it loads form the config map. Phase 2 binds each flag from the command line, then the environment, then its key in the config map, then its default tag, and the first source that supplies a value wins.</title>
  <style>
    .precedence-diagram { font-family: inherit; max-width: 960px; }
    .precedence-diagram .phase { fill: var(--sl-color-gray-6, #edeff4); stroke: var(--sl-color-accent, #3448f5); stroke-width: 1.5; }
    .precedence-diagram .box { fill: var(--sl-color-black, #ffffff); stroke: var(--sl-color-accent, #3448f5); stroke-width: 1.5; }
    .precedence-diagram .map { fill: var(--sl-color-accent-low, #d1d6fb); stroke: var(--sl-color-accent, #3448f5); stroke-width: 1.5; }
    .precedence-diagram .flow { fill: none; stroke: var(--sl-color-accent, #3448f5); stroke-width: 1.5; }
    .precedence-diagram .head { fill: var(--sl-color-accent, #3448f5); stroke: none; }
    .precedence-diagram .title { fill: var(--sl-color-white, #17181c); font-size: 17px; font-weight: 600; }
    .precedence-diagram .label { fill: var(--sl-color-white, #17181c); font-size: 15px; font-weight: 600; text-anchor: middle; }
    .precedence-diagram .note { fill: var(--sl-color-gray-3, #545964); font-size: 14px; font-style: italic; text-anchor: middle; }
    .precedence-diagram .note.end { text-anchor: end; }
  </style>
  <rect class="phase" x="10" y="10" width="940" height="130" rx="4"/>
  <text class="title" x="26" y="36">Phase 1: config file discovery</text>
  <text class="note end" x="934" y="36">first that applies wins</text>
  <rect class="box" x="26" y="60" width="200" height="58" rx="3"/>
  <text class="label" x="126" y="81">ConfigFile flag</text>
  <text class="note" x="126" y="102">--config-file by default</text>
  <path class="flow" d="M226 89 H261"/>
  <path class="head" d="M262 89 L253 83 L253 95 Z"/>
  <rect class="box" x="262" y="60" width="200" height="58" rx="3"/>
  <text class="label" x="362" y="81">ConfigExplicit</text>
  <text class="note" x="362" y="102">path set in code</text>
  <path class="flow" d="M462 89 H497"/>
  <path class="head" d="M498 89 L489 83 L489 95 Z"/>
  <rect class="box" x="498" y="60" width="200" height="58" rx="3"/>
  <text class="label" x="598" y="81">Config variable</text>
  <text class="note" x="598" y="102">&lt;PREFIX&gt;_CONFIG</text>
  <path class="flow" d="M698 89 H733"/>
  <path class="head" d="M734 89 L725 83 L725 95 Z"/>
  <rect class="box" x="734" y="60" width="200" height="58" rx="3"/>
  <text class="label" x="834" y="81">Search paths</text>
  <text class="note" x="834" y="102">ConfigPaths or XDG</text>
  <rect class="phase" x="10" y="238" width="940" height="130" rx="4"/>
  <text class="title" x="26" y="264">Phase 2: flag value binding</text>
  <text class="note end" x="934" y="264">first that applies wins</text>
  <rect class="box" x="26" y="288" width="200" height="58" rx="3"/>
  <text class="label" x="126" y="309">Command line</text>
  <text class="note" x="126" y="330">--flag</text>
  <path class="flow" d="M226 317 H261"/>
  <path class="head" d="M262 317 L253 311 L253 323 Z"/>
  <rect class="box" x="262" y="288" width="200" height="58" rx="3"/>
  <text class="label" x="362" y="309">Environment</text>
  <text class="note" x="362" y="330">&lt;PREFIX&gt;_&lt;FLAG&gt;</text>
  <path class="flow" d="M462 317 H497"/>
  <path class="head" d="M498 317 L489 311 L489 323 Z"/>
  <rect class="box" x="498" y="288" width="200" height="58" rx="3"/>
  <text class="label" x="598" y="309">Config file</text>
  <text class="note" x="598" y="330">key in the config map</text>
  <path class="flow" d="M698 317 H733"/>
  <path class="head" d="M734 317 L725 311 L725 323 Z"/>
  <rect class="box" x="734" y="288" width="200" height="58" rx="3"/>
  <text class="label" x="834" y="309">Default</text>
  <text class="note" x="834" y="330">default tag</text>
  <path class="flow" d="M598 140 V161"/>
  <path class="head" d="M598 162 L592 153 L604 153 Z"/>
  <rect class="map" x="508" y="162" width="180" height="40" rx="3"/>
  <text class="label" x="598" y="187">Loaded config map</text>
  <path class="flow" d="M598 202 V287"/>
  <path class="head" d="M598 288 L592 279 L604 279 Z"/>
</svg>

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
