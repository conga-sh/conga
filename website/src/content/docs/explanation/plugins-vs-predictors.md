---
title: About plugins versus predictors
description: Builder plugins for standard library types, decoupled domain predictors, and linker-friendly dependency boundaries.
---

CONGA separates completion-capable types into two decoupled layers:

- `plugin/*`: builder plugins that mount standard library types (`*time.Location`, `syscall.Signal`, `os/user` types) onto an application via `app.Use(...)`. Each plugin registers a type loader and predictor for a standard library type.
- `predict/<domain>`: standalone domain types and predictors (covering network, operating system, and time zones) with zero dependency on the root `conga` framework.

This page explains why these systems are decoupled, how they preserve linker boundaries, and how plugins and domain predictors interact with the engine.

## Linker-friendly dependency boundaries

Core CONGA packages enforce strict linker boundaries to keep minimal binaries small. In particular, core packages never import `net` (which pulls networking runtime and name resolution into the binary) or `os/user` (which pulls system user lookup and Cgo dependencies).

If core CONGA imported these packages to provide built-in completion or parsing, every CLI application built with the framework would carry those dependencies, whether or not it needed them.

To preserve a trim footprint, CONGA isolates these capabilities:

1. **Standard library type loaders** live in `plugin/network`, `plugin/system`, and `plugin/timezone`. They import `conga` to register types via `app.RegisterType`, but link into an application only when mounted via `app.Use(...)`.
2. **Domain types and predictors** live in `predict/network`, `predict/system`, and `predict/timezone`. They depend exclusively on `package predict`, with zero dependency on the root `conga` package.

## Builder plugins (`plugin/*`)

Plugins are functions of type `func(*conga.App[T])`, applied with `app.Use`:

```go
import (
	"github.com/conga-sh/conga/plugin/network"
	"github.com/conga-sh/conga/plugin/system"
	"github.com/conga-sh/conga/plugin/timezone"
)

app := conga.New[CLI]().
	Name("opsctl").
	Use(system.User, system.Group, system.Signal).
	Use(timezone.Location).
	Use(network.Interface)
```

| Plugin | Mounts stdlib type |
|--------|--------------------|
| `plugin/system.User` | `*user.User` (via `system.User` domain type) |
| `plugin/system.Group` | `*user.Group` |
| `plugin/system.Signal` | `syscall.Signal` |
| `plugin/timezone.Location` | `*time.Location` |
| `plugin/network.Interface` | `network.NetInterface` |

Plugins build on `app.RegisterType` — the same primitive used for custom application types ([How to add custom flag types](/how-to/add-custom-flag-types/)).

## Operating system domain (`predict/system`)

The `predict/system` package provides typed representations for operating system resources alongside corresponding shell completion providers.

The package exposes domain types for `system.User`, `system.Group`, `system.Signal`, `system.PID`, `system.Process`, `system.EnvVar`, and `system.Locale`. Each type pairs with an autocompletion predictor: `system.Users()`, `system.Groups()`, `system.Signals()`, `system.ProcessIDs()`, `system.ProcessNames()`, `system.EnvVars()`, and `system.Locales()`.

Two types pair with constructors that add a noun rather than pluralising the type name: `system.PID` pairs with `ProcessIDs()` and `system.Process` (which stores an executable name) pairs with `ProcessNames()`.

```go
type Kill struct {
	Target  system.PID    `help:"Process to signal"`
	Signal_ system.Signal `default:"SIGTERM" help:"Signal to send"`
}
```

## Network and web domain (`predict/network`)

The `predict/network` package provides types and predictors for network addresses, protocols, and HTTP values.

Domain types include `network.Host`, `network.Port`, `network.HTTPMethod`, `network.HTTPStatus`, `network.MIMEType`, and `network.NetInterface`. Corresponding predictors include `network.Hosts()`, `network.Ports()`, `network.HTTPMethods()`, `network.HTTPStatusCodes()`, `network.MIMETypes()`, and `network.NetInterfaces()`.

```go
type Curl struct {
	URL    network.Host       `help:"Target host"`
	Method network.HTTPMethod `default:"GET" help:"HTTP method"`
	Status network.HTTPStatus `help:"Expected status code"`
}
```

## Time zones (`predict/timezone`)

The `timezone.Timezones()` predictor completes IANA time zone identifiers derived from the standard library `time` package database:

```go
app.Predictor("timezone", timezone.Timezones)
```

Because domain predictor packages depend only on `predict`, they can also be used in applications built on other CLI frameworks by invoking them inside a `predict.Callback`.
