---
title: How to validate input
description: Require flags, limit values to a set, bound numbers, durations, lengths and counts, reject conflicting flags, demand exactly one of a group, require flags together, and check values against each other with Validator.
---

This guide builds an `export` command that sends records to a file or a URL, and adds one kind of check at each step. Each check runs before `Run`, and a value that fails one is a usage error. The program prints the problem and exits with status 2.

## Require a flag

Declare the field as a `conga.RequiredFlag[...]` container, in place of `conga.OptionalFlag[...]`:

```go title="main.go" hidden={1-8}
package main

import (
	"context"
	"fmt"

	"github.com/conga-sh/conga"
)

type Export struct {
	Token  conga.RequiredFlag[string] `help:"API token"`
	Format conga.OptionalFlag[string] `default:"json" help:"Output format"`
}

func (e *Export) Run(ctx context.Context, session *conga.Session) error {
	fmt.Fprintf(session.Stdout(), "exporting as %s\n", e.Format.Get())

	return nil
}

func main() {
	app := conga.New[Export]().Name("export")

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
}
```

```sh error={2}
$ export
export: error: missing required flag: --token

Usage:
  export --token=STRING [flags]

Run 'export --help' for more information.
```

The usage line shows `--token=STRING` outside `[flags]`. When the app reads the environment or a config file, a value from either also satisfies a required flag, so a user who sets the token in the flag's variable can leave it off the command line. A required flag can't have a `default` tag.

## Allow only a set of values

