---
title: package config
description: Config-file authoring surface — resolution options and decoder registration.
---

Deep detail lives on [pkg.go.dev/github.com/conga-sh/conga/config](https://pkg.go.dev/github.com/conga-sh/conga/config); workflow guidance is in [How to manage config files](/how-to/manage-config-files/) and the rationale in [About the config cascade](/explanation/config-cascade/).

## Options

```go
func Paths(paths ...string) config.Option
func Explicit(path string) config.Option
func Cascading() config.Option
```

Pass options to `app.ConfigOptions(...)`; they compose and are independent of call order. `config.Option` is a sealed interface: options are produced exclusively by the constructors above and cannot be implemented by other packages.

For example:

```go
app.ConfigOptions(
	config.Paths("./cloudctl.yaml", "~/.config/cloudctl/cloudctl.yaml"),
	config.Cascading(),
)
```

| Option | Effect |
|--------|--------|
| `config.Paths(paths...)` | Declares the author search tier in precedence order; the first existing file wins unless cascading. Replaces the built-in conventional tier. Panics on a zero-argument or blank-path call. |
| `config.Explicit(path)` | Names one concrete file with fatal-if-missing semantics; outranks the environment variable and the search tier. Panics on a blank path. |
| `config.Cascading()` | Merges every existing tier file, lowest to highest precedence. Requires `multi:"true"` on the root `ConfigFile` flag. |

`config.Apply` resolves options into a `config.Settings` snapshot. It is engine-facing: `app.ConfigOptions(...)` calls it and converts the snapshot into the engine's own settings, so application code never needs it directly.

## Decoders

```go
type Decoder func(data []byte) (map[string]any, error)
```

Register decoders per application with `app.RegisterDecoder(extension, decoder)`. See [Decoders](/reference/decoders/).
