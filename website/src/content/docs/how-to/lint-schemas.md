---
title: How to lint CONGA schemas
description: Catch leaf Runner violations, struct tag typos, and ancestor flag shadowing in CI with conga-lint.
---

`conga-lint` is a static analysis pass that mirrors the runtime schema contract on Go type information, so contract violations surface before any binary runs. It ships with the module as a Go tool and requires a **Go 1.27+ toolchain** to parse CONGA schemas, which use method-level type parameters (`func (a *App[T]) RegisterType[U any](...)`).

## Run the analyzer

Add the analyzer as a tool dependency of your module, then run it over your packages:

```sh
go get -tool github.com/conga-sh/conga/cmd/conga-lint
go tool conga-lint ./...
```

Inside the CONGA repository, the `lint:schema` task wires it into the default lint pipeline:

```sh
task lint:schema
# equivalent to:
go tool conga-lint ./... ./_examples/...
```

## What it checks

| Check | Mirrors |
|-------|---------|
| Leaf command Runner enforcement | The bind-time `noRunnerError`: a command struct without `Command[T]` fields must implement `Run(context.Context, *conga.Session) error` |
| Struct tag linting | Tag syntax, `required:"true"` rejection, `help` on `Command[T]`, directive validity, predictor vocabulary, `range`/`len`/`items` bounds, and — under strict validation — unknown tag keys and unregistered `predictor:"..."` names |
| Ancestor flag shadowing | The runtime `ErrDuplicateFlag` rule: a flag name, alias, or short rune declared on an ancestor cannot be redeclared on a descendant |
| Cascading guard | The schema rejection of `config.Cascading()` without a `multi:"true"` root `ConfigFile` flag |
| Throwaway root detection | Lint-only: flags `conga.New(root)` when the root instance is never referenced outside the app, steering authors to the engine-allocating `conga.New[T]()` form. Caller-owned instances (seeded before or read after execution) and multi-root panic guards are left alone; unlike anchor discovery, this check also covers `_test.go` files |

All checks delegate to the exported validators in `internal/spec`, so lint-time and runtime validation share one source of truth and produce the same messages.

## The discovery contract

The analyzer finds hierarchies exclusively from `conga.New` anchors — both the explicit `conga.New[T]()` and the inferred `conga.New(root)` forms — then walks the `Command[T]` fields of the root type. Strict-mode settings are reconstructed from literal builder arguments:

- `app.StrictTagValidation(true)` / `app.StrictTagValidation(false)`
- `app.AllowTags("...", ...)`
- `app.Predictor("name", ...)`

Non-literal (computed) arguments leave the evidence ambiguous; the analyzer then degrades conservatively to the runtime default. It may miss diagnostics, but it never invents them. To force strict validation regardless of detected calls, pass `-strict`:

```sh
go tool conga-lint -strict ./...
```

## Relationship to runtime errors

The runtime remains authoritative: schema build still returns self-describing `ErrSchema` errors from `App.Run` for every violation, and lint is a fast, editor- and CI-friendly mirror. Because both consult the same validators in `internal/spec`, anything lint reports is a genuine schema error — but a clean lint run does not waive the runtime check. See [Errors & Exit Codes](../reference/errors/) for the error categories and [Struct Tag Reference](../reference/struct-tags/) for the tag vocabulary.
