---
title: How to load .env files
description: Load .env and .env.local files into the process environment with variable expansion and DoS-hardened limits.
---

CONGA provides zero-dependency `.env` file loading through `app.Dotenv()`. Loaded variables populate the process environment before flag resolution, feeding directly into the environment tier of the resolution ladder.

## Enable default .env loading

Call `Dotenv()` on the application builder without arguments to enable conventional loading:

```go title="main.go"
package main

import (
	"context"
	"fmt"

	"github.com/conga-sh/conga"
)

type CLI struct {
	Port conga.OptionalFlag[int] `default:"8080" help:"Listen port"`
}

func (c *CLI) Run(ctx context.Context, session *conga.Session) error {
	fmt.Fprintf(session.Stdout(), "port: %d\n", c.Port.Get())

	return nil
}

func main() {
	app := conga.New[CLI]().
		EnvPrefix("CLOUDCTL").
		Dotenv()

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
}
```

When invoked without arguments, `Dotenv()` probes `.env` then `.env.local` in the current working directory. Missing files are silently ignored, allowing applications to run normally in production environments where configuration is injected directly through the operating system. Existing process environment variables are preserved, and `.env.local` overrides a key defined in `.env`. [Dotenv files](/reference/dotenv/) has the complete loading contract.

Shell completion loads the same files before resolving candidates, so `.env`-sourced values — including feature gates — shape suggestions identically to a real run. A malformed or missing explicit file is tolerated during completion and only fails the real run.

## Preserve host environment variables

At startup, `app.Dotenv()` adheres to the 12-factor application principle: host environment variables take precedence over `.env` files. Variables set in the shell, container, or CI runner override matching entries in `.env` or `.env.local`. Variable expansion observes the same precedence: a reference such as `${HOST}` resolves to the host value, keeping derived entries consistent with the preserved variable:

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

## Load .env from a flag-derived directory

`app.Dotenv` resolves paths at startup, before flags are bound. When the `.env` location derives from a flag such as `--dir`, implement [`conga.Bootstrapper`](/how-to/load-domain-files/) and load it with `Bootstrap.Dotenv`:

```go title="main.go"
type CLI struct {
	Dir conga.OptionalFlag[string] `bootstrap:"true" default:"." help:"Project directory"`
}

func (c *CLI) Bootstrap(_ context.Context, b *conga.Bootstrap) error {
	return b.Dotenv(filepath.Join(c.Dir.Get(), ".env"))
}
```

`Bootstrap.Dotenv` runs after bootstrap-field binding and tolerates missing files, so `--dir ./sub` loads `./sub/.env` and omitting `--dir` loads `./.env`. `Bootstrap.DotenvExplicit` requires each named file to exist. [Dotenv files](/reference/dotenv/) covers the shared origin map and `--why` attribution.

## Test .env loads

`Dotenv` writes loaded variables into the process environment through `os.Setenv` by default. In tests, chain `SetenvFunc(t.Setenv)` to route every loaded variable through `testing.T.Setenv`, so each test restores the environment when it finishes:

```go title="main_test.go"
app := conga.New[CLI]().
	Dotenv("testdata/.env").
	SetenvFunc(t.Setenv)
```

Tests that load `.env` files this way mutate process-wide state and cannot call `t.Parallel`. [How to test a command](/how-to/test-a-command/) covers the surrounding test setup.

## Where to go next

- [Dotenv files](/reference/dotenv/) — loading rules, grammar, and the 16 MB size limit.
- [How to bind environment variables](/how-to/bind-environment-variables/) — map environment variables to command flags.
- [How to manage config files](/how-to/manage-config-files/) — register file decoders and search paths.
- [About value precedence](/explanation/value-precedence/) — the deterministic four-tier resolution ladder.
