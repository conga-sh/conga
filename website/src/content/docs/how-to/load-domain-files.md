---
title: How to load domain documents
description: Parse Taskfile-, compose-, and pipeline-style documents with conga.Bootstrapper, keep their application data, and contribute their configuration keys to the config tier.
---

A domain document such as a `Taskfile`, `docker-compose.yml`, or pipeline manifest holds application data — tasks, build steps, service dependencies — alongside configuration settings that map to flags (`parallelism`, `log_level`, `env`). Standard config decoders expect every top-level key to map to a flag, so they reject these documents. A `conga.Bootstrapper` parses the document directly, keeps the domain structures for your command, and contributes the settings with `Bootstrap.SetConfig`.

## When to use a Bootstrapper

Use the standard config mechanism when the file is pure configuration and its location is fixed or typed by the user directly: register decoders with `app.RegisterDecoder`, declare `config.Paths`/`config.Explicit`/`config.Cascading`, or use the built-in `--config-file` flag. CONGA then discovers, decodes, and maps every top-level key to a flag. [How to manage config files](/how-to/manage-config-files/) covers that setup.

Use `conga.Bootstrapper` when any of these holds:

- **The path depends on another value.** `--pipeline deploy` selects a section of a file, and `--dir` builds a path such as `dir/taskfile.yaml`. Standard resolution needs the path before flags bind, so it cannot compute these.
- **The document mixes configuration with application data.** A decoder requires every top-level key to map to a flag, which a task runner or compose file cannot satisfy.
- **The merge needs custom logic.** The config tier replaces slices; a `Bootstrapper` can concatenate `steps` or deduplicate `needs` across files.

A `.env` file located through a flag uses the same hook. [How to load .env files](/how-to/load-env-files/) covers that case, including `Bootstrap.Dotenv` and `Bootstrap.DotenvExplicit`.

## Declare bootstrap fields

Mark the fields that must resolve before the document can be found with `bootstrap:"true"`:

```go title="main.go"
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/conga-sh/conga"
	"gopkg.in/yaml.v3"
)

type CLI struct {
	File     conga.OptionalFlag[string] `bootstrap:"true" default:"pipeline.yaml" help:"Path to pipeline file" short:"f"`
	Pipeline conga.OptionalFlag[string] `bootstrap:"true" help:"Target pipeline name"`

	Parallelism conga.OptionalFlag[int]               `help:"Worker concurrency"`
	LogLevel    conga.OptionalFlag[string]            `default:"info" help:"Log verbosity"`
	Env         conga.OptionalFlag[map[string]string] `help:"Environment variables"`
}

func (c *CLI) Run(ctx context.Context, session *conga.Session) error {
	fmt.Fprintf(session.Stdout(), "parallelism=%d log-level=%s\n", c.Parallelism.Get(), c.LogLevel.Get())

	return nil
}
```

CONGA binds these fields from `CLI > environment > default` before calling `Bootstrap`, so the method sees the resolved path and pipeline name. Other fields retain their zero or caller-seeded values during `Bootstrap`. CONGA binds all fields in the subsequent normal pass with the configuration tier active, so keep bootstrap-tagged fields idempotent.

Constraints:

- The root command, or an ancestor command, must implement `conga.Bootstrapper`; otherwise schema build rejects the `bootstrap` tag with `ErrSchema`.
- `bootstrap` cannot be combined with `feature`: the bootstrap phase executes before feature gates resolve.
- Types are restricted to scalars, slices of scalars, and maps of scalars. Pointers, structs, and interfaces are rejected.
- `Bootstrap` also runs for `--why` and shell completion. Keep it deterministic and limited to local reads.

## Implement Bootstrap

Implement `Bootstrap` on the root command struct. The method receives the `*conga.Bootstrap` handle and installs the configuration document with `b.SetConfig`:

