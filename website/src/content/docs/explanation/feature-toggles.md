---
title: About feature toggles
description: Architectural design of struct-declared feature toggles, sparse gate merging, removal of inactive members, and maturity stages.
---

In a server, a feature toggle picks a code path. In a command-line tool it also decides the interface, including whether `--quic` exists, whether `admin` is a command, what `--help` lists, what completion offers, and whether a config key is valid. A toggle that only guards an `if` in `Run` leaves the flag visible and accepted while the code behind it is switched off.

CONGA separates two things. A *feature* is a named value the user sets, declared as a field of `conga.Features[T]`. A *gated member* is a flag or subcommand whose `feature:"..."` expression decides whether it exists for this run. One feature can gate many members, gate none (and be read only in code), or gate a member while switched *off*, as `!strict-tls` does for a plaintext escape hatch.

This page explains why features are typed struct fields, why their sources merge feature by feature, why inactive members are removed rather than rejected late, and why each source fails differently. For the steps, see [How to gate flags and subcommands behind features](/how-to/gate-behind-features/). For tag syntax, see [Struct tags](/reference/struct-tags/#feature-toggles-congafeaturest).

## Struct-declared toggles versus dynamic registries

Command-line frameworks frequently implement feature flags using dynamic string-based registries or ad-hoc environment lookups:

```go
// Dynamic string-based approach common in other frameworks
if featureManager.IsEnabled("http3") { ... }
```

String lookups are easy to start with and hard to maintain. A misspelled name such as `"htttp3"` compiles, runs, and quietly returns the default. Code that checks a toggle removed months ago still compiles. Names, descriptions, and defaults end up spread across handlers and config parsers, so no one place says which toggles exist.

CONGA instead declares every feature once, as a field of the struct `T` in `conga.Features[T]` on the root command:

```go
type Experiments struct {
	HTTP3    bool `stage:"alpha" help:"Experimental HTTP/3 listener"`
	SyntaxV2 int  `stage:"beta"  enum:"1,2" help:"Syntax version (0 = off)"`
}

type Server struct {
	Features conga.Features[Experiments] `flag:"feature-gates" env:"SERVER_FEATURES" config:"features"`
}
```

Handlers read features as struct fields (`c.Features.Get().HTTP3`), so renaming or removing a feature breaks the build instead of a run. The struct is the one list of names, stages, levels, and defaults, and the help screen, completion, `--why`, and `conga-lint` all read from it. The engine checks the struct when the application starts, before it reads any user input, so a malformed declaration never reaches a user.

## Sparse key-by-key map merging

Feature toggles rarely originate from a single source. A baseline configuration file might enable experimental features across a cluster, an operator might override one toggle using a local environment variable, and a developer might toggle another via a command-line flag during an ad-hoc run.

Scalar and slice flags in CONGA resolve under a winning-source model, where an explicit command-line argument discards any value provided by an environment variable or configuration file. For feature flags, a winning-source model fails. If an operator sets `--feature-gates http3=true` on the command line, they want to enable `http3` without accidentally wiping out ten other features enabled by their team's shared configuration file.

CONGA evaluates feature toggles using sparse key-by-key merging across all four sources:

```text no-copy
Struct Defaults  <  Configuration Files  <  Environment Variables  <  CLI Flags
```

The struct's field types and `default` tags give every feature a starting value. Config, then environment, then command line each lay `name=value` pairs over it, and each changes only the names it mentions. A higher source can turn a feature off or change its level, but it cannot make a lower source's setting disappear. A feature no source names keeps its default.

### Key canonicalization across sources

Human operators format keys differently across environments. Command-line flags favor kebab-case (`--feature-gates gentle-force=true`), configuration files often use snake_case (`gentle_force: true`), and Go source code uses camelCase (`GentleForce`).

CONGA canonicalizes all supplied feature keys to the declared feature name before executing the merge. Regardless of whether a configuration file sets `gentle_force`, an environment variable sets `GENTLE_FORCE`, or the command line sets `gentle-force`, all three refer to the same toggle. Precedence follows the source, never the spelling variant.

### One environment shape per container

The environment source reaches features through one of two exclusive shapes, one combined map variable (the container's `env` tag) or one variable per feature (the container's `envprefix` tag). A container can't use both, because two shapes writing the same keys would need an arbitrary precedence rule between them, and the `--why` report could not say which variable won.

The application-level `EnvPrefix(...)` never reaches features. It namespaces normal flags. Features bind their environment explicitly, so an ambient prefix cannot silently turn feature names into flag overrides.

Display follows the same split. The combined-map variable is documented on the synthesized flag, where the container `help` tag may inline it with `{{env}}`. Per-feature variables are documented next to the feature they bind, in the help screen's Features section, because the synthesized flag cannot present them meaningfully when it is hidden. The two locations never list the same variable.

For the same reason, schema build rejects two features bound to one variable. The [reference](/reference/struct-tags/#feature-field-tags) lists the rules.

## Removing inactive members

A flag for an unreleased feature that still shows up in `--help` invites users to try it, and leaves them guessing why it does nothing. So when a member's `feature` expression does not hold, CONGA removes the member for that run. It is missing from `Flags:` and `Commands:` in help, from completion candidates, and from the `--why` table. The interface a user sees therefore matches what will run.

## Error semantics by source

When an external source references a feature or gated member that cannot be resolved, CONGA applies differentiated error semantics based on the source:

### Command-line invocations (exit code 2)
The command line represents direct operator intent for the immediate run. Supplying an unknown feature key or passing a flag whose gating feature is inactive indicates an operator mistake. CONGA aborts before lifecycle execution with a usage error (exit code 2), displaying the command path, the exact missing prerequisite, and syntax usage.

### Configuration files (exit code 1)
Configuration files represent persistent settings applied to hosts or projects. A configuration file that names an unknown feature or targets a flag removed by an inactive gate indicates an invalid or stale deployment configuration. CONGA fails during the configuration loading phase with a configuration error (`ErrConfig`, exit code 1), providing "did you mean" suggestions for near misses.

A bad *value* is different. `http3: maybe` in the file is reported exactly like `--feature-gates http3=maybe` on the command line, as a usage error (exit code 2) naming the config key it came from. Every source reports a bad value the same way, for features and flags alike.

### Environment variables (silently ignored)
Environment variables leak in from parent shells, CI runners, and container orchestrators. If `API_QUIC=true` is left over in a shell where `http3` is off, failing would break a run the variable was never meant for, so CONGA ignores variables that set inactive gated flags.

Feature variables themselves are different. `API_FEATURES` or `API_FEATURE_HTTP3` exists only to set features, so an unknown name or bad value in it is a usage error (exit code 2), just as on the command line.

## Opt-in activation and linker footprint

Feature support means expression parsing, name canonicalization, per-feature merging, and provenance tracking. A small tool that never declares a feature shouldn't carry that code, so features are opt-in through `app.EnableFeatures()`. Without the call the linker drops the feature code entirely. A struct that declares features without the call fails at startup, and `conga-lint` reports it earlier, so the opt-in can't be forgotten silently.

## Feature maturity stages

CONGA models feature maturity through the `stage` struct tag:

| Stage | Intended meaning |
|---|---|
| `alpha` | Early development, usually off by default, and may change incompatibly. |
| `beta` | Functionally complete and being tested in real use, and may be on by default. |
| `ga` | Generally available, with stable and permanent behavior. |
| `deprecated` | Marked for removal in a future release. |

The meanings are guidance for the author. A stage decides only whether the feature is listed in the help screen's `Features:` section, and never sets its default.

### The all-or-nothing stage rule

Declaring feature stages is optional. However, when any field on `T` declares a `stage` tag, every field on `T` must declare one. Mixed declarations—where some features declare stages and others do not—are rejected at schema build with `ErrSchema`. This rule prevents ambiguous help listings, where staged and unstaged features would sit side by side with no way to tell whether a missing label means stable or forgotten.

### Mature features leave the Features section

As in Kubernetes, the automatic `Features:` section on the root help screen displays only experimental toggles (`alpha` and `beta`).

Once a feature reaches `ga` or `deprecated`, it drops out of the help screen's `Features:` table. GA features are expected to remain active, while deprecated features should not be advertised to new users. Both stages remain fully functional, validated, and completable, and their state continues to display in `--why` reports.
