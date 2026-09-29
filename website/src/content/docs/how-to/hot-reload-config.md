---
title: How to hot-reload configuration
description: Hot-reload configuration in daemon-style applications with the watch package.
---

`package watch` reloads configuration in daemon-style applications when watched files change. The function catalogue is in [package watch](/reference/watch/), and the resolution rules behind each reload are explained in [About the config cascade](/explanation/config-cascade/).

## Watch the app's configuration

```go
func (d *Daemon) Run(ctx context.Context, session *conga.Session) error {
	onChange := func(newCfg *CLI, err error) {
		if err != nil {
			fmt.Fprintf(session.Stderr(), "config invalid, keeping previous: %v\n", err)

			return
		}

		applyConfig(newCfg) // swap your settings atomically
	}

	return watch.Config(ctx, session, onChange)
}
```

`watch.Config[T](ctx, session, onChange)`:

- watches exactly the files reported by `session.LoadedConfigFiles()` / `session.CandidateConfigFiles()` for the active execution;
- calls `onChange` with a freshly bound, fully validated `*T` after each change (the same result as `session.Reload[T](ctx)`);
- calls `onChange` with a non-nil error when a change produces an invalid configuration — your previous settings stay in effect;
- returns `watch.ErrNoCandidates` when the candidate set is empty.

## Watch arbitrary files

```go
err := watch.File(ctx, "/etc/motd", func(err error) { ... })

err := watch.Paths(ctx, []string{"a.yaml", "b.yaml"}, func(err error) { ... })
```

`Paths` watches any file list; `File` is the single-file convenience form.

## Combining with signal shutdown

`app.Run()` cancels its context on `SIGINT`/`SIGTERM`. Block on the watch call inside your command; when the context is cancelled the watchers stop and your `Finally` hook performs teardown — see example 16 (`16-live-config-watcher`) for a complete daemon and [About the lifecycle](/explanation/lifecycle/) for teardown semantics.

:::note
`onChange` callbacks run while the application is running. Keep them fast and hand work to your own goroutines; the watcher guarantees the fresh `*T` is fully validated before delivery.
:::
