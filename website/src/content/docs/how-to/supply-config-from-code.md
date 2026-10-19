---
title: How to supply configuration from code
description: Build the run's configuration document in code with conga.ConfigLoader — for embedded files, remote stores, or computed defaults.
---

Config files are not the only place configuration can come from. `app.ConfigOptions(conga.ConfigLoader(...))` installs a programmatic document source that replaces file resolution (decoders, search paths, and probing) for every run. The loader runs after the command line is tokenized and after fields marked `bootstrap:"true"` are bound, but before configuration resolution and full flag binding, so it can shape the document from the typed invocation itself. The returned document is validated exactly like a decoded config file.

Use it when configuration lives in an embedded filesystem, a remote store, a secrets manager, or any in-process computation — and when the *interpretation* of a file is application-specific enough that a generic decoder cannot express it. A domain document such as a pipeline file mixes configuration with data: the loader maps the configuration keys into the tier, keeps the rest as application data, and merges list values that the config tiers replace instead of concatenating.

## Install the loader

```go title="main.go"
type Server struct {
	Profile conga.OptionalFlag[string] `bootstrap:"true" help:"Deployment profile"`

	Host conga.OptionalFlag[string]
	Port conga.OptionalFlag[int]
}

func main() {
	app := conga.New[Server]().
		Name("server").
		ConfigOptions(conga.ConfigLoader(loadDocument))

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
}

func loadDocument(_ conga.ConfigContext, root *Server) (*conga.ConfigDocument, error) {
	document := &conga.ConfigDocument{}
	document.Set("host", "localhost", "embedded defaults")
	document.Set("port", 8080, "embedded defaults")

	if root.Profile.IsSet() {
		document.Set("host", root.Profile.Get()+".example.com", "profile "+root.Profile.Get())
	}

	return document, nil
}
```

Fields marked `bootstrap:"true"` are bound on the root before the loader runs,
using the normal CLI > environment > default precedence. The loader reads their
typed containers directly; aliases, `env` tags, `EnvPrefix`, and defaults need no
loader-side parsing. Every other field holds its zero or caller-seeded value
during the loader call.

`Set(key, value, origin)` records one top-level document entry in the same key space a decoder produces: a flag name, a config key, a subcommand section, or a namespace section. Values accept the shapes a decoder returns — scalars, slices, and nested maps for sections and map flags:

```go
document.Set("headers", map[string]string{"X-Region": "eu"}, "embedded defaults")
document.Set("deploy", map[string]any{"replicas": 3}, "embedded defaults")
```

`SetKey` records one map entry and merges it key by key with earlier writes. Use it when a higher-precedence layer overrides individual map keys without re-supplying the whole map; each entry key keeps the origin that last wrote it:

```go
document.SetKey("headers", "X-Region", "eu", "embedded defaults")
```

A later `Set` of the same top-level key replaces the whole entry and clears the `SetKey` contributions recorded for it.

The origin is a free-form label CONGA never parses; it attributes the value in the `--why` report. When a value comes from a region within the origin, record that region as a scope with `SetScoped` (or `SetKeyScoped` for one map entry):

```go
document.SetScoped("log-level", "debug", "pipeline.local.yaml", "pipeline deploy")
```

`--why` attributes the value to `pipeline.local.yaml › pipeline deploy`, while its Config paths table lists `pipeline.local.yaml` once.

Returning a nil document leaves the configuration tier inactive.

## Shape the document from the invocation

Bootstrap fields are ordinary typed containers:

```go
type Server struct {
	Profile conga.OptionalFlag[string] `bootstrap:"true" help:"Deployment profile"`
	Dir     conga.OptionalFlag[string] `bootstrap:"true" help:"Working directory"`
}

func loadDocument(ctx conga.ConfigContext, root *Server) (*conga.ConfigDocument, error) {
	profile := "dev"
	if root.Profile.IsSet() {
		profile = root.Profile.Get()
	}

	if root.Dir.Changed() {
		return loadFromDirectory(root.Dir.Get(), profile) // error wraps ErrConfig
	}

	if dir, ok := ctx.LookupEnv("SERVER_CONFIG_DIR"); ok {
		return loadFromDirectory(dir, profile) // error wraps ErrConfig
	}

	document := &conga.ConfigDocument{}
	document.Set("host", profile+".example.com", "profile "+profile)

	return document, nil
}
```

- Bootstrap containers expose the normal typed API: `Get`, `IsSet`, `Changed`, `Source`, and `SourceDetail`.
- `ctx.LookupEnv(key)` is the escape hatch for environment variables not tied to a flag. Normal runs see `.env`; completion uses the shell/process environment and does not load `.env`.
- `ctx.CommandPath()` returns the selected invocation path, such as `server deploy worker`.

`bootstrap` is valid only on scalar, slice-of-scalar, and map-of-scalar fields
whose app declares a loader. It cannot be combined with `feature`, because the
loader runs before feature gates resolve. The loader runs at most once per
execution; applications that enable features may have it run for `--help` and
`--why` rendering, and shell completion invocations run it too, so keep it
deterministic and bounded.

## Load a domain document

A configuration decoder maps a file onto the flag schema. A domain document carries more than flags: a pipeline file stores steps and dependencies alongside the settings that become flags. Map only the configuration keys into the document, keep the rest as application data, and merge lists in the loader — the config tiers replace slices instead of concatenating them:

```go
document := &conga.ConfigDocument{}

for _, file := range files { // lowest precedence first
	document.Set("parallelism", file.parallelism, file.path)
	document.Set("log-level", file.logLevel, file.path)

	for key, value := range file.env {
		document.SetKey("env", key, value, file.path)
	}
}

return document, nil
```

The [30 — Programmatic config loader](https://github.com/conga-sh/conga/tree/main/_examples/30-config-loader) example loads pipeline files this way: `--pipeline` selects which pipeline's configuration overlay applies before any flag binds, `steps` and `needs` concatenate across files, and the per-key origins of merged `env` entries surface in `--why`.

## Precedence and validation

Document values occupy the configuration tier unchanged: `CLI > environment > document > default`. `default` tags stay the lowest layer, map flags merge key by key across all tiers, and `Source()` reports `SourceConfig` for document-supplied values.

Validation is identical to a decoded file. Unknown keys fail with `ErrConfig` and a did-you-mean hint, `config:"-"` exclusions are honored, and `enum`, `range`, and other constraints apply after coercion:

```text no-copy
conga: config file "profile prod": unknown field "hots" in command "server" (did you mean "host"?)
```

A loader error fails the run and is classified under `ErrConfig` (exit code 1, app-authored data).

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

Bootstrap fields are bound twice on a normal run: once for the loader and again
during final binding, which also applies the document tier. Keep bootstrap
fields cheap and idempotent; use a plain scalar or slice rather than a
side-effecting domain type.

Applications without a loader are unaffected, and the file resolver stays out of their binaries.

Related: [How to manage config files](/how-to/manage-config-files/), [About the config cascade](/explanation/config-cascade/), and the runnable [30 — Programmatic config loader](https://github.com/conga-sh/conga/tree/main/_examples/30-config-loader) example.
