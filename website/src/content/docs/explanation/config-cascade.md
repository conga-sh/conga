---
title: About the config cascade
description: How config files are discovered, merged, and classified — the XDG tier, cascading, and error semantics.
---

CONGA loads config files into a flat `map[string]string` view that feeds tier 3 of the [value precedence ladder](/explanation/value-precedence/). Formats are opt-in packages; only registered formats are linked into the application binary. The practical steps — registering decoders, declaring search paths, profiles, and reload — live in [How to manage config files](/how-to/manage-config-files/).

## Cascading merges

By default a search tier is first-hit: `config.Paths(...)` lists files in precedence order and the first existing one wins. Declaring `config.Cascading()` changes the model to a merge: every existing file contributes, from lowest to highest precedence, so higher-precedence paths override keys set earlier. Partial cascades are fine — any subset of the list may exist.

Because a cascade can load more than one file, a declared root config-file flag must accept multiple values (`multi:"true"`) so it can report every merged file; the schema rejects the combination otherwise.

## Existence semantics

| Path source | Missing file |
|-------------|--------------|
| CLI flag value (`--config-file`) | Fatal: descriptive error (`ErrConfig` base) |
| `<PREFIX>_CONFIG` environment variable | Fatal: descriptive error (`ErrConfig` base) |
| `config.Explicit(...)` | Fatal: descriptive error (`ErrConfig` base) |
| Search tier (`config.Paths(...)`, built-in XDG tier) | Silently skipped; probing continues |
| Multiple decoder-matching candidates in one tier | Fatal conflict error (`ErrConfig` base) |
| Nothing resolves anywhere | Silent no-op — the run continues on defaults |

A named path must exist; a probed search path is checked opportunistically. This matches the modern CLI norm — `kubectl --kubeconfig`, `curl --config`, and `ssh -F` all fail on named paths, while search lists like `KUBECONFIG` skip missing entries.

## The built-in conventional tier

When the app is config-capable (at least one registered decoder and a resolvable name) and declares no search tier, CONGA probes:

```text no-copy
${XDG_CONFIG_HOME:-~/.config}/<app-name>/config.<ext>
${XDG_CONFIG_HOME:-~/.config}/<app-name>/<app-name>.<ext>
```

for each registered extension. This is the lowest-precedence tier and is replaced entirely by `config.Paths(...)`. A `ConfigFile` container does not accept the `default` tag: a file loads only when it is named explicitly (`--config-file`, the environment variable, or `config.Explicit(...)`) or discovered inside a probed tier. The [`--why` report](/how-to/inspect-value-provenance/) and `session.CandidateConfigFiles()` list the candidates an invocation evaluates.

## Decoder capability

Resolution only reads formats the application registered: search tiers probe only registered extensions, and no wire-format library is linked unless a decoder package is. A decoderless application is still config-aware enough to surface candidates — the config-file flag's help text appends `(no configuration decoder registered)`, and `session.CandidateConfigFiles()` reports the environment targets — but a named target (an explicit `--config-file`, the `<PREFIX>_CONFIG` variable, or `config.Explicit(...)`) must have a decoder for its format: resolving one without a decoder is a fatal `ErrConfig` error naming the missing decoder or the unsupported format, never a silent no-op. The `--why` report marks the failing candidate row.

## Error semantics

| Situation | Result |
|-----------|--------|
| Named path missing (`--config-file`, `<PREFIX>_CONFIG`, `Explicit(...)`) | Fatal: descriptive error (`ErrConfig` base) |
| Named target whose format has no registered decoder | Fatal: `ErrConfig` base (missing decoder or unsupported format) |
| File larger than 16 MB | Fatal: size-limit error (DoS hardening, `ErrConfig` base) |
| Malformed content | Fatal: descriptive error (`ErrConfig` base) |
| Unknown keys with strict validation | `ErrConfig` base, with did-you-mean hints on near misses |
| Multiple conflicting candidates in a probed tier | `ErrConfig` base |
