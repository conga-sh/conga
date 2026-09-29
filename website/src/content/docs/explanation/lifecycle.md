---
title: About the lifecycle
description: PreRun → Run → PostRun → Finally hooks, validation, and signal handling.
---

A command struct participates in a four-stage lifecycle by implementing marker interfaces. Every stage is optional; unimplemented stages are skipped.

## Hook order

```
Validate(ctx, session)     — side-effect-free constraint checks (Validator interface)
PreRun(ctx, session)       — initialize resources: open handles, connect, warm caches (PreRunner)
Run(ctx, session)          — the command body (Runner — required for a runnable command)
PostRun(ctx, session)      — follow-up logic; runs ONLY after a successful Run (PostRunner)
Finally(ctx, session, err) — guaranteed teardown; runs on success AND failure (Finalizer)
```

```go
type Import struct {
	Source conga.RequiredArg[string] `help:"Data source path"`
	Target conga.RequiredArg[string] `help:"Target path"`
	dsn    string
	pool   *sql.DB
}

func (i *Import) Validate(ctx context.Context, session *conga.Session) error {
	if i.Source.Get() == i.Target.Get() {
		return errors.New("source and target must differ")
	}

	return nil
}

func (i *Import) PreRun(ctx context.Context, session *conga.Session) error {
	pool, err := sql.Open("postgres", i.dsn)
	if err != nil {
		return err
	}

	i.pool = pool

	return nil
}

func (i *Import) Run(ctx context.Context, session *conga.Session) error {
	// ... main work, using i.pool
	return nil
}

func (i *Import) PostRun(ctx context.Context, session *conga.Session) error {
	return i.pool.Ping() // sanity check after success
}

func (i *Import) Finally(ctx context.Context, session *conga.Session, err error) error {
	if i.pool != nil {
		return i.pool.Close()
	}

	return nil
}
```

## Semantics

- **Error short-circuiting**: a stage returning an error stops the chain. `PostRun` runs only after a successful `Run`.
- **Guaranteed teardown**: `Finally(ctx, session, err)` receives the outcome error (nil on success) and always runs — the safe place for cleanup.
- **Exit codes**: an error returned from `Run` exits with status **1** (runtime failure); usage errors from earlier stages exit with **2**. See [About error reporting](/explanation/error-reporting/) and [Errors and exit codes](/reference/errors/).
- **Validation split**: implement `SelfValidator` (`Validate(ctx) error`) on *custom value types* for self-validation right after coercion, and `Validator` (`Validate(ctx, session) error`) on *command structs* for cross-field semantic checks. Both run before any hook with side effects.
- **Signals and context**: `app.Run()` installs a `signal.NotifyContext` for `SIGINT`/`SIGTERM`; cancellation surfaces as `ctx.Err()` in your hooks, while `RunContext(ctx)` passes the caller's context through untouched. See [About app execution](/explanation/execution/).

## Schema errors are programmer errors

Struct tag linting, duplicate flags, invalid argument order, and similar problems are reported as errors from `App.Run`/schema building (wrapping `ErrSchema`); each renders as a fully self-describing message, e.g.:

```
struct Serve field Port: schema error: invalid struct tag: unknown struct tag "ragne" (did you mean "range"?). ...
```

Panics are reserved for builder-API misuse (e.g. registering a predictor with an empty name).
