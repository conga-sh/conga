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
| `Dotenv()` | `.env`, then `.env.local`, in the working directory | Silently ignored | The first definition wins; the process environment outranks both files |
| `Dotenv(path1, path2, ...)` | Each named path, in order | Fatal error wrapping `conga.ErrConfig` | The first definition wins; the process environment outranks files |

At startup, `app.Dotenv()` loads non-destructively: a key already present in the process environment is not replaced, so shell, container, and CI values outrank file values. With both conventional files present, `.env` supplies a key defined in both files, and `.env.local` contributes the keys `.env` does not define.

`session.Reload[T](ctx)` re-reads the files with overwrite semantics: file values replace existing process environment entries, and later files win over earlier ones, so edits to `.env` files apply without restarting the process.

## Grammar

| Feature | Behaviour |
|---------|-----------|
| Comments | Lines whose first non-space character is `#` are ignored; unquoted values drop a trailing `# comment` after whitespace. |
| `export` prefix | `export PORT=8080` is parsed identically to `PORT=8080`. |
| Unquoted values | Taken verbatim after inline-comment stripping. |
| Single quotes | Literal; no escape processing. |
| Double quotes | The escapes `\n`, `\r`, `\t`, `\"`, `\\`, and `\$` are evaluated; only whitespace or a comment may follow the closing quote. |
| Variable expansion | `$VAR` and `${VAR}` expand from keys defined earlier in the file first, then from the process environment. |

```ini title=".env"
BASE_DIR=/var/data
LOG_DIR=${BASE_DIR}/logs
PATH=${PATH}:/opt/bin
```

## Size limit

`.env` files are capped at 16 MB; a larger file is rejected with an error wrapping `conga.ErrConfig` for DoS hardening.
