---
title: About the shell completion architecture
description: How native shell completion works without third-party drivers, the wire protocol, init-free linking, and predictor isolation.
---

CONGA implements shell completion natively through dedicated drivers for six shells (bash, zsh, fish, powershell, elvish, and nushell). Completion scripts derive directly from the application schema, and dynamic completion candidates travel over a standardised wire protocol. Procedural setup instructions live in [How to enable shell completion](/how-to/enable-shell-completion/).

## Native drivers derived from schema

Many command-line frameworks rely on external completion generators or generic bash-compatibility wrappers. CONGA replaces these with six built-in shell drivers housed in `internal/completion`.

Because registration scripts are generated from the same Go struct schema that powers CLI flag parsing and help screens, completions stay synchronised with the command hierarchy by definition:

- Subcommand hierarchies, flag names, aliases, and short runes are derived directly from struct tags and field declarations.
- Enum constraints (`enum:"a,b,c"`) complete their permitted values without requiring custom callback scripts.
- Mutually exclusive flags (`xor:"group"`) hide or suppress invalid completions once a conflicting flag appears on the command line.

## The wire protocol (`__complete`)

Dynamic completion relies on a two-party protocol between the shell registration script and the hidden `__complete` entry point inside the application binary:

```text no-copy
<cmd> __complete <shell> <words...>
```

1. **The shell snippet**: A shell-native script emitted by `app completion <shell>` intercepts the tab-completion event in the host shell, gathers current input tokens, and invokes the binary via `__complete`.
2. **The entry point**: The application binary parses input tokens with `spec.Complete`, evaluates active predictors, and outputs candidate suggestions formatted for the requesting shell.

The wire protocol handles shell-specific tokenisation and escaping before invoking Go completion routines:

- All tokens preceding the cursor arrive fully resolved, while the final token is evaluated as the prefix to complete.
- Shell-specific quirks—such as readline word breaks in bash, span arrays in nushell, or quoting state in zsh—are normalised by the driver before candidate filtering.
- Sideband environment variables (`CONGA_WIDTH`, `CONGA_THEME`) communicate terminal dimensions and palette styling without altering command-line arguments.

When invoked without a known shell identifier, the entry point falls back to the canonical `export` JSON framing, providing a machine-readable stream for automated testing and external tooling.

## Init-free subsystems and linker trimming

Shell completion adds roughly 409 KB to binary size across its six drivers and predictor evaluation graph. An application that does not enable completion should not pay this cost.

As explained in [About binary size and linking](/explanation/binary-size/), the Go linker eliminates dead code at function granularity, but dynamic package-level initialisers emit compiler `..inittask` records that prevent the linker from discarding unreferenced packages.

To ensure that the Go linker can eliminate completion code completely when `app.EnableCompletion()` is omitted, the `internal/completion` packages adhere to an **init-free** contract:

- No `func init()` functions exist anywhere in the completion drivers.
- No package-level variables execute dynamic initialisation expressions (such as `errors.New`, map literals, or function calls).
- Error sentinels reside in `internal/sentinel`, an always-linked leaf package where initialisation overhead is paid once across the entire binary.
- Driver state relies on statically initialised literals or `sync.Once` accessors.

An automated test (`task test:size`) verifies that binaries built without `EnableCompletion()` contain zero completion symbols and zero dynamic initialisation tasks.

## Linker footprint: exclusion of encoding/json

Standard library `encoding/json` relies heavily on reflection, type descriptor registries, and dynamic map allocations. Importing `encoding/json` in completion drivers would drag these dependencies into every binary that enables completion, increasing binary size.

Instead, the completion wire protocol uses a specialised RFC 8259-compliant string escaper implemented in `internal/jsonwire`. This internal escaper performs allocation-free string escaping without linking reflection or formatting subsystems. Both completion export framing and `--version` JSON reporting share this escaper.

## Decoupled predictor SDK

The autocompletion engine separates candidate prediction from command evaluation.

The `predict` package defines prediction interfaces, filtering primitives, and callback types with zero dependency on root `conga`. Domain-specific predictor packages (`predict/network`, `predict/system`, `predict/timezone`) depend strictly on `predict`.

This decoupling yields two architectural benefits:

1. **Standalone predictability**: External packages or non-CONGA tools can use domain predictors directly by invoking them inside a `predict.Callback`.
2. **Dependency isolation**: Domain packages that interact with system lookups or network types remain outside core packages, preventing unwanted dependencies from entering minimal application binaries. For details on how domain types interact with builder plugins, see [About plugins versus predictors](/explanation/plugins-vs-predictors/).
