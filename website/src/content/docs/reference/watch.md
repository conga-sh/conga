---
title: package watch
description: Live config file watching and hot reload for daemon-style CLIs.
---

API reference documentation is on [pkg.go.dev/github.com/conga-sh/conga/watch](https://pkg.go.dev/github.com/conga-sh/conga/watch); workflow guidance is in [How to hot-reload configuration](/how-to/hot-reload-config/).

## Functions

```go
func Config[T any](ctx context.Context, session *conga.Session, onChange func(newCfg *T, err error)) error

func File(ctx context.Context, filePath string, onChange func(err error)) error

func Paths(ctx context.Context, filePaths []string, onChange func(err error)) error
```

| Function | Watches | Delivers |
|----------|---------|----------|
| `Config[T]` | The execution's candidate config files | Freshly bound, validated `*T` — or the validation error |
| `File` | One file | Change notifications (parse errors surface in `err`) |
| `Paths` | Any file list | Change notifications |

## Behavior

- Each call runs its watch loop until `ctx` is cancelled; pair with `app.Run()`'s signal-aware context or supply a custom context. All three functions panic on a nil `ctx`.
- `Config[T]` requires a running execution session: a nil session returns an error, and `watch.ErrNoCandidates` is returned when the candidate set is empty.
- `Config[T]` monitors every path returned by `session.CandidateConfigFiles()`, including parent directories, so atomic updates (editor temporary-file renames, Kubernetes ConfigMap symlink swaps) trigger notifications.
- Filesystem write bursts are debounced to avoid duplicate reloads during rapid multi-step writes.
- On change, the watcher executes `session.Reload[T](ctx)`, re-reading files, decoding keys, and running validation. The `onChange` callback receives either a freshly bound, validated `*T` or an error with a nil pointer.
- Configuration errors during reload keep the watcher running with the last good configuration still effective.
- The `onChange` callback executes on a background watcher goroutine; caller access to shared state requires synchronization.
