---
title: About the command lifecycle
description: How CONGA orders validation, setup, the command body, follow-up, and teardown across a command chain, and why each stage sits where it does.
---

A command must validate input before touching disk or network resources, set up connections, carry out its action, run follow-up tasks, and clean up even when an operation fails. CONGA divides execution into stages for these steps, each governed by an interface. Commands implement the lifecycle hooks, of which the selected leaf must implement `Runner` and the rest are optional. Value types can implement `SelfValidator` to validate themselves. Each stage has its own error classification, cancellation behavior, and traversal order across the selected command chain.

[Lifecycle interfaces](/reference/conga/#lifecycle-interfaces) lists the signatures, [How to use lifecycle hooks](/how-to/use-lifecycle-hooks/) builds a command that uses them, and [How to test a command](/how-to/test-a-command/) covers testing patterns.

## The execution pipeline

Execution has two phases, schema build and the runtime lifecycle. `Run` or `RunContext` first builds and validates the schema from the struct definitions and tags. Once the schema is valid, the runtime pipeline resolves values from command-line tokens, environment variables, and config files, then runs the hooks:

<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 780 651" role="img" aria-labelledby="pipeline-title" class="pipeline-diagram">
  <title id="pipeline-title">Execution runs ten stages in order. Schema build runs first and exits with status 1 on ErrSchema. The bootstrap phase, config resolution, and binding follow, and a --why request prints its report after binding and exits with status 0. Value validation with SelfValidator and command validation with Validator, top-down, each exit with status 2 on an error. PreRun runs top-down, Run runs on the leaf command and is the one required hook, and PostRun runs bottom-up. An error in PreRun, Run, or PostRun skips to Finally, which runs bottom-up. After Finally the program exits with status 0, or with status 1 if a stage failed.</title>
  <style>
    .pipeline-diagram { font-family: inherit; max-width: 780px; }
    .pipeline-diagram .stage { fill: var(--sl-color-gray-6, #edeff4); stroke: var(--sl-color-accent, #3448f5); stroke-width: 1.5; }
    .pipeline-diagram .required { stroke-width: 3.5; }
    .pipeline-diagram .exit { stroke-width: 1.5; }
    .pipeline-diagram .exit.ok { fill: var(--sl-color-green-low, #defbd0); stroke: var(--sl-color-green-high, #28650b); }
    .pipeline-diagram .exit.fail { fill: var(--sl-color-red-low, #fad1df); stroke: var(--sl-color-red-high, #8a0f3a); }
    .pipeline-diagram .flow { fill: none; stroke: var(--sl-color-accent, #3448f5); stroke-width: 1.5; }
    .pipeline-diagram .error { fill: none; stroke: var(--sl-color-red-high, #8a0f3a); stroke-width: 1.5; stroke-dasharray: 5 4; }
    .pipeline-diagram .head { fill: var(--sl-color-accent, #3448f5); stroke: none; }
    .pipeline-diagram .head.err { fill: var(--sl-color-red-high, #8a0f3a); }
    .pipeline-diagram .title { fill: var(--sl-color-white, #17181c); font-size: 15px; font-weight: 600; dominant-baseline: central; }
    .pipeline-diagram .label { fill: var(--sl-color-white, #17181c); font-size: 14px; font-weight: 600; text-anchor: middle; dominant-baseline: central; }
    .pipeline-diagram .note { fill: var(--sl-color-gray-3, #545964); font-size: 14px; font-style: italic; text-anchor: end; dominant-baseline: central; }
    .pipeline-diagram .edge { fill: var(--sl-color-gray-3, #545964); font-size: 14px; font-style: italic; text-anchor: middle; }
    .pipeline-diagram .edge.start { text-anchor: start; dominant-baseline: central; }
  </style>
  <rect class="stage" x="20" y="20" width="420" height="40" rx="4"/>
  <text class="title" x="36" y="40">Schema build</text>
  <text class="note" x="424" y="40">Run / RunContext</text>
  <path class="flow" d="M230 60 V75"/>
  <path class="head" d="M230 76 L224 67 L236 67 Z"/>
  <rect class="stage" x="20" y="76" width="420" height="40" rx="4"/>
  <text class="title" x="36" y="96">Bootstrap</text>
  <text class="note" x="424" y="96">Bootstrapper, bootstrap fields</text>
  <path class="flow" d="M230 116 V131"/>
  <path class="head" d="M230 132 L224 123 L236 123 Z"/>
  <rect class="stage" x="20" y="132" width="420" height="40" rx="4"/>
  <text class="title" x="36" y="152">Config resolution</text>
  <text class="note" x="424" y="152">files, SetConfig document</text>
  <path class="flow" d="M230 172 V187"/>
  <path class="head" d="M230 188 L224 179 L236 179 Z"/>
  <rect class="stage" x="20" y="188" width="420" height="40" rx="4"/>
  <text class="title" x="36" y="208">Binding and coercion</text>
  <text class="note" x="424" y="208">CLI, env, config, defaults</text>
  <path class="flow" d="M230 228 V243"/>
  <path class="head" d="M230 244 L224 235 L236 235 Z"/>
  <rect class="stage" x="20" y="244" width="420" height="40" rx="4"/>
  <text class="title" x="36" y="264">Value validation</text>
  <text class="note" x="424" y="264">SelfValidator</text>
  <path class="flow" d="M230 284 V299"/>
  <path class="head" d="M230 300 L224 291 L236 291 Z"/>
  <rect class="stage" x="20" y="300" width="420" height="40" rx="4"/>
  <text class="title" x="36" y="320">Command validation</text>
  <text class="note" x="424" y="320">Validator, top-down</text>
  <path class="flow" d="M230 340 V355"/>
  <path class="head" d="M230 356 L224 347 L236 347 Z"/>
  <rect class="stage" x="20" y="356" width="420" height="40" rx="4"/>
  <text class="title" x="36" y="376">Resource setup</text>
  <text class="note" x="424" y="376">PreRunner, top-down</text>
  <path class="flow" d="M230 396 V411"/>
  <path class="head" d="M230 412 L224 403 L236 403 Z"/>
  <rect class="stage required" x="20" y="412" width="420" height="40" rx="4"/>
  <text class="title" x="36" y="432">Command body</text>
  <text class="note" x="424" y="432">Runner, leaf only</text>
  <path class="flow" d="M230 452 V467"/>
  <path class="head" d="M230 468 L224 459 L236 459 Z"/>
  <rect class="stage" x="20" y="468" width="420" height="40" rx="4"/>
  <text class="title" x="36" y="488">Post-execution</text>
  <text class="note" x="424" y="488">PostRunner, bottom-up</text>
  <path class="flow" d="M230 508 V523"/>
  <path class="head" d="M230 524 L224 515 L236 515 Z"/>
  <rect class="stage" x="20" y="524" width="420" height="40" rx="4"/>
  <text class="title" x="36" y="544">Teardown</text>
  <text class="note" x="424" y="544">Finalizer, bottom-up</text>
  <path class="error" d="M440 40 H639"/>
  <path class="head err" d="M640 40 L631 34 L631 46 Z"/>
  <text class="edge" x="540" y="32">ErrSchema</text>
  <ellipse class="exit fail" cx="700" cy="40" rx="60" ry="18"/><text class="label" x="700" y="40">Exit 1</text>
  <path class="flow" d="M440 208 H639"/>
  <path class="head" d="M640 208 L631 202 L631 214 Z"/>
  <text class="edge" x="540" y="200">--why</text>
  <ellipse class="exit ok" cx="700" cy="208" rx="60" ry="18"/><text class="label" x="700" y="208">Exit 0</text>
  <path class="error" d="M440 264 H639"/>
  <path class="head err" d="M640 264 L631 258 L631 270 Z"/>
  <text class="edge" x="540" y="256">error</text>
  <ellipse class="exit fail" cx="700" cy="264" rx="60" ry="18"/><text class="label" x="700" y="264">Exit 2</text>
  <path class="error" d="M440 320 H639"/>
  <path class="head err" d="M640 320 L631 314 L631 326 Z"/>
  <text class="edge" x="540" y="312">error</text>
  <ellipse class="exit fail" cx="700" cy="320" rx="60" ry="18"/><text class="label" x="700" y="320">Exit 2</text>
  <path class="error" d="M440 376 H490"/>
  <path class="error" d="M440 432 H490"/>
  <path class="error" d="M440 488 H490"/>
  <path class="error" d="M490 376 V544 H441"/>
  <path class="head err" d="M440 544 L449 538 L449 550 Z"/>
  <text class="edge start" x="500" y="432">error</text>
  <path class="flow" d="M120 564 V599"/>
  <path class="head" d="M120 600 L114 591 L126 591 Z"/>
  <ellipse class="exit ok" cx="120" cy="620" rx="60" ry="19"/><text class="label" x="120" y="620">Exit 0</text>
  <path class="error" d="M340 564 V599"/>
  <path class="head err" d="M340 600 L334 591 L346 591 Z"/>
  <text class="edge start" x="352" y="581.5">if a stage failed</text>
  <ellipse class="exit fail" cx="340" cy="620" rx="60" ry="19"/><text class="label" x="340" y="620">Exit 1</text>
</svg>

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
