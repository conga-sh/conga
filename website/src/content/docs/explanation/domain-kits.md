---
title: About domain packages
description: Linker-friendly dependency boundaries, topical domain packages, and the decoupled completion SDK.
---

CONGA packages completion-capable domain types and predictors into three modular domain packages: `network`, `system`, and `timezone`. The `network` and `system` packages bundle domain value types alongside standard library bindings, while `timezone` provides an autocompletion predictor and a binding for `*time.Location`. This layout satisfies two engineering requirements: keeping minimal binaries small across linker boundaries, and adapting standard library types that cannot implement framework interfaces.

## Linker boundaries and binary footprint

Core CONGA packages never import `net` or `os/user`. Importing `net` pulls the Go networking runtime, socket machinery, and resolver code into the compiled binary; importing `os/user` pulls system user database lookup and Cgo dependencies.

If the root package registered handlers for these domains, every CLI built with the framework would carry those dependencies regardless of need; a file manipulation tool would link the networking runtime.

These packages isolate external domain dependencies behind explicit imports, so applications link only the packages they import.

## Two ways to declare a domain field

Each domain package provides value types and standard library bindings for its domain.

- Domain types (`network.Host`, `system.User`, `system.Signal`, and the rest) implement `encoding.TextUnmarshaler` and `predict.Predictable` directly. Declaring a field with one enables automatic coercion and autocompletion through interface detection at schema build.
- The `Register*` bindings adapt Go standard library types (`*net.Interface`, `*user.User`, `syscall.Signal`, `*time.Location`) to CONGA's parsing and completion interfaces. Each calls `App.RegisterType` with a parser and a predictor, mounting via `app.Use(...)`. Command fields then use the native standard library type.

```go
type Config struct {
	Timezone *time.Location `help:"Target time zone"`
	Signal   syscall.Signal `default:"SIGTERM" help:"Signal to send"`
}

app := conga.New[Config]().
	Use(timezone.RegisterLocation, system.RegisterSignal)
```

Choose the standard library binding when business logic already expects the standard library type (`*time.Location`, `*user.User`, `syscall.Signal`); choose the domain type when the command needs a self-contained representation with its own validation (`system.PID`, `network.Host`).

## The completion SDK

`package predict` is the pure, decoupled autocompletion SDK. A predictor is an evaluation function that inspects a command-line context and returns candidate strings independently of reflection, struct tags, or the root `conga` package. Predictors can be tested in isolation and reused in other contexts, and the domain predictors in `network`, `system`, and `timezone` are built on this SDK.

These packages import `conga` only for their `Register*` bindings; their value types and predictors depend solely on `package predict`.

For the type inventory and the bindings, see [Domain types](/reference/domain-types/); for the combinator surface, see [package predict](/reference/predict/).
