---
title: About domain packages
description: Why the network, system, and timezone packages live outside core CONGA, how they adapt standard library types, and why the predict package stands alone.
---

CONGA packages completion-capable domain types and predictors into three modular domain packages, `network`, `system`, and `timezone`. The `network` and `system` packages bundle domain value types alongside standard library bindings, while `timezone` provides an autocompletion predictor and a binding for `*time.Location`. The split serves two purposes. It keeps binaries that don't use these types small, and it adapts standard library types that can't implement CONGA's interfaces themselves.

## Linker boundaries and binary footprint

Core CONGA packages never import `net` or `os/user`. Importing `net` pulls the Go networking runtime, socket machinery, and resolver code into the compiled binary. Importing `os/user` pulls system user database lookup and Cgo dependencies.

If the root package registered handlers for these domains, every CLI built with the framework would carry those dependencies regardless of need, and a file manipulation tool would link the networking runtime.

These packages isolate external domain dependencies behind explicit imports, so applications link only the packages they import.

## Two ways to declare a domain field

Each domain package provides value types and standard library bindings for its domain.

- Domain types (`network.Host`, `system.User`, `system.Signal`, and the rest) implement `encoding.TextUnmarshaler` and `predict.Predictable` directly. Declaring a field with one enables automatic coercion and autocompletion through interface detection at schema build.
- The `Register*` bindings adapt Go standard library types (`*net.Interface`, `*user.User`, `syscall.Signal`, `*time.Location`) to CONGA's parsing and completion interfaces. Each calls `App.RegisterType` with a parser and a predictor, mounting via `app.Use(...)`. Command fields then hold the standard library type in a container, such as `conga.OptionalFlag[*time.Location]`.

```go
type Config struct {
	Timezone conga.OptionalFlag[*time.Location] `help:"Target time zone"`
	Signal   conga.OptionalFlag[syscall.Signal] `default:"TERM" help:"Signal to send"`
}

app := conga.New[Config]().
	Use(timezone.RegisterLocation, system.RegisterSignal)
```

Choose the standard library binding when business logic already expects the standard library type (`*time.Location`, `*user.User`, `syscall.Signal`). Choose the domain type when the command needs a self-contained representation with its own validation (`system.PID`, `network.Host`).

## The completion SDK

`package predict` is the pure, decoupled autocompletion SDK. A predictor is an evaluation function that inspects a command-line context and returns candidate strings independently of reflection, struct tags, or the root `conga` package. Predictors can be tested in isolation and reused in other contexts, and the domain predictors in `network`, `system`, and `timezone` are built on this SDK.

These packages import `conga` for their `Register*` bindings and for compile-time checks that their types satisfy `conga.HelpProvider` and `conga.Predictable`. The types' methods and predictors call only `package predict`.

For the type inventory and the bindings, see [Domain types](/reference/domain-types/). For the combinators, see [package predict](/reference/predict/).
