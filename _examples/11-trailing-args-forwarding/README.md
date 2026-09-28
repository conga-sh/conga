# 11 — Trailing args forwarding

Two argument-forwarding styles: `conga.DashArgs` (everything after `--`, the Go
test / Cargo test pattern) and `conga.RawArgs` (bare trailing tokens, the Docker
run pattern). Also shows `session.Args()` as the session-level raw tail.

## Run it

```sh
go run ./_examples/11-trailing-args-forwarding test ./pkg/server -- -v -run TestAuth
go run ./_examples/11-trailing-args-forwarding exec my-container ls -la /var/log
```

## What to look for

- `RawArgs` swallows tokens that look like the app's own flags, no `--` needed.
- `session.Args()` mirrors the RawArgs tail and stays empty for `DashArgs`,
  which parses its tokens into the container.

## Test

```sh
go test ./_examples/11-trailing-args-forwarding
```

Related: [04 — Positional args](../04-positional-args/),
[99 — Kitchen sink](../99-kitchen-sink/).
