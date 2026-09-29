---
title: How to manage config files
description: Register decoders, configure search paths, enable cascading file merges, and reload config files.
---

Config files in CONGA are opt-in. An application registers file decoders for desired formats, then configures resolution rules using `app.ConfigOptions(...)`. This guide explains format registration, search-tier probing, cascading merges, key mapping, and runtime reloading.

## Register configuration formats

Register decoders on the application builder for each file format the application accepts:

```go title="main.go"
import (
	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/config"
	"github.com/conga-sh/conga/decoder/json"
	"github.com/conga-sh/conga/decoder/toml"
	"github.com/conga-sh/conga/decoder/yaml"
)

func main() {
	app := conga.New[CLI]().
		Name("cloudctl").
		RegisterDecoder("json", json.RawDecoder).
		RegisterDecoder("toml", toml.RawDecoder).
		RegisterDecoder("yaml", yaml.RawDecoder)

	app.FatalIfError(app.Run())
}
```

Standard decoder packages provide `RawDecoder` instances for JSON, TOML, YAML, and HCL. Registering `"yaml"` automatically matches files with `.yaml` and `.yml` extensions.

## Configure resolution tiers and search paths

Configure file discovery using `app.ConfigOptions(...)`:

```go title="main.go"
app.ConfigOptions(
	config.Paths(
		"./cloudctl.yaml",                  // local: highest precedence
		"~/.config/cloudctl/cloudctl.yaml", // user
		"/etc/cloudctl/cloudctl.yaml",      // system: lowest precedence
	),
)
```

CONGA supports three config file discovery tiers:

- **Built-in conventional tier (XDG)**: If decoders are registered and an application name is configured, CONGA automatically searches `${XDG_CONFIG_HOME:-~/.config}/<app-name>/config.<ext>` and `<app-name>.<ext>` as a fallback tier.
- **Custom search tier (`config.Paths`)**: Overrides the built-in XDG tier. In standard single-file mode, CONGA loads the **first existing file** in the list and ignores the remaining paths.
- **Explicit path (`config.Explicit`)**: Designates a specific file with fatal-if-missing semantics. It outranks environment variables and search paths, bypassing conventional tiers.

## Merge multiple files with cascading

To merge settings across system, user, and project files, add `config.Cascading()` to the options:

```go title="main.go"
app.ConfigOptions(
	config.Paths(
		"./cloudctl.yaml",
		"~/.config/cloudctl/cloudctl.yaml",
		"/etc/cloudctl/cloudctl.yaml",
	),
	config.Cascading(),
)
```

In cascading mode, CONGA reads **every existing file** in the list from lowest to highest precedence (`/etc` -> `~/.config` -> `./`). Higher-precedence files override individual keys defined in earlier files.

When `config.Cascading()` is declared, any root `conga.ConfigFile` container on the command struct must accept multiple values by declaring `multi:"true"`:

```go title="main.go"
type CLI struct {
	Config conga.ConfigFile `multi:"true" help:"Configuration files to load"`
}
```

Omitting `multi:"true"` on a root `ConfigFile` flag when cascading is active produces an `ErrSchema` error during schema validation.

## Accept config files on the command line

To let users specify custom config files during invocation, declare a `conga.ConfigFile` container:

```go title="main.go"
type CLI struct {
	ConfigFile conga.ConfigFile `help:"Custom config file path" short:"c"`
}
```

Files supplied via `--config-file` or `-c` take precedence over environment variables, search paths, and conventional XDG tiers.

## Map fields to configuration keys

By default, config keys match the flag's kebab-case field name. Use struct tags to customise mapping:

```go title="main.go"
type Database struct {
	URL      conga.RequiredFlag[string] `config:"connection_url" help:"Database connection string"`
	Password conga.OptionalFlag[string] `config:"-" help:"Never read password from config files"`
}
```

- `config:"connection_url"`: Reads the value from the specified key in the config file.
- `config:"-"`: Disables config file resolution for that field, requiring it to be set via CLI, environment variable, or default value.

Files are subject to a strict 16 MB maximum size limit to protect against unbounded allocations.

## Inspect and reload config files at runtime

Access active file paths and trigger reloads using the session:

```go title="main.go"
func (c *CLI) Run(ctx context.Context, session *conga.Session) error {
	// Candidate paths evaluated during resolution
	candidates := session.CandidateConfigFiles()

	// Paths of files that were actually found and decoded
	loaded := session.LoadedConfigFiles()

	// Re-read and re-validate configuration from disk
	freshCLI, err := session.Reload[CLI](ctx)
	if err != nil {
		return fmt.Errorf("reloading configuration: %w", err)
	}

	return nil
}
```

`session.Reload[T](ctx)` executes the entire resolution ladder again, decodes existing files, and runs validation hooks, returning a freshly bound `*T`.

## Where to go next

- [How to hot-reload configuration](/how-to/hot-reload-config/) — live filesystem watching with `package watch`.
- [About the config cascade](/explanation/config-cascade/) — search-tier mechanics, merging order, and XDG resolution.
- [Package decoder reference](/reference/decoders/) — decoders for JSON, TOML, YAML, and HCL.
