---
title: How to use lifecycle hooks
description: Acquire resources before a command runs with PreRun, commit its work after success with PostRun, release resources on every outcome with Finally, and stop cleanly on Ctrl+C.
---

After CONGA parses and validates the command line, a command can run three hooks around `Run`:

- `PreRun` acquires resources before `Run` starts.
- `PostRun` runs follow-up work only after `Run` returns `nil`.
- `Finally` cleans up after `Run` and `PostRun`, whether they succeeded or failed.

When the user runs a subcommand, CONGA calls `PreRun` top-down from the root command to the subcommand, calls `Run` on the subcommand, and calls `PostRun` and `Finally` bottom-up from the subcommand back to the root. Only commands on the active path run their hooks, and every hook except `Run` is optional.

This guide builds `snap`, which copies a directory into a snapshot store. The root `snap` command locks the store in `PreRun` and unlocks it in `Finally`. The `snap create` subcommand creates a staging directory in `PreRun`, copies files into it in `Run`, renames it into place in `PostRun`, and deletes it in `Finally` if an earlier stage failed:

<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 950 258" role="img" aria-labelledby="lifecycle-title" class="lifecycle-diagram">
  <title id="lifecycle-title">A snap create run goes PreRun (snap then create), Run (create), PostRun (create), Finally (create then snap). If snap fails in PreRun, the run exits with status 1 without calling Finally. If create fails in PreRun, CONGA skips Run, PostRun, and create's Finally, and calls snap's Finally before exiting with status 1. If create fails in Run or PostRun, CONGA calls create's Finally and then snap's Finally before exiting with status 1. After Finally on a successful run, the program exits with status 0.</title>
  <style>
    .lifecycle-diagram { font-family: inherit; max-width: 950px; }
    .lifecycle-diagram .stage { fill: var(--sl-color-gray-6, #edeff4); stroke: var(--sl-color-accent, #3448f5); stroke-width: 1.5; }
    .lifecycle-diagram .required { stroke-width: 3.5; }
    .lifecycle-diagram .cmd { fill: var(--sl-color-black, #ffffff); stroke: var(--sl-color-accent, #3448f5); stroke-width: 1.5; }
    .lifecycle-diagram .exit { stroke-width: 1.5; }
    .lifecycle-diagram .exit.ok { fill: var(--sl-color-green-low, #defbd0); stroke: var(--sl-color-green-high, #28650b); }
    .lifecycle-diagram .exit.fail { fill: var(--sl-color-red-low, #fad1df); stroke: var(--sl-color-red-high, #8a0f3a); }
    .lifecycle-diagram .flow { fill: none; stroke: var(--sl-color-accent, #3448f5); stroke-width: 1.5; }
    .lifecycle-diagram .error { fill: none; stroke: var(--sl-color-red-high, #8a0f3a); stroke-width: 1.5; stroke-dasharray: 5 4; }
    .lifecycle-diagram .head { fill: var(--sl-color-accent, #3448f5); stroke: none; }
    .lifecycle-diagram .head.err { fill: var(--sl-color-red-high, #8a0f3a); }
    .lifecycle-diagram .title { fill: var(--sl-color-white, #17181c); font-size: 17px; font-weight: 600; text-anchor: middle; }
    .lifecycle-diagram .label { fill: var(--sl-color-white, #17181c); font-size: 14px; font-weight: 600; text-anchor: middle; dominant-baseline: central; }
    .lifecycle-diagram .note { fill: var(--sl-color-gray-3, #545964); font-size: 14px; font-style: italic; text-anchor: middle; }
  </style>
  <rect class="stage" x="10" y="20" width="236" height="112" rx="4"/>
  <text class="title" x="128" y="46">PreRun</text>
  <rect class="cmd" x="28" y="68" width="86" height="34" rx="3"/><text class="label" x="71" y="85">snap</text>
  <rect class="cmd" x="142" y="68" width="86" height="34" rx="3"/><text class="label" x="185" y="85">create</text>
  <rect class="stage required" x="278" y="20" width="122" height="112" rx="4"/>
  <text class="title" x="339" y="46">Run</text>
  <rect class="cmd" x="296" y="68" width="86" height="34" rx="3"/><text class="label" x="339" y="85">create</text>
  <text class="note" x="339" y="122">required</text>
  <rect class="stage" x="432" y="20" width="122" height="112" rx="4"/>
  <text class="title" x="493" y="46">PostRun</text>
  <rect class="cmd" x="450" y="68" width="86" height="34" rx="3"/><text class="label" x="493" y="85">create</text>
  <rect class="stage" x="586" y="20" width="236" height="112" rx="4"/>
  <text class="title" x="704" y="46">Finally</text>
  <rect class="cmd" x="604" y="68" width="86" height="34" rx="3"/><text class="label" x="647" y="85">create</text>
  <rect class="cmd" x="718" y="68" width="86" height="34" rx="3"/><text class="label" x="761" y="85">snap</text>
  <path class="flow" d="M114 85 H141"/>
  <path class="head" d="M142 85 L133 79 L133 91 Z"/>
  <path class="flow" d="M228 85 H295"/>
  <path class="head" d="M296 85 L287 79 L287 91 Z"/>
  <path class="flow" d="M382 85 H449"/>
  <path class="head" d="M450 85 L441 79 L441 91 Z"/>
  <path class="flow" d="M536 85 H603"/>
  <path class="head" d="M604 85 L595 79 L595 91 Z"/>
  <path class="flow" d="M690 85 H717"/>
  <path class="head" d="M718 85 L709 79 L709 91 Z"/>
  <path class="error" d="M71 102 V204"/>
  <path class="head err" d="M71 205 L65 196 L77 196 Z"/>
  <text class="note" x="96" y="164">error</text>
  <ellipse class="exit fail" cx="71" cy="226" rx="52" ry="21"/><text class="label" x="71" y="226">Exit 1</text>
  <path class="error" d="M339 102 V158 H647 V103"/>
  <path class="error" d="M493 102 V158"/>
  <path class="head err" d="M647 102 L641 111 L653 111 Z"/>
  <text class="note" x="570" y="150">error</text>
  <path class="error" d="M185 102 V188 H745 V103"/>
  <path class="head err" d="M745 102 L739 111 L751 111 Z"/>
  <text class="note" x="465" y="206">error</text>
  <path class="flow" d="M777 102 V204"/>
  <path class="head" d="M777 205 L771 196 L783 196 Z"/>
  <ellipse class="exit ok" cx="777" cy="226" rx="52" ry="21"/><text class="label" x="777" y="226">Exit 0</text>
  <path class="error" d="M804 85 H890 V204"/>
  <path class="head err" d="M890 205 L884 196 L896 196 Z"/>
  <text class="note" x="884" y="150" style="text-anchor: end">on error</text>
  <ellipse class="exit fail" cx="890" cy="226" rx="52" ry="21"/><text class="label" x="890" y="226">Exit 1</text>
</svg>

## Acquire and release a shared resource

To set up before a command runs and clean up afterwards, implement `conga.PreRunner` and `conga.Finalizer`:

```go
type PreRunner interface {
	PreRun(ctx context.Context, session *Session) error
}

type Finalizer interface {
	Finally(ctx context.Context, session *Session, err error) error
}
```

Here the root `CLI` struct creates a `.lock` file in `PreRun` so only one run at a time modifies the store, and removes `.lock` in `Finally`:

```go title="main.go" hidden={1-14}
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/conga-sh/conga"
)

type CLI struct {
	Store  conga.OptionalFlag[string] `default:"snapshots" help:"Directory that holds the snapshots"`
	Create conga.Command[CreateCmd]   `summary:"Snapshot a directory"`
	List   conga.Command[ListCmd]     `summary:"List snapshots"`

	lock string
}

func (c *CLI) PreRun(ctx context.Context, session *conga.Session) error {
	store := c.Store.Get()
	if err := os.MkdirAll(store, 0o755); err != nil {
		return err
	}

	lock := filepath.Join(store, ".lock")
	file, err := os.OpenFile(lock, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("%s is in use by another run (delete %s if none is running)", store, lock)
	}
	if err != nil {
		return err
	}

	c.lock = lock

	return file.Close()
}

func (c *CLI) Finally(ctx context.Context, session *conga.Session, err error) error {
	return os.Remove(c.lock)
}

func main() {
	app := conga.New[CLI]().Name("snap").Description("Keep snapshots of a directory")

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
}
```

CONGA binds only container fields, such as `OptionalFlag` and `Command`, so any ordinary struct field, exported or unexported, can hold state between hooks. Define the hook methods on a pointer receiver, as `*CLI` does here, so a value that `PreRun` sets, such as `c.lock`, is still there when `Finally` runs.

CONGA calls a command's `Finally` only when every `PreRun` from the root down to that command succeeded. When a second process tries to run while `.lock` exists, the root's `PreRun` returns an error:

```sh error={2}
$ snap create docs v2
snap: error: snapshots is in use by another run (delete snapshots/.lock if none is running)
```

Because the root's `PreRun` failed, CONGA stops before `create` and skips the root's `Finally`, so the lock owned by the active run stays in place.

## Set up and roll back in a subcommand

`CreateCmd` implements `PreRun`, `Run`, and `Finally` for its own staging directory. Because the root's `PreRun` runs first, the store is already locked when `CreateCmd.PreRun` starts.

`CreateCmd.PreRun` reads the root's `--store` flag through `session.Root[CLI]()`, returns an error if a snapshot with that name exists, and creates a `.partial-*` directory inside the store. `Run` copies the source tree into that directory. The listing collapses `copyFile`:

```go title="main.go" hidden={60-79}
type CreateCmd struct {
	Source conga.RequiredArg[conga.Dir] `existing:"true" help:"Directory to snapshot"`
	Name   conga.RequiredArg[string]    `help:"Name of the snapshot"`

	partial string
}

func (c *CreateCmd) PreRun(ctx context.Context, session *conga.Session) error {
	store := session.Root[CLI]().Store.Get()
	final := filepath.Join(store, c.Name.Get())

	if _, err := os.Stat(final); err == nil {
		return fmt.Errorf("snapshot %q already exists", c.Name.Get())
	}

	partial, err := os.MkdirTemp(store, ".partial-")
	if err != nil {
		return err
	}

	c.partial = partial

	return nil
}

func (c *CreateCmd) Run(ctx context.Context, session *conga.Session) error {
	source := c.Source.Get().String()

	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// [!callout /ctx.Err/] Stops the copy if `ctx` is cancelled.
		if err := ctx.Err(); err != nil {
			return err
		}

		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}

		target := filepath.Join(c.partial, relative)

		switch {
		case entry.IsDir():
			return os.MkdirAll(target, 0o755)
		case entry.Type()&fs.ModeSymlink != 0:
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}

			return os.Symlink(link, target)
		default:
			return copyFile(path, target)
		}
	})
}

func copyFile(source, target string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(target)
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}

	return out.Close()
}
```

`Finally` receives the error that stopped the run, or `nil` when `Run` and `PostRun` succeeded. `CreateCmd.Finally` removes the `.partial-*` directory only when something failed:

```go title="main.go"
func (c *CreateCmd) Finally(ctx context.Context, session *conga.Session, err error) error {
	if err == nil {
		return nil
	}

	if removeErr := os.RemoveAll(c.partial); removeErr != nil {
		return removeErr
	}

	fmt.Fprintln(session.Stderr(), "discarded the partial snapshot")

	return nil
}
```

When `Run` meets a file it can't read, `CreateCmd.Finally` removes the partial directory, and then `CLI.Finally` removes `.lock`. Here `docs/secret.txt` has mode `000`:

```sh error={3}
$ snap create docs v2
discarded the partial snapshot
snap: error: open docs/secret.txt: permission denied
```

When a snapshot named `v1` already exists, `snap create docs v1` fails in `CreateCmd.PreRun` before any files are copied. CONGA skips `CreateCmd.Finally`, because its `PreRun` failed before it created a `.partial-*` directory, and runs `CLI.Finally` to release `.lock`:

```sh error={2}
$ snap create docs v1
snap: error: snapshot "v1" already exists
```

If `Finally` returns an error on a run that otherwise succeeded, the run fails with that error. If `Finally` returns an error after an earlier stage failed, CONGA joins both with `errors.Join` and prints the run's error first, followed by each `Finally` error on its own line.

## Commit work after success

To run code only after `Run` returns `nil`, implement `conga.PostRunner`:

```go
type PostRunner interface {
	PostRun(ctx context.Context, session *Session) error
}
```

`CreateCmd.PostRun` renames the finished `.partial-*` directory to the snapshot's final name, so a snapshot appears in the store only after every file has been copied:

```go title="main.go"
func (c *CreateCmd) PostRun(ctx context.Context, session *conga.Session) error {
	store := session.Root[CLI]().Store.Get()
	final := filepath.Join(store, c.Name.Get())

	if err := os.Rename(c.partial, final); err != nil {
		return err
	}

	fmt.Fprintf(session.Stdout(), "created snapshot %s\n", c.Name.Get())

	return nil
}
```

`ListCmd` prints the snapshots in the store and skips `.lock` and `.partial-*`, whose names start with a dot:

```go title="main.go"
type ListCmd struct{}

func (l *ListCmd) Run(ctx context.Context, session *conga.Session) error {
	entries, err := os.ReadDir(session.Root[CLI]().Store.Get())
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() && entry.Name()[0] != '.' {
			fmt.Fprintln(session.Stdout(), entry.Name())
		}
	}

	return nil
}
```

```sh
$ snap create docs v1
created snapshot v1
$ snap list
v1
```

Renaming in `PostRun` keeps a failed rename safe. When `os.Rename` fails in `PostRun`, CONGA passes that error to `CreateCmd.Finally`, which deletes `c.partial`. A rename inside `Finally`, behind `if err == nil`, would run in the success branch, and a failure there would leave `c.partial` in the store, because no hook runs after `Finally` to remove it.

## Stop cleanly on Ctrl+C

`app.Run()` passes the hooks a context that is cancelled on Ctrl+C (`SIGINT`) or `SIGTERM`, as [Default signal context versus custom context](/explanation/lifecycle/#default-signal-context-versus-custom-context) explains. The process keeps running after the signal. To stop cleanly, a long-running hook checks `ctx` and returns its error, and the `Finally` hooks then clean up.

In `CreateCmd.Run`, the `filepath.WalkDir` callback checks `ctx.Err()` before each entry. Pressing Ctrl+C partway through a large copy returns `context.Canceled`, skips `PostRun`, removes the partial snapshot in `CreateCmd.Finally`, and releases `.lock` in `CLI.Finally`. The run exits with status 1:

```sh error={3}
$ snap create photos trip
discarded the partial snapshot
snap: error: context canceled
```

`app.RunContext(ctx)` passes the caller's context to every hook as it is and installs no signal handler. Under a context such as `context.Background()`, Ctrl+C ends the process at once, before any `Finally` hook runs, and leaves `.lock` and the `.partial-*` directory in the store. To stop cleanly under `RunContext`, build the context with `signal.NotifyContext`:

```go title="main.go" {21} ins={8,10,16-17}
import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/conga-sh/conga"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	app := conga.New[CLI]().Name("snap").Description("Keep snapshots of a directory")

	if err := app.RunContext(ctx); err != nil {
		app.Exit(err)
	}
}
```

:::caution[A command that ignores ctx ignores Ctrl+C]
Under a context that is cancelled on Ctrl+C, a command that never checks `ctx` runs until `Run` finishes, however often the user presses Ctrl+C. Check `ctx.Err()` in loops and pass `ctx` to context-aware calls such as `exec.CommandContext` and `http.NewRequestWithContext`.
:::

When `Finally` runs after Ctrl+C, `ctx` is already cancelled. If a cleanup step calls a function that takes a `context.Context`, such as one that rolls back a database transaction or deletes a remote object, pass it `context.WithoutCancel(ctx)` so the call gets a live context. Wrap that in `context.WithTimeout` when the call can hang.

## Know what users see

| What fails | `Finally` hooks that run | Exit status |
|---|---|---|
| Flag parsing, a struct tag check (such as `existing:"true"`), or `Validate` | None | 2 |
| `CLI.PreRun` (the store is locked) | None | 1 |
| `CreateCmd.PreRun` (the snapshot name already exists) | `CLI.Finally` | 1 |
| `CreateCmd.Run` (a file cannot be read) | `CreateCmd.Finally`, then `CLI.Finally` | 1 |
| `CreateCmd.PostRun` (`os.Rename` fails) | `CreateCmd.Finally`, then `CLI.Finally` | 1 |
| A `Finally` hook | The remaining `Finally` hooks, with every error joined | 1 |
| Ctrl+C under `app.Run()`, in a command that checks `ctx` | Each `Finally` whose `PreRun` succeeded, receiving `context.Canceled` | 1 |
| Ctrl+C under `app.Run()`, in a command that ignores `ctx` | Every `Finally` hook, once `Run` finishes | The command's own |
| Ctrl+C under `app.RunContext` with a context that has no signal handler | None | 130, set by the shell |

## Where to go next

- [How to validate input](/how-to/validate-input/#check-values-against-each-other): check values against each other in `Validate`, before `PreRun`.
- [About the command lifecycle](/explanation/lifecycle/): why CONGA splits a run into these stages, and how they compose across a command tree.
- [Lifecycle interfaces](/reference/conga/#lifecycle-interfaces): the interface behind each hook.
- [Lifecycle hooks example](https://github.com/conga-sh/conga/tree/main/_examples/12-lifecycle-hooks): every hook on a root and three subcommands, printing as it runs.
