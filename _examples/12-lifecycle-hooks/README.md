# 12 — Lifecycle hooks

The full hook order — `PreRun` → `Run` → `PostRun` → `Finally` — across root
and leaf commands, including error short-circuiting and guaranteed teardown.

## Run it

```sh
go run ./_examples/12-lifecycle-hooks migrate -d
go run ./_examples/12-lifecycle-hooks abort
go run ./_examples/12-lifecycle-hooks deploy
```

## What to look for

- Hook order is printed step by step for successful and failing runs.
- `abort` fails in `Run`: `PostRun` is skipped, `Finally` still runs.
- `deploy` fails in `PreRun`: `Run`, `PostRun`, and `Finally` are all skipped,
  because a command whose `PreRun` failed is never considered initialized.
- Runtime failures exit with code 1.

## Test

```sh
go test ./_examples/12-lifecycle-hooks
```

Related: [07 — Dynamic validation](../07-dynamic-validation/),
[16 — Live config watcher](../16-live-config-watcher/).
