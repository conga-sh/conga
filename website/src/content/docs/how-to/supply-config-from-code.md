---
title: How to supply configuration from code
description: Build the run's configuration document in code with conga.ConfigLoader — for embedded files, remote stores, or computed defaults.
---

Config files are not the only place configuration can come from. `app.ConfigOptions(conga.ConfigLoader(...))` installs a programmatic document source that replaces file resolution (decoders, search paths, and probing) for every run. The loader runs after the command line is tokenized and before flags bind, so it can shape the document from the invocation itself, and the returned document is validated exactly like a decoded config file.

Use it when configuration lives in an embedded filesystem, a remote store, a secrets manager, or any in-process computation — and when the *interpretation* of a file is application-specific enough that a generic decoder cannot express it.

## Install the loader

```go title="main.go"
func main() {
	app := conga.New[Server]().
		Name("server").
		ConfigOptions(conga.ConfigLoader(loadDocument))

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
}

func loadDocument(ctx conga.ConfigContext) (*conga.ConfigDocument, error) {
	document := &conga.ConfigDocument{}
	document.Set("host", "localhost", "embedded defaults")
	document.Set("port", 8080, "embedded defaults")

	return document, nil
}
```

`Set(key, value, origin)` records one top-level document entry in the same key space a decoder produces: a flag name, a config key, a subcommand section, or a namespace section. Values accept the shapes a decoder returns — scalars, slices, and nested maps for sections and map flags:

```go
document.Set("headers", map[string]string{"X-Region": "eu"}, "embedded defaults")
document.Set("deploy", map[string]any{"replicas": 3}, "embedded defaults")
```

The origin is a free-form label CONGA never parses; it attributes the value in the `--why` report. Returning a nil document leaves the configuration tier inactive.

## Shape the document from the invocation

`conga.ConfigContext` exposes what the engine knows when the loader runs:

```go
func loadDocument(ctx conga.ConfigContext) (*conga.ConfigDocument, error) {
	profile := "dev"
	if named, ok := ctx.FlagValue("profile"); ok { // last value of --profile
		profile = named
	}

	if dir, ok := ctx.LookupEnv("SERVER_CONFIG_DIR"); ok { // post-.env environment
		return loadFromDirectory(dir, profile) // error wraps ErrConfig
	}

	document := &conga.ConfigDocument{}
	document.Set("host", profile+".example.com", "profile "+profile)

	return document, nil
}
```

- `FlagValue(name)` returns the last raw command-line value for a flag or alias.
- `FlagValues(name)` returns every supplied value in order (repeated flags).
- `LookupEnv(key)` reads the run's environment view, with `.env` files already applied.
- `CommandPath()` returns the selected invocation path, such as `server deploy worker`.

The loader runs at most once per execution. Applications that enable features may have it run for `--help` and `--why` rendering, and shell completion invocations run it too, so keep it free of side effects, prompts, and blocking I/O.

## Precedence and validation

Document values occupy the configuration tier unchanged: `CLI > environment > document > default`. `default` tags stay the lowest layer, map flags merge key by key across all tiers, and `Source()` reports `SourceConfig` for document-supplied values.

Validation is identical to a decoded file. Unknown keys fail with `ErrConfig` and a did-you-mean hint, `config:"-"` exclusions are honored, and `enum`, `range`, and other constraints apply after coercion:

```text no-copy
conga: config file "profile prod": unknown field "hots" in command "server" (did you mean "host"?)
```

A loader error fails the run and is classified under `ErrConfig` (exit code 1, app-authored data). An error type implementing `ConfigPath() string` is attributed to its candidate row in the tolerant `--why` report.

## Feature gates ride the document

Declare the `config:` key on the `Features` container and include the resolved gates in the returned document. The document merges with every other gate tier key by key:

```go
type Server struct {
	Features conga.Features[Experiments] `config:"experiments" env:"-"`

	ForceAll conga.OptionalFlag[bool] `feature:"force-all"`
}
```

With `env:"-"` the document is the only gate channel; without it a derived `<PREFIX>_FEATURES` environment variable outranks the document. Gated-off flags and subcommands are pruned from `--help`, `--why`, and completion, and invoking them is a usage error (exit 2).

## What a loader replaces

A loader application must not also declare file-based configuration: combining `conga.ConfigLoader` with `conga.ConfigPaths`, `conga.ConfigExplicit`, `conga.ConfigCascading`, `app.RegisterDecoder`, or a root `conga.ConfigFile` field fails schema build with `ErrSchema`. There is no probing, no file-size limit, and no decoder registry; `--config-file` does nothing because the flag cannot exist alongside a loader.

Applications without a loader are unaffected, and the file resolver stays out of their binaries.

Related: [How to manage config files](/how-to/manage-config-files/), [About the config cascade](/explanation/config-cascade/), and the runnable [30 — Programmatic config loader](https://github.com/conga-sh/conga/tree/main/_examples/30-config-loader) example.
