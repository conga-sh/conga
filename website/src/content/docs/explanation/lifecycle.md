---
title: About the command lifecycle
description: How CONGA coordinates validation, resource acquisition, leaf execution, and guaranteed teardown across command hierarchies.
---

Command-line execution involves more than invoking a single function. A command-line program must validate input before touching disk or network resources, allocate connections, execute business logic, run post-action tasks, and guarantee cleanup even when operations fail.

CONGA divides execution into discrete lifecycle stages governed by optional marker interfaces. Each stage serves a specific operational purpose, with distinct error classifications, cancellation handling, and traversal orders across command trees.

For signature definitions and interface contracts, see [Lifecycle interfaces](/reference/conga/#lifecycle-interfaces). For testing patterns, see [How to test a command](/how-to/test-a-command/).

## The execution pipeline

Every CONGA command execution operates in two distinct phases: **schema compilation** and the **runtime lifecycle**.

Before input tokens are parsed or lifecycle hooks run, the engine compiles and validates the application schema from struct definitions and tags. Once the schema is verified, the runtime pipeline resolves values from command-line tokens, environment variables, and config files, moving through sequential phases:

```text no-copy
Schema Compilation (conga.New / buildSpec) ──[ErrSchema]──► Exit 1 (Programmer Error)
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
Cross-Field Validation (Validator, top-down)  ──[error]──► Exit 2 (Usage Error)
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

Every phase is optional except for `Runner`, which must be implemented by the selected leaf command. Unimplemented hooks are skipped with zero overhead.

## The bootstrap and configuration phase

Applications reading domain documents or flag-driven configuration face a chicken-and-egg dependency: `Bootstrap` needs command-line flags (such as `--file`, `--profile`, or `--dir`) to locate and interpret the config file, but normal flag binding depends on the config file already being loaded.

CONGA breaks the cycle by splitting binding into two passes and running the hook between them:
1. **Bootstrap pass**: Fields marked `bootstrap:"true"` bind from the command line, environment variables, and default tags onto their command structs. Configuration files are not resolved yet, so the config tier is unavailable.
2. **Bootstrap hook**: `Bootstrapper.Bootstrap` runs with those fields populated. It can load `.env` files and install a document with `Bootstrap.SetConfig`.
3. **Configuration resolution**: The engine discovers and decodes config files, then overlays the bootstrap document key by key.
4. **Normal binding pass**: CONGA resolves every flag and argument across the four-tier ladder (`CLI > environment > config > default`), including the bootstrap-tagged fields, which are bound a second time.

The normal pass is authoritative. Because the bootstrap-tagged fields are bound twice, keep them idempotent and limited to the flags needed to locate the external input.

## Interception by built-in diagnostic flags

When invoked with `--why`, CONGA runs the complete resolution pipeline through bootstrap binding, configuration loading, and flag coercion. Once all values are bound and provenance is recorded, CONGA prints the provenance report (or JSON document) and exits with code 0 before any lifecycle hooks (`Validator`, `PreRun`, `Run`, `PostRun`, `Finally`) execute.

Because `--why` evaluates real binding, invalid configuration or malformed flag values fail fast with their normal errors. Commands can also inspect provenance programmatically within lifecycle hooks by calling `session.Why()` or `session.WhyJSON()`.

## Validation before resource allocation

A central invariant of the CONGA lifecycle is that input validation must conclude before any hook acquires external resources. Opening database pools, acquiring file locks, or starting network clients before checking inputs leads to partial initialization and leaked resources when an operator enters invalid arguments.

CONGA separates validation into two distinct tiers:

### Domain-level type validation

Custom value types can implement `SelfValidator` (`Validate(ctx context.Context) error`). The engine runs this method immediately during value coercion, validating format, bounds, or internal invariants on individual values as they are decoded from CLI flags, environment variables, or config files. Because it runs during decoding, `SelfValidator` catches malformed values before the complete command structure is assembled.

### Command-level semantic validation

Command structs implement `Validator` (`Validate(ctx context.Context, session *conga.Session) error`) to verify cross-field relationships after all values have resolved. Examples include verifying that `--start-date` precedes `--end-date`, or asserting that mutually exclusive flags were not combined:

```go
func (c *DeployCommand) Validate(ctx context.Context, session *conga.Session) error {
	if c.MinReplicas.Get() > c.MaxReplicas.Get() {
		return errors.New("--min-replicas cannot exceed --max-replicas")
	}

	return nil
}
```

Validation in both tiers is strictly side-effect-free. Returning an error halts execution immediately and wraps the cause in `ErrValidation`. Because invalid input indicates an operator mistake rather than a runtime crash, the engine classifies this failure as a usage error, triggering usage guidance and terminating with exit code 2.

## Staged execution: PreRun, Run, and PostRun

Once validation passes, the engine transitions to active execution. This progression separates setup, primary work, and follow-up logic into distinct phases.

### Resource setup with PreRun

Commands implement `PreRunner` (`PreRun(ctx context.Context, session *conga.Session) error`) to establish resources that the command body requires. Typical uses include opening database connections, warming caches, authenticating remote clients, and creating scratch directories.

Because setup involves external dependencies, operations here can encounter environmental faults such as network timeouts or permission errors. Any error returned from `PreRun` halts forward progress and produces a runtime failure (exit code 1).

### Command body execution with Run

Terminal leaf commands must implement `Runner` (`Run(ctx context.Context, session *conga.Session) error`). The leaf command carries out the concrete action requested by the operator. Ancestor commands in a hierarchy act as grouping routers and context providers, leaving core business logic to the selected leaf.

### Follow-up logic with PostRun

Commands implement `PostRunner` (`PostRun(ctx context.Context, session *conga.Session) error`) for operations that should run only when the primary command succeeds. Common examples include flushing telemetry batches, sending completion notifications, updating local caches, or recording audit events.

If `Run` returns an error, `PostRun` is skipped entirely. This guarantees that follow-up logic never operates on partial or failed executions.

## Guaranteed teardown and outcome-aware cleanup

Clean teardown is essential for command-line tools that manipulate persistent state, and the cleanup handler must observe the invocation's terminal outcome.

CONGA solves this through `Finalizer` (`Finally(ctx context.Context, session *conga.Session, err error) error`).

### Visibility into the outcome

The `Finally` hook receives the terminal error returned by preceding execution phases. If `PreRun`, `Run`, or `PostRun` returned an error, that error is passed into `Finally` as the `err` argument. If all stages completed successfully, `err` is nil.

This parameter allows cleanup logic to branch on whether the operation succeeded or failed:

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

Unlike `PostRun`, `Finally` is guaranteed to run whenever a command has been initialized, regardless of whether execution succeeded or failed. Even if the process receives a termination signal (`SIGINT` or `SIGTERM`) or an intermediate hook fails, the engine invokes `Finally` to release locks, close file descriptors, and remove temporary files.

If `Finally` itself encounters an error while cleaning up after an earlier failure, the engine combines both errors using `errors.Join`, preserving both the primary execution error and the secondary cleanup failure.

## Hierarchical hook composition

In multi-command tools, an invocation activates a path through the command tree, from the root application struct down through intermediate categories to the leaf command. CONGA composes lifecycle hooks across this active chain using a nested, stack-like model.

### Top-down initialization

The engine traverses the active command chain from root to leaf, executing `Validator` and `PreRunner` hooks in top-down order. This sequence allows root commands to establish global services, such as loggers, tracing providers, or connection pools, before child commands run. Subcommands inspect these shared resources by calling `session.Root[T]()`:

```go
func (c *WorkerCommand) PreRun(ctx context.Context, session *conga.Session) error {
	root := session.Root[RootCommand]()
	c.queue = root.QueuePool.GetQueue(c.QueueName.Get())

	return nil
}
```

### Bottom-up teardown

Teardown follows the inverse order. Both `PostRun` and `Finally` unwind bottom-up, starting from the leaf command and moving back up to the root. This Last-In, First-Out order ensures that child commands release dependent resources before parent infrastructure shuts down.

### Safe partial initialization

If an error occurs during `PreRun`, forward execution stops immediately. The engine tracks which commands successfully finished their `PreRun` step in an internal initialization list.

When unwinding teardown after a `PreRun` failure, `Finally` executes strictly on commands that completed `PreRun` successfully. If an intermediate subcommand fails during setup, CONGA limits `Finally` calls to visited parent commands, preventing resource leaks while protecting cleanup handlers from uninitialized state.

## Context propagation and cancellation

The application context flows through every lifecycle stage, reaching `SelfValidator`, `Validator`, `PreRunner`, `Runner`, `PostRunner`, and `Finalizer`. Each hook receives `ctx context.Context` as its first parameter, allowing commands to observe cancellation, deadlines, and ambient metadata.

### Default signal context versus custom context

`app.Run()` executes the pipeline under a context derived from `context.Background()` using `signal.NotifyContext`, listening for `os.Interrupt` (`SIGINT` or Ctrl+C) and `syscall.SIGTERM`. When the process receives either signal, the context cancels. The signal listener detaches as soon as execution returns.

`app.RunContext(ctx)` hands control of the context entirely to the caller. CONGA passes the supplied context into the pipeline untouched, adding no signal handlers, timeouts, or wrappers of its own. Developers supply their own context whenever execution needs to follow a caller-managed lifecycle rather than the default OS signal handler.

### Cancellation and teardown guarantees

When a context cancels, the cancellation propagates through `ctx` to all active and pending hooks. Long-running operations in `PreRun` or `Run` observe cancellation by checking `ctx.Done()`, inspecting `ctx.Err()`, or passing `ctx` to cancellable I/O functions.

Returning an error from a cancelled hook stops forward progress immediately. If `PreRun` or `Run` returns after cancellation, the engine skips any remaining execution steps, including `PostRun`.

Teardown is guaranteed on cancellation: when forward execution halts, the engine unwinds through every command that finished its `PreRun` step, calling `Finally` in reverse hierarchy order. The `Finally` hook receives the cancellation error as its `err` argument, allowing cleanup code to roll back transactions, release locks, and remove temporary files before the process exits.

## Static schema verification versus runtime lifecycle

The execution pipeline governs application runtime. In contrast, schema verification occurs earlier, during application construction in `conga.New` and initial schema building.

Understanding the boundary between these two phases clarifies how errors are classified, when hooks execute, and why teardown behaves differently during startup failures.

### Two distinct failure domains

| Dimension | Schema verification | Runtime lifecycle |
|---|---|---|
| **Phase** | Application setup and schema compile | Argument parsing, validation, and hook execution |
| **Trigger** | `conga.New[T]()`, first `Run()`, or `conga-lint` | `app.Run()` or `app.RunContext(ctx)` |
| **Checks** | Struct tags, type compatibility, flag collisions, leaf `Runner` presence | Value ranges, business constraints, external resources, command actions |
| **Error category** | `ErrSchema` (Programmer mistake) | `ErrValidation` (Input mistake) or Runtime failure |
| **Teardown (`Finally`)** | **Never invoked** (pipeline never initialized) | **Guaranteed** for all initialized commands |
| **Exit code** | Exit 1 (Programmer error) | Exit 2 (Validation) or Exit 1 (Runtime) |

### Why schema failures bypass teardown

A fundamental guarantee of CONGA's lifecycle is that `Finally` runs only for commands that successfully completed resource initialization (`PreRun`).

Because schema verification occurs before token parsing or command instantiation, an `ErrSchema` failure halts the process before any command struct receives values or establishes state. Executing lifecycle cleanup hooks on an unverified command tree would risk inspecting uninitialized fields or invoking handlers on broken command graphs.

### Shifting verification to compile time

While the runtime engine catches schema errors immediately at startup, schema defects can be caught even earlier. The [`conga-lint`](/how-to/lint-schemas/) static analysis tool inspects Go AST and type declarations in CI, catching invalid tags, duplicate flags, and malformed command trees before binaries are built.
