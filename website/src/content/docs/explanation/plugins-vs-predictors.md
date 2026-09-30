---
title: About plugins versus predictors
description: Linker-friendly dependency boundaries, standard library type adaptation, and decoupled completion providers.
---

CONGA separates completion-capable types into two distinct layers:

- `plugin/*`: Builder plugins that adapt Go standard library types (`*time.Location`, `*net.Interface`, `syscall.Signal`, `os/user` types) to an application through `app.Use(...)`.
- `predict/<domain>`: Standalone domain types and autocompletion providers (covering network, operating system, and time zones) that operate with zero dependency on the root `conga` framework.

This separation resolves two core engineering challenges in Go CLI development: preserving small binary footprints across linker boundaries, and adapting third-party standard library types that cannot implement framework interfaces.

## Linker boundaries and binary footprint

Core CONGA packages enforce strict linker boundaries to keep minimal binaries small. In particular, core packages never import `net` or `os/user`. Importing `net` pulls the Go networking runtime, socket machinery, and resolver code into the compiled binary; importing `os/user` pulls system user database lookup and Cgo dependencies.

If core CONGA registered handlers for these packages by default, every CLI utility built with the framework would carry those dependencies, regardless of whether the utility needed them. A single-purpose file manipulation tool would link the entire networking runtime.

To prevent dependency bloat, CONGA isolates domain capabilities:

- Standard library loaders live in `plugin/network`, `plugin/system`, and `plugin/timezone`. They import `conga` to register types on `*App[T]`, but link into an application only when explicitly mounted via `app.Use(...)`.
- Predictors and domain types live in `predict/network`, `predict/system`, and `predict/timezone`. They depend exclusively on `package predict`, maintaining zero dependency on the root `conga` package.

An application that needs neither plugin nor predictor links neither into its final executable.

## Adapting standard library types via builder plugins

Go's type system forbids defining methods on types outside their declaring package. Standard library types such as `*user.User`, `*time.Location`, `*net.Interface`, and `syscall.Signal` cannot implement CONGA interfaces like `encoding.TextUnmarshaler` or `predict.Predictable`.

Because the standard library cannot know about CONGA, the framework uses an adapter mechanism. A plugin is a generic function matching the signature `func(*conga.App[T])`:

```go
func Location[T any](app *conga.App[T]) {
	app.RegisterType(time.LoadLocation, conga.WithPredictor(timezone.Timezones()))
}
```

Applying a plugin via `app.Use(...)` connects the standard library parsing function (`time.LoadLocation`, `user.Lookup`, `net.InterfaceByName`) with a domain predictor from `predict/*`. This allows command structs to declare fields using native standard library types directly:

```go
type Config struct {
	Timezone *time.Location `help:"Target time zone"`
	Signal   syscall.Signal `default:"SIGTERM" help:"Signal to send"`
}
```

The command handler receives the standard library types already parsed and validated, without requiring manual type conversions.

## Decoupled completion providers (`predict/*`)

While `plugin/*` packages adapt standard library types into CONGA's builder, packages under `predict/*` provide pure autocompletion logic.

A predictor is an evaluation function that inspects a command-line context and returns candidate strings. Predictors do not depend on reflection, struct tags, or the root `conga` package. Because of this decoupling:

- Predictors can be tested in isolation without instantiating a CLI application or building a command schema.
- Predictors can complete values in other contexts, including applications built on other CLI frameworks.
- Domain types defined in `predict/*` (such as `system.PID` or `network.Host`) implement standard interfaces like `encoding.TextUnmarshaler` and `predict.Predictable` directly. Unlike standard library types, these domain types require no plugin registration; CONGA detects their interfaces at schema build time.

## Choosing between plugins and domain types

When designing command structs, developers choose between standard library types mounted via plugins and domain types from `predict/*`:

- Standard library types via `plugin/*`: Choose this approach when application business logic already expects standard library types (`*time.Location`, `*user.User`, `syscall.Signal`). Using `app.Use(...)` ensures that command struct fields match the signatures of downstream Go functions.
- Domain types via `predict/*`: Choose this approach when you need specialized representations with self-contained validation (such as `system.PID` or `network.Host`), or when authoring reusable predictors independent of standard library structures.

For complete lists of available plugins and predictors, see [package plugin reference](/reference/plugins/), [package predict reference](/reference/predict/), and [domain types reference](/reference/domain-types/).
