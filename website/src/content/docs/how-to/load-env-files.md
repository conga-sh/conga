---
title: How to load .env files
description: 12-factor .env loading with expansion, quoting, and DoS-hardened limits.
---

`app.Dotenv(...)` enables zero-dependency `.env` loading into the process environment before parsing and resolution:

```go
app := conga.New[CLI]().
	Name("cloudctl").
	EnvPrefix("CLOUDCTL").
	Dotenv() // loads ".env", then ".env.local" from the working directory
```

## Semantics

- **Default behaviour** — loads `.env` followed by `.env.local` from the current directory. `.env.local` values override `.env` values, which makes the pair suitable for committed defaults + local overrides.
- **Explicit paths** — `app.Dotenv("config/defaults.env", "config/secrets.env")` loads the given files in order, later files winning.
- Loaded variables are real process environment entries, so they feed the [environment tier](/explanation/value-precedence/) of every flag — including flags whose `env` tag points at a secret.
- Syntax supports quotes (single and double) and variable expansion (`PATH=${PATH}:/extra`).
- Files are capped at 16 MB to bound memory under adversarial input.

## Errors

`.env` failures (missing explicit file, malformed line, size limit exceeded) surface as descriptive errors wrapping the `conga.ErrConfig` base category; the message identifies the offending file and line.

The default `.env`/`.env.local` pair is optional: a project with no dotenv files starts normally. Explicit paths follow the "missing file is fatal" convention of configuration loading.

:::note
`.env` files configure the *environment tier*; structured configuration belongs to [config files](/explanation/config-cascade/) — the two mechanisms compose (e.g. `CLOUDCTL_CONFIG=/etc/app.toml` inside `.env`).
:::
