---
title: About the shell completion architecture
description: The native driver, the wire protocol, and the decoupled predict SDK.
---

Completion is served end-to-end by CONGA: there is no third-party shell driver, and dynamic candidates travel over a documented wire protocol. The user-facing steps are in [How to enable shell completion](../how-to/enable-shell-completion/).

## Native driver

- The driver lives in `internal/completion` with one package per shell plus an internal JSON export framing (`export`) and an RFC 8259-compliant string escaper (`wire`, delegating to `internal/jsonwire`, which the version report shares); `encoding/json` is banned from the driver.
- Scripts are generated from the same schema that powers parsing and help, so completions stay in sync with the command tree by construction.
- Dynamic candidate generation runs through a documented wire protocol (`<cmd> __complete <shell> <words...>` with `CONGA_*` sideband variables). The full protocol — entry convention, sideband variables, directives, style vocabulary, and per-shell framing — is specified in [`docs/completion-wire-protocol.md`](https://github.com/conga-sh/conga/blob/main/docs/completion-wire-protocol.md) in the repository.

## Decoupled predictor SDK

The `predict` package is decoupled from the root `conga` package: external consumers can use it as a standalone SDK, and the `predict/<domain>` packages (`network`, `system`, `timezone`) depend only on `predict`. See [package predict](../reference/predict/) and [About plugins vs predictors](plugins-vs-predictors/).
