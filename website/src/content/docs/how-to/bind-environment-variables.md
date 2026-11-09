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

Each flag now reads `<PREFIX>_<FLAG NAME>`, with the flag name in upper case and hyphens turned into underscores: `--worker-id` reads `SERVER_WORKER_ID`. Users find the names in `--help`:

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

By default a subcommand's flags use the application prefix. To change that, add an `envprefix` tag to the `Command[T]` field. The tag sets the subcommand's prefix exactly as written, in place of the parent's. Repeating the application prefix is up to you: `SERVER_WORKER` keeps the subcommand's variables next to the application's, and `MIGRATE` gives them a name of their own.

This variation adds three subcommands under `EnvPrefix("SERVER")`:

```go title="main.go"
type CLI struct {
	Worker  conga.Command[WorkerCmd]  `envprefix:"SERVER_WORKER" summary:"Background worker"`
	Cron    conga.Command[CronCmd]    `summary:"Scheduled jobs"`
	Migrate conga.Command[MigrateCmd] `envprefix:"MIGRATE" summary:"Run migrations"`
}
```

| Subcommand tag | Flag | Variable |
|---|---|---|
| `envprefix:"SERVER_WORKER"` | `worker --concurrency` | `SERVER_WORKER_CONCURRENCY` |
| none, so it inherits `SERVER` | `cron --schedule` | `SERVER_SCHEDULE` |
| `envprefix:"MIGRATE"` | `migrate --steps` | `MIGRATE_STEPS` |

Each subcommand's `--help` shows its own variables:

```text no-copy
      --concurrency=4  Worker concurrency [env: SERVER_WORKER_CONCURRENCY]
```

Under `envprefix:"MIGRATE"`, the program reads `MIGRATE_STEPS`, so a user who sets `SERVER_MIGRATE_STEPS=9` still gets the default of `1`.

## Set lists, maps, and booleans from the environment

In the same variation, users write the same formats they would pass on the command line:

| Flag type | Example variable | Value |
|---|---|---|
| `[]string` | `SERVER_TAGS=a,b` | `["a" "b"]` |
| `map[string]string` | `SERVER_LABELS=env=prod,team=core` | `map[env:prod team:core]` |
| `bool` | `SERVER_DEBUG=true` | `true` |

Spaces after commas are trimmed (`SERVER_TAGS="a, b"` gives the same list). Booleans accept `true`/`false`, `1`/`0`, and upper-case forms such as `TRUE`.

A list on the command line replaces the variable's list rather than adding to it: `SERVER_TAGS=a,b server --tags c` gives `["c"]`.

## Check where a value came from

Run with `--why`. The `SOURCE` column shows `env` for values from a variable, and the `ENV` column lists the variables each flag reads:

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
| A required flag's variable, such as `TOKEN=t` | Satisfies the requirement |

A bad value names both the flag and the variable it came from:

```text no-copy error={1}
twelve-factor-precedence: error: invalid integer value "abc" for flag --port (from environment variable PORT)
```

To pass an empty string on purpose, use the command line: `twelve-factor-precedence --host ""`.

`Features` containers bind their variables through their own `env` or `envprefix` tags, independently of `EnvPrefix`; see [How to gate flags and subcommands behind features](/how-to/gate-behind-features/#give-each-feature-its-own-environment-variable).

## Where to go next

- [How to load .env files](/how-to/load-env-files/): feed `.env` and `.env.local` files into the environment.
- [How to inspect value provenance](/how-to/inspect-value-provenance/): the full `--why` report.
- [About value precedence](/explanation/value-precedence/): how command line, environment, config, and defaults combine.
- [Struct tag reference](/reference/struct-tags/): every `env` and `envprefix` rule.
- [Twelve-factor precedence example](https://github.com/conga-sh/conga/tree/main/_examples/13-twelve-factor-precedence): the program used on this page.
