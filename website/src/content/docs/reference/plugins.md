---
title: package plugin
description: Builder plugins mounting standard library types onto the app.
---

Deep detail lives on [pkg.go.dev](https://pkg.go.dev/github.com/conga-sh/conga/plugin); the conceptual distinction is in [About plugins vs predictors](../explanation/plugins-vs-predictors/).

## Loader functions

Each function is a plugin `func(*conga.App[T])` — apply with `app.Use(...)`:

| Plugin | Package | Registers |
|--------|---------|-----------|
| `system.User` | `plugin/system` | OS user lookup type |
| `system.Group` | `plugin/system` | OS group lookup type |
| `system.Signal` | `plugin/system` | `syscall.Signal` parsing + completion |
| `timezone.Location` | `plugin/timezone` | `*time.Location` parsing + IANA completion |
| `network.Interface` | `plugin/network` | Network interface type |

```go
app := conga.New[CLI]().Name("opsctl").Use(system.Signal, timezone.Location)
```

## Plugin contract

A plugin is any function that mutates the `*App[T]` builder — typically by calling `app.RegisterType` to bind a parser (and optionally `conga.WithPredictor`) for a type:

```go
func Signal[T any](app *conga.App[T]) {
	app.RegisterType(parseSignal, conga.WithPredictor(system.Signals()))
}
```

Write your own the same way to ship reusable type support in libraries.
