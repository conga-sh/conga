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
- `task lint` runs golangci-lint with `--fix` (it rewrites files) over the
  core packages, examples, and integration suites, plus the `conga-lint`
  schema analyzer.
- `task test:all` additionally runs the shell-completion and binary-size
  integration suites.

These gates are hardening-phase requirements. Iterate during prototyping with
`task proto -- ./path/to/pkg/...` (compile plus focused tests), then run
`task harden` (auto-fix lint, declaration reordering, and the full CI gate)
before opening the pull request; see [AGENTS.md](AGENTS.md) §1.

## Declaration Ordering

Every non-test Go source file follows top-down public-to-private declaration
ordering (public API first, then the public/private boundary, then internal
helpers). `*_test.go` files are exempt, and `conga-reorder` skips them.
The full ordering hierarchy is documented in [AGENTS.md](AGENTS.md) §4.

## Pull Requests

- **One behaviour per pull request.** Keep changes focused and independently
  reviewable; unrelated cleanups belong in their own PR.
- **Tests follow [docs/TESTING.md](docs/TESTING.md).** Every test asserts a
  single scenario, uses `github.com/alecthomas/assert/v2`, and runs with
  `t.Parallel()` unless it mutates process-wide state.
- **Update documentation** for any user-visible API or behaviour change, and
  keep [AGENTS.md](AGENTS.md) in sync with architectural decisions.

## Releases

Releases are cut from conventional commits on `main` through Release Please.
See [RELEASING.md](RELEASING.md) for the version policy, tag format, and release
checklist.
