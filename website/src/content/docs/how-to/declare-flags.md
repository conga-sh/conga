---
title: How to declare flags
description: Declare typed flags with defaults, short names and aliases, negatable booleans, repeat counters, list flags, and key=value map flags.
---

This guide builds a `fetch` command that downloads a URL, and gives it one flag of each kind. [Flags](/reference/struct-tags/#flags-optionalflagt-requiredflagt-counter) lists every flag tag and rule. If you're new to CONGA, [the quickstart](/tutorials/quickstart/) builds a first command step by step.

## Give a flag a type and a default

CONGA builds a command's flags from the fields of its struct. The type of each flag field is a container: `conga.OptionalFlag[...]` for an optional flag, `conga.RequiredFlag[...]` for a required one. The type in the brackets is the type of the flag's value, so a `conga.OptionalFlag[int]` field is an optional flag that takes an integer.

The field name sets the flag name in kebab case, so `Retries` becomes `--retries`, and the `default` tag sets the value used when the user doesn't give the flag. Here `fetch` has three flags, plus the URL as a [positional argument](/how-to/accept-positional-arguments/):

```go title="main.go" hidden={1-9}
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/conga-sh/conga"
)

type Fetch struct {
	URL     conga.RequiredArg[string]         `help:"Address to download"`
	Output  conga.OptionalFlag[string]        `default:"-" help:"File to write"`
	Retries conga.OptionalFlag[int]           `default:"3" help:"Attempts before giving up"`
	Timeout conga.OptionalFlag[time.Duration] `default:"30s" help:"Time limit per attempt"`
}

func (f *Fetch) Run(ctx context.Context, session *conga.Session) error {
	fmt.Fprintf(session.Stdout(), "fetching %s to %s: %d tries, %s each\n", f.URL.Get(), f.Output.Get(), f.Retries.Get(), f.Timeout.Get())

	return nil
}

func main() {
	app := conga.New[Fetch]().Name("fetch")

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
}
```

The command's `Run` method reads each flag's value with the container's `Get` method. Help shows each flag with its default:

```sh
$ fetch --help
Usage:
  fetch [flags] <url>

Arguments:
  <url>  Address to download

Flags:
      --output="-"   File to write
      --retries=3    Attempts before giving up
      --timeout=30s  Time limit per attempt
      --help         Show help
      --version      Show version
```

```sh
$ fetch https://example.com
fetching https://example.com to -: 3 tries, 30s each
```

```sh
$ fetch https://example.com --output page.html --retries 5 --timeout 2m
fetching https://example.com to page.html: 5 tries, 2m0s each
```

A value that CONGA can't convert to the flag's type is a usage error, exit 2:

```sh error={2}
$ fetch https://example.com --timeout 30
fetch: error: invalid duration "30" (valid units: ms, s, m, h) for flag --timeout
```

A `name` tag replaces the derived name. For types CONGA doesn't parse, see [How to add custom flag types](/how-to/add-custom-flag-types/).

## Add a short name and aliases

A `short` tag adds a one-letter form, and an `aliases` tag adds more long names, separated by commas:

```go title="main.go" {3-4}
type Fetch struct {
	URL     conga.RequiredArg[string]         `help:"Address to download"`
	Output  conga.OptionalFlag[string]        `default:"-" help:"File to write" short:"o"`
	Retries conga.OptionalFlag[int]           `aliases:"tries" default:"3" help:"Attempts before giving up"`
	Timeout conga.OptionalFlag[time.Duration] `default:"30s" help:"Time limit per attempt"`
}
```

```sh
$ fetch https://example.com -o page.html --tries 5
fetching https://example.com to page.html: 5 tries, 30s each
```

Help shows the short form beside the long one, and lists only the main name of an aliased flag:

```text no-copy
Flags:
  -o, --output="-"   File to write
      --retries=3    Attempts before giving up
```

Every short name and alias must be unique in the command and its ancestors. A clash fails at startup with `ErrSchema`, exit 1. Here `Timeout` also has `short:"o"`:

```text no-copy wrap
fetch: error: struct Fetch field Timeout: schema error: duplicate flag: short flag -o (already declared by field Output)
```

## Let users turn off a default-on flag

A `negatable` tag on a `bool` flag adds a `--no-` form. Give it `default:"true"` for a behaviour that is on unless the user turns it off:

```go title="main.go" ins={6,11}
type Fetch struct {
	URL       conga.RequiredArg[string]         `help:"Address to download"`
	Output    conga.OptionalFlag[string]        `default:"-" help:"File to write" short:"o"`
	Retries   conga.OptionalFlag[int]           `aliases:"tries" default:"3" help:"Attempts before giving up"`
	Timeout   conga.OptionalFlag[time.Duration] `default:"30s" help:"Time limit per attempt"`
	Redirects conga.OptionalFlag[bool]          `default:"true" help:"Follow redirects" negatable:"true"`
}

func (f *Fetch) Run(ctx context.Context, session *conga.Session) error {
	// ...
	fmt.Fprintf(session.Stdout(), "follow redirects: %t\n", f.Redirects.Get())
	// ...
}
```

```text no-copy
      --[no-]redirects  Follow redirects [default: true]
```

```sh
$ fetch https://example.com --no-redirects
fetching https://example.com to -: 3 tries, 30s each
follow redirects: false
```

When both forms appear, the last one wins, so a user can override a wrapper script that passes `--no-redirects`:

```sh
$ fetch https://example.com --no-redirects --redirects
fetching https://example.com to -: 3 tries, 30s each
follow redirects: true
```

`--redirects=false` also works. Without the `negatable` tag, `--no-redirects` is an unknown flag. `negatable` on a non-`bool` flag fails at startup with `ErrSchema`.

## Count how often a flag is repeated

`conga.Counter` is a flag container whose value is the number of times the user gives the flag. Short forms cluster, so `-vvv` counts three:

```go title="main.go" ins={7,12}
type Fetch struct {
	URL       conga.RequiredArg[string]         `help:"Address to download"`
	Output    conga.OptionalFlag[string]        `default:"-" help:"File to write" short:"o"`
	Retries   conga.OptionalFlag[int]           `aliases:"tries" default:"3" help:"Attempts before giving up"`
	Timeout   conga.OptionalFlag[time.Duration] `default:"30s" help:"Time limit per attempt"`
	Redirects conga.OptionalFlag[bool]          `default:"true" help:"Follow redirects" negatable:"true"`
	Verbose   conga.Counter                     `help:"Print more detail; repeat for more" short:"v"`
}

func (f *Fetch) Run(ctx context.Context, session *conga.Session) error {
	// ...
	fmt.Fprintf(session.Stdout(), "verbosity: %d\n", f.Verbose.Get())
	// ...
}
```

```sh hidden={3}
$ fetch https://example.com -vvv
fetching https://example.com to -: 3 tries, 30s each
follow redirects: true
verbosity: 3
```

A cluster can end in a short flag that takes a value, as in `-vvvo page.html`, and `--verbose=2` sets the count directly. A counter starts at 0 and rejects a `default` tag at startup; use `OptionalFlag[int]` when you need a different starting value.

## Accept a list of values

A flag with a slice type, such as `[]int`, collects every value the user gives. The user can repeat the flag, or separate values with commas:

```go title="main.go" ins={8-9,14-15}
type Fetch struct {
	URL       conga.RequiredArg[string]         `help:"Address to download"`
	Output    conga.OptionalFlag[string]        `default:"-" help:"File to write" short:"o"`
	Retries   conga.OptionalFlag[int]           `aliases:"tries" default:"3" help:"Attempts before giving up"`
	Timeout   conga.OptionalFlag[time.Duration] `default:"30s" help:"Time limit per attempt"`
	Redirects conga.OptionalFlag[bool]          `default:"true" help:"Follow redirects" negatable:"true"`
	Verbose   conga.Counter                     `help:"Print more detail; repeat for more" short:"v"`
	RetryOn   conga.OptionalFlag[[]int]         `help:"Status codes to retry"`
	Header    conga.OptionalFlag[[]string]      `help:"Request header" sep:"none" short:"H"`
}

func (f *Fetch) Run(ctx context.Context, session *conga.Session) error {
	// ...
	fmt.Fprintf(session.Stdout(), "retry on: %v\n", f.RetryOn.Get())
	fmt.Fprintf(session.Stdout(), "headers: %q\n", f.Header.Get())
	// ...
}
```

A header value can contain commas, so `--header` has `sep:"none"`, which takes each value whole. `sep` can also name another separator, such as `sep:";"`:

```sh hidden={3-4}
$ fetch https://example.com --retry-on 502,503 --retry-on 504 -H 'Accept: text/html, application/json' -H 'X-Trace: 1'
fetching https://example.com to -: 3 tries, 30s each
follow redirects: true
verbosity: 0
retry on: [502 503 504]
headers: ["Accept: text/html, application/json" "X-Trace: 1"]
```

CONGA converts each value to the slice's element type, so `--retry-on 50x` fails with `invalid integer value "50x" for flag --retry-on`, exit 2.

A list from the command line replaces a list from the environment or a config file. Here the builder also calls `EnvPrefix("FETCH")`:

```sh hidden={3-4,6}
$ FETCH_RETRY_ON=502,503 fetch https://example.com --retry-on 504
fetching https://example.com to -: 3 tries, 30s each
follow redirects: true
verbosity: 0
retry on: [504]
headers: []
```

## Accept key=value pairs

A flag with the type `map[string]string` collects `key=value` pairs, repeated or separated by commas:

```go title="main.go" ins={10,15}
type Fetch struct {
	URL       conga.RequiredArg[string]             `help:"Address to download"`
	Output    conga.OptionalFlag[string]            `default:"-" help:"File to write" short:"o"`
	Retries   conga.OptionalFlag[int]               `aliases:"tries" default:"3" help:"Attempts before giving up"`
	Timeout   conga.OptionalFlag[time.Duration]     `default:"30s" help:"Time limit per attempt"`
	Redirects conga.OptionalFlag[bool]              `default:"true" help:"Follow redirects" negatable:"true"`
	Verbose   conga.Counter                         `help:"Print more detail; repeat for more" short:"v"`
	RetryOn   conga.OptionalFlag[[]int]             `help:"Status codes to retry"`
	Header    conga.OptionalFlag[[]string]          `help:"Request header" sep:"none" short:"H"`
	Form      conga.OptionalFlag[map[string]string] `help:"Form field" short:"F"`
}

func (f *Fetch) Run(ctx context.Context, session *conga.Session) error {
	// ...
	fmt.Fprintf(session.Stdout(), "form: %v\n", f.Form.Get())
	// ...
}
```

```sh hidden={3-6}
$ fetch https://example.com -F name=ada -F lang=go
fetching https://example.com to -: 3 tries, 30s each
follow redirects: true
verbosity: 0
retry on: []
headers: []
form: map[lang:go name:ada]
```

The value runs to the end of the word, so `-F q=a=b` sets `q` to `a=b`. A repeated key keeps its last value.

Unlike a list, a map from the command line merges with one from the environment or a config file key by key, and the command line wins for keys in both. With `EnvPrefix("FETCH")` again:

```sh hidden={3-6}
$ FETCH_FORM=name=ada,lang=go fetch https://example.com -F lang=rust
fetching https://example.com to -: 3 tries, 30s each
follow redirects: true
verbosity: 0
retry on: []
headers: []
form: map[lang:rust name:ada]
```

:::caution[Other map types]
Only `map[string]string` binds without registration. Any other map type, such as `map[string]int`, needs `app.RegisterMap`; see [Register a map type with `RegisterMap`](/how-to/add-custom-flag-types/#register-a-map-type-with-registermap). Without it the program starts, and fails only when a user sets the flag, exit 2:

```text no-copy
fetch: error: map type map[string]int requires registration via app.RegisterMap
```
:::

## Know what users see

| What happens | When | Result |
|---|---|---|
| The user gives a value of the wrong type | Run | `invalid duration "30" (valid units: ms, s, m, h) for flag --timeout`, exit 2 |
| An environment variable holds a value of the wrong type | Run | `invalid boolean value "no", expected true or false (or 1 or 0) for flag --redirects (from environment variable FETCH_REDIRECTS)`, exit 2 |
| The user misspells a flag | Run | `unknown flag: --redirect. Did you mean "redirects"?`, then usage, exit 2 |
| The user gives `--no-` to a flag without `negatable` | Run | `unknown flag: --no-verbose`, then usage, exit 2 |
| The user gives a map entry without `=` | Run | `invalid key-value pair "name", expected KEY=VALUE for flag --form`, exit 2 |
| The user sets a map flag whose type isn't registered | Run | `map type map[string]int requires registration via app.RegisterMap`, then usage, exit 2 |
| Two flags share a short name or alias | Startup | `schema error: duplicate flag: short flag -o (already declared by field Output)`, exit 1 |
| `negatable` is on a flag that isn't `bool` | Startup | `schema error: invalid struct tag: tag "negatable" is only valid on boolean flags, got time.Duration`, exit 1 |
| A `Counter` has a `default` tag | Startup | `schema error: invalid struct tag: tag "default" is not supported on counter flags; …`, exit 1 |

## Where to go next

- [Flags](/reference/struct-tags/#flags-optionalflagt-requiredflagt-counter): every flag tag and rule.
- [How to validate input](/how-to/validate-input/): require flags, bound values, and reject conflicting flags.
- [How to bind environment variables](/how-to/bind-environment-variables/): set flags from the environment.
- [How to customize help screens](/how-to/customize-help-screens/): group flags into sections and hide flags from help.
