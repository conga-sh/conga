---
title: How to hot-reload configuration
description: Monitor config files for live updates using watch.Config in daemon applications.
---

Long-running daemon services often require configuration updates without restarting the process. CONGA provides `package watch` to monitor config files on disk, automatically re-evaluating and re-validating configuration as files change.

## Watch application configuration

In your command's `Run` method, call `watch.Config` to monitor all active config files:

```go title="main.go"
import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/watch"
)

type Daemon struct {
	activeConfig atomic.Pointer[CLI]
}

func (d *Daemon) Run(ctx context.Context, session *conga.Session) error {
	// Store initial configuration
	d.activeConfig.Store(session.Root[CLI]())

	onChange := func(freshConfig *CLI, err error) {
		if err != nil {
			fmt.Fprintf(session.Stderr(), "config reload failed, keeping previous configuration: %v\n", err)
			return
		}

		// Swap validated configuration pointer atomically
		d.activeConfig.Store(freshConfig)
		fmt.Fprintln(session.Stdout(), "configuration reloaded successfully")
	}

	return watch.Config(ctx, session, onChange)
}
```

## Understand watcher mechanics

`watch.Config[T](ctx, session, onChange)` executes the following lifecycle:

- **File discovery**: Automatically monitors every path returned by `session.CandidateConfigFiles()` and `session.LoadedConfigFiles()`. If no candidate files were evaluated during startup, it returns `watch.ErrNoCandidates`.
- **Parent directory watching**: Monitors both the config files and their parent directories. This ensures atomic updates (such as text editors saving via temporary file renames or Kubernetes ConfigMap symlink swaps) trigger reload notifications reliably.
- **Event debouncing**: Filesystem write bursts are debounced to prevent duplicate reloads during rapid multi-step writes.
- **Atomic re-validation**: On change, the watcher executes `session.Reload[T](ctx)`. The engine re-reads files, decodes keys, and runs all validation rules (`Validator` interface, bounds, and enum checks).
- **Failure safety**: If the modified file contains invalid syntax or fails schema validation, `onChange` receives a non-nil error and a nil struct pointer. The application keeps its previous valid configuration running without interruption.
- **Concurrency**: The `onChange` callback executes on a background watcher goroutine. Protect mutable shared state using `sync.Mutex` or `atomic.Pointer[T]`.

## Integrate with graceful shutdown

`watch.Config` blocks until the provided `ctx` is cancelled. When running with `app.Run()`, CONGA automatically handles `SIGINT` and `SIGTERM` signals by cancelling the execution context:

```go title="main.go"
type Daemon struct {
	activeConfig atomic.Pointer[CLI]
}

func (d *Daemon) Run(ctx context.Context, session *conga.Session) error {
	d.activeConfig.Store(session.Root[CLI]())

	return watch.Config(ctx, session, func(freshConfig *CLI, err error) {
		if err == nil {
			d.activeConfig.Store(freshConfig)
		}
	})
}

func (d *Daemon) Finally(ctx context.Context, session *conga.Session, runErr error) error {
	fmt.Fprintln(session.Stdout(), "shutting down daemon workers")
	return nil
}
```

When a termination signal arrives:
1. CONGA cancels `ctx`.
2. `watch.Config` exits and returns `context.Canceled`.
3. CONGA executes any declared `Finally` lifecycle hooks to perform clean teardown.

## Watch arbitrary files

To monitor non-config files (such as certificates or token files), use `watch.File` or `watch.Paths`:

```go
// Watch a single file
err := watch.File(ctx, "/etc/ssl/certs/service.pem", func(err error) {
	if err == nil {
		reloadCertificates()
	}
})

// Watch multiple files
err := watch.Paths(ctx, []string{"hosts.txt", "routes.json"}, func(err error) {
	if err == nil {
		reloadRoutingTable()
	}
})
```

## Where to go next

- [Package watch reference](/reference/watch/) — API reference for `Config`, `File`, and `Paths`.
- [About the lifecycle](/explanation/lifecycle/) — execution phases, signal handling, and teardown guarantees.
- [About the config cascade](/explanation/config-cascade/) — the resolution order behind `session.Reload[T]`.
