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

## Watcher behavior

`watch.Config[T](ctx, session, onChange)` monitors the evaluated config files and their parent directories, debounces write bursts, and re-validates on change; [Package watch](/reference/watch/) documents the lifecycle, failure handling, and concurrency contract.

## Integrate with graceful shutdown

`watch.Config` blocks until the provided `ctx` is cancelled, so the watch loop and the process lifetime end together under `app.Run()`; [About the command lifecycle](/explanation/lifecycle/#default-signal-context-versus-custom-context) covers signal handling and `Finally` teardown.

```go title="main.go"
func (d *Daemon) Finally(ctx context.Context, session *conga.Session, runErr error) error {
	fmt.Fprintln(session.Stdout(), "shutting down daemon workers")
	return nil
}
```

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
