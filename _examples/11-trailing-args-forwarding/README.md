# 11 — Trailing args forwarding

Two argument-forwarding styles: `conga.DashArgs` (everything after `--`, the Go
test / Cargo test pattern) and `conga.RawArgs` (bare trailing tokens, the Docker
run pattern).

## Run it

```sh
go run ./_examples/11-trailing-args-forwarding test ./pkg/server -- -v -run TestAuth
go run ./_examples/11-trailing-args-forwarding exec my-container ls -la /var/log
```

## What to look for

- `DashArgs` captures tokens passed after the standard POSIX `--` delimiter into its container.
- `RawArgs` swallows trailing tokens that look like flags into its container, no `--` needed.

## Test

```sh
go test ./_examples/11-trailing-args-forwarding
```

Related: [04 — Positional args](../04-positional-args/),
[99 — Kitchen sink](../99-kitchen-sink/).
