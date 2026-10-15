---
title: About feature toggles
description: Architectural design of struct-declared feature toggles, sparse gate merging, member pruning, and maturity stages.
---

Feature toggles allow software teams to merge unfinished work into main branches, run controlled canary experiments, and disable broken functionality in production without redeploying code.

In command-line tools, feature toggles must govern more than boolean branch conditions in application code. They determine which command-line flags and subcommands exist, how configuration files validate, which candidates shell completion suggests, and what help screens display.

CONGA models feature toggles as typed Go struct fields declared once on the root command struct. This page explains the architectural rationale behind this struct-declared model, the sparse key-by-key resolution cascade, member pruning, tiered error semantics, and the feature stage lifecycle. Step-by-step instructions live in [How to gate features](/how-to/feature-flags/), and tag syntax is catalogued in the [Struct tag reference](/reference/struct-tags/#feature-toggles-congafeaturest).

## Struct-declared toggles versus dynamic registries

Command-line frameworks frequently implement feature flags using dynamic string-based registries or ad-hoc environment lookups:

```go
// Dynamic string-based approach common in other frameworks
if featureManager.IsEnabled("http3") { ... }
```

While simple to start with, string-based toggles introduce several maintenance failure modes:
- **Typo vulnerability**: A misspelling in a feature name (such as `"htttp3"`) fails silently at runtime, falling back to a default value without warning.
- **Lost compile-time verification**: Code referencing an obsolete or removed toggle continues to compile and execute.
- **Scattered declarations**: Toggle names, descriptions, and defaults scatter across disparate source files and configuration handlers.

CONGA solves these issues by requiring feature toggles to be declared once on the root command struct inside a typed generic container, `conga.Features[T]`:

```go
type Experiments struct {
	HTTP3    bool `stage:"alpha" help:"Experimental HTTP/3 listener"`
	SyntaxV2 int  `stage:"beta"  enum:"1,2" help:"Syntax version (0 = off)"`
}

type Server struct {
	Features conga.Features[Experiments] `flag:"feature-gates" env:"SERVER_FEATURES" config:"features"`
}
```

Every feature is an exported field on `T`. This structure provides three concrete guarantees:
1. **Compiler-checked access**: Handlers read feature state through typed struct fields (`s.Features.Get().HTTP3`), catching field renames and removals at compile time.
2. **Single source of schema truth**: Toggle names, stages, integer levels, and default values are declared in one location, feeding the help screen generator, shell completion engine, `--why` report, and static analyzer.
3. **Deterministic schema compilation**: The engine inspects `T` during schema build, verifying field types, tag syntax, stage consistency, and enum levels before processing any input.

## Sparse key-by-key map merging

Feature toggles rarely originate from a single source. A baseline configuration file might enable experimental features across a cluster, an operator might override one toggle using a local environment variable, and a developer might toggle another via a command-line flag during an ad-hoc run.

Scalar and slice flags in CONGA resolve under a winning-tier model: an explicit command-line argument discards any value provided by an environment variable or configuration file. For feature flags, a winning-tier model fails. If an operator sets `--feature-gates http3=true` on the command line, they want to enable `http3` without accidentally wiping out ten other features enabled by their team's shared configuration file.

CONGA evaluates feature toggles using sparse key-by-key merging across all four tiers:

```text no-copy
Struct Defaults  <  Configuration Files  <  Environment Variables  <  CLI Flags
```

1. **Base defaults**: The inner feature struct defines base values through field types and `default:"..."` struct tags.
2. **Ascending key-by-key overlay**: Each subsequent tier (`config`, then `env`, then `CLI`) contributes `key=value` pairs. A higher tier overrides only the specific keys it names.
3. **No removal**: A higher tier can flip a toggle to `false` or set a different integer level, but it cannot remove a key defined in a lower tier.
4. **Sparse defaults**: Any toggle not explicitly configured across external tiers retains its base struct default.

### Key canonicalization across tiers

Human operators format keys differently across environments: command-line flags favor kebab-case (`--feature-gates gentle-force=true`), configuration files often use snake_case (`gentle_force: true`), and Go source code uses camelCase (`GentleForce`).

CONGA canonicalizes all supplied feature keys to the declared feature name before executing the merge. Regardless of whether a configuration file sets `gentle_force`, an environment variable sets `GENTLE_FORCE`, or the command line sets `gentle-force`, all three refer to the same toggle. Precedence follows the resolution tier, never the spelling variant.

## Member pruning and the active execution surface

A disabled feature should not leave ghost interfaces across your application. Exposing flags or commands for unreleased features confuses users, clutters documentation, and invites accidental misuse.

When a flag or subcommand carries a `feature:"..."` expression and that expression evaluates to false, CONGA prunes the member from the active application surface:

- **Help screens**: Inactive flags and subcommands do not appear under `Flags:` or `Commands:`.
- **Shell completion**: Shell drivers omit inactive flags and subcommands from tab-completion candidates.
- **Diagnostic reports**: Inactive members do not render in `--why` provenance tables.

Pruning ensures that the command-line interface strictly reflects what is active and executable for that run.

## Tiered error semantics

When an external source references a feature or gated member that cannot be resolved, CONGA applies differentiated error semantics based on the source tier:

### Command-line invocations (exit code 2)
The command line represents direct operator intent for the immediate run. Supplying an unknown feature key or passing a flag whose gating feature is inactive indicates an operator mistake. CONGA aborts before lifecycle execution with a usage error (exit code 2), displaying the command path, the exact missing prerequisite, and syntax usage.

### Configuration files (exit code 1)
Configuration files represent persistent settings applied to hosts or projects. A configuration file that names an unknown feature or targets a flag pruned by an inactive gate indicates an invalid or stale deployment configuration. CONGA fails during the configuration loading phase with a configuration error (`ErrConfig`, exit code 1), providing "did you mean" suggestions for near misses.

### Environment variables (silently ignored)
Environment variables often leak from parent shells, continuous integration runners, or container orchestrators. If an ambient environment variable sets an option for an inactive feature, failing the application would break unrelated runs. CONGA silently ignores environment variables targeting inactive gated flags, ensuring ambient environment noise never causes unexplained crashes.

## Opt-in activation and linker footprint

CONGA maintains a strict zero-overhead guarantee: applications link and execute only the subsystems they actually use.

The feature toggle engine includes expression parsing, case-folding tokenizers, sparse map mergers, and provenance trackers. Linking this machinery into small utilities that never declare feature toggles would unnecessarily inflate binary size.

Feature toggles are strictly opt-in:
- Applications must call `app.EnableFeatures()` on the builder.
- If a command struct declares a `conga.Features[T]` field or a `feature:"..."` struct tag without calling `EnableFeatures()`, schema compilation aborts at startup with `ErrSchema`.
- The static analyzer `conga-lint` detects this mistake at build time.

When `EnableFeatures()` is absent, the linker's dead-code elimination removes the entire feature resolution engine from the resulting binary.

## Feature maturity stages

Features follow a standard engineering lifecycle from initial prototype to stable production adoption. CONGA models this progression through the `stage` struct tag:

| Stage | Meaning | Help screen visibility |
|---|---|:---:|
| `alpha` | Early development, disabled by default, subject to breaking changes. | Shown in `Features:` section with `[alpha]` tag |
| `beta` | Functionally complete, testing in real-world scenarios, may be enabled by default. | Shown in `Features:` section with `[beta]` tag |
| `ga` | General Availability; stable, permanent feature behavior. | Omitted from `Features:` section |
| `deprecated` | Marked for removal in a future release. | Omitted from `Features:` section |

### The all-or-nothing stage rule

Declaring feature stages is optional. However, when any field on `T` declares a `stage` tag, every field on `T` must declare one. Mixed declarations—where some features declare stages and others do not—are rejected at schema build with `ErrSchema`. This invariant prevents ambiguous help screen listings and inconsistent lifecycle tracking.

### Section pruning for mature features

Following the model pioneered by Kubernetes, the automatic `Features:` section on the root help screen displays only experimental toggles (`alpha` and `beta`).

Once a feature reaches `ga` or `deprecated`, it drops out of the help screen's `Features:` table. GA features are expected to remain active, while deprecated features should not be advertised to new users. Both stages remain fully functional, validated, and completable, and their state continues to display in `--why` reports.
