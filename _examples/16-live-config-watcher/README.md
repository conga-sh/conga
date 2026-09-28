# 16 — Live config watcher

A daemon that hot-reloads configuration: `watch.Config` (wired to
`session.Reload[T]`), `watch.File` for an auxiliary blocklist, SIGHUP manual
reload, `session.CandidateConfigFiles()`/`LoadedConfigFiles()`, and graceful
signal shutdown behind a live HTTP `/config` endpoint.

## Run it

```sh
printf 'rate_limit: 500\ndebug: false\n' > config.yaml
printf 'bad-host\n' > blocklist.txt
go run ./_examples/16-live-config-watcher -c config.yaml --blocklist blocklist.txt
curl http://127.0.0.1:8080/config
```

The complete scenarios, fixtures included, live in
[`testdata/watcher.txt`](testdata/watcher.txt).

## What to look for

- Editing `config.yaml` triggers a live reload; `reloads` increments and the
  endpoint serves the new values.
- SIGHUP forces a manual `session.Reload` independent of file events.
- SIGINT/SIGTERM shut the server down gracefully through the run context.

## Test

```sh
go test ./_examples/16-live-config-watcher
```

Related: [13 — Twelve-factor precedence](../13-twelve-factor-precedence/),
[How to hot-reload config](https://docs.conga.sh/how-to/hot-reload-config/).
