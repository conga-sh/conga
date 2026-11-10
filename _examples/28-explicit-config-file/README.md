# 28 — Explicit config file

`app.ConfigOptions(conga.ConfigExplicit(...))` names the application's default
config file, and a missing file is fatal. A file the user names outranks it:
the `--config-file` flag first, then the `WEB_CONFIG` environment variable
(`<PREFIX>_CONFIG`). The built-in search location is not consulted while the
option is declared.

## Run it

```sh
printf 'host: explicit.example.com\nport: 9100\n' > bootstrap.yaml
printf 'host: override.example.com\nport: 9200\n' > override.yaml
printf 'host: env.example.com\nport: 9300\n' > env.yaml
go run ./_examples/28-explicit-config-file
go run ./_examples/28-explicit-config-file --why
WEB_CONFIG=env.yaml go run ./_examples/28-explicit-config-file
go run ./_examples/28-explicit-config-file -c override.yaml
rm bootstrap.yaml
go run ./_examples/28-explicit-config-file   # fatal: bootstrap.yaml missing (exit 1)
go run ./_examples/28-explicit-config-file --why   # also fatal (exit 1)
```

The complete scenarios, fixtures included, live in
[`testdata/explicit.txt`](testdata/explicit.txt).

## What to look for

- `bootstrap.yaml` loads on every run that names no other file, and the
  `--config-file` row reports its source as `config`.
- `WEB_CONFIG` replaces it, and `--config-file` replaces both.
- A missing explicit file is a runtime error (exit code 1), not usage exit 2.
- `--why` shows the loaded explicit candidate. With the file missing, it stops
  with the same read error and exit code 1.

## Test

```sh
go test ./_examples/28-explicit-config-file
```

Related: [13 — Twelve-factor precedence](../13-twelve-factor-precedence/),
[18 — Config search paths](../18-config-searchpaths/).
