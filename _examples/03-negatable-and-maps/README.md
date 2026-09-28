# 03 — Negatable and maps

Negatable booleans (`--[no-]color`, last flag wins), native `map[K]V` key-value
flags with `items` bounds, and slice separator controls (`sep:"none"`,
`sep:";"`, default comma splitting).

## Run it

```sh
go run ./_examples/03-negatable-and-maps --no-color -e A=1 -e B=2
go run ./_examples/03-negatable-and-maps -r web=3,api=2
go run ./_examples/03-negatable-and-maps -H 'Accept=text/html,application/json'
```

## What to look for

- `--color` then `--no-color` leaves the last occurrence in effect.
- `sep:"none"` preserves embedded commas in `--headers`.
- The custom `map[string]int` needs `app.RegisterMap`.

## Test

```sh
go test ./_examples/03-negatable-and-maps
```

Related: [06 — Flag constraints](../06-flag-constraints/),
[99 — Kitchen sink](../99-kitchen-sink/).
