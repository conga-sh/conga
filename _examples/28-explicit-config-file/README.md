# 28 — Explicit config file

`app.ConfigOptions(conga.ConfigExplicit(...))`: one programmatic concrete config
file with fatal-if-missing semantics. Only an explicit `--config-file` flag
outranks it; the `<PREFIX>_CONFIG` environment variable and the search tier are
not consulted while the option is declared.

## Run it

```sh
printf 'host: explicit.example.com\nport: 9100\n' > bootstrap.yaml
printf 'host: override.example.com\nport: 9200\n' > override.yaml
go run ./_examples/28-explicit-config-file
go run ./_examples/28-explicit-config-file -c override.yaml
rm bootstrap.yaml
go run ./_examples/28-explicit-config-file   # fatal: bootstrap.yaml missing (exit 1)
```

The complete scenarios, fixtures included, live in
[`testdata/explicit.txt`](testdata/explicit.txt).

## What to look for

- `bootstrap.yaml` loads on every run without any flag.
- `--config-file` wins; `EXPLICIT_CONFIG` is ignored while the option is set.
- A missing explicit file is a runtime error (exit code 1), not usage exit 2.

## Test

```sh
go test ./_examples/28-explicit-config-file
```

Related: [13 — Twelve-factor precedence](../13-twelve-factor-precedence/),
[18 — Config search paths](../18-config-searchpaths/).
