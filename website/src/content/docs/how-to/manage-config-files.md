---
title: How to manage config files
description: Register decoders, declare search paths, cascade tiers, use profiles, and reload configuration at runtime.
---

Config files are opt-in: register a decoder for each format you use, then declare how files are found with `app.ConfigOptions(...)`. The resolution rules — existence semantics, the built-in XDG tier, and cascading — are explained in [About the config cascade](../explanation/config-cascade/).

## Register a configuration format

```go
import (
	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/config"
	"github.com/conga-sh/conga/decoder/json"
	"github.com/conga-sh/conga/decoder/yaml"
	"github.com/conga-sh/conga/decoder/toml"
	"github.com/conga-sh/conga/decoder/hcl"
)

func main() {
	app := conga.New[CLI]().
		Name("cloudctl").
		RegisterDecoder("json", json.RawDecoder).
		RegisterDecoder("yaml", yaml.RawDecoder).
		RegisterDecoder("toml", toml.RawDecoder).
		RegisterDecoder("hcl", hcl.RawDecoder)
	...
}
```

Register additional formats with `app.RegisterDecoder(extension, decoder)`; the `config.Decoder` signature and built-in sentinels are catalogued in [package decoder](../reference/decoders/).

## Declare where configuration lives

One method — `app.ConfigOptions(...)` — declares the entire resolution with composable options:

```go
app.ConfigOptions(
	config.Paths(
		"./cloudctl.yaml",                  // local — highest precedence
		"~/.config/cloudctl/cloudctl.yaml", // user
		"/etc/cloudctl/cloudctl.yaml",      // system — lowest precedence
	),
	config.Cascading(),
)
```

- `config.Explicit(path)` names one concrete file with **fatal-if-missing** semantics; it outranks the configuration environment variable and the search tier, and bypasses the built-in conventional tier.
- `config.Paths(paths...)` declares the author search tier in precedence order (highest first). The **first existing file** is used; the rest are ignored — unless `config.Cascading()` is also declared. Declaring paths replaces the built-in conventional tier.
- `config.Cascading()` merges **all existing files** from lowest to highest precedence, so higher-precedence paths override earlier keys. Any subset of the list may exist. The declared root config-file flag must accept multiple values (`multi:"true"`) so it can report every merged file; the schema rejects the combination otherwise.

Options compose and are independent of call order; repeating an option replaces its slot.

## Merge a base file with profiles

Multi-file merging is also available as an explicit flag via the `multi:"true"` tag on the `conga.ConfigFile` container — a base file plus `--config-file` overrides (base + profile pattern):

```go
type CLI struct {
	Config conga.ConfigFile `multi:"true" help:"Config files, later files override earlier"`
}
```

## Inspect and reload at runtime

```go
candidates := session.CandidateConfigFiles() // evaluated for this execution
loaded := session.LoadedConfigFiles()        // actually loaded

fresh, err := session.Reload[CLI](ctx) // re-reads and re-validates the whole config
```

`session.Reload[T]` returns a freshly bound `*T` — the foundation for [How to hot-reload configuration](hot-reload-config/).
