---
title: How to bind environment variables
description: Give flags environment variables with an application prefix, per-flag names, and subcommand prefixes, and see what users set and get.
---

An environment variable gives a flag a value when the user leaves the flag off the command line. This guide gives every flag a variable with one prefix, renames one flag's variable, keeps one flag on the command line only, and gives a subcommand its own prefix.

The running example is the [twelve-factor precedence example](https://github.com/conga-sh/conga/tree/main/_examples/13-twelve-factor-precedence).

## Give every flag a variable with an application prefix

Call `EnvPrefix` on the application builder:

```go title="main.go"
type CLI struct {
	Host        conga.OptionalFlag[string] `default:"127.0.0.1" safe:"true" help:"Server host"`
	Environment conga.OptionalFlag[string] `default:"dev" enum:"dev,staging,prod" short:"e" safe:"true" help:"Target environment"`
	Debug       conga.OptionalFlag[bool]   `default:"false" short:"d" help:"Enable debug logging"`
	WorkerID    conga.OptionalFlag[string] `safe:"true" help:"Worker instance identifier"`
}

func main() {
	app := conga.New[CLI]().
		Name("twelve-factor-precedence").
		EnvPrefix("SERVER").
		EnableWhy()

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
}
```

Each flag now reads `<PREFIX>_<FLAG NAME>`, with the flag name in upper case and hyphens turned into underscores, so `--worker-id` reads `SERVER_WORKER_ID`. Users find the names in `--help`:

```text no-copy
Flags:
      --host="127.0.0.1"   Server host [env: SERVER_HOST]
  -e, --environment="dev"  Target environment [env: SERVER_ENVIRONMENT] [enum: dev, staging, prod]
  -d, --debug              Enable debug logging [env: SERVER_DEBUG]
      --worker-id=STRING   Worker instance identifier [env: SERVER_WORKER_ID]
```

A flag on the command line beats its variable, and the variable beats config files and the `default` tag. See [About value precedence](/explanation/value-precedence/) for the full order.

Positional arguments take values from the command line alone.

## Choose a specific name for one flag

Add an `env` tag. The flag reads exactly the names you list, whatever the prefix:

```go title="main.go"
Port conga.OptionalFlag[int] `default:"8080" env:"PORT" placeholder:"PORT" short:"p" safe:"true" help:"Server port"`
```

```text no-copy
  -p, --port=PORT          Server port [default: 8080] [env: PORT]
```

To accept more than one name, list them in order: `env:"API_TOKEN,TOKEN"`. The first variable that is set and non-empty wins, so with `API_TOKEN=a TOKEN=b` the flag gets `a`, and with `API_TOKEN= TOKEN=b` it gets `b`. Help lists both: `[env: API_TOKEN, TOKEN]`.

## Keep a flag on the command line only

Set `env:"-"` on a flag that users should pass explicitly every time:

```go title="main.go"
Trace conga.OptionalFlag[bool] `env:"-" help:"Enable trace output (never environment-bound)"`
```

The flag's help line carries no `[env: ...]` hint, and `SERVER_TRACE=true` leaves `--trace` unset:

```text no-copy
      --trace              Enable trace output (never environment-bound)
```

The flag still takes its value from the command line, a config file, or its `default` tag.

## Give a subcommand its own prefix

A subcommand's flags use the application prefix, so flags with the same name on sibling subcommands read the same variable. A flag that means the same thing in every subcommand belongs on the parent, where subcommands [read it through `session.Root`](/how-to/declare-subcommands/#read-root-flags-from-a-subcommand). When the name means something different in each command, the shared variable sets both.

This variation adds `worker`, `cron`, and `migrate` subcommands under `EnvPrefix("SERVER")`. `worker --timeout` limits each job, and `migrate --timeout` is how long to wait for the schema lock:

```go title="main.go"
type CLI struct {
	Worker  conga.Command[WorkerCmd]  `summary:"Background worker"`
	Cron    conga.Command[CronCmd]    `summary:"Scheduled jobs"`
	Migrate conga.Command[MigrateCmd] `summary:"Run migrations"`
}

type WorkerCmd struct {
	Timeout conga.OptionalFlag[time.Duration] `default:"5m" help:"Time limit for each job" safe:"true"`
}

type MigrateCmd struct {
	Timeout conga.OptionalFlag[time.Duration] `default:"30s" help:"Time to wait for the schema lock" safe:"true"`
}
```

Both help screens name `SERVER_TIMEOUT`:

```text no-copy
      --timeout=5m  Time limit for each job [env: SERVER_TIMEOUT]
      --timeout=30s  Time to wait for the schema lock [env: SERVER_TIMEOUT]
```

A user who exports `SERVER_TIMEOUT=10m` to give jobs more time also makes `migrate` wait ten minutes for the lock:

```sh
$ SERVER_TIMEOUT=10m twelve-factor-precedence worker
worker: job timeout 10m0s
```

```sh
$ SERVER_TIMEOUT=10m twelve-factor-precedence migrate
migrate: lock timeout 10m0s
```

A config file keeps the two apart, because each command's keys sit under its own section (`[worker]` and `[migrate]` in TOML).

To give a subcommand its own variables, add an `envprefix` tag to the `Command[T]` field ([subcommand tags](/reference/struct-tags/#subcommands-commandt)). The tag sets the subcommand's prefix exactly as written, in place of the parent's. You choose whether to repeat the application prefix. `SERVER_WORKER` keeps the subcommand's variables next to the application's, and `MIGRATE` gives them a name of their own:

```go title="main.go" {2,4}
type CLI struct {
	Worker  conga.Command[WorkerCmd]  `envprefix:"SERVER_WORKER" summary:"Background worker"`
	Cron    conga.Command[CronCmd]    `summary:"Scheduled jobs"`
	Migrate conga.Command[MigrateCmd] `envprefix:"MIGRATE" summary:"Run migrations"`
}
```

| Subcommand tag | Flag | Variable |
|---|---|---|
| `envprefix:"SERVER_WORKER"` | `worker --timeout` | `SERVER_WORKER_TIMEOUT` |
| none, so it inherits `SERVER` | `cron --schedule` | `SERVER_SCHEDULE` |
| `envprefix:"MIGRATE"` | `migrate --timeout` | `MIGRATE_TIMEOUT` |

Each subcommand's `--help` shows its own variables:

```text no-copy
      --timeout=30s  Time to wait for the schema lock [env: MIGRATE_TIMEOUT]
```

`SERVER_TIMEOUT` now reaches neither command. Under `envprefix:"MIGRATE"`, the program reads `MIGRATE_TIMEOUT`, so a user who sets `SERVER_MIGRATE_TIMEOUT=2m` still gets the default of `30s`.

To separate a single flag instead, give it an [`env` tag](#choose-a-specific-name-for-one-flag). With `env:"MIGRATE_LOCK_TIMEOUT"` on `migrate`'s flag, and no `envprefix` tags, `SERVER_TIMEOUT` sets `worker` alone:

```text no-copy
      --timeout=30s  Time to wait for the schema lock [env: MIGRATE_LOCK_TIMEOUT]
```

## Set lists, maps, and booleans from the environment

This variation adds a list flag and a map flag to the example's root struct, next to its `Debug` flag:

```go title="main.go"
Tags   conga.OptionalFlag[[]string]          `help:"Instance tags"`
Labels conga.OptionalFlag[map[string]string] `help:"Instance labels"`
```

```text no-copy
      --tags=STRING,...       Instance tags [env: SERVER_TAGS]
      --labels=KEY=VALUE,...  Instance labels [env: SERVER_LABELS]
```

Each variable takes its value in the format the flag takes on the command line, a comma-separated list for `--tags` and `key=value` pairs for `--labels`:

| Flag type | Example variable | Value |
|---|---|---|
| `[]string` | `SERVER_TAGS=a,b` | `["a" "b"]` |
| `map[string]string` | `SERVER_LABELS=env=prod,team=core` | `map[env:prod team:core]` |
| `bool` | `SERVER_DEBUG=true` | `true` |

Spaces after commas are trimmed (`SERVER_TAGS="a, b"` gives the same list). Booleans accept `true`/`false`, `1`/`0`, and upper-case forms such as `TRUE`. Any other value, such as `yes`, is a usage error that names the variable.

A list on the command line replaces the variable's list, so `SERVER_TAGS=a,b` with `--tags c` gives `["c"]`. A map on the command line merges with the variable's map key by key, and the command line wins on a shared key. The variation's `Run` prints `--tags`, `--labels`, and `--debug`:

```sh
$ SERVER_LABELS=env=prod,team=core twelve-factor-precedence --labels team=web
tags=[] labels=map[env:prod team:web] debug=false
```

[Map flags](/explanation/value-precedence/#map-flags) gives the merge order across every source.

## Check where a value came from

Back in the example program, run with `--why`. The `SOURCE` column shows `env` for values from a variable, and the `ENV` column lists the variables each flag reads:

```sh {6-7} hidden={13-23}
$ SERVER_HOST=0.0.0.0 PORT=9001 twelve-factor-precedence -p 9003 --why
Value provenance for "twelve-factor-precedence":

  FLAG            VALUE     SOURCE    ENV                  CONFIG PATH
  --config-file             unset     SERVER_CONFIG        -
  --port          9003      cli       PORT                 port
  --host          0.0.0.0   env       SERVER_HOST          host
  --environment   dev       default   SERVER_ENVIRONMENT   environment
  --debug         false     default   SERVER_DEBUG         debug
  --worker-id               unset     SERVER_WORKER_ID     worker-id
  --trace                   unset     -                    trace
  --auth-token              unset     SERVER_AUTH_TOKEN    -

Config decoders: .toml

Config paths:
  /home/alice/.config/twelve-factor-precedence   directory does not exist
    looked for: /home/alice/.config/twelve-factor-precedence.toml

Create a config file at one of:
  /home/alice/.config/twelve-factor-precedence/config.toml
  /home/alice/.config/twelve-factor-precedence/twelve-factor-precedence.toml
  /home/alice/.config/twelve-factor-precedence.toml
```

`PORT=9001` lost to `-p 9003` on the command line. `--why` prints values only for flags tagged `safe:"true"` and shows the rest as `██████`. See [How to inspect value provenance](/how-to/inspect-value-provenance/) for the rest of the report.

## Know what users see

| What the user sets | Result |
|---|---|
| `SERVER_HOST=0.0.0.0` | `--host` is `0.0.0.0`, source `env` |
| `SERVER_HOST=` (empty) | Treated as unset: `--host` falls back to the config file or `default` |
| `PORT=9001` and `-p 9003` | The command line wins: `--port` is `9003` |
| `PORT=abc` | Usage error, exit 2, naming the variable |
| `SERVER_TRACE=true` on an `env:"-"` flag | Not read: `--trace` stays unset |
| `SERVER_TIMEOUT=10m`, with `worker` and `migrate` both declaring `--timeout` and neither tagged | Both commands read `10m` |
| The variable of a `RequiredFlag` | Satisfies the requirement |

A bad value names both the flag and the variable it came from:

```text no-copy error={1}
twelve-factor-precedence: error: invalid integer value "abc" for flag --port (from environment variable PORT)
```

To pass an empty string on purpose, use the command line, as in `twelve-factor-precedence --host ""`.

`Features` containers bind their variables through their own `env` or `envprefix` tags, independently of `EnvPrefix`. [How to gate flags and subcommands behind features](/how-to/gate-behind-features/#give-each-feature-its-own-environment-variable) covers them.

## Where to go next

- [How to load .env files](/how-to/load-env-files/): feed `.env` and `.env.local` files into the environment.
- [How to inspect value provenance](/how-to/inspect-value-provenance/): the full `--why` report.
- [About value precedence](/explanation/value-precedence/): how command line, environment, config, and defaults combine.
- [Struct tags](/reference/struct-tags/): every `env` and `envprefix` rule.
- [Twelve-factor precedence example](https://github.com/conga-sh/conga/tree/main/_examples/13-twelve-factor-precedence): the program used on this page.
