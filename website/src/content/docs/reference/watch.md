---
title: package watch
description: Live config file watching and hot reload for daemon-style CLIs.
---

Deep detail lives on [pkg.go.dev/github.com/conga-sh/conga/watch](https://pkg.go.dev/github.com/conga-sh/conga/watch); workflow guidance is in [How to hot-reload configuration](../how-to/hot-reload-config/).

## Functions

```go
func Config[T any](ctx context.Context, session *conga.Session, onChange func(newCfg *T, err error)) error

func File(ctx context.Context, filePath string, onChange func(err error)) error

func Paths(ctx context.Context, filePaths []string, onChange func(err error)) error
```

| Function | Watches | Delivers |
|----------|---------|----------|
| `Config[T]` | The execution's loaded/candidate config files | Freshly bound, validated `*T` — or the validation error |
| `File` | One file | Change notifications (parse errors surface in `err`) |
| `Paths` | Any file list | Change notifications |

## Behaviour

- Blocking: each call runs its watch loop until `ctx` is cancelled — pair with `app.Run()`'s signal-aware context or your own. All three panic on a nil `ctx`.
- `Config[T]` requires a running execution session: a nil session returns an error, and `watch.ErrNoCandidates` is returned when the candidate set is empty.
- Configuration errors during reload keep the watcher running with the last good configuration still effective.
