# 15 — Cascading search paths

`app.ConfigOptions(conga.ConfigPaths(...), conga.ConfigCascading())`: every existing file
in the search tier merges in precedence order (system → user → local, lowest to
highest), partial cascades skip missing files silently, and an explicit
`--config-file` disables cascading entirely.

## Run it

```sh
go run ./_examples/15-cascading-searchpaths
go run ./_examples/15-cascading-searchpaths --why
go run ./_examples/15-cascading-searchpaths --theme solarized --timeout 5s
```

The complete scenarios, fixtures included, live in
[`testdata/cascading.txt`](testdata/cascading.txt).

## What to look for

- Values resolved from different files at once (`● config cascading-app.yaml`)
  show the merge, and each value names the file it came from.
- Later tiers override earlier ones per key; missing tier files are skipped.
- A missing explicit file is fatal (exit code 1).
- `--why` attributes each winning value to its contributing file and lists every
  loaded cascade file in resolution order.

## Test

```sh
go test ./_examples/15-cascading-searchpaths
```

Related: [17 — Config profiles](../17-config-profiles/),
[18 — Config search paths](../18-config-searchpaths/).
