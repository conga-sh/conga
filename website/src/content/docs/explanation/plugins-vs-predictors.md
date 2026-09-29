---
title: About plugins vs predictors
description: plugin/* loaders for standard library types and predict/network, predict/system, predict/timezone domain packages.
---

CONGA splits "completion-capable types" into two families:

- **`plugin/*`** — builder plugins that mount **standard library types** (`*time.Location`, `syscall.Signal`, `os/user` types) onto your app via `app.Use(...)`. Each plugin registers a parser (and predictor) for a stdlib type.
- **`predict/<domain>`** — pure, self-contained **domain types and predictors** (network, operating system, time zones) with zero dependency on the root framework.

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

Plugins are built on `app.RegisterType` — the same primitive you can use for your own types ([How to add custom types and decoders](/how-to/custom-types-and-decoders/)).

## `predict/system` — operating system domain

Types: `system.User`, `system.Group`, `system.Signal`, `system.PID`, `system.Process`, `system.EnvVar`, `system.Locale`.

Predictors: `system.Users()`, `system.Groups()`, `system.Signals()`, `system.ProcessIDs()`, `system.ProcessNames()`, `system.EnvVars()`, `system.Locales()`.

Two types pair with constructors that add a noun rather than pluralizing the type name: `system.PID` pairs with `ProcessIDs()` and `system.Process` (which stores an executable name) pairs with `ProcessNames()`.

```go
type Kill struct {
	Target  system.PID    `help:"Process to signal"`
	Signal_ system.Signal `default:"SIGTERM" help:"Signal to send"`
}
```

## `predict/network` — network and web domain

Types: `network.Host`, `network.Port`, `network.HTTPMethod`, `network.HTTPStatus`, `network.MIMEType`, `network.NetInterface`.

Predictors: `network.Hosts()`, `network.Ports()`, `network.HTTPMethods()`, `network.HTTPStatusCodes()`, `network.MIMETypes()`, `network.NetInterfaces()`.

```go
type Curl struct {
	URL    network.Host    `help:"Target host"`
	Method network.HTTPMethod `default:"GET" help:"HTTP method"`
	Status network.HTTPStatus `help:"Expected status code"`
}
```

## `predict/timezone` — time zones

`timezone.Timezones()` completes IANA time zone identifiers (from the standard `time` package data):

```go
app.Predictor("timezone", timezone.Timezones)
```

:::note
Domain predictor packages depend only on `predict` — you can use them in apps built on other CLI frameworks by invoking them inside a `predict.Callback`.
:::
