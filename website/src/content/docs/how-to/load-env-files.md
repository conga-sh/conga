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

When invoked without arguments, `Dotenv()` probes `.env` then `.env.local` in the current working directory. Missing files are silently ignored, allowing applications to run normally in production environments where configuration is injected directly through the operating system. Existing process environment variables are preserved, and `.env.local` overrides a key defined in `.env`. [Dotenv files](/reference/dotenv/) has the complete loading contract.

## Preserve host environment variables

At startup, `app.Dotenv()` adheres to the 12-factor application principle: existing host environment variables are never overwritten by `.env` files. If an environment variable is already set in the operating system shell, Docker container, or CI runner, CONGA preserves that value and ignores the corresponding entry in `.env` or `.env.local`. Variable expansion observes the same precedence: a reference such as `${HOST}` resolves to the host value, so derived entries stay consistent with the preserved variable:

```sh
# OS environment variable takes precedence over .env entries
CLOUDCTL_PORT=9090 cloudctl
```

## Load explicit file paths

To load custom files or require that the files exist, pass explicit paths to `Dotenv`:

```go title="main.go"
app.Dotenv("config/base.env", "config/secrets.env")
```

When explicit paths are provided:
- Files are loaded sequentially in the specified order; every listed file must exist, and a missing file returns an error wrapping `conga.ErrConfig`.
- Existing environment variables are preserved; a key defined in more than one file takes the value from the last file that defines it.

## Where to go next

- [Dotenv files](/reference/dotenv/) — loading rules, grammar, and the 16 MB size limit.
- [How to bind environment variables](/how-to/bind-environment-variables/) — map environment variables to command flags.
- [How to manage config files](/how-to/manage-config-files/) — register file decoders and search paths.
- [About value precedence](/explanation/value-precedence/) — the deterministic four-tier resolution ladder.
