---
title: About the command lifecycle
description: How CONGA orders validation, setup, the command body, follow-up, and teardown across a command chain, and why each stage sits where it does.
---

A command must validate input before touching disk or network resources, set up connections, carry out its action, run follow-up tasks, and clean up even when an operation fails. CONGA divides execution into stages for these steps, each governed by an interface. Commands implement the lifecycle hooks, of which the selected leaf must implement `Runner` and the rest are optional. Value types can implement `SelfValidator` to validate themselves. Each stage has its own error classification, cancellation behaviour, and traversal order across the selected command chain.

[Lifecycle interfaces](/reference/conga/#lifecycle-interfaces) lists the signatures, [How to use lifecycle hooks](/how-to/use-lifecycle-hooks/) builds a command that uses them, and [How to test a command](/how-to/test-a-command/) covers testing patterns.

## The execution pipeline

Execution has two phases, schema build and the runtime lifecycle. `Run` or `RunContext` first builds and validates the schema from the struct definitions and tags. Once the schema is valid, the runtime pipeline resolves values from command-line tokens, environment variables, and config files, then runs the hooks:

```text no-copy
Schema build (Run / RunContext) ──[ErrSchema]──► Exit 1 (Programmer Error)
                │
                ▼
Bootstrap Phase (fields tagged bootstrap:"true" bound; Bootstrapper.Bootstrap runs)
                │
                ▼
Config Resolution (files discovered and decoded; SetConfig document overlaid)
                │
                ▼
Input Binding & Coercion (CLI, env, config, defaults) ──[--why]──► Exit 0 (Diagnostics)
                │
                ▼
Value Validation (SelfValidator)              ──[error]──► Exit 2 (Usage Error)
                │
                ▼
Command Validation (Validator, top-down)      ──[error]──► Exit 2 (Usage Error)
                │
                ▼
Resource Setup (PreRunner, top-down)          ──[error]──┐
                │                                        │
                ▼                                        ▼
Command Body (Runner, leaf only)              ──[error]──┼──► Teardown (Finally, bottom-up) ──► Exit 1 (Runtime Error)
                │                                        │
                ▼                                        │
Post-Execution (PostRunner, bottom-up)        ──[error]──┘
                │
                ▼
Teardown (Finally, bottom-up)                 ───────────► Exit 0 (Success)
```

## The bootstrap phase

Applications that read domain documents or flag-driven configuration face a chicken-and-egg dependency. A flag such as `--file`, `--profile`, or `--dir` locates the config file, but normal flag binding depends on the config file already being loaded. The same cycle applies to `.env` files, whose variables feed environment binding, so `Bootstrap` can load either before the second pass. Programs have usually broken the cycle by parsing their arguments twice by hand, once for the locating flags and again after loading the file. CONGA runs the two passes itself.

The first pass binds the fields tagged `bootstrap:"true"`, and `Bootstrapper.Bootstrap` runs with them set. The second pass binds every field with config files included, and is authoritative. Because the bootstrap-tagged fields are bound twice, keep them idempotent and limited to the flags needed to locate the external input. [Bootstrap fields](/reference/struct-tags/#bootstrap-fields) lists the rules.

## Interception by `--why`

`--why` runs the resolution pipeline through bootstrap binding, config loading, and flag coercion, then prints the provenance report and exits with code 0 before `SelfValidator`, `Validator`, or any later hook runs. Because it performs real binding, malformed configuration or flag values fail with their normal errors. A hook can render the same report with `session.Why()` or `session.WhyJSON()`.

## Validation before resource allocation

Input validation completes before any hook acquires external resources. Opening connections or acquiring locks before checking input leads to partial initialization and leaked resources when the input turns out to be invalid.

Validation runs at two levels after all values are bound. A value checks itself, then each command checks how its fields relate:

- A value type that implements `SelfValidator` (`Validate(ctx context.Context) error`) checks its own format, bounds, or invariants. CONGA calls it for every set flag and argument, on the final value after all sources have been merged.
- A command that implements `Validator` (`Validate(ctx context.Context, session *conga.Session) error`) checks relationships between fields, such as one flag's value exceeding another's. Validators run top-down along the selected chain.

```go
func (c *DeployCommand) Validate(ctx context.Context, session *conga.Session) error {
	if c.MinReplicas.Get() > c.MaxReplicas.Get() {
		return errors.New("--min-replicas cannot exceed --max-replicas")
	}

	return nil
}
```

Validation is intended to be side-effect-free. An error from either level wraps `ErrValidation`, and because invalid input is a user mistake rather than a runtime failure, it is classified as a usage error (exit code 2).

## PreRun, Run, and PostRun

After validation, execution separates setup, the primary action, and follow-up into distinct stages.

`PreRunner` establishes the resources the command body needs. Setup depends on external systems, so it meets environmental faults such as network timeouts or permission errors rather than input mistakes. An error from `PreRun` halts forward progress and is a runtime failure (exit code 1).

`Runner` on the selected leaf performs the action. Ancestor commands act as routers and context providers, and leave the action to the leaf.

`PostRunner` holds operations that should run only when the primary action succeeds. If `Run` returns an error, `PostRun` is skipped, so follow-up logic never operates on a failed execution.

## Guaranteed teardown and outcome-aware cleanup

Command-line tools that manipulate persistent state need cleanup that runs on every outcome, and the cleanup handler must observe that outcome. CONGA provides both through `Finalizer` (`Finally(ctx context.Context, session *conga.Session, err error) error`).

### Visibility into the outcome

`Finally` receives the terminal error of the preceding stages. If `PreRun`, `Run`, or `PostRun` returned an error, that error is the `err` argument. If all stages succeeded, `err` is nil. Cleanup logic can therefore branch on whether the operation succeeded:

```go
func (c *MigrateCommand) Finally(ctx context.Context, session *conga.Session, err error) error {
	if c.transaction != nil {
		if err != nil {
			_ = c.transaction.Rollback()
		} else {
			return c.transaction.Commit()
		}
	}

	if c.database != nil {
		return c.database.Close()
	}

	return nil
}
```

### Unconditional execution

Unlike `PostRun`, `Finally` runs for every initialized command whether execution succeeded or failed, so locks, file descriptors, and temporary files are released after a failing hook as well. Under `app.Run()`, a `SIGINT` or `SIGTERM` cancels the context instead of terminating the process, so `Finally` still runs once the active hook returns.

If `Finally` fails while cleaning up after an earlier failure, CONGA combines both errors with `errors.Join`, preserving the primary execution error and the cleanup failure.

## Hook order across the command chain

An invocation selects a path from the root to a leaf, and CONGA composes the hooks along that path as a stack.

`Validator` and `PreRunner` run top-down, from root to leaf. A root command can therefore establish shared services, such as a logger or a connection pool, before its subcommands' hooks run, and subcommands reach them through `session.Root[T]()`:

```go
func (c *WorkerCommand) PreRun(ctx context.Context, session *conga.Session) error {
	root := session.Root[RootCommand]()
	c.queue = root.QueuePool.GetQueue(c.QueueName.Get())

	return nil
}
```

`PostRun` and `Finally` run bottom-up, from leaf to root. The last-in, first-out order releases a child's resources before the parent infrastructure they depend on.

A command counts as initialized once its `PreRun` returns nil, or immediately when it has no `PreRun`. If a `PreRun` fails, forward execution stops, and `Finally` runs only on the commands initialized before it, in reverse order. The failing command's own `Finally` doesn't run, so cleanup never sees state that its setup didn't finish.

## Context and cancellation

The invocation's context flows through every stage, reaching `SelfValidator`, `Validator`, `PreRunner`, `Runner`, `PostRunner`, and `Finalizer`. Each receives `ctx context.Context` as its first parameter, so commands can observe cancellation, deadlines, and context values.

### Default signal context versus custom context

`app.Run()` executes the pipeline under a context derived from `context.Background()` with `signal.NotifyContext`, listening for `os.Interrupt` (`SIGINT`, Ctrl+C) and `syscall.SIGTERM`. When the process receives either signal, the context is cancelled. The signal listener is removed when `Run` returns.

`app.RunContext(ctx)` hands control of the context to the caller. CONGA passes the context into the pipeline unchanged, adding no signal handlers, timeouts, or wrappers. Supply your own context when execution must follow a caller-managed lifecycle rather than the default signal handling.

### Cancellation and teardown guarantees

Cancellation reaches the running hook through `ctx`. Long-running operations in `PreRun` or `Run` observe it by checking `ctx.Done()`, inspecting `ctx.Err()`, or passing `ctx` to cancellable I/O.

A hook that returns an error after cancellation stops forward progress, and the remaining stages, `PostRun` included, are skipped. Teardown then runs as for any other failure. CONGA calls `Finally` in reverse order on every command that completed its `PreRun`, with the cancellation error as `err`, so cleanup can roll back transactions, release locks, and remove temporary files before the process exits.

## Schema build versus the runtime lifecycle

Schema build validates the program's declarations, and the runtime lifecycle validates and acts on user input. The two fail differently:

| | Schema build | Runtime lifecycle |
|---|---|---|
| Runs | At the start of `Run`/`RunContext`, and statically in `conga-lint` | After schema build, on every invocation |
| Checks | Struct tags, type compatibility, flag collisions, grammar | Input values, constraints, external resources, the command's action |
| Error category | `ErrSchema` (programmer mistake) | `ErrValidation` (input mistake) or a runtime failure |
| `Finally` | Never invoked | Invoked for every initialized command |
| Exit code | 1 | 2 for usage errors, 1 for runtime failures |

A schema failure stops the run before any command struct receives values. Running cleanup hooks on an unvalidated command tree would expose them to uninitialized fields, so no hook runs. [`conga-lint`](/how-to/lint-schemas/) inspects the Go AST and type declarations in CI, and reports invalid tags, duplicate flags, and malformed command trees before the program is built.
