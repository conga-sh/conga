---
title: How to load .env files
description: Load .env and .env.local files into the process environment with variable expansion and DoS-hardened limits.
---

CONGA provides zero-dependency `.env` file loading through `app.Dotenv()`. Loaded variables populate the process environment before flag resolution, feeding directly into the environment tier of the resolution ladder.

## Enable default .env loading

Call `Dotenv()` on the application builder without arguments to enable conventional loading:

```go title="main.go"
func main() {
	app := conga.New[CLI]().
		Name("cloudctl").
		EnvPrefix("CLOUDCTL").
		Dotenv()

	app.FatalIfError(app.Run())
}
```

When invoked without arguments, `Dotenv()`:
1. Probes `.env` in the current working directory.
2. Probes `.env.local` in the current working directory.
3. Overrides `.env` entries with values from `.env.local`.

Missing files in default mode are silently ignored, allowing applications to run normally in production environments where configuration is injected directly through the operating system.

## Preserve host environment variables

CONGA adheres strictly to the 12-factor application principle: **existing host environment variables are never overwritten by `.env` files**.

If an environment variable is already set in the operating system shell, Docker container, or CI runner, CONGA preserves that value and ignores the corresponding entry in `.env` or `.env.local`:

```sh
# OS environment variable takes precedence over .env entries
$ CLOUDCTL_PORT=9090 cloudctl
```

## Load explicit file paths

To load custom files or enforce that config files exist, pass explicit paths to `Dotenv`:

```go title="main.go"
app.Dotenv("config/base.env", "config/secrets.env")
```

When explicit paths are provided:
- Files are loaded sequentially in the specified order, with later files overriding keys defined in earlier files.
- Every listed file must exist. If an explicit file is missing, CONGA returns an error wrapping `conga.ErrConfig`.

## Supported grammar and syntax

The `.env` parser supports standard dotenv syntax:

- **Comments and whitespace**: Lines starting with `#` and empty lines are ignored.
- **Export prefix**: Lines prefixed with `export ` are parsed cleanly (`export PORT=8080`).
- **Quotes**: Values may be unquoted, single-quoted, or double-quoted. Single-quoted strings (`'raw\nstring'`) treat characters literally. Double-quoted strings (`"line1\nline2"`) evaluate escape sequences (`\n`, `\t`, `\"`, `\\`).
- **Variable expansion**: Values support `$VAR` and `${VAR}` syntax. Variables expand from previously defined keys in the file or from the existing process environment:

```ini title=".env"
BASE_DIR=/var/data
LOG_DIR=${BASE_DIR}/logs
PATH=${PATH}:/opt/bin
```

## Enforce file size limits

To prevent unbounded memory allocation under adversarial input, CONGA enforces a strict 16 MB maximum file size limit on `.env` files. Files exceeding 16 MB are rejected with an error wrapping `conga.ErrConfig`.

## Where to go next

- [How to bind environment variables](/how-to/bind-environment-variables/) — map environment variables to command flags.
- [How to manage config files](/how-to/manage-config-files/) — register file decoders and search paths.
- [About value precedence](/explanation/value-precedence/) — the deterministic four-tier resolution ladder.
