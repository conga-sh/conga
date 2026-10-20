---
title: How to load configuration from domain files
description: Parse domain manifests and task files with conga.ConfigLoader, and bind flag-driven paths with bootstrap tags.
---

A standard configuration file mirrors a CLI flag schema 1:1. Domain documents such as a `Taskfile`, `docker-compose.yml`, or pipeline manifest mix configuration settings with application data: tasks, build steps, and service dependencies.

Standard file decoders expect every key to map to a flag, rejecting application data. With `app.ConfigOptions(conga.ConfigLoader(...))`, your application parses the domain document directly, maps configuration keys into CONGA's configuration tier, and keeps the remaining structures for execution.

## Declare bootstrap flags

When the configuration file path or profile depends on command-line flags (such as `--file`, `--dir`, or `--pipeline`), mark those container fields with `bootstrap:"true"`:

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

CONGA binds fields tagged `bootstrap:"true"` on the root struct before running the loader. They resolve through normal precedence (`CLI > environment > default`). The loader reads their typed containers directly, with aliases, environment prefixes, and default tags already applied. All non-bootstrap fields hold their zero or caller-seeded values during the loader call.

Constraints:
- The application must declare `conga.ConfigLoader`; otherwise schema build rejects the `bootstrap` tag with `ErrSchema`.
- `bootstrap` cannot be combined with `feature`: the loader executes before feature gates resolve.
- Types are restricted to scalars, slices of scalars, and maps of scalars. Pointers, structs, and interfaces are rejected.
- `DashArgs` and `RawArgs` also accept `bootstrap`.

## Register the loader

Register the loader function on the application builder using `conga.ConfigLoader`:

```go title="main.go"
func main() {
	app := conga.New[CLI]().
		Name("flowctl").
		ConfigOptions(conga.ConfigLoader(loadPipelines))

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
}
```

The loader function receives the `ConfigContext` and the typed root command struct pointer:

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

func loadPipelines(ctx conga.ConfigContext, root *CLI) (*conga.ConfigDocument, error) {
	filePath := root.File.Get()
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) && !root.File.Changed() {
			return nil, nil // Default file absent; leave tier inactive
		}
		return nil, fmt.Errorf("reading pipeline file %q: %w", filePath, conga.ErrConfig)
	}

	var file pipelineFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", filePath, conga.ErrConfig)
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
	if root.Pipeline.IsSet() {
		name := root.Pipeline.Get()
		p, ok := file.Pipelines[name]
		if !ok {
			return nil, fmt.Errorf("pipeline %q not found in %s: %w", name, filePath, conga.ErrConfig)
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

	return document, nil
}
```

## Record provenance with document methods

`ConfigDocument` provides four write methods to feed the configuration tier:

- `Set(key, value, origin)` records a top-level setting. The key matches a flag name or custom `config:"..."` tag. Values accept scalars, slices, and nested maps.
- `SetScoped(key, value, origin, scope)` adds an optional sub-origin scope. In `--why`, the value's source renders as `origin › scope` (such as `pipeline.yaml › pipeline deploy`), while the Config paths table lists `pipeline.yaml` once.
- `SetKey(key, entryKey, value, origin)` writes one entry inside a map flag, merging key by key with earlier writes. Each key retains the origin that wrote it. A later `Set` of the same top-level key replaces the entire map and clears previous `SetKey` records.
- `SetKeyScoped(key, entryKey, value, origin, scope)` combines per-key map writing with scoped origin attribution.

Origins are free-form strings CONGA displays in provenance reports. Returning a `nil` document leaves the configuration tier inactive.

## Drive shell completion from configuration

When an application enables shell completion (`app.EnableCompletion()`), CONGA runs the declared `ConfigLoader` before generating completion candidates.

During completion:
1. CONGA binds bootstrap fields from the partial command line (flags, aliases, short runes, environment, and defaults).
2. The loader executes, populating the `ConfigDocument`.
3. CONGA binds the document tier.
4. Candidate generation runs with access to loaded settings and active feature gates.
5. CONGA restores bootstrap and selection state before returning candidates to the shell.

If a bootstrap field fails coercion during completion or `--help`, CONGA tolerates the error, restores the field to its pre-bootstrap state, and runs the loader with the field unset.

## Precedence and validation

Document values occupy Tier 3 unchanged: `CLI > environment > document > default`. Values outrank `default` struct tags, while environment variables and command-line flags outrank document values. Map flags merge key by key across all tiers.

CONGA validates document entries against the application schema:
- Unknown keys produce `ErrConfig` with a did-you-mean suggestion.
- `config:"-"` exclusions are enforced.
- Constraints such as `enum`, `range`, and `len` apply after value coercion.

Any error returned by the loader aborts execution and classifies under `ErrConfig` (exit code 1).

## Exclusivity rules

A loader application controls its own I/O. Combining `conga.ConfigLoader` with file resolution options (`conga.ConfigPaths`, `conga.ConfigExplicit`, `conga.ConfigCascading`), `app.RegisterDecoder`, or a root `conga.ConfigFile` container field fails schema build with `ErrSchema`.

## Where to go next

- [About the config cascade](/explanation/config-cascade/) — search-tier mechanics and cascading file merges.
- [About value precedence](/explanation/value-precedence/) — resolution ladder, provenance tracking, and map folding.
- [How to inspect value provenance](/how-to/inspect-value-provenance/) — diagnose source tiers and scoped origins with `--why`.
- [30 — Programmatic config loader](https://github.com/conga-sh/conga/tree/main/_examples/30-config-loader) — runnable example with pipeline files and overlay merging.
