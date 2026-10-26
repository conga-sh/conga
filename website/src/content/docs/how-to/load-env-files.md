---
title: How to load .env files
description: Load .env and .env.local files into the process environment — including locations selected by flags — with variable expansion and DoS-hardened limits.
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

## Load .env from a flag-derived location

`app.Dotenv` fixes its paths when the application starts, before CONGA binds any flag. That covers conventional locations and paths known at build time. When the location depends on a flag — `--dir` selecting a project, or `--env-file` naming the file — the path does not exist at startup. Implement [`conga.Bootstrapper`](/reference/conga/) to bind that flag early and load the file once you know it.

### Bind the location flag early

Tag the location flag with `bootstrap:"true"` and implement `Bootstrap` on the root command:

```go title="main.go"
type CLI struct {
	Dir  conga.OptionalFlag[string] `bootstrap:"true" default:"." help:"Project directory"`
	Port conga.OptionalFlag[int]    `default:"8080" help:"Listen port"`
}

func (c *CLI) Bootstrap(_ context.Context, b *conga.Bootstrap) error {
	return b.Dotenv(filepath.Join(c.Dir.Get(), ".env"))
}
```

CONGA binds `Dir` from the command line, the environment, and its default before calling `Bootstrap`, so `c.Dir.Get()` already holds the resolved directory. `--dir ./sub` loads `./sub/.env`; omitting `--dir` falls back to `.` and loads `./.env`.

`Bootstrap.Dotenv` tolerates missing files, matching `app.Dotenv()`. With no arguments it probes `.env` and then `.env.local` in the working directory, so an application can keep conventional loading while gaining access to bootstrap-tagged fields.

### Require a flag-named file

When a flag names the file itself, use `Bootstrap.DotenvExplicit` instead:

```go title="main.go"
type CLI struct {
	EnvFile conga.OptionalFlag[string] `bootstrap:"true" name:"env-file" help:"Path to the environment file"`
}

func (c *CLI) Bootstrap(_ context.Context, b *conga.Bootstrap) error {
	if !c.EnvFile.IsSet() {
		return nil
	}

	return b.DotenvExplicit(c.EnvFile.Get())
}
```

A missing file fails with an error wrapping `conga.ErrConfig`, which is what a user who named the file expects; `Bootstrap.Dotenv` would silently skip it.

### Combine with app.Dotenv

Both phases write through the same origin map. Variables already present in the host process environment are never overwritten, and a file loaded by `Bootstrap.Dotenv` overrides a key an earlier `app.Dotenv` file set. Loading a conventional file at startup and a flag-located file in `Bootstrap` is therefore safe:

```go title="main.go"
app := conga.New[CLI]().
	EnvPrefix("CLOUDCTL").
	Dotenv() // .env, then .env.local in the working directory
```

`--why` attributes each env-sourced value to the file that set it (such as `env (sub/.env)`), while host variables render as bare `env`.

### Constraints

- The root command or an ancestor must implement `conga.Bootstrapper`; otherwise schema build rejects the `bootstrap` tag with `ErrSchema`.
- Bootstrap-tagged fields bind twice — once early, once in the normal pass — so keep them idempotent.
- `Bootstrap` also runs for `--why` and shell completion. Keep it to bounded local reads. A coercion failure during completion or `--help` is tolerated: CONGA restores the field to its pre-bootstrap state and runs `Bootstrap` with the field unset.

[Dotenv files](/reference/dotenv/) has the full loading contract, grammar, and `--why` attribution. [How to load domain documents](/how-to/load-domain-files/) covers the other use of the same hook: documents that mix configuration with application data.

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
- [How to load domain documents](/how-to/load-domain-files/) — parse Taskfile-, compose-, and pipeline-style files that mix configuration with application data.
- [About value precedence](/explanation/value-precedence/) — the deterministic four-tier resolution ladder.
