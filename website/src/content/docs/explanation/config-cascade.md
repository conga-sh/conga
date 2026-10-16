---
title: About the config cascade
description: How config files are discovered, merged, and classified — the XDG tier, cascading, and error semantics.
---

CONGA validates config files into the tier-3 value view that feeds the [value precedence ladder](/explanation/value-precedence/): scalar and slice values flatten to strings, and map flags keep their per-key pairs with the contributing file recorded as their origin. Formats are opt-in packages; only registered formats are linked into the application binary. The practical steps — registering decoders, declaring search paths, and cascading — live in [How to manage config files](/how-to/manage-config-files/).

## Cascading merges

By default a search tier is first-hit: `conga.ConfigPaths(...)` lists files in precedence order and the first existing one wins. Declaring `conga.ConfigCascading()` changes the model to a merge: every existing file contributes, from lowest to highest precedence. Partial cascades are supported: any subset of the list may exist.

Merge semantics differ by value shape. Scalar and slice flags take the nearest file's value: a later file replaces what an earlier file set. Map flags (`map[K]V` fields, registered `RegisterMap` types, and `conga.Features[T]` gates) merge key by key across the files: files apply in lowest-to-highest precedence, a later file overrides only the keys it names, and keys set only by an earlier file survive. The `--why` report attributes each merged map key to the file that last wrote it.

Because a cascade can load more than one file, a declared root config-file flag must accept multiple values (`multi:"true"`) so it can report every merged file; the schema rejects the combination otherwise.

`conga.ConfigExplicit(...)` specifies an absolute, single configuration file target. It cannot be combined with `conga.ConfigPaths(...)` or `conga.ConfigCascading()`; attempting to combine them fails schema initialization with `ErrSchema` and is reported as a violation by `conga-lint`.

## Discovery and failure semantics

| Situation | Result |
|-----------|--------|
| CLI flag value (`--config-file`) missing | Fatal: descriptive error (`ErrConfig` base) |
| `<PREFIX>_CONFIG` environment variable pointing at a missing file | Fatal: descriptive error (`ErrConfig` base) |
| `conga.ConfigExplicit(...)` path missing | Fatal: descriptive error (`ErrConfig` base) |
| Search-tier path (`conga.ConfigPaths(...)`, built-in XDG tier) missing | Silently skipped; probing continues |
| Multiple decoder-matching candidates in one probed tier | Fatal conflict error (`ErrConfig` base) |
| Nothing resolves anywhere | Silent no-op — the run continues on defaults |
| File larger than 16 MB | Fatal: size-limit error (DoS hardening, `ErrConfig` base) |
| Malformed content | Fatal: descriptive error (`ErrConfig` base) |
| Unknown keys with strict validation | `ErrConfig` base, with did-you-mean hints on near misses |

A named path must exist; a probed search path is checked opportunistically. This matches standard command-line tools: `kubectl --kubeconfig`, `curl --config`, and `ssh -F` all fail on named paths, while search lists like `KUBECONFIG` skip missing entries. Named targets outrank the environment variable, which outranks the search tier.

## The built-in conventional tier

When the app is config-capable (at least one registered decoder and a resolvable name) and declares no search tier, CONGA probes:

```text no-copy
${XDG_CONFIG_HOME:-~/.config}/<app-name>/config.<ext>
${XDG_CONFIG_HOME:-~/.config}/<app-name>/<app-name>.<ext>
```

for each registered extension. This is the lowest-precedence tier and is replaced entirely by `conga.ConfigPaths(...)`. A `ConfigFile` container loads a file only when an invocation names it explicitly (`--config-file`, the environment variable, or `conga.ConfigExplicit(...)`) or a probed tier discovers it; [About value precedence](/explanation/value-precedence/#why-configfile-sits-outside-the-four-tier-ladder) explains why the container sits outside the four-tier ladder. The [`--why` report](/how-to/inspect-value-provenance/) and `session.CandidateConfigFiles()` list the candidates an invocation evaluates.

## Decoder capability

Resolution reads only formats the application registered: search tiers probe only registered extensions, and no wire-format library is linked unless a decoder package is. A decoderless application still surfaces candidates: the config-file flag's help text appends `(no configuration decoder registered)`, and `session.CandidateConfigFiles()` reports the environment targets. A named target whose format has no decoder is fatal. The missing-decoder rule and registration mechanics are catalogued in [package decoder](/reference/decoders/); a decoderless app leaves tier 3 inactive, as described in [About value precedence](/explanation/value-precedence/#decoderless-applications).
