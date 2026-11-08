---
title: Examples
description: The 33 runnable example programs in the repository's _examples directory, what each one does, and the CONGA API it uses.
---

The repository's [`_examples/`](https://github.com/conga-sh/conga/tree/main/_examples) directory holds 33 runnable programs, 01 to 32 and 99. Each has its own README and an end-to-end test suite. The suites assert CONGA's [exit codes](/reference/errors/#exit-codes): 0 for success, including `--help` and `--version`, 2 for usage errors, and 1 for schema, configuration, and runtime failures.

Run an example from the repository root:

```sh
$ go run ./_examples/01-minimal-hello --name Alice
Hello, Alice!
```

Run every example's tests:

```sh
go test ./_examples/[0-9]*
```

Each entry below says what the program does, shows one or more runs, and lists the API it demonstrates: types and interfaces in `PascalCase`, struct tag keys in lowercase, and methods as `Type.Method`. [Find an example by API](#find-an-example-by-api) indexes those lists.

Most examples print a table of resolved values and where each came from. The runs below keep that table's header and the rows that matter, or leave the table out. After a failing run, `go run` adds its own `exit status N` line, which the blocks leave out.

## Flags and values

### 01 minimal-hello

[Source](https://github.com/conga-sh/conga/tree/main/_examples/01-minimal-hello) · Tutorial: [Quickstart](/tutorials/quickstart/)

A greeter with one command. `--name` (`-n`) falls back to `World`, `--loud` (`-l`) prints in capitals, and the help screen is generated from the struct.

:::code-tabs
```sh label="Default"
$ go run ./_examples/01-minimal-hello
Hello, World!
```
```sh label="Help"
$ go run ./_examples/01-minimal-hello --help
Minimal single-command hello world CLI

Usage:
  minimal-hello [flags]

Flags:
  -n, --name="World"  Name of the person to greet
  -l, --loud          Print greeting in uppercase
      --help          Show help
      --version       Show version
```
:::

Uses: `OptionalFlag[T]`, `default`, `short`

### 02 flag-primitives

[Source](https://github.com/conga-sh/conga/tree/main/_examples/02-flag-primitives) · Guide: [How to declare flags](/how-to/declare-flags/)

Flags of type `string`, `bool`, `int`, `float64` and `time.Duration`, plus a `-v` counter. `-vvv` counts to 3, `--dbg` is an alias of `--debug`, `--secret-key` is left out of help, and the flags sit under help sections. A malformed duration is a usage error.

:::code-tabs
```sh label="Counter and alias"
$ go run ./_examples/02-flag-primitives -vvv --dbg
    FIELD             TYPE            VALUE   SOURCE
    -v, --verbosity   conga.Counter   3       ● cli --verbosity
    -d, --debug       bool            true    ● cli --debug
```
```sh label="Bad duration" error={2}
$ go run ./_examples/02-flag-primitives --timeout 5x
flag-primitives: error: invalid duration "5x" (valid units: ms, s, m, h) for flag --timeout
```
:::

Uses: `Counter`, `aliases`, `hidden`, `section`

### 03 negatable-and-maps

[Source](https://github.com/conga-sh/conga/tree/main/_examples/03-negatable-and-maps) · Guide: [How to declare flags](/how-to/declare-flags/)

`--color` also takes a `--no-color` form, and the last one given wins. `-e` collects `KEY=VALUE` pairs into a `map[string]string`. `-H` keeps the commas inside a value because its slice is declared with `sep:"none"`.

:::code-tabs
```sh label="Negation and map"
$ go run ./_examples/03-negatable-and-maps --no-color -e A=1 -e B=2
    FIELD            TYPE                VALUE      SOURCE
    -C, --color      bool                false      ● cli --color
    -e, --env-vars   map[string]string   A=1, B=2   ● cli --env-vars
```
```sh label="No separator"
$ go run ./_examples/03-negatable-and-maps -H 'Accept=text/html,application/json'
    FIELD            TYPE                VALUE                                 SOURCE
    -H, --headers    []string            [Accept=text/html,application/json]   ● cli --headers
```
:::

Uses: `negatable`, `map[K]V`, `items`, `sep`, `App.RegisterMap`

### 05 domain-types

[Source](https://github.com/conga-sh/conga/tree/main/_examples/05-domain-types) · Guide: [How to add custom flag types](/how-to/add-custom-flag-types/)

Flags typed as files, directories, byte sizes, and two custom types. `--input-file` must name an existing file, `--max-memory` reads `2GiB`, and `--ports` parses and checks a `PortRange`. Each failure is a usage error.

:::code-tabs
```sh label="Valid"
$ go run ./_examples/05-domain-types -i go.mod -m 2GiB -p 8000-8080 -H Accept=text/html
    FIELD               TYPE                VALUE              SOURCE
    -i, --input-file    conga.File          go.mod             ● cli --input-file
    -m, --max-memory    conga.ByteSize      2GiB               ● cli --max-memory
    -p, --ports         main.PortRange      8000-8080          ● cli --ports
    -H, --headers       main.HeaderMap      Accept=text/html   ● cli --headers
Validated Domain Types: max_memory_bytes=2147483648
```
```sh label="Missing file" error={2}
$ go run ./_examples/05-domain-types -i missing.txt
domain-types: error: stat missing.txt: no such file or directory for flag --input-file
```
```sh label="Bad range" error={2}
$ go run ./_examples/05-domain-types -i go.mod -p 9000-8000
domain-types: error: invalid port range 9000-8000 (must be 1..65535 and min <= max) for flag --ports
```
:::

Uses: `File`, `Dir`, `ByteSize`, `FileContent`, `SelfValidator`, `existing`, `stdin`, `stdout`, `symlink`, `glob`

### 21 pluggable-types

[Source](https://github.com/conga-sh/conga/tree/main/_examples/21-pluggable-types) · Guide: [How to add custom flag types](/how-to/add-custom-flag-types/)

Flags of the standard library types `*time.Location` and `syscall.Signal`, which `App.Use` adds through the `timezone` and `system` packages, and a custom `Percent` type added with `App.RegisterType`.

:::code-tabs
```sh label="Standard library types"
$ go run ./_examples/21-pluggable-types -z Europe/Berlin -s HUP
    FIELD          TYPE               VALUE           SOURCE
    -z, --tz       *time.Location     Europe/Berlin   ● cli --tz
    -s, --signal   syscall.Signal     hangup          ● cli --signal
```
```sh label="Custom type" error={2}
$ go run ./_examples/21-pluggable-types --load 150%
pluggable-types: error: invalid load percentage "150%" (expected 0%-100%) for flag --load
```
:::

Uses: `App.Use`, `timezone.RegisterLocation`, `system.RegisterSignal`, `App.RegisterType`, `WithPredictor`

## Positional and trailing arguments

### 04 positional-args

[Source](https://github.com/conga-sh/conga/tree/main/_examples/04-positional-args) · Guide: [How to accept positional arguments](/how-to/accept-positional-arguments/)

A notifier with a required `<channel>`, an optional `[message]`, and a trailing `[recipients...]` slice that takes every remaining word. Leaving out the channel is a usage error.

:::code-tabs
```sh label="All arguments"
$ go run ./_examples/04-positional-args email 'build failed' oncall pager
    FIELD             TYPE       VALUE             SOURCE
    <channel>         string     email             ● cli
    [message]         string     build failed      ● cli
    [recipients...]   []string   [oncall, pager]   ● cli
Broadcast: channel=email message="build failed" urgent=false extra_recipients=2
```
```sh label="Missing argument" error={2}
$ go run ./_examples/04-positional-args
positional-args: error: missing required positional argument <channel>

Usage:
  positional-args [flags] <channel> [message] [recipients...]

Run 'positional-args --help' for more information.
```
:::

Uses: `RequiredArg[T]`, `OptionalArg[T]`, `OptionalArg.GetOr`, `name`

### 11 trailing-args-forwarding

[Source](https://github.com/conga-sh/conga/tree/main/_examples/11-trailing-args-forwarding) · Guide: [How to accept positional arguments](/how-to/accept-positional-arguments/)

Two ways to hand arguments to another program. `test` forwards whatever follows `--`. `exec` forwards everything after the container name, flags included, with no `--`.

:::code-tabs
```sh label="After --"
$ go run ./_examples/11-trailing-args-forwarding test ./pkg/server -- -v -run TestAuth
    FIELD            TYPE       VALUE              SOURCE
    <pkg>            string     ./pkg/server       ● cli
    [-- <args...>]   DashArgs   -v -run TestAuth   ● cli
DashArgs container: [-v -run TestAuth]
```
```sh label="Bare"
$ go run ./_examples/11-trailing-args-forwarding exec my-container ls -la /var/log
    FIELD                 TYPE      VALUE             SOURCE
    <container>           string    my-container      ● cli
    [command] [args...]   RawArgs   ls -la /var/log   ● cli
RawArgs container: [ls -la /var/log]
```
:::

Uses: `DashArgs`, `RawArgs`

## Subcommands

### 08 subcommands-basic

[Source](https://github.com/conga-sh/conga/tree/main/_examples/08-subcommands-basic) · Guide: [How to declare subcommands](/how-to/declare-subcommands/)

A service-management CLI whose `create`, `delete` and `list` commands sit under Lifecycle and Inspection headings in help. `delete` also answers to `rm` and `del`, a hidden `doctor` command still runs, and a mistyped command gets a suggestion.

:::code-tabs
```sh label="Typo" error={2}
$ go run ./_examples/08-subcommands-basic delte
subcommands-basic: error: unknown command: delte. Did you mean "delete"?
```
```sh label="Alias"
$ go run ./_examples/08-subcommands-basic rm web-api -f
    FIELD              TYPE      VALUE     SOURCE
    delete (rm, del)   command   —         ▸ selected
    <name>             string    web-api   ● cli
    -f, --force        bool      true      ● cli --force
```
```sh label="Hidden command"
$ go run ./_examples/08-subcommands-basic doctor
    FIELD              TYPE      VALUE   SOURCE
    doctor             command   —       ▸ selected
    -f, --fix          bool      false   ○ unset
```
:::

Uses: `Command[T]`, `aliases`, `section`, `hidden`, `summary`

### 09 deep-hierarchy-context

[Source](https://github.com/conga-sh/conga/tree/main/_examples/09-deep-hierarchy-context) · Guide: [How to declare subcommands](/how-to/declare-subcommands/)

A fleet tool four commands deep (`cluster node pod list`). The leaf command reads every ancestor's flags by walking down from the root. The `pod` subtree reads `FLEET_POD_*` variables instead of the `FLEET_*` prefix the rest of the tool uses.

:::code-tabs
```sh label="Deep path"
$ go run ./_examples/09-deep-hierarchy-context cluster node pod list -s pending
=== Deep Hierarchy Resolution ===
Full Command Path:    deep-hierarchy-context cluster node pod list
Active Leaf Name:     list
Root Global Region:   us-east-1
Ancestor Cluster:     default-cluster
Ancestor Node:        node-01
Parent Pod Wait:      5s
Leaf Status Filter:   pending
```
```sh label="Subtree variable"
$ FLEET_POD_STATUS=running go run ./_examples/09-deep-hierarchy-context cluster node pod list
Leaf Status Filter:   running
```
:::

Uses: `Session.Root`, `Command[T].Get`, `Session.Path`, `Session.Name`, `envprefix`, `App.EnvPrefix`

### 32 implicit-command

[Source](https://github.com/conga-sh/conga/tree/main/_examples/32-implicit-command) · Guide: [How to declare subcommands](/how-to/declare-subcommands/)

A script runner where a bare file path runs the `run` command, so `implicit-command ./script.ts` and `implicit-command run ./script.ts` do the same thing. Naming the sibling `install` still selects `install`, and help marks `run` as `(default)`.

:::code-tabs
```sh label="Shorthand"
$ go run ./_examples/32-implicit-command ./script.ts
run file=script.ts dry-run=false args=[]
```
```sh label="Explicit"
$ go run ./_examples/32-implicit-command run ./script.ts
run file=script.ts dry-run=false args=[]
```
```sh label="Flag selects run"
$ go run ./_examples/32-implicit-command -n ./script.ts
run file=script.ts dry-run=true args=[]
```
```sh label="Help"
$ go run ./_examples/32-implicit-command --help
Commands:
  run      Execute a file (default)
  install  Install dependencies
```
:::

Uses: `Command[T]`, `implicit`, `name`

## Validation

### 06 flag-constraints

[Source](https://github.com/conga-sh/conga/tree/main/_examples/06-flag-constraints)

An API client whose flags carry declared limits. `--json`, `--yaml` and `--table` exclude each other, `--password` needs at least 8 characters, and `--port`, `--timeout` and `--memory` have ranges. A violation is a usage error, and this program prints the full help screen after it.

:::code-tabs
```sh label="Exclusive flags" error={2}
$ go run ./_examples/06-flag-constraints -k secret --json --yaml
flag-constraints: error: flags --json and --yaml are mutually exclusive

Declarative safety constraints (XOR, And, Range, Len, Items, Enum)

Usage:
  flag-constraints --api-key=STRING [flags]

Flags:
      --json              Output in JSON format
      --yaml              Output in YAML format
      --table             Output in Table format
  -k, --api-key=STRING    API key for authentication (required)
  -u, --username=STRING   Authentication username [len: 3..20]
      --password=STRING   Authentication password [len: >= 8]
  -p, --port=8080         Port number [1..65535]
      --timeout=2s        Request timeout [100ms..5s]
      --memory=256MiB     Byte size (e.g., 10MiB, 1.5GB, 500KiB) [1MiB..1GiB]
      --env="dev"         Deployment environment [enum: dev, staging, prod]
  -z, --zones=STRING,...  Availability zones [items: 2..3]
      --completion=SHELL  Generate shell completion script
      --help              Show help
      --version           Show version
```
```sh label="Too short" error={2}
$ go run ./_examples/06-flag-constraints -k secret --json -u admin --password hunter2 -z a -z b
flag-constraints: error: value "hunter2" length 7 must be >= 8 for flag --password
```
:::

Uses: `RequiredFlag[T]`, `xor`, `and`, `range`, `len`, `items`, `enum`, `App.UsageOnError`

### 07 dynamic-validation

[Source](https://github.com/conga-sh/conga/tree/main/_examples/07-dynamic-validation)

A scheduler that checks flags against each other in a `Validate` method. An end date before the start date, or retries with a zero backoff, is a usage error reported before any hook runs.

:::code-tabs
```sh label="Date order" error={2}
$ go run ./_examples/07-dynamic-validation --start-date 2026-03-01 --end-date 2026-01-01
dynamic-validation: error: invalid date range: end-date (2026-01-01) cannot be earlier than start-date (2026-03-01)

Usage:
  dynamic-validation [flags]

Run 'dynamic-validation --help' for more information.
```
```sh label="Retry backoff" error={2}
$ go run ./_examples/07-dynamic-validation --max-retries 3 --backoff 0s
dynamic-validation: error: invalid backoff duration: backoff duration must be positive when max-retries > 0

Usage:
  dynamic-validation [flags]

Run 'dynamic-validation --help' for more information.
```
:::

Uses: `Validator`

### 27 strict-tag-validation

[Source](https://github.com/conga-sh/conga/tree/main/_examples/27-strict-tag-validation)

A struct that also carries `validate` and `db` tags for other libraries. Schema build rejects unknown tag keys with `ErrSchema`, so the program registers those keys with `App.AllowTags`. They never appear in help or in the value table.

```sh
$ go run ./_examples/27-strict-tag-validation --email ops@example.com
    FIELD       TYPE     VALUE             SOURCE
    --email     string   ops@example.com   ● cli --email
    --retries   int      3                 · default
```

Uses: `App.AllowTags`

### 31 oneof-groups

[Source](https://github.com/conga-sh/conga/tree/main/_examples/31-oneof-groups)

A reader that takes its input from exactly one of `--input-file` or `--input-url`. Giving neither, or both, is a usage error, and the usage line shows the choice.

:::code-tabs
```sh label="One source"
$ go run ./_examples/31-oneof-groups --input-file data.txt
source=file:data.txt
```
```sh label="Neither" error={2}
$ go run ./_examples/31-oneof-groups
oneof-groups: error: missing required flag: one of --input-file, --input-url

Usage:
  oneof-groups (--input-file=PATH | --input-url=URL)

Run 'oneof-groups --help' for more information.
```
```sh label="Both" error={2}
$ go run ./_examples/31-oneof-groups --input-file data.txt --input-url https://example.com
oneof-groups: error: flags --input-file and --input-url are mutually exclusive

Usage:
  oneof-groups (--input-file=PATH | --input-url=URL)

Run 'oneof-groups --help' for more information.
```
:::

Uses: `oneof`, `placeholder`

## Help, version, and deprecation

### 10 builtins-help-version

[Source](https://github.com/conga-sh/conga/tree/main/_examples/10-builtins-help-version) · Guide: [How to customize help screens](/how-to/customize-help-screens/)

A service tool with `help` and `version` commands, `-h` and `-V` as the short forms of `--help` and `--version`, and custom commands that render help, usage and version themselves. `help status` shows a multi-line description, examples and a footer.

:::code-tabs
```sh label="help status"
$ go run ./_examples/10-builtins-help-version help status
Reports the health of every subsystem builtins-help-version manages.

Pass --detailed for per-subsystem diagnostics.

Usage:
  builtins-help-version status [flags]

Examples:
  builtins-help-version status
  builtins-help-version status --detailed

Flags:
  -d, --detailed  Show detailed subsystem diagnostics
  -h, --help      Show help

builtins-help-version home: https://example.com/builtins-help-version
```
```sh label="version --json"
$ go run ./_examples/10-builtins-help-version version --json
{"version":"2.4.1","gitCommit":"","built":"","goVersion":"go1.27.3","os":"linux","arch":"amd64","modified":false}
```
```sh label="-V"
$ go run ./_examples/10-builtins-help-version -V
builtins-help-version has version 2.4.1 built with go1.27.3
```
:::

Uses: `HelpCommand`, `VersionCommand`, `App.HelpShortFlag`, `App.VersionShortFlag`, `App.Examples`, `App.HelpFooter`, `Session.Help`, `Session.Usage`, `Session.Version`

### 16 help-variables

[Source](https://github.com/conga-sh/conga/tree/main/_examples/16-help-variables) · Guide: [How to customize help screens](/how-to/customize-help-screens/)

A help screen that shows both ways to state a constraint. `--automatic` gets the generated `[1..65535]` hint. `--inlined` writes the same bound into its own help text with `{{range}}`, and the generated hint goes away.

```sh
$ go run ./_examples/16-help-variables --help
Help-variable interpolation: state a constraint once

Usage:
  help-variables [flags]

Flags:
      --automatic=8080      Port using the automatic hint [1..65535]
      --inlined=8080        Port stated once (1..65535)
      --environment=STRING  Environment (dev, staging, prod)
      --zones=STRING,...    Availability zones (2..3 items)
      --profile=STRING      Build profile (default dev)
      --region=STRING       Region (from REGION)
      --literal=STRING      Literal braces: {{not-a-variable}}
      --help                Show help
      --version             Show version
```

Uses: `{{range}}`, `{{enum}}`, `{{items}}`, `{{default}}`, `{{env}}`

### 22 app-deprecation

[Source](https://github.com/conga-sh/conga/tree/main/_examples/22-app-deprecation)

A retired tool that still works but prints a deprecation warning on every run, `--help` included.

```sh
$ go run ./_examples/22-app-deprecation deploy -f
WARNING: app-deprecation is deprecated: use the new unified tool
deployment started
```

Uses: `App.Deprecated`

### 23 member-deprecation

[Source](https://github.com/conga-sh/conga/tree/main/_examples/23-member-deprecation)

A CLI where the `--old` flag and the `legacy` command still work but warn and name their replacements. Completion stops offering them: the candidates below include `--modern` and leave out `--old`.

:::code-tabs
```sh label="Deprecated flag"
$ go run ./_examples/23-member-deprecation --old value
WARNING: flag '--old' is deprecated: use --modern instead
old="value" modern=""
```
```sh label="Completion"
$ go run ./_examples/23-member-deprecation __complete export --
{"candidates":[{"value":"--modern","display":"--modern","style":"blue","tag":"longhand flags"},{"value":"--help","display":"--help","description":"Show help","tag":"longhand flags"},{"value":"--version","display":"--version","description":"Show version","tag":"longhand flags"}],"directives":{"keeporder":false,"messages":[],"noprefix":false,"nospace":false},"usage":""}
```
:::

Uses: `deprecated`

## Configuration and environment

### 13 twelve-factor-precedence

[Source](https://github.com/conga-sh/conga/tree/main/_examples/13-twelve-factor-precedence) · Guide: [How to bind environment variables](/how-to/bind-environment-variables/)

A server CLI that reports where each setting came from: the command line, the environment, a config file, or the default. `--why` lists each flag's environment variable and config key.

:::code-tabs
```sh label="Environment"
$ PORT=9001 go run ./_examples/13-twelve-factor-precedence
    FIELD               TYPE               VALUE       SOURCE
    -p, --port          int                9001        ● env PORT
    --host              string             127.0.0.1   · default
```
```sh label="--why"
$ go run ./_examples/13-twelve-factor-precedence -p 9003 --why
Value provenance for "twelve-factor-precedence":

  FLAG            VALUE       SOURCE    ENV                  CONFIG PATH
  --config-file               unset     SERVER_CONFIG        -
  --port          9003        cli       PORT                 port
  --host          127.0.0.1   default   SERVER_HOST          host
  --environment   dev         default   SERVER_ENVIRONMENT   environment
  --debug         false       default   SERVER_DEBUG         debug
  --worker-id                 unset     SERVER_WORKER_ID     worker-id
  --trace                     unset     -                    trace
  --auth-token                unset     SERVER_AUTH_TOKEN    -
```
:::

Uses: `ConfigFile`, `env`, `config`, `placeholder`, `App.EnvPrefix`, `App.EnableWhy`

### 14 multiformat-and-dotenv

[Source](https://github.com/conga-sh/conga/tree/main/_examples/14-multiformat-and-dotenv) · Guide: [How to load .env files](/how-to/load-env-files/)

A service that reads `.env` and `.env.local` and accepts a config file in JSON, TOML, YAML or HCL. A misspelled config key fails with a suggestion. `--why` lists the registered decoders and redacts `--database-url` and `--api-key`.

```sh
$ go run ./_examples/14-multiformat-and-dotenv --why
Value provenance for "multiformat-and-dotenv":

  FLAG             VALUE     SOURCE    ENV            CONFIG PATH
  --config-file              unset     -              -
  --service-name   api-svc   default   SERVICE_NAME   service-name
  --port           8080      default   PORT           port
  --database-url             unset     DATABASE_URL   database-url
  --max-workers    4         default   MAX_WORKERS    max-workers
  --api-key                  unset     API_KEY        api-key

Config decoders: .hcl, .json, .toml, .yaml, .yml
```

Uses: `App.Dotenv`, `App.RegisterDecoder`, `safe`

### 15 cascading-searchpaths

[Source](https://github.com/conga-sh/conga/tree/main/_examples/15-cascading-searchpaths) · Guide: [How to manage config files](/how-to/manage-config-files/)

A client that merges every config file it finds in `/etc`, `~/.config` and the working directory, with the working directory's file winning key by key. An explicit `--config-file` loads that one file instead. `--why` lists the directories it searched. Paths in the output use `/home/alice` as the home directory.

```sh
$ go run ./_examples/15-cascading-searchpaths --why
Config paths:
  .                                             no config file
    looked for: config.yaml
    looked for: config.yml
    looked for: cascading-searchpaths.yaml
    looked for: cascading-searchpaths.yml
  /home/alice/.config/cascading-searchpaths     directory does not exist
    looked for: /home/alice/.config/cascading-searchpaths.yaml
    looked for: /home/alice/.config/cascading-searchpaths.yml
  /etc/cascading-searchpaths                    directory does not exist
    looked for: /etc/cascading-searchpaths.yaml
    looked for: /etc/cascading-searchpaths.yml
```

Uses: `App.ConfigOptions`, `ConfigPaths`, `ConfigCascading`

### 17 config-profiles

[Source](https://github.com/conga-sh/conga/tree/main/_examples/17-config-profiles) · Guide: [How to manage config files](/how-to/manage-config-files/)

A deploy tool whose `-c` takes several config files and merges them in order, so a profile file overrides a base file key by key. The two files come from the example's `testdata/profiles.txt`; create them to repeat the run.

:::code-tabs
```yaml label="deploy/base.yaml"
host: base.example.com
port: 9000
log_level: debug
```
```yaml label="deploy/prod.yaml"
port: 9443
log_level: warn
```
```sh label="Run"
$ go run ./_examples/17-config-profiles -c deploy/base.yaml -c deploy/prod.yaml
    FIELD                TYPE               VALUE                                SOURCE
    -c, --config-files   conga.ConfigFile   deploy/base.yaml, deploy/prod.yaml   ● cli --config-file
    --host               string             base.example.com                     ● config deploy/base.yaml
    -p, --port           int                9443                                 ● config deploy/prod.yaml
    --log-level          string             warn                                 ● config deploy/prod.yaml
```
:::

Uses: `ConfigFile`, `multi`, `ConfigCascading`

### 18 config-searchpaths

[Source](https://github.com/conga-sh/conga/tree/main/_examples/18-config-searchpaths) · Guide: [How to manage config files](/how-to/manage-config-files/)

A client that searches three directories for one HCL file and loads the first it finds, with no merging. An explicit `-c` replaces the whole search. Paths in the output use `/home/alice` as the home directory.

```sh
$ go run ./_examples/18-config-searchpaths --why
Config paths:
  .                                          no config file
    looked for: config.hcl
    looked for: config-searchpaths.hcl
  /home/alice/.config/config-searchpaths     directory does not exist
    looked for: /home/alice/.config/config-searchpaths.hcl
  /etc/config-searchpaths                    directory does not exist
    looked for: /etc/config-searchpaths.hcl
```

Uses: `App.ConfigOptions`, `ConfigPaths`

### 28 explicit-config-file

[Source](https://github.com/conga-sh/conga/tree/main/_examples/28-explicit-config-file) · Guide: [How to manage config files](/how-to/manage-config-files/)

A web server that always loads `bootstrap.yaml` and stops with exit code 1 when the file is missing. Only `--config-file` replaces it; the `WEB_CONFIG` variable does not. The file comes from the example's `testdata/explicit.txt`; create it to repeat the first run.

:::code-tabs
```yaml label="bootstrap.yaml"
host: explicit.example.com
port: 9100
```
```sh label="File present"
$ go run ./_examples/28-explicit-config-file
    FIELD               TYPE               VALUE                  SOURCE
    --host              string             explicit.example.com   ● config bootstrap.yaml
    -p, --port          int                9100                   ● config bootstrap.yaml
```
```sh label="File missing" error={2}
$ go run ./_examples/28-explicit-config-file
explicit-config-file: error: cannot read config file "bootstrap.yaml": stat bootstrap.yaml: no such file or directory
```
:::

Uses: `App.ConfigOptions`, `ConfigExplicit`

### 29 feature-toggles

[Source](https://github.com/conga-sh/conga/tree/main/_examples/29-feature-toggles) · Guide: [How to gate flags and subcommands behind features](/how-to/feature-flags/)

An HTTP server with named features declared on the root. `--quic` exists only while the `http3` feature is on, and the `admin` command only while `admin-api` is on. Features are switched on from `--feature-gates`, the `API_FEATURES` variable, or a `features` config section.

:::code-tabs
```sh label="Gated flag" error={2}
$ go run ./_examples/29-feature-toggles --quic
feature-toggles: error: flag '--quic' requires feature "http3"

Usage:
  feature-toggles [flags]

Run 'feature-toggles --help' for more information.
```
```sh label="Flag enabled"
$ go run ./_examples/29-feature-toggles --feature-gates http3=true --quic
listen=:8080 cache=128 quic=true insecure=false
features: http3=true admin-api=false strict-tls=true metrics=true
metrics: collecting on :8080
```
```sh label="Command enabled"
$ API_FEATURES=admin-api=true go run ./_examples/29-feature-toggles admin
admin: API listening on :9090
```
:::

Uses: `Features[T]`, `feature`, `flag`, `stage`, `App.EnableFeatures`

### 30 bootstrapper

[Source](https://github.com/conga-sh/conga/tree/main/_examples/30-bootstrapper) · Guide: [How to load domain documents](/how-to/load-domain-files/)

A pipeline runner whose `-f` names a YAML file that has to be read before configuration resolves. `Bootstrap` reads it, keeps the pipelines for `Run`, and passes the file's `parallelism`, `log_level` and `env` keys to the config source.

```sh
$ go run ./_examples/30-bootstrapper -f _examples/30-bootstrapper/testdata/fixtures/pipeline.yaml --pipeline deploy
    FIELD           TYPE                VALUE                                                         SOURCE
    --log-level     string              debug                                                         ● config _examples/30-bootstrapper/testdata/fixtures/pipeline.yaml › pipeline deploy
pipeline deploy: needs=[build] steps=[./deploy.sh]
```

Uses: `Bootstrapper`, `Bootstrap.SetConfig`, `ConfigDocument`, `bootstrap`

## Shell completion

### 19 shell-completion

[Source](https://github.com/conga-sh/conga/tree/main/_examples/19-shell-completion) · Guide: [How to wire custom completion](/how-to/wire-custom-completion/)

A deploy tool that completes values three ways: types that complete themselves on `deploy`, named predictors on `exec`, and the `predict` package's constructors and combinators on `logs`. `--list-candidates` calls the predictors directly, outside completion.

:::code-tabs
```sh label="Completion request"
$ go run ./_examples/19-shell-completion __complete export deploy --cluster ''
{"candidates":[{"value":"staging-cluster-alpha","display":"staging-cluster-alpha","tag":"values"},{"value":"staging-cluster-beta","display":"staging-cluster-beta","tag":"values"}],"directives":{"keeporder":false,"messages":[],"noprefix":false,"nospace":false},"usage":"Target cluster (contextual)"}
```
```sh label="Direct call"
$ go run ./_examples/19-shell-completion logs api --list-candidates
Severities: [debug info warn error]
Severities matching "w": [warn]
KeepOrder directive: true
Tag: severities
```
:::

Uses: `Predictable`, `predict.FileFilterable`, `predictor`, `App.Predictor`, `App.EnableCompletion`, `CompletionEnvPrefix`

### 20 modular-predictors

[Source](https://github.com/conga-sh/conga/tree/main/_examples/20-modular-predictors)

`system` and `network` commands whose flags and arguments use the types from CONGA's `system` and `network` packages. The types complete themselves and carry methods such as `Signal.Signal()` and `Port.IsPrivileged()`.

:::code-tabs
```sh label="system kill"
$ go run ./_examples/20-modular-predictors system kill -s SIGKILL 42
    FIELD           TYPE             VALUE                  SOURCE
    -s, --signal    system.Signal    SIGKILL                ● cli --signal
    <target>        system.PID       42                     ● cli
Signaling PID 42 with SIGKILL (signal 9)
```
```sh label="network serve"
$ go run ./_examples/20-modular-predictors network serve -p 80
    FIELD                TYPE                 VALUE                  SOURCE
    -p, --port           network.Port         80                     ● cli --port
Mock responder would listen on port 80 (returning 200 OK, application/json)
port privileged: true
status class: success=true client_error=false server_error=false
media: type=application subtype=json
```
:::

Uses: `system.Signal`, `system.PID`, `system.Process`, `network.Port`, `network.HTTPStatus`, `network.MIMEType`

### 24 flat-completion-flag

[Source](https://github.com/conga-sh/conga/tree/main/_examples/24-flat-completion-flag) · Guide: [How to enable shell completion](/how-to/enable-shell-completion/)

A minifier with no subcommands. Enabling completion adds a `--completion <shell>` flag, and help still has no Commands section.

```sh
$ go run ./_examples/24-flat-completion-flag --help
Asset minifier (flat CLI with default --completion flag)

Usage:
  flat-completion-flag [flags] <input>

Arguments:
  <input>  Input file to minify

Flags:
  -l, --level=1           Minification level [1..9]
  -o, --output=STRING     Output file path
      --completion=SHELL  Generate shell completion script
      --help              Show help
      --version           Show version
```

Uses: `App.EnableCompletion`

### 25 flat-completion-subcommand

[Source](https://github.com/conga-sh/conga/tree/main/_examples/25-flat-completion-subcommand) · Guide: [How to enable shell completion](/how-to/enable-shell-completion/)

A task runner with no other subcommands that declares `CompletionCommand`, so help lists a `completion` command in place of the `--completion` flag.

```sh
$ go run ./_examples/25-flat-completion-subcommand --help
Task runner (flat CLI with opt-in completion subcommand)

Usage:
  flat-completion-subcommand [flags] <task>

Commands:
  completion  Generate shell completion script

Arguments:
  <task>  Task name to execute

Flags:
  -d, --dry-run  Simulate execution without modifying files
      --help     Show help
      --version  Show version
```

Uses: `CompletionCommand`, `App.EnableCompletion`

## Lifecycle and embedding

### 12 lifecycle-hooks

[Source](https://github.com/conga-sh/conga/tree/main/_examples/12-lifecycle-hooks)

A migration tool that prints each hook as it runs, on the root and on the selected command. When `Run` fails, `PostRun` is skipped and `Finally` still runs. When a command's `PreRun` fails, its own `Run`, `PostRun` and `Finally` are skipped, and the root's `Finally` still runs. Both failures exit with code 1.

:::code-tabs
```sh label="Success"
$ go run ./_examples/12-lifecycle-hooks migrate -d
[HOOK] 1. Root.PreRun: Initializing global logger & telemetry
[HOOK] 2. MigrateCmd.PreRun: Opening database connection pool
[HOOK] 3. MigrateCmd.Run: Executing migration (dry_run=true)
[HOOK] 3.5. MigrateCmd.PostRun: Committing transaction log
[HOOK] 4. Root.PostRun: Flushing telemetry & metrics
[HOOK] 4.5. MigrateCmd.Finally: Closing database connection pool
[HOOK] 5. Root.Finally: Global teardown completed successfully for "migrate"
```
```sh label="Run fails" error={7}
$ go run ./_examples/12-lifecycle-hooks abort
[HOOK] 1. Root.PreRun: Initializing global logger & telemetry
[HOOK] 2. AbortCmd.PreRun: Allocating temporary resources
[HOOK] 3. AbortCmd.Run: Encountered fatal simulation error!
[HOOK] 4. AbortCmd.Finally: Rolling back temporary resources (error: simulated fatal transaction failure)
[HOOK] 5. Root.Finally: Global teardown executed for "abort" (handling error: simulated fatal transaction failure)
lifecycle-hooks: error: simulated fatal transaction failure
```
```sh label="PreRun fails" error={5}
$ go run ./_examples/12-lifecycle-hooks deploy
[HOOK] 1. Root.PreRun: Initializing global logger & telemetry
[HOOK] 2. DeployCmd.PreRun: Pulling container image
[HOOK] 5. Root.Finally: Global teardown executed for "deploy" (handling error: unable to reach container registry)
lifecycle-hooks: error: unable to reach container registry
```
:::

Uses: `PreRunner`, `Runner`, `PostRunner`, `Finalizer`

### 26 library-embedding

[Source](https://github.com/conga-sh/conga/tree/main/_examples/26-library-embedding) · Guide: [How to test a command](/how-to/test-a-command/)

A host program that runs a CONGA CLI in-process. It supplies the arguments and streams, calls `RunContext`, classifies the returned error, and captures the exit code through `ExitFunc` instead of exiting. Lines starting `[host]` are the host's own output.

:::code-tabs
```sh label="Success"
$ go run ./_examples/26-library-embedding migrate-data --priority 7
Running job migrate-data (priority 7)
stdin payload: "host-provided stdin\n"
[host] captured exit code: 0
```
```sh label="Usage error" error={6}
$ go run ./_examples/26-library-embedding migrate-data --priority notanumber
[host] run error: invalid integer value "notanumber" for flag --priority
[host] usage-classified: true
[host] usage error at command path: library-embedding
[host] coercion failure: invalid integer value "notanumber" for flag --priority
library-embedding: error: invalid integer value "notanumber" for flag --priority
[host] captured exit code: 2
```
:::

Uses: `App.Args`, `App.Stdin`, `App.Stdout`, `App.Stderr`, `App.RunContext`, `App.ExitFunc`, `App.DisableHelpFlag`, `IsUsageError`, `UsageError`, `ErrCoerce`

## Everything together

### 99 kitchen-sink

[Source](https://github.com/conga-sh/conga/tree/main/_examples/99-kitchen-sink)

One CLI that uses most of CONGA at once: every primitive width, the filesystem types, all four decoders over one schema, map encodings, provenance, features, environment binding, and `.env` loading. Its `configs/` directory holds the same configuration in each format.

```sh
$ go run ./_examples/99-kitchen-sink --help
Exhaustive type, configuration-mapping, and value-provenance reference

Usage:
  kitchen-sink <command> [flags]

Commands:
  types          Type system matrix (primitives, filesystem, value types)
  config         Configuration mapping patterns
  advanced       Required flags, arguments, trailing args, provenance
  feature-state  Resolved feature state, per-key provenance, and lifecycle hooks
  help           Show help for any command
  version        Show version information
  completion     Generate shell completion scripts

Flags:
  -c, --config-file=PATH             Path to config file [decoders: .hcl, .json, .toml, .yaml, .yml]
                                     [env: KITCHEN_SINK_CONFIG]
      --feature-gates=KEY=VALUE,...  Enable or disable features (name=value)
  -v, --verbosity                    [env: KITCHEN_SINK_VERBOSITY]
      --format="json"                [env: KITCHEN_SINK_FORMAT] [enum: json, yaml, toml, hcl]
      --[no-]cache                   Enable cache [default: true] [env: KITCHEN_SINK_CACHE]
      --secret-key=STRING            API secret (config key remapped, explicit env name)
                                     [env: KITCHEN_SECRET]
      --temp-flag=STRING             CLI only (excluded from config files and environment)
      --metadata=STRING              Arbitrary deployment metadata [env: KITCHEN_SINK_METADATA]
      --storm                        Enable storm mode [env: KITCHEN_SINK_STORM]
      --why[=FORMAT]                 Show resolved field values and where they came from (values
                                     redacted unless tagged safe) [enum: text, json]
      --help                         Show help
      --version                      Show version

Features:
  http3        Experimental QUIC/HTTP3 listener [alpha] [env: KITCHEN_SINK_FEATURE_HTTP3]
  strict-tls   Require TLS and reject weak ciphers [beta] [default: true] [env: KITCHEN_SINK_STRICT]
  metrics      Expose the metrics endpoint [beta] [default: true]
  compression  Response compression level (0 = off) [beta] [levels: 1, 2] [default: 1]
               [env: KITCHEN_SINK_FEATURE_COMPRESSION]
```

Uses: `ConfigFile`, `Features[T]`, `RequiredFlag[T]`, `DashArgs`, `executable`, `App.AllowTags`, `App.Dotenv`, `App.RegisterDecoder`

## Find an example by API

| API | Examples |
|---|---|
| `aliases` | [02](#02-flag-primitives), [08](#08-subcommands-basic) |
| `and` | [06](#06-flag-constraints) |
| `App.AllowTags` | [27](#27-strict-tag-validation), [99](#99-kitchen-sink) |
| `App.Args` | [26](#26-library-embedding) |
| `App.ConfigOptions` | [15](#15-cascading-searchpaths), [18](#18-config-searchpaths), [28](#28-explicit-config-file) |
| `App.Deprecated` | [22](#22-app-deprecation) |
| `App.DisableHelpFlag` | [26](#26-library-embedding) |
| `App.Dotenv` | [14](#14-multiformat-and-dotenv), [99](#99-kitchen-sink) |
| `App.EnableCompletion` | [19](#19-shell-completion), [24](#24-flat-completion-flag), [25](#25-flat-completion-subcommand) |
| `App.EnableFeatures` | [29](#29-feature-toggles) |
| `App.EnableWhy` | [13](#13-twelve-factor-precedence) |
| `App.EnvPrefix` | [09](#09-deep-hierarchy-context), [13](#13-twelve-factor-precedence) |
| `App.Examples` | [10](#10-builtins-help-version) |
| `App.ExitFunc` | [26](#26-library-embedding) |
| `App.HelpFooter` | [10](#10-builtins-help-version) |
| `App.HelpShortFlag` | [10](#10-builtins-help-version) |
| `App.Predictor` | [19](#19-shell-completion) |
| `App.RegisterDecoder` | [14](#14-multiformat-and-dotenv), [99](#99-kitchen-sink) |
| `App.RegisterMap` | [03](#03-negatable-and-maps) |
| `App.RegisterType` | [21](#21-pluggable-types) |
| `App.RunContext` | [26](#26-library-embedding) |
| `App.Stderr` | [26](#26-library-embedding) |
| `App.Stdin` | [26](#26-library-embedding) |
| `App.Stdout` | [26](#26-library-embedding) |
| `App.UsageOnError` | [06](#06-flag-constraints) |
| `App.Use` | [21](#21-pluggable-types) |
| `App.VersionShortFlag` | [10](#10-builtins-help-version) |
| `bootstrap` | [30](#30-bootstrapper) |
| `Bootstrapper` | [30](#30-bootstrapper) |
| `Bootstrap.SetConfig` | [30](#30-bootstrapper) |
| `ByteSize` | [05](#05-domain-types) |
| `Command[T]` | [08](#08-subcommands-basic), [32](#32-implicit-command) |
| `Command[T].Get` | [09](#09-deep-hierarchy-context) |
| `CompletionCommand` | [25](#25-flat-completion-subcommand) |
| `CompletionEnvPrefix` | [19](#19-shell-completion) |
| `config` | [13](#13-twelve-factor-precedence) |
| `ConfigCascading` | [15](#15-cascading-searchpaths), [17](#17-config-profiles) |
| `ConfigDocument` | [30](#30-bootstrapper) |
| `ConfigExplicit` | [28](#28-explicit-config-file) |
| `ConfigFile` | [13](#13-twelve-factor-precedence), [17](#17-config-profiles), [99](#99-kitchen-sink) |
| `ConfigPaths` | [15](#15-cascading-searchpaths), [18](#18-config-searchpaths) |
| `Counter` | [02](#02-flag-primitives) |
| `DashArgs` | [11](#11-trailing-args-forwarding), [99](#99-kitchen-sink) |
| `default` | [01](#01-minimal-hello) |
| `{{default}}` | [16](#16-help-variables) |
| `deprecated` | [23](#23-member-deprecation) |
| `Dir` | [05](#05-domain-types) |
| `enum` | [06](#06-flag-constraints) |
| `{{enum}}` | [16](#16-help-variables) |
| `{{env}}` | [16](#16-help-variables) |
| `env` | [13](#13-twelve-factor-precedence) |
| `envprefix` | [09](#09-deep-hierarchy-context) |
| `ErrCoerce` | [26](#26-library-embedding) |
| `executable` | [99](#99-kitchen-sink) |
| `existing` | [05](#05-domain-types) |
| `feature` | [29](#29-feature-toggles) |
| `Features[T]` | [29](#29-feature-toggles), [99](#99-kitchen-sink) |
| `File` | [05](#05-domain-types) |
| `FileContent` | [05](#05-domain-types) |
| `Finalizer` | [12](#12-lifecycle-hooks) |
| `flag` | [29](#29-feature-toggles) |
| `glob` | [05](#05-domain-types) |
| `HelpCommand` | [10](#10-builtins-help-version) |
| `hidden` | [02](#02-flag-primitives), [08](#08-subcommands-basic) |
| `implicit` | [32](#32-implicit-command) |
| `IsUsageError` | [26](#26-library-embedding) |
| `items` | [03](#03-negatable-and-maps), [06](#06-flag-constraints) |
| `{{items}}` | [16](#16-help-variables) |
| `len` | [06](#06-flag-constraints) |
| `map[K]V` | [03](#03-negatable-and-maps) |
| `multi` | [17](#17-config-profiles) |
| `name` | [04](#04-positional-args), [32](#32-implicit-command) |
| `negatable` | [03](#03-negatable-and-maps) |
| `network.HTTPStatus` | [20](#20-modular-predictors) |
| `network.MIMEType` | [20](#20-modular-predictors) |
| `network.Port` | [20](#20-modular-predictors) |
| `oneof` | [31](#31-oneof-groups) |
| `OptionalArg.GetOr` | [04](#04-positional-args) |
| `OptionalArg[T]` | [04](#04-positional-args) |
| `OptionalFlag[T]` | [01](#01-minimal-hello) |
| `placeholder` | [13](#13-twelve-factor-precedence), [31](#31-oneof-groups) |
| `PostRunner` | [12](#12-lifecycle-hooks) |
| `Predictable` | [19](#19-shell-completion) |
| `predict.FileFilterable` | [19](#19-shell-completion) |
| `predictor` | [19](#19-shell-completion) |
| `PreRunner` | [12](#12-lifecycle-hooks) |
| `range` | [06](#06-flag-constraints) |
| `{{range}}` | [16](#16-help-variables) |
| `RawArgs` | [11](#11-trailing-args-forwarding) |
| `RequiredArg[T]` | [04](#04-positional-args) |
| `RequiredFlag[T]` | [06](#06-flag-constraints), [99](#99-kitchen-sink) |
| `Runner` | [12](#12-lifecycle-hooks) |
| `safe` | [14](#14-multiformat-and-dotenv) |
| `section` | [02](#02-flag-primitives), [08](#08-subcommands-basic) |
| `SelfValidator` | [05](#05-domain-types) |
| `sep` | [03](#03-negatable-and-maps) |
| `Session.Help` | [10](#10-builtins-help-version) |
| `Session.Name` | [09](#09-deep-hierarchy-context) |
| `Session.Path` | [09](#09-deep-hierarchy-context) |
| `Session.Root` | [09](#09-deep-hierarchy-context) |
| `Session.Usage` | [10](#10-builtins-help-version) |
| `Session.Version` | [10](#10-builtins-help-version) |
| `short` | [01](#01-minimal-hello) |
| `stage` | [29](#29-feature-toggles) |
| `stdin` | [05](#05-domain-types) |
| `stdout` | [05](#05-domain-types) |
| `summary` | [08](#08-subcommands-basic) |
| `symlink` | [05](#05-domain-types) |
| `system.PID` | [20](#20-modular-predictors) |
| `system.Process` | [20](#20-modular-predictors) |
| `system.RegisterSignal` | [21](#21-pluggable-types) |
| `system.Signal` | [20](#20-modular-predictors) |
| `timezone.RegisterLocation` | [21](#21-pluggable-types) |
| `UsageError` | [26](#26-library-embedding) |
| `Validator` | [07](#07-dynamic-validation) |
| `VersionCommand` | [10](#10-builtins-help-version) |
| `WithPredictor` | [21](#21-pluggable-types) |
| `xor` | [06](#06-flag-constraints) |
