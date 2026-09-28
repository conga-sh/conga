# 18 — Config search paths

Single-file search mode: `app.ConfigOptions(config.Paths(...))` without
`Cascading()`, so the first existing file wins and no merging happens. An
explicit `-c` overrides the whole search list.

## Run it

```sh
go run ./_examples/18-config-searchpaths
go run ./_examples/18-config-searchpaths --help
```

The complete scenarios, fixtures included, live in
[`testdata/searchpaths.txt`](testdata/searchpaths.txt).

## What to look for

- The first existing path wins; the others are never read.
- Removing the top candidate falls through to the next one.
- With no file at all, struct defaults apply and the container is `○ unset`.

## Test

```sh
go test ./_examples/18-config-searchpaths
```

Related: [15 — Cascading search paths](../15-cascading-searchpaths/),
[28 — Explicit config file](../28-explicit-config-file/).
