---
title: How to load .env files
description: Load .env and .env.local files, named files, and files a flag points to, and see what users get when a file is missing or broken.
---

A `.env` file sets environment variables for a run, so flags bound to those variables pick up its values. This guide loads the conventional `.env` and `.env.local` pair, keeps the host environment in charge, loads named files, and loads a file whose location comes from a flag.

The running example is the [multiformat and dotenv example](https://github.com/conga-sh/conga/tree/main/_examples/14-multiformat-and-dotenv).

## Load .env and .env.local from the working directory

Call `Dotenv()` on the application builder. The example's flags read their values from environment variables:

```go title="main.go"
type CLI struct {
	ServiceName conga.OptionalFlag[string] `default:"api-svc" env:"SERVICE_NAME" safe:"true"`
	Port        conga.OptionalFlag[int]    `default:"8080" env:"PORT" range:"1..65535" safe:"true" short:"p"`
	DatabaseURL conga.OptionalFlag[string] `env:"DATABASE_URL" help:"Database connection" short:"d"`
	MaxWorkers  conga.OptionalFlag[int]    `default:"4" env:"MAX_WORKERS" help:"Worker thread count" range:"1.." safe:"true"`
	APIKey      conga.OptionalFlag[string] `env:"API_KEY" help:"API key loaded from .env.local"`
}

func main() {
	app := conga.New[CLI]().
		Name("multiformat-and-dotenv").
		EnableWhy().
		Dotenv()

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
}
```

The program reads `.env`, then `.env.local`, from the working directory. A value in `.env.local` replaces the same key from `.env`, and `${VAR}` expands from keys defined earlier:

```ini title=".env"
# Database and service defaults (variable expansion supported)
DB_HOST=localhost
DATABASE_URL=postgres://${DB_HOST}:5432/db
PORT=8000
SERVICE_NAME='local svc'
API_KEY=from-env
```

```ini title=".env.local"
# Machine-local values, never committed to source control; API_KEY overrides .env
API_KEY=local-dev-key-42
```

The example prints each resolved value:

```sh
$ multiformat-and-dotenv
    --service-name       string             local svc                      ● env SERVICE_NAME
    -p, --port           int                8000                           ● env PORT
    -d, --database-url   string             postgres://localhost:5432/db   ● env DATABASE_URL
    --max-workers        int                4                              · default
    --api-key            string             local-dev-key-42               ● env API_KEY
```

`--why` names the file behind each value:

```sh hidden={5,11-38}
$ multiformat-and-dotenv --why
Value provenance for "multiformat-and-dotenv":

  FLAG             VALUE       SOURCE             ENV            CONFIG PATH
  --config-file                unset              -              -
  --service-name   local svc   env (.env)         SERVICE_NAME   service-name
  --port           8000        env (.env)         PORT           port
  --database-url   ██████      env (.env)         DATABASE_URL   database-url
  --max-workers    4           default            MAX_WORKERS    max-workers
  --api-key        ██████      env (.env.local)   API_KEY        api-key

Config decoders: .hcl, .json, .toml, .yaml, .yml

Config paths:
  /home/alice/.config/multiformat-and-dotenv   directory does not exist
    looked for: /home/alice/.config/multiformat-and-dotenv.hcl
    looked for: /home/alice/.config/multiformat-and-dotenv.json
    looked for: /home/alice/.config/multiformat-and-dotenv.toml
    looked for: /home/alice/.config/multiformat-and-dotenv.yaml
    looked for: /home/alice/.config/multiformat-and-dotenv.yml

Create a config file at one of:
  /home/alice/.config/multiformat-and-dotenv/config.hcl
  /home/alice/.config/multiformat-and-dotenv/config.json
  /home/alice/.config/multiformat-and-dotenv/config.toml
  /home/alice/.config/multiformat-and-dotenv/config.yaml
  /home/alice/.config/multiformat-and-dotenv/config.yml
  /home/alice/.config/multiformat-and-dotenv/multiformat-and-dotenv.hcl
  /home/alice/.config/multiformat-and-dotenv/multiformat-and-dotenv.json
  /home/alice/.config/multiformat-and-dotenv/multiformat-and-dotenv.toml
  /home/alice/.config/multiformat-and-dotenv/multiformat-and-dotenv.yaml
  /home/alice/.config/multiformat-and-dotenv/multiformat-and-dotenv.yml
  /home/alice/.config/multiformat-and-dotenv.hcl
  /home/alice/.config/multiformat-and-dotenv.json
  /home/alice/.config/multiformat-and-dotenv.toml
  /home/alice/.config/multiformat-and-dotenv.yaml
  /home/alice/.config/multiformat-and-dotenv.yml

values are redacted — tag fields safe:"true" to display
```

When either file is missing, the program skips it, so the same binary runs where the environment comes from the host alone.

## Let the host environment win

A variable already set in the shell, container, or CI runner keeps its value, and the files fill in only what the host leaves unset. `--why` shows a host value as plain `env`:

```sh {7} hidden={5,8-38}
$ PORT=9090 multiformat-and-dotenv --why
Value provenance for "multiformat-and-dotenv":

  FLAG             VALUE       SOURCE             ENV            CONFIG PATH
  --config-file                unset              -              -
  --service-name   local svc   env (.env)         SERVICE_NAME   service-name
  --port           9090        env                PORT           port
  --database-url   ██████      env (.env)         DATABASE_URL   database-url
  --max-workers    4           default            MAX_WORKERS    max-workers
  --api-key        ██████      env (.env.local)   API_KEY        api-key

Config decoders: .hcl, .json, .toml, .yaml, .yml

Config paths:
  /home/alice/.config/multiformat-and-dotenv   directory does not exist
    looked for: /home/alice/.config/multiformat-and-dotenv.hcl
    looked for: /home/alice/.config/multiformat-and-dotenv.json
    looked for: /home/alice/.config/multiformat-and-dotenv.toml
    looked for: /home/alice/.config/multiformat-and-dotenv.yaml
    looked for: /home/alice/.config/multiformat-and-dotenv.yml

Create a config file at one of:
  /home/alice/.config/multiformat-and-dotenv/config.hcl
  /home/alice/.config/multiformat-and-dotenv/config.json
  /home/alice/.config/multiformat-and-dotenv/config.toml
  /home/alice/.config/multiformat-and-dotenv/config.yaml
  /home/alice/.config/multiformat-and-dotenv/config.yml
  /home/alice/.config/multiformat-and-dotenv/multiformat-and-dotenv.hcl
  /home/alice/.config/multiformat-and-dotenv/multiformat-and-dotenv.json
  /home/alice/.config/multiformat-and-dotenv/multiformat-and-dotenv.toml
  /home/alice/.config/multiformat-and-dotenv/multiformat-and-dotenv.yaml
  /home/alice/.config/multiformat-and-dotenv/multiformat-and-dotenv.yml
  /home/alice/.config/multiformat-and-dotenv.hcl
  /home/alice/.config/multiformat-and-dotenv.json
  /home/alice/.config/multiformat-and-dotenv.toml
  /home/alice/.config/multiformat-and-dotenv.yaml
  /home/alice/.config/multiformat-and-dotenv.yml

values are redacted — tag fields safe:"true" to display
```

Expansion follows the same rule. With `DB_HOST=db.prod` on the host, `${DB_HOST}` in `.env` expands to the host value:

```sh
$ DB_HOST=db.prod multiformat-and-dotenv
    -d, --database-url   string             postgres://db.prod:5432/db   ● env DATABASE_URL
```

## Load named files

Pass paths to `Dotenv` to load exactly those files, in order. A later file replaces keys from an earlier one:

```go title="main.go"
app.Dotenv("config/base.env", "config/secrets.env")
```

In this variation, `base.env` sets `PORT` and `SERVICE_NAME`, and `secrets.env` replaces `SERVICE_NAME`:

```sh
$ multiformat-and-dotenv --why
Value provenance for "multiformat-and-dotenv":

  FLAG             VALUE        SOURCE              ENV            CONFIG PATH
  --service-name   secret-svc   env (secrets.env)   SERVICE_NAME   service-name
  --port           8100         env (base.env)      PORT           port
  --database-url                unset               DATABASE_URL   database-url
  --max-workers    4            default             MAX_WORKERS    max-workers
  --api-key                     unset               API_KEY        api-key
```

`--why` shows a file by its name, and adds the directory when two loaded files share a name (`env (.env)` beside `env (sub/.env)`).

Every named file must exist:

```sh error={2} wrap
$ multiformat-and-dotenv
multiformat-and-dotenv: error: config error: cannot read .env file "config/secrets.env": open config/secrets.env: no such file or directory
```

## Load a .env from a directory the user picks

`Dotenv` fixes its paths when the application starts, before any flag is read. When a flag chooses the location, implement the `conga.Bootstrapper` [interface](/reference/conga/#bootstrap-and-config-documents) and tag the flag `bootstrap:"true"`. CONGA binds the flag before calling `Bootstrap`:

```go title="main.go" ins={2,7-9}
type CLI struct {
	Dir         conga.OptionalFlag[string] `bootstrap:"true" default:"." help:"Project directory" safe:"true"`
	ServiceName conga.OptionalFlag[string] `default:"api-svc" env:"SERVICE_NAME" safe:"true"`
	Port        conga.OptionalFlag[int]    `default:"8080" env:"PORT" safe:"true" short:"p"`
}

func (c *CLI) Bootstrap(_ context.Context, b *conga.Bootstrap) error {
	return b.Dotenv(filepath.Join(c.Dir.Get(), ".env"))
}
```

`--dir sub` loads `sub/.env`, and leaving `--dir` off loads `./.env`:

```sh
$ multiformat-and-dotenv --dir sub --why
Value provenance for "multiformat-and-dotenv":

  FLAG             VALUE     SOURCE       ENV            CONFIG PATH
  --dir            sub       cli          -              dir
  --service-name   api-svc   default      SERVICE_NAME   service-name
  --port           8200      env (.env)   PORT           port
```

Like `Dotenv()`, `Bootstrap.Dotenv` skips a missing file. Called with no arguments, it reads `.env` and `.env.local` from the working directory.

## Load a file the user names

When a flag names the file itself, use `Bootstrap.DotenvExplicit`, which requires the file to exist:

```go title="main.go"
type CLI struct {
	EnvFile conga.OptionalFlag[string] `bootstrap:"true" name:"env-file" help:"Path to the environment file"`
	// ...
}

func (c *CLI) Bootstrap(_ context.Context, b *conga.Bootstrap) error {
	if !c.EnvFile.IsSet() {
		return nil
	}

	return b.DotenvExplicit(c.EnvFile.Get())
}
```

```sh error={2} wrap
$ multiformat-and-dotenv --env-file nope.env
multiformat-and-dotenv: error: config error: cannot read .env file "nope.env": open nope.env: no such file or directory
```

## Combine startup and bootstrap loading

`app.Dotenv()` and `Bootstrap.Dotenv` can both run. The host environment still wins, and a file loaded in `Bootstrap` replaces keys from a startup file. Here `Dotenv()` loads `./.env` and `--dir sub` adds `sub/.env`:

```sh
$ multiformat-and-dotenv --dir sub --why
Value provenance for "multiformat-and-dotenv":

  FLAG             VALUE      SOURCE           ENV            CONFIG PATH
  --dir            sub        cli              -              dir
  --service-name   root-svc   env (.env)       SERVICE_NAME   service-name
  --port           8200       env (sub/.env)   PORT           port
```

`Bootstrap` also runs for `--why` and shell completion, so keep it to quick local reads. [Bootstrap fields](/reference/struct-tags/#bootstrap-fields) lists the rules for bootstrap-tagged fields.

## Test .env loading

`Dotenv` writes variables into the process environment with `os.Setenv`. In tests, chain `SetenvFunc(t.Setenv)` so each test restores the environment when it ends:

```go title="main_test.go"
app := conga.New[CLI]().
	Dotenv("testdata/.env").
	SetenvFunc(t.Setenv)
```

Tests that load `.env` files this way change process-wide state, so they run without `t.Parallel`. [How to test a command](/how-to/test-a-command/) covers the rest of the test setup.

## Know what users see

| Situation | Result |
|---|---|
| `.env` or `.env.local` missing, with `Dotenv()` or `Bootstrap.Dotenv` | Skipped |
| A named file missing, with `Dotenv(paths...)` or `Bootstrap.DotenvExplicit` | Config error, exit 1 |
| A line without `=`, such as `PORT 8400` | Config error, exit 1, naming the file and line |
| A file over 16 MiB | Config error, exit 1, naming the size and the limit |
| An empty value, such as `PORT=` | Treated as unset: the flag falls back to config or its default |
| The host sets the same variable | The host value wins |

```text no-copy error={1-2} wrap
multiformat-and-dotenv: error: config error: cannot parse .env file "bad.env": line 1: dotenv error: parsing .env file: missing '=' in statement
multiformat-and-dotenv: error: config error: cannot read .env file "big.env": file size limit exceeded: file size 17000002 bytes exceeds maximum limit of 16777216 bytes
```

Shell completion reads the same files before suggesting values, so `.env` settings, including [feature gates](/how-to/gate-behind-features/), shape completion the same way they shape a run. A missing or broken file stays quiet during completion and fails the run itself.

## Where to go next

- [Dotenv files](/reference/dotenv/): loading rules, the file grammar, and the size limit.
- [How to bind environment variables](/how-to/bind-environment-variables/): connect variables to flags.
- [How to manage config files](/how-to/manage-config-files/): the source below the environment.
- [How to load domain documents](/how-to/load-domain-documents/): the other use of `Bootstrap`.
- [Multiformat and dotenv example](https://github.com/conga-sh/conga/tree/main/_examples/14-multiformat-and-dotenv): the program used on this page.
