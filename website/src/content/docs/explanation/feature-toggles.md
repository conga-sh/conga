---
title: About feature toggles
description: Why CONGA declares features as typed struct fields, merges their sources feature by feature, removes inactive members, and fails differently for each source.
---

In a server, a feature toggle picks a code path. In a command-line tool it also decides the interface, including whether `--quic` exists, whether `admin` is a command, what `--help` lists, what completion offers, and whether a config key is valid. A toggle that only guards an `if` in `Run` leaves the flag visible and accepted while the code behind it is switched off.

CONGA separates two things. A *feature* is a named value the user sets, declared as a field of `conga.Features[T]`. A *gated member* is a flag or subcommand whose `feature:"..."` expression decides whether it exists for this run. One feature can gate many members, gate none (and be read only in code), or gate a member while switched *off*, as `!strict-tls` does for a plaintext escape hatch.

[How to gate flags and subcommands behind features](/how-to/gate-behind-features/) shows the steps, and [Struct tags](/reference/struct-tags/#feature-toggles-congafeaturest) lists the tags.

## Struct-declared toggles versus dynamic registries

Many programs check features by name at run time, through a string-keyed registry or an environment lookup:

```go
// A lookup by name, as many feature-flag libraries do it
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

Features often come from several sources at once. A shared config file might turn on experimental features for a whole team, an environment variable might change one of them, and a flag might change another for a single run.

A scalar or slice flag takes its value from the single source that wins, so a value on the command line discards the environment's and the config file's. Features can't work that way. A user who runs with `--feature-gates http3=true` wants `http3` on, and still wants the ten other features the team's config file turned on.

CONGA instead merges features one by one, across all four sources:

```text no-copy
Struct Defaults  <  Configuration Files  <  Environment Variables  <  CLI Flags
```

The struct's field types and `default` tags give every feature a starting value. Config, then environment, then command line each lay `name=value` pairs over it, and each changes only the names it mentions. A higher source can turn a feature off or change its level, but it cannot make a lower source's setting disappear. A feature no source names keeps its default.

### Key canonicalization across sources

Each place a feature can be set has its own naming convention. Command-line flags use kebab-case (`--feature-gates gentle-force=true`), config files often use snake_case (`gentle_force: true`), environment variables use upper snake case (`GENTLE_FORCE`), and the Go field is camelCase (`GentleForce`).

CONGA maps every spelling to the declared feature name before merging, so `gentle_force` in a config file, `GENTLE_FORCE` in an environment variable, and `gentle-force` on the command line all set the same feature. Precedence follows the source, never the spelling.

### One environment shape per container

The environment source reaches features through one of two exclusive shapes, one combined map variable (the container's `env` tag) or one variable per feature (the container's `envprefix` tag). A container can't use both, because two shapes writing the same keys would need an arbitrary precedence rule between them, and the `--why` report could not say which variable won.

The application-level `EnvPrefix(...)` never reaches features. It namespaces normal flags. Features bind their environment explicitly, so an ambient prefix cannot silently turn feature names into flag overrides.

Display follows the same split. The combined-map variable is documented on the synthesized flag, where the container `help` tag may inline it with `{{env}}`. Per-feature variables are documented next to the feature they bind, in the help screen's Features section, because the synthesized flag cannot present them meaningfully when it is hidden. The two locations never list the same variable.

For the same reason, schema build rejects two features bound to one variable. The [reference](/reference/struct-tags/#feature-field-tags) lists the rules.

## Removing inactive members

A flag for an unreleased feature that still shows up in `--help` invites users to try it, and leaves them guessing why it does nothing. So when a member's `feature` expression does not hold, CONGA removes the member for that run. It is missing from `Flags:` and `Commands:` in help, from completion candidates, and from the `--why` table. The interface a user sees therefore matches what will run.

## Error semantics by source

When a source names a feature that doesn't exist, or a gated member that is inactive for the run, the result depends on the source.

### Command-line invocations (exit code 2)
The command line is what the user typed for this run, so an unknown feature name, or a flag whose feature is off, is a mistake to report to them. The run stops before any hook with a usage error (exit code 2) that names the missing feature and shows the usage.

### Configuration files (exit code 1)
A config file holds settings that outlast any one run, so a file that names an unknown feature, or sets a flag that an inactive feature removed, is wrong or out of date. Loading it fails with a config error (`ErrConfig`, exit code 1), with a "did you mean" hint for near misses.

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

Declaring feature stages is optional. However, when any field on `T` declares a `stage` tag, every field on `T` must declare one, and schema build rejects a mix with `ErrSchema`. This rule prevents ambiguous help listings, where staged and unstaged features would sit side by side with no way to tell whether a missing label means stable or forgotten.

### Mature features leave the Features section

As in Kubernetes, the automatic `Features:` section on the root help screen displays only experimental toggles (`alpha` and `beta`).

Once a feature reaches `ga` or `deprecated`, it drops out of the help screen's `Features:` table. A GA feature is expected to stay on, and a deprecated one shouldn't be advertised to new users. Features at either stage still work, validate, and complete, and `--why` still shows their state.
