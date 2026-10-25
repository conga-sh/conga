---
title: Dotenv files
description: app.Dotenv loading rules, .env grammar, and size limits.
---

`app.Dotenv(paths...)` enables zero-dependency `.env` loading. Loaded variables populate the process environment before flag resolution, feeding tier 2 of the [value precedence ladder](/explanation/value-precedence/). The task-oriented setup is in [How to load .env files](/how-to/load-env-files/).

## Loading rules

```go
func (a *App[T]) Dotenv(paths ...string) *App[T]
```

| Call | Files | Missing file | Key conflicts |
|------|-------|--------------|---------------|
| `Dotenv()` | `.env`, then `.env.local`, in the working directory | Silently ignored | The last definition wins; the process environment outranks both files |
| `Dotenv(path1, path2, ...)` | Each named path, in order | Fatal error wrapping `conga.ErrConfig` | The last definition wins; the process environment outranks files |

`app.Dotenv()` loads non-destructively: process environment variables take precedence over file values, preserving shell, container, and CI configurations. Files apply in order, so `.env.local` overrides keys defined in `.env`, and each named path overrides preceding paths.

Loaded variables are written into the process environment through `os.Setenv`. `app.SetenvFunc` replaces that writer; tests pass `testing.T.Setenv` (`.SetenvFunc(t.Setenv)`) so loaded values revert automatically when the test finishes.

## Bootstrap-phase loading

A [`conga.Bootstrapper`](/how-to/load-domain-files/) loads `.env` files whose paths derive from bootstrap-tagged flags with `Bootstrap.Dotenv(paths...)`. It runs after bootstrap-field binding and before configuration resolution, so a path such as `filepath.Join(c.Dir.Get(), ".env")` is available. `Bootstrap.Dotenv` tolerates missing files; `Bootstrap.DotenvExplicit` fails with `conga.ErrConfig` when a named file is absent, matching `Dotenv(path1, path2, ...)`. An argument-less `Bootstrap.Dotenv()` probes `.env` and then `.env.local` in the working directory, matching `app.Dotenv()`.

Both phases share one origin map: variables already present in the host process environment are never overwritten, and a file loaded by `Bootstrap.Dotenv` overrides a key an earlier `app.Dotenv` file set. `--why` attributes an env-sourced value to its `.env` file (for example `env (sub/.env)`), while host variables render as bare `env`.

Completion invocations (`__complete`) run `Bootstrap` before resolving candidates, so env-sourced values — including [feature gates](/how-to/feature-flags/) — shape suggestions identically to a real run. Loading failures are tolerated during completion rather than aborting it; the real run surfaces them through the regular error pipeline.

## Grammar

| Feature | Behaviour |
|---------|-----------|
| Comments | Lines whose first non-space character is `#` are ignored; unquoted values drop a trailing `# comment` after whitespace. |
| `export` prefix | `export PORT=8080` is parsed identically to `PORT=8080`. |
| Unquoted values | Taken verbatim after inline-comment stripping. |
| Single quotes | Literal; escapes remain uninterpreted. |
| Double quotes | The escapes `\n`, `\r`, `\t`, `\"`, `\\`, and `\$` are evaluated; only whitespace or a comment may follow the closing quote. |
| Variable expansion | `$VAR` and `${VAR}` resolve from the process environment first, then from keys defined earlier in the file; a file overriding a key from an earlier file in the same call uses its own value. |

```ini title=".env"
BASE_DIR=/var/data
LOG_DIR=${BASE_DIR}/logs
PATH=${PATH}:/opt/bin
```

## Size limit

`.env` files are capped at 16 MB; a larger file is rejected with an error wrapping `conga.ErrConfig` for DoS hardening.
