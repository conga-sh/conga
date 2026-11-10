---
title: How to use the network, system, and timezone packages
description: Give flags host, port, signal, and user types from the network and system packages, look a value up when the command needs it, and bind standard library types such as *time.Location and *user.User with app.Use.
---

This guide builds a `restart` command that sends a signal to a service on a host, as a given user. Its flags take types from three CONGA packages: `network`, `system`, and `timezone`. A program links a package only when it imports one, so these types add nothing to a binary that doesn't use them.

## Use a package's value types

Import the package and give the flag one of [its types](/reference/domain-types/#topical-domain-packages). `network.Host` holds a host name or address, and `network.Port` a port number or a service name:

```go title="main.go"
package main

import (
	"context"
	"fmt"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/network"
)

type Restart struct {
	Host conga.RequiredFlag[network.Host]
	Port conga.OptionalFlag[network.Port] `default:"https"`
}

func (r *Restart) Run(ctx context.Context, session *conga.Session) error {
	fmt.Fprintf(session.Stdout(), "restarting %s:%d\n", r.Host.Get(), r.Port.Get())

	return nil
}

func main() {
	app := conga.New[Restart]().Name("restart")

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
}
```

A service name resolves to its port, so the default `https` becomes 443:

```sh
$ restart --host api.internal
restarting api.internal:443
```

Each type brings its own help text, so the flags need no `help` tag:

```sh
$ restart --help
Usage:
  restart --host=HOST [flags]

Flags:
      --host=HOST   Host (hostname, domain, or IP address)
      --port=https  Port (1-65535, or a service name like "http")
      --help        Show help
      --version     Show version
```

`Port` rejects a number out of range or an unknown service name, as a usage error:

```sh error={2}
$ restart --host api.internal --port 70000
restart: error: invalid port 70000 (must be between 1 and 65535) for flag --port
```

```sh error={2}
$ restart --host api.internal --port httpz
restart: error: invalid port: unknown service name "httpz" for flag --port
```

`Host` accepts any text, and its `LookupIP` method resolves the name when the command needs an address.

## Accept a signal

`system.Signal` accepts a signal name or number, and rejects one the system doesn't know:

```go title="main.go" ins={7,13} {17}
import (
	"context"
	"fmt"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/network"
	"github.com/conga-sh/conga/system"
)

type Restart struct {
	Host   conga.RequiredFlag[network.Host]
	Port   conga.OptionalFlag[network.Port]  `default:"https"`
	Signal conga.OptionalFlag[system.Signal] `default:"SIGHUP"`
}

func (r *Restart) Run(ctx context.Context, session *conga.Session) error {
	fmt.Fprintf(session.Stdout(), "sending %s to %s:%d\n", r.Signal.Get(), r.Host.Get(), r.Port.Get())
	// ...
}
```

```sh
$ restart --host api.internal --signal SIGTERM
sending SIGTERM to api.internal:443
```

```sh error={2}
$ restart --host api.internal --signal SIGFOO
restart: error: unknown or invalid signal "SIGFOO" for flag --signal
```

The value keeps the text the user typed, such as `SIGTERM` or `9`, and its `Signal` method returns the `syscall.Signal` to pass to `os.Process.Signal`.

## Look up a user when the command needs it

`system.User` holds a user name. It accepts any name, and its `Lookup` method finds the account:

```go title="main.go" ins={5,9-13} {14}
type Restart struct {
	Host   conga.RequiredFlag[network.Host]
	Port   conga.OptionalFlag[network.Port]  `default:"https"`
	Signal conga.OptionalFlag[system.Signal] `default:"SIGHUP"`
	As     conga.OptionalFlag[system.User]   `default:"root"`
}

func (r *Restart) Run(ctx context.Context, session *conga.Session) error {
	account, err := r.As.Get().Lookup()
	if err != nil {
		return err
	}

	fmt.Fprintf(session.Stdout(), "sending %s to %s:%d as %s (uid %s)\n", r.Signal.Get(), r.Host.Get(), r.Port.Get(), account.Username, account.Uid)

	return nil
}
```

```sh
$ restart --host api.internal
sending SIGHUP to api.internal:443 as root (uid 0)
```

The lookup runs inside `Run`, so an unknown user is an error from the command, exit 1:

```sh error={2}
$ restart --host api.internal --as nosuchuser
restart: error: user lookup: user: unknown user nosuchuser
```

`system.Group` and `network.NetInterface` work the same way. They accept a name, and a method looks it up. Use them when the command should decide whether, and when, to look the value up.

## Bind standard library types

To have CONGA resolve the value while it reads the command line, give the flag the standard library's own type and register its parser with `app.Use`. `timezone.RegisterLocation` binds `*time.Location`, and `system.RegisterUser` binds `*user.User`:

```go title="main.go" ins={6-7,12,20} {19,24-25,31}
package main

import (
	"context"
	"fmt"
	"os/user"
	"time"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/network"
	"github.com/conga-sh/conga/system"
	"github.com/conga-sh/conga/timezone"
)

type Restart struct {
	Host   conga.RequiredFlag[network.Host]
	Port   conga.OptionalFlag[network.Port]   `default:"https"`
	Signal conga.OptionalFlag[system.Signal]  `default:"SIGHUP"`
	As     conga.OptionalFlag[*user.User]     `default:"root" help:"User to run as"`
	Zone   conga.OptionalFlag[*time.Location] `default:"UTC" help:"Time zone for log times"`
}

func (r *Restart) Run(ctx context.Context, session *conga.Session) error {
	account := r.As.Get()
	fmt.Fprintf(session.Stdout(), "sending %s to %s:%d as %s (uid %s), log times in %s\n", r.Signal.Get(), r.Host.Get(), r.Port.Get(), account.Username, account.Uid, r.Zone.Get())

	return nil
}

func main() {
	app := conga.New[Restart]().Name("restart").Use(timezone.RegisterLocation, system.RegisterUser)

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
}
```

```sh
$ restart --host api.internal --zone Europe/London
sending SIGHUP to api.internal:443 as root (uid 0), log times in Europe/London
```

The account is looked up before `Run` starts, so an unknown user is now a usage error, exit 2, like an unknown time zone:

```sh error={2}
$ restart --host api.internal --as nosuchuser
restart: error: user: unknown user nosuchuser for flag --as
```

```sh error={2}
$ restart --host api.internal --zone Mars/Base
restart: error: unknown time zone Mars/Base for flag --zone
```

Standard library types carry no help text of their own, so give these flags a `help` tag. `system.RegisterGroup`, `system.RegisterSignal`, and `network.RegisterInterface` bind `*user.Group`, `syscall.Signal`, and `*net.Interface` the same way.

:::caution[Register every standard library type]
A flag of a standard library type with no registered parser passes startup, and fails when it needs a value. With a `default`, that is every run that leaves the flag out:

```text no-copy
restart: error: unsupported type *user.User for flag --as
```
:::

## Know what users see

| What the user does | `network` and `system` types | Types bound with `app.Use` |
|---|---|---|
| Gives a value the type rejects (port 70000, `SIGFOO`) | Usage error, exit 2 | Usage error, exit 2 |
| Names a user that doesn't exist | Accepted; `Lookup` in `Run` fails, exit 1 | `unknown user nosuchuser for flag --as`, exit 2 |
| Names a time zone that doesn't exist | – | `unknown time zone Mars/Base for flag --zone`, exit 2 |
| Gives any text as a host | Accepted; `LookupIP` resolves it when called | – |

The same checks apply when the value comes from an environment variable or a config file, and the message then names the source, as in `(from environment variable RESTART_ZONE)`.

## Where to go next

- [Topical domain packages](/reference/domain-types/#topical-domain-packages): every type, binding, and predictor in the three packages.
- [About domain packages](/explanation/domain-packages/): why the packages are separate, and what they add to a binary.
- [How to add custom flag types](/how-to/add-custom-flag-types/): register a parser of your own with `RegisterType`.
