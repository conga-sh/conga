# 30 — Programmatic config loader

`app.ConfigOptions(conga.ConfigLoader(...))`: the configuration tier comes from
code instead of files. The loader runs after the command line is tokenized and
before flags bind, so it can shape the document from the invocation itself —
here `--profile` selects a stored deployment profile. The returned
`conga.ConfigDocument` is validated exactly like a decoded config file: values
keep the config rank (`CLI > environment > document > default`), map flags
merge key by key, and the origins recorded with each `Set` surface in `--why`.

## Run it

```sh
go run ./_examples/30-config-loader
go run ./_examples/30-config-loader --profile prod --force-all
go run ./_examples/30-config-loader --host cli.example.com
go run ./_examples/30-config-loader --profile prod --why
```

The complete scenarios, fixtures included, live in
[`testdata/loader.txt`](testdata/loader.txt).

## What to look for

- Document values bind with `● config profile dev` provenance and outrank
  `default` tags while losing to `--host` and `DEPLOY_HOST`.
- The prod profile's document carries the `force-all` feature gate; the dev
  profile gates it off, so `--force-all` is a usage error (exit 2).
- `--why` attributes scalars, map keys, and features to the origin labels the
  loader recorded (`config (profile prod)`, `loaded: profile prod`).
- A failing loader is app-authored data: exit 1 with the wrapped cause, never a
  usage error.

## Test

```sh
go test ./_examples/30-config-loader
```

Related: [13 — Twelve-factor precedence](../13-twelve-factor-precedence/),
[29 — Feature toggles](../29-feature-toggles/).
