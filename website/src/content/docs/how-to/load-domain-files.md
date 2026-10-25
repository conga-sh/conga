---
title: How to bootstrap before config resolution
description: Break the flag-before-config cycle with conga.Bootstrapper, then load domain documents or .env files from early-bound bootstrap fields.
---

CONGA resolves config files before the normal flag binding pass, but a config file or domain document selected by a flag (such as `--file`, `--dir`, or `--pipeline`) is unavailable until that flag resolves. `conga.Bootstrapper` breaks the cycle: CONGA binds only fields tagged `bootstrap:"true"`, runs `Bootstrap`, and then performs the normal binding pass with the configuration tier available.

Domain documents such as a `Taskfile`, `docker-compose.yml`, or pipeline manifest add a second problem: they mix configuration settings with application data (tasks, build steps, service dependencies). Standard file decoders expect every key to map to a flag. `Bootstrap` parses the document directly, contributes configuration keys with `Bootstrap.SetConfig`, and keeps the remaining structures for execution.

## Understand the two binding passes

`bootstrap:"true"` marks the fields that must resolve before configuration exists. On a normal run CONGA binds them twice:

1. Bootstrap pass: `CLI > environment > default`. Configuration files are not resolved yet, so a value supplied by a config file is unavailable.
2. Normal pass: `CLI > environment > config file > default`. Every field, tagged or not, is bound, and the normal pass is authoritative.

The bootstrap pass binds fields on every command in the selected chain, not only the root command. Keep the tagged set small — the fields needed to locate or select an external document — and keep `Bootstrap` deterministic and limited to bounded local I/O, because it also runs for `--why` and completion.

## Declare bootstrap fields

When the file path or profile depends on command-line flags (such as `--file`, `--dir`, or `--pipeline`), mark those container fields with `bootstrap:"true"`:

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

CONGA binds fields tagged `bootstrap:"true"` before running `Bootstrap`. The bootstrap pass resolves them through `CLI > environment > default`. The method reads their typed containers directly, with aliases, environment prefixes, and default tags already applied. All non-bootstrap fields hold their zero or caller-seeded values during the `Bootstrap` call.

Constraints:
- The root command, or an ancestor command, must implement `conga.Bootstrapper`; otherwise schema build rejects the `bootstrap` tag with `ErrSchema`.
- `bootstrap` cannot be combined with `feature`: the bootstrap phase executes before feature gates resolve.
- Types are restricted to scalars, slices of scalars, and maps of scalars. Pointers, structs, and interfaces are rejected.
- `DashArgs` and `RawArgs` also accept `bootstrap`.

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

`Bootstrap` runs at most once per execution, after the bootstrap pass and before configuration resolution, feature gating, and the normal binding pass. A returned error aborts execution and classifies under `ErrConfig` (exit code 1).

## Load environment files

`Bootstrap.Dotenv(paths...)` loads each existing `.env` file in order through the writer configured by `App.SetenvFunc`, which defaults to `os.Setenv`. Missing files are ignored; malformed or oversized files fail with `ErrConfig`. Because the paths can be derived from bootstrap fields, an application can load `.env` next to a discovered project directory rather than the process working directory:

```go title="main.go"
func (c *CLI) Bootstrap(_ context.Context, b *conga.Bootstrap) error {
	if err := b.Dotenv(filepath.Join(c.Dir.Get(), ".env")); err != nil {
		return err
	}

	return nil
}
```

`Bootstrap.DotenvExplicit(paths...)` is the strict counterpart: a missing file fails with `ErrConfig`, which suits a path supplied by an `--env-file` flag. Variables already present in the host process environment are never overwritten; a file loaded later in the run overrides a key an earlier `.env` file set.

## Record provenance with document methods

`ConfigDocument` provides four write methods to feed the configuration tier:

- `Set(key, value, origin)` records a top-level setting. The key matches a flag name or custom `config:"..."` tag. Values accept scalars, slices, and nested maps.
- `SetScoped(key, value, origin, scope)` adds an optional sub-origin scope. In `--why`, the value's source renders as `origin › scope` (such as `pipeline.yaml › pipeline deploy`), while the Config paths table lists `pipeline.yaml` once.
- `SetKey(key, entryKey, value, origin)` writes one entry inside a map flag, merging key by key with earlier writes. Each key retains the origin that wrote it. A later `Set` of the same top-level key replaces the entire map and clears previous `SetKey` records.
- `SetKeyScoped(key, entryKey, value, origin, scope)` combines per-key map writing with scoped origin attribution.

Origins are free-form strings CONGA displays in provenance reports. Calling `b.SetConfig(nil)`, or never calling it, leaves the configuration tier inactive.

## Compose with config files

A document supplied by `b.SetConfig` overlays file-resolved values key by key: the document wins the keys it names, while file keys it does not name survive. `Bootstrap` therefore composes with `conga.ConfigPaths`, `conga.ConfigExplicit`, `conga.ConfigCascading`, `app.RegisterDecoder`, and a root `conga.ConfigFile` field. An application that only loads `.env` files and leaves file resolution untouched implements `Bootstrap` without calling `b.SetConfig`.

## Drive shell completion from configuration

When an application enables shell completion (`app.EnableCompletion()`), CONGA runs `Bootstrap` before generating completion candidates.

During completion:
1. CONGA binds bootstrap fields from the partial command line (flags, aliases, short runes, environment, and defaults).
2. `Bootstrap` executes, loading `.env` files and installing the document.
3. CONGA binds the document and file tiers.
4. Candidate generation runs with access to loaded settings and active feature gates.
5. CONGA restores bootstrap and selection state before returning candidates to the shell.

If a bootstrap field fails coercion during completion or `--help`, CONGA tolerates the error, restores the field to its pre-bootstrap state, and runs `Bootstrap` with the field unset.

## Precedence and validation

Document values occupy Tier 3 unchanged: `CLI > environment > document > default`. Values outrank `default` struct tags, while environment variables and command-line flags outrank document values. Within the config tier the document outranks every config file. Map flags merge key by key across all tiers.

CONGA validates document entries against the application schema:
- Unknown keys produce `ErrConfig` with a did-you-mean suggestion.
- `config:"-"` exclusions are enforced.
- Constraints such as `enum`, `range`, and `len` apply after value coercion.

Any error returned by `Bootstrap` aborts execution and classifies under `ErrConfig` (exit code 1).

## Where to go next

- [About the config cascade](/explanation/config-cascade/) — search-tier mechanics and cascading file merges.
- [About value precedence](/explanation/value-precedence/) — resolution ladder, provenance tracking, and map folding.
- [How to inspect value provenance](/how-to/inspect-value-provenance/) — diagnose source tiers and scoped origins with `--why`.
- [30 — Bootstrapping flag-selected files](https://github.com/conga-sh/conga/tree/main/_examples/30-bootstrapper) — runnable example with pipeline files and overlay merging.