```go title="main.go"
type pipelineFile struct {
	Version     int               `yaml:"version"`
	Parallelism int               `yaml:"parallelism"`
	LogLevel    string            `yaml:"log_level"`
	Env         map[string]string `yaml:"env"`
	Pipelines   map[string]struct {
		Parallelism int               `yaml:"parallelism"`
		LogLevel    string            `yaml:"log_level"`
		Env         map[string]string `yaml:"env"`
		Steps       []string          `yaml:"steps"`
	} `yaml:"pipelines"`
}

func (c *CLI) Bootstrap(_ context.Context, b *conga.Bootstrap) error {
	filePath := c.File.Get()
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) && !c.File.Changed() {
			return nil // Default file absent; leave tier inactive
		}
		return fmt.Errorf("reading pipeline file %q: %w", filePath, conga.ErrConfig)
	}

	var file pipelineFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return fmt.Errorf("parsing %s: %w", filePath, conga.ErrConfig)
	}

	document := &conga.ConfigDocument{}

	// Map top-level configuration keys
	if file.Parallelism > 0 {
		document.Set("parallelism", file.Parallelism, filePath)
	}
	if file.LogLevel != "" {
		document.Set("log-level", file.LogLevel, filePath)
	}
	for k, v := range file.Env {
		document.SetKey("env", k, v, filePath)
	}

	// Apply pipeline overlay if selected
	if c.Pipeline.IsSet() {
		name := c.Pipeline.Get()
		p, ok := file.Pipelines[name]
		if !ok {
			return fmt.Errorf("pipeline %q not found in %s: %w", name, filePath, conga.ErrConfig)
		}
		scope := "pipeline " + name
		if p.Parallelism > 0 {
			document.SetScoped("parallelism", p.Parallelism, filePath, scope)
		}
		if p.LogLevel != "" {
			document.SetScoped("log-level", p.LogLevel, filePath, scope)
		}
		for k, v := range p.Env {
			document.SetKeyScoped("env", k, v, filePath, scope)
		}
	}

	b.SetConfig(document)

	return nil
}
```

`Bootstrap` runs once per execution, after the bootstrap pass and before configuration resolution, feature gating, and the normal binding pass. A returned error aborts execution and classifies under `ErrConfig` (exit code 1).

### Run it

For a pipeline file whose `deploy` section sets `parallelism: 1` and `log_level: debug`:

```sh
$ go run . -f pipeline.yaml --pipeline deploy
parallelism=1 log-level=debug
```

Running the same command with `--why` attributes each value to `pipeline.yaml › pipeline deploy`, the origin and scope recorded by `SetScoped`.

## Contribute configuration keys

`conga.ConfigDocument` reports values to the config tier:

| Method | Use for |
|---|---|
| `Set(key, value, origin)` | A top-level scalar, slice, or nested map. `key` matches a flag name or a custom `config:"..."` tag. |
| `SetKey(key, entryKey, value, origin)` | One entry of a map flag, merged key by key with earlier writes. |
| `SetScoped(...)`, `SetKeyScoped(...)` | The same writes with a scope, so `--why` renders the origin as `pipeline.yaml › pipeline deploy`. |

The origin is a free-form string that CONGA displays in provenance reports. A later `Set` of the same top-level key replaces the whole entry, including anything written with `SetKey`. Calling `b.SetConfig(nil)`, or never calling it, leaves the configuration tier inactive. [The conga reference](/reference/conga/) lists the exact signatures.

## Compose with config files

A document supplied by `b.SetConfig` overlays file-resolved values key by key: the document wins the keys it names, while file keys it does not name survive. `Bootstrap` therefore composes with `conga.ConfigPaths`, `conga.ConfigExplicit`, `conga.ConfigCascading`, `app.RegisterDecoder`, and a root `conga.ConfigFile` field. An application that leaves file resolution untouched can also implement `Bootstrap` only to load `.env` files, without calling `b.SetConfig`.

## Drive shell completion from configuration

When shell completion is enabled (`app.EnableCompletion()`), CONGA runs `Bootstrap` before generating candidates so predictors and feature gates have access to the document's configuration. Coercion errors during completion are silently tolerated to avoid breaking candidate generation. [Built-in commands](/reference/builtins/) has the completion details.

## Validation and errors

CONGA validates document entries against the application schema: unknown keys, `config:"-"` exclusions, and validation constraints (`enum`, `range`, `len`) produce `ErrConfig`.

Document values occupy Tier 3 of the resolution ladder (`CLI > environment > document > default`), overlaying file-resolved configuration. See [About value precedence](/explanation/value-precedence/) for the full ladder.

Any error returned by `Bootstrap` aborts execution and classifies under `ErrConfig` (exit code 1).

## Where to go next

- [How to load .env files](/how-to/load-env-files/) — load `.env` files from flag-derived directories and `--env-file` paths.
- [About the config cascade](/explanation/config-cascade/) — search-tier mechanics and cascading file merges.
- [About value precedence](/explanation/value-precedence/) — resolution ladder, provenance tracking, and map folding.
- [How to inspect value provenance](/how-to/inspect-value-provenance/) — diagnose source tiers and scoped origins with `--why`.
- [30 — Bootstrapping flag-selected files](https://github.com/conga-sh/conga/tree/main/_examples/30-bootstrapper) — runnable example with pipeline files and overlay merging.