An `enum` [tag](/reference/struct-tags/#flags-optionalflagt-requiredflagt-counter) lists the values the flag accepts, separated by commas:

```go title="main.go" {3}
type Export struct {
	Token  conga.RequiredFlag[string] `help:"API token"`
	Format conga.OptionalFlag[string] `default:"json" enum:"json,csv,yaml" help:"Output format"`
}
```

```sh error={2}
$ export --token abc --format xml
export: error: invalid value "xml" (must be one of: json, csv, yaml) for flag --format
```

Help lists the values after the flag's help text:

```text no-copy
      --format="json"  Output format [enum: json, csv, yaml]
```

Shell completion offers the same values.

## Bound numbers, lengths, and counts

Three tags put bounds on a value, written as `min..max`. `Timeout` is a `time.Duration`, so the file now imports `time`:

- `range` bounds a number, a duration, or a size.
- `len` bounds the length of a string.
- `items` bounds the number of values in a list or a map.

```go title="main.go" ins={4-7}
type Export struct {
	Token   conga.RequiredFlag[string]        `help:"API token"`
	Format  conga.OptionalFlag[string]        `default:"json" enum:"json,csv,yaml" help:"Output format"`
	Limit   conga.OptionalFlag[int]           `default:"100" help:"Records to export" range:"1..1000"`
	Timeout conga.OptionalFlag[time.Duration] `default:"30s" help:"Time limit" range:"1s..5m"`
	Label   conga.OptionalFlag[string]        `help:"Label for the export" len:"3..20"`
	Fields  conga.OptionalFlag[[]string]      `help:"Fields to include" items:"1..5"`
}
```

Each bound has its own message:

```sh error={2}
$ export --token abc --limit 0
export: error: value 0 is out of range [1..1000] for flag --limit
```

```sh error={2}
$ export --token abc --timeout 10m
export: error: value 10m0s is out of range [1s..5m] for flag --timeout
```

```sh error={2}
$ export --token abc --label ab
export: error: value "ab" length 2 is out of range [3..20] for flag --label
```

```sh error={2}
$ export --token abc --fields a,b,c,d,e,f
export: error: item count 6 is out of range [1..5] for flag --fields
```

Help shows each bound:

```text no-copy
      --limit=100          Records to export [1..1000]
      --timeout=30s        Time limit [1s..5m]
      --label=STRING       Label for the export [len: 3..20]
      --fields=STRING,...  Fields to include [items: 1..5]
```

An optional flag with a bound can still be left out. Make it a required flag when it must be given. Either side of a [bound](/reference/struct-tags/#bounds-syntax) can be left open, as in `range:"1.."`, and a single number, as in `items:"5"`, means exactly that many.

:::caution[Defaults are checked too]
CONGA checks a `default` against the field's bound at every run. A `default:"5000"` on `Limit` passes startup, then fails every run that leaves `--limit` out, as a usage error the user can't fix:

```text no-copy
export: error: value 5000 is out of range [1..1000] for flag --limit
```
:::

## Reject flags that conflict

Give flags the same `xor` group name, and the user can give at most one of them:

```go title="main.go" ins={3-4}
type Export struct {
	// ...
	Gzip    conga.OptionalFlag[bool]          `help:"Compress with gzip" xor:"compression"`
	Zstd    conga.OptionalFlag[bool]          `help:"Compress with zstd" xor:"compression"`
}
```

```sh error={2}
$ export --token abc --gzip --zstd
export: error: flags --gzip and --zstd are mutually exclusive

Usage:
  export --token=STRING [flags]

Run 'export --help' for more information.
```

A flag counts as given when the user writes it at all, whatever its value, so `--gzip=false --zstd` fails the same way. Help lists `--gzip` and `--zstd` as ordinary flags, so name the other flag in each `help` tag if users need to know before they meet the error.

## Demand exactly one of a group

Give flags the same `oneof` group name, and the user must give exactly one of them. Here the export goes to a file or a URL:

```go title="main.go" ins={3-4}
type Export struct {
	// ...
	ToFile  conga.OptionalFlag[string]        `help:"Write to a file" oneof:"destination" placeholder:"PATH"`
	ToURL   conga.OptionalFlag[string]        `help:"Upload to a URL" oneof:"destination" placeholder:"URL"`
}
```

Leaving both out fails:

```sh error={2}
$ export --token abc
export: error: missing required flag: one of --to-file, --to-url

Usage:
  export --token=STRING (--to-file=PATH | --to-url=URL) [flags]

Run 'export --help' for more information.
```

Giving both fails too:

```sh error={2}
$ export --token abc --to-file out.json --to-url https://example.com/upload
export: error: flags --to-file and --to-url are mutually exclusive

Usage:
  export --token=STRING (--to-file=PATH | --to-url=URL) [flags]

Run 'export --help' for more information.
```

The usage line shows the group as `(--to-file=PATH | --to-url=URL)`. A member of a `oneof` group can't have a `default` tag.

## Require flags together

Give flags the same `and` group name, and the user must give all of them or none:

```go title="main.go" ins={3-4}
type Export struct {
	// ...
	Username conga.OptionalFlag[string]        `and:"auth" help:"Proxy user name"`
	Password conga.OptionalFlag[string]        `and:"auth" help:"Proxy password"`
}
```

```sh error={2}
$ export --token abc --to-file out.json --username ada
export: error: flag --username must be given together with --password

Usage:
  export --token=STRING (--to-file=PATH | --to-url=URL) [flags]

Run 'export --help' for more information.
```

```sh
$ export --token abc --to-file out.json --username ada --password s3cret
exporting as json
```

## Check values against each other

When a rule involves more than one value, or more than a tag can say, implement `conga.Validator` on the command:

```go
type Validator interface {
	Validate(ctx context.Context, session *Session) error
}
```

Here the export covers a date range, and the last day can't come before the first. The file now also imports `errors`:

```go title="main.go" ins={3-4,7-23}
type Export struct {
	// ...
	Since    conga.OptionalFlag[string]        `default:"2026-01-01" help:"First day to export (YYYY-MM-DD)"`
	Until    conga.OptionalFlag[string]        `default:"2026-01-31" help:"Last day to export (YYYY-MM-DD)"`
}

func (e *Export) Validate(ctx context.Context, session *conga.Session) error {
	since, err := time.Parse(time.DateOnly, e.Since.Get())
	if err != nil {
		return fmt.Errorf("--since %q is not a date (YYYY-MM-DD)", e.Since.Get())
	}

	until, err := time.Parse(time.DateOnly, e.Until.Get())
	if err != nil {
		return fmt.Errorf("--until %q is not a date (YYYY-MM-DD)", e.Until.Get())
	}

	if until.Before(since) {
		return errors.New("--until must not be before --since")
	}

	return nil
}
```

An error from `Validate` is a usage error, like a failed tag check:

```sh error={2}
$ export --token abc --to-file out.json --since 2026-02-01
export: error: --until must not be before --since

Usage:
  export --token=STRING (--to-file=PATH | --to-url=URL) [flags]

Run 'export --help' for more information.
```

`Validate` runs after every tag check has passed, and before `Run` and any lifecycle hook, so it can trust that the flags hold valid values and the program has opened nothing yet. Write the message the user should see. CONGA prints it after `export: error:`.

## Know what users see

Here the app was also given `EnvPrefix("EXPORT")`, so each flag reads a variable such as `EXPORT_LIMIT`.

| What the user does | Result |
|---|---|
| Leaves out a required flag | `missing required flag: --token`, then usage, exit 2 |
| Sets the required flag in the environment only | Works |
| Gives a value outside the `enum` list | `invalid value "xml" (must be one of: json, csv, yaml) for flag --format`, exit 2 |
| Gives a value outside a bound | `value 0 is out of range [1..1000] for flag --limit`, exit 2 |
| Sets a value outside a bound in the environment | `value 5000 is out of range [1..1000] for flag --limit (from environment variable EXPORT_LIMIT)`, exit 2 |
| Gives two flags from one `xor` group, from any mix of command line and environment | `flags --gzip and --zstd are mutually exclusive`, then usage, exit 2 |
| Gives `--gzip=false` with `--zstd` | The same error: `--gzip` counts as given |
| Gives no flag from a `oneof` group | `missing required flag: one of --to-file, --to-url`, then usage, exit 2 |
| Sets one `oneof` flag in the environment and gives the other on the command line | `flags --to-file and --to-url are mutually exclusive`, then usage, exit 2 |
| Gives only part of an `and` group, from any source | `flag --username must be given together with --password`, then usage, exit 2 |
| Gives values that `Validate` rejects | The returned message, then usage, exit 2 |
| Leaves out a flag whose `default` is outside its bound | `value 5000 is out of range [1..1000] for flag --limit`, exit 2, on every such run |

## Where to go next

- [Flags](/reference/struct-tags/#flags-optionalflagt-requiredflagt-counter): every validation tag, and the group rules.
- [Bounds syntax](/reference/struct-tags/#bounds-syntax): open bounds, and bounds for sizes and durations.
- [How to declare flags](/how-to/declare-flags/): the flag types these checks apply to.
- [About error reporting](/explanation/error-reporting/): why a failed check exits with status 2.
