# Contributing to CONGA

Thanks for your interest in improving CONGA. This guide covers the local
workflow; the authoritative coding standards live in [AGENTS.md](AGENTS.md).

## Prerequisites

- **Go 1.27 or newer.** The module targets `go 1.27` and uses current generics
  and standard-library APIs.

## Build & Verify

Run these from the repository root before opening a pull request:

```sh
task test
```

```sh
task lint
```

```sh
task test:all
```

- `task test` runs the unit suite with the race detector enabled.
- `task lint` runs `golangci-lint run --fix ./...` plus the `conga-lint` schema
  analyzer.
- `task test:all` additionally runs the shell-completion and binary-size
  integration suites.

## Declaration Ordering

Every Go source file follows top-down public-to-private declaration ordering
(public API first, then the public/private boundary, then internal helpers).
The full ordering hierarchy and its rationale are documented in
[AGENTS.md](AGENTS.md) §1.

## Pull Requests

- **One behaviour per pull request.** Keep changes focused and independently
  reviewable; unrelated cleanups belong in their own PR.
- **Tests follow [docs/TESTING.md](docs/TESTING.md).** Every test asserts a
  single scenario, uses `github.com/alecthomas/assert/v2`, and runs with
  `t.Parallel()` unless it mutates process-wide state.
- **Update documentation** for any user-visible API or behaviour change, and
  keep [AGENTS.md](AGENTS.md) in sync with architectural decisions.
