---
title: How to manage config files
description: Register decoders, configure search paths, enable cascading file merges, and inspect config files at runtime.
---

Config files in CONGA are opt-in. An application registers file decoders for desired formats, then configures resolution rules using `app.ConfigOptions(...)`. This guide explains format registration, search-tier probing, cascading merges, key mapping, and runtime inspection.

## Register configuration formats

Register decoders on the application builder for each file format the application accepts:

```go title="main.go"
package main

import (
	"context"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/decoder/hcl"
	"github.com/conga-sh/conga/decoder/json"
	"github.com/conga-sh/conga/decoder/toml"
	"github.com/conga-sh/conga/decoder/yaml"
)

type CLI struct {
	Port conga.OptionalFlag[int] `default:"8080" help:"Listen port"`
}

func (c *CLI) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

func main() {
	app := conga.New[CLI]().
		RegisterDecoder(".json", json.Decode).
		RegisterDecoder(".toml", toml.Decode).
		RegisterDecoder(".yaml", yaml.Decode).
		RegisterDecoder(".yml", yaml.Decode).
		RegisterDecoder(".hcl", hcl.Decode)

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
}
```

CONGA provides four opt-in decoder packages: JSON, TOML, YAML, and HCL. Each package exposes a `Decode` function matching the `conga.ConfigDecoder` func type; [package decoder](/reference/decoders/) covers extension normalization and the missing-decoder rule.

## Implement and register a custom decoder

To support custom, proprietary, or legacy formats, provide a function with the `conga.ConfigDecoder` signature, `func(data []byte) (map[string]any, error)` (see [package decoder](/reference/decoders/)). The decoder receives the raw config file bytes and parses them into a generic `map[string]any`. Keys in the map correspond to flag names (or custom `config:"..."` tags), and nested `map[string]any` values correspond to subcommands or native Go map flags.

```go title="main.go"
import (
	"bufio"
	"bytes"
	"fmt"
	"strings"
)

// iniDecoder parses simple key=value lines into a configuration map.
func iniDecoder(data []byte) (map[string]any, error) {
	result := make(map[string]any)
	scanner := bufio.NewScanner(bytes.NewReader(data))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		key, value, found := strings.Cut(line, "=")
		if !found {
			return nil, fmt.Errorf("malformed line: %q", line)
		}

		result[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return result, nil
}
```

Register the custom decoder on the application builder:

```go title="main.go"
app.RegisterDecoder(".ini", iniDecoder)
```

When a decoder returns a non-nil error, CONGA aborts configuration resolution and returns a categorized `conga.ErrConfig` error identifying the file and parse failure.

## Structure nested configuration for subcommands

When command structs declare nested subcommands using `conga.Command[T]`, the command hierarchy forms nested configuration namespaces; [About subcommands and hierarchy](/explanation/subcommands/#hierarchical-configuration-mapping) defines the mapping.

```go title="main.go"
type RootCLI struct {
	Verbose conga.OptionalFlag[bool] `help:"Global verbose logging" short:"v"`
	Server  conga.Command[ServerCmd] `summary:"Server settings"`
}

type ServerCmd struct {
	Port     conga.OptionalFlag[int]    `default:"8080" help:"Listen port"`
	Database conga.Command[DatabaseCmd] `summary:"Database settings"`
}

type DatabaseCmd struct {
	Host     conga.OptionalFlag[string] `default:"localhost" help:"Database host"`
	PoolSize conga.OptionalFlag[int]    `default:"10" help:"Connection pool size"`
}
```

A YAML file for the same schema nests subcommand keys under their command names:

```yaml title="config.yaml"
verbose: true

server:
  port: 9000
  database:
    host: "db.internal"
    pool-size: 20
```

## Configure resolution tiers and search paths

Configure file discovery using `app.ConfigOptions(...)`:

```go title="main.go"
app.ConfigOptions(
	conga.ConfigPaths(
		"./cloudctl.yaml",                  // local: highest precedence
		"~/.config/cloudctl/cloudctl.yaml", // user
		"/etc/cloudctl/cloudctl.yaml",      // system: lowest precedence
	),
)
```

Use `conga.ConfigPaths(...)` to declare search tiers and `conga.ConfigExplicit(...)` to name one concrete file. [About the config cascade](/explanation/config-cascade/) covers discovery precedence, the built-in XDG tier, and per-tier existence semantics.

## Merge multiple files with cascading

To merge settings across system, user, and project files, add `conga.ConfigCascading()` to the options:

```go title="main.go"
app.ConfigOptions(
	conga.ConfigPaths(
		"./cloudctl.yaml",
		"~/.config/cloudctl/cloudctl.yaml",
		"/etc/cloudctl/cloudctl.yaml",
	),
	conga.ConfigCascading(),
)
```

`conga.ConfigCascading()` merges all matching configuration files across tiers in precedence order; [About the config cascade](/explanation/config-cascade/) describes the merge sequence. The root `ConfigFile` flag must then declare `multi:"true"`:

```go title="main.go"
type CLI struct {
	Config conga.ConfigFile `multi:"true"`
}
```

Omitting `multi:"true"` on a root `ConfigFile` flag when cascading is active fails schema validation.

## Accept config files on the command line

To let users specify custom config files during invocation, declare a `conga.ConfigFile` container on your root command struct:

```go title="main.go"
type CLI struct {
	ConfigFile conga.ConfigFile `short:"c"`
}
```

`conga.ConfigFile` can only be declared on the root command struct. Placing it on a subcommand struct produces an `ErrSchema` error (`ConfigFile can only be declared on the root command`) during schema initialization. Subcommands inspect the active files through `session.LoadedConfigFiles()` and reach root state with `session.Root[T]()` ([session accessors](/reference/conga/#session-and-hierarchy-accessors)).

## Map fields to configuration keys

By default, config keys match the flag's kebab-case field name. Use struct tags to customise mapping:

```go title="main.go"
type Database struct {
	URL      conga.RequiredFlag[string] `config:"connection_url" help:"Database connection string"`
	Password conga.OptionalFlag[string] `config:"-" help:"Read password from flags or environment only"`
}
```

- `config:"connection_url"`: Reads the value from the specified key in the config file.
- `config:"-"`: Disables config file resolution for that field, requiring it to be set via CLI, environment variable, or default value.

## Inspect config files at runtime

Access the evaluated file paths using the session:

```go title="main.go"
func (c *CLI) Run(ctx context.Context, session *conga.Session) error {
	// Candidate paths evaluated during resolution
	candidates := session.CandidateConfigFiles()

	// Paths of files that were actually found and decoded
	loaded := session.LoadedConfigFiles()

	return nil
}
```

`CandidateConfigFiles` lists every path probed during resolution; `LoadedConfigFiles` lists the subset that existed and was decoded.

## Where to go next

- [About the config cascade](/explanation/config-cascade/) — search-tier mechanics, merging order, and XDG resolution.
- [Package decoder reference](/reference/decoders/) — decoders for JSON, TOML, YAML, and HCL.
