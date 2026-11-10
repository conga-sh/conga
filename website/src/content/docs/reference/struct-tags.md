---
title: Struct tags
description: Every struct tag CONGA understands, per construct.
---

CONGA derives the CLI schema from struct fields and their tags. The engine validates tags at schema initialization: unknown tags, typos, or tags on incompatible field types fail schema construction with `ErrSchema` (see [How to lint schemas](/how-to/lint-schemas/)). Recognized foreign tags (such as `json` or `yaml`) pass validation silently.

## Flags (`OptionalFlag[T]`, `RequiredFlag[T]`, `Counter`)

| Tag | Optional | Required | Counter | Meaning |
|-----|:--------:|:--------:|:-------:|---------|
| `aliases` | ✅ | ✅ | – | Comma-separated long-name aliases. |
| `and` | ✅ | ✅ | ✅ | Required co-occurrence group name whose members must all be set together. |
| `bootstrap` | ✅ | ✅ | ✅ | Bind the field before the bootstrap phase runs and expose it typed to `Bootstrap` (`bootstrap` or `bootstrap:"true"`). Requires a `conga.Bootstrapper`; rejected with `feature`; restricted to scalar, slice-of-scalar, and map-of-scalar types. |
| `config` | ✅ | ✅ | ✅ | Config file key feeding the flag (default: flag name, also matched in snake_case, so `api_endpoint` sets `--api-endpoint`). `config:"-"` excludes the field from config files. |
| `default` | ✅ | – | – | Default value when no other source provides one. Coerced with the same parser as CLI input. Rejected on required flags and `oneof` members. |
| `deprecated` | ✅ | – | ✅ | Deprecation notice; suppresses flag from completion. |
| `enum` | ✅ | ✅ | – | Comma-separated allowed values, checked against the coerced value, rendered in help as `[enum: …]`. |
| `enumhint` | ✅ | ✅ | – | Set to `-` to suppress the automatic `[enum: …]` help hint while keeping values validated and completed. |
| `env` | ✅ | ✅ | ✅ | Comma-separated environment variable names; the first one set to a non-empty value wins. `env:"-"` disables environment binding. Explicit names ignore all prefixes. |
| `feature` | ✅ | – | ✅ | Feature-gate expression: the flag exists only while the expression is satisfied (see [Feature toggles](#feature-toggles-congafeaturest)). Required flags reject this tag. |
| `help` | ✅ | ✅ | ✅ | Help text displayed on the help screen. Supports [help variables](#help-variables). |
| `hidden` | ✅ | ✅ | ✅ | Suppress from help and completion (`hidden` or `hidden:"true"`). |
| `items` | ✅ | ✅ | – | Element count bound for slices or maps (see [Bounds syntax](#bounds-syntax)). |
| `len` | ✅ | ✅ | – | Length bound for strings, slices, or maps. |
| `name` | ✅ | ✅ | ✅ | Flag name override (default: kebab-case of field name, e.g. `MaxRetries` → `--max-retries`). |
| `negatable` | ✅ (bool) | – | – | Boolean flags only: enables `--[no-]flag` syntax (`negatable` or `negatable:"true"`). |
| `oneof` | ✅ | – | ✅ | Exactly-one group name: exactly one member must be supplied by CLI, environment, or configuration. `default` is rejected on members. |
| `placeholder` | ✅ | ✅ | – | Argument placeholder displayed in syntax and help. |
| `predictor` | ✅ | ✅ | – | Completion predictor identifier (see [Predictors](#predictors)). |
| `range` | ✅ | ✅ | – | Numeric or duration bounds (see [Bounds syntax](#bounds-syntax)). |
| `safe` | ✅ | ✅ | ✅ | Controls value display in `--why` provenance (`true` or `false`). Boolean and counter flags default to `true`; other flags default to `false`. Values from `default` are always shown (see [the `--why` report](/reference/builtins/#the---why-report)). |
| `section` | ✅ | ✅ | ✅ | Help-screen section title. Flags sharing a section render under their own titled section. |
| `sep` | ✅ (slices) | ✅ (slices) | – | Slice separator (default `","`). Disabling separators with `none`, `off`, or `-` disables multi-value splitting; `sep:"none"` preserves string elements verbatim, including whitespace. |
| `short` | ✅ | ✅ | ✅ | Single-rune short flag (e.g. `short:"n"`). |
| `xor` | ✅ | – | ✅ | Mutually exclusive group name: at most one member may be supplied. Optional flags only. |

The `default` tag is rejected on `Counter` flags, `RequiredFlag`s, and `oneof` members. `xor` and `oneof` tags are also rejected on `RequiredFlag`s. Counters count command-line occurrences starting at 0; seed a base level with an `env` or config-file binding, or use `OptionalFlag[int]` for a numeric default. A required flag must receive a value from the CLI, environment, or configuration. A `default` on a `oneof` member is rejected because it sets the flag's value and marks it as set in `--why` without satisfying the `Changed()`-based exactly-one group.

Group names are scoped to the command that declares them: a same-named group on an ancestor or descendant command is an independent group, and validation, completion suppression, and the usage synopsis treat them separately. A flag may belong to one group of each kind. Every declared `xor` or `and` group needs at least two members; a one-member group is a no-op, so it fails schema construction. A `oneof` group needs at least two members unless its sole member is a `Counter`, where it means "require at least one occurrence" (use `RequiredFlag` for a single mandatory flag instead). Two members of the same `oneof` group may not also share an `and` group, and two members of the same `xor` group may not also share an `and` group: supplying either member would force the other, so the exclusive rule could never hold, and the schema rejects the pair at startup. A group whose members are all `hidden` renders no synopsis token — like a hidden required flag — but is still enforced, and the missing-group error names the hidden members.

Group membership is provenance-based: `--flag=false`, `--flag=`, `--count=0`, and `--no-flag` all count as supplying the flag because `Changed()` is true. Shell completion suppresses sibling members only after a member is present on the command line; a group satisfied by an environment variable or config file still offers siblings, and selecting one then fails at runtime.

Plain struct fields and exported non-container fields are ignored by schema building. CONGA tags on such fields are rejected at schema build instead of being silently dropped; `app.AllowTags(...)` accepts the listed keys, and tag syntax is validated on every field. Unexported embedded behavior mixins remain supported.

A map flag merges key by key across sources. A config value written as a nested map binds per key without `sep` splitting, so a `labels` section containing `hosts: a,b` keeps `a,b` as one value, while the string value `labels: hosts=a,b` is split on `sep`.

The `enum` tag validates against the field's coerced value. When a type's `UnmarshalText` normalizes input (such as `get` to `GET`), validation compares against that normalized form, so declared enum values must match the normalized output.

## Positional arguments (`OptionalArg[T]`, `RequiredArg[T]`)

Positional arguments bind to command-line input. They belong to leaf commands: a command that declares positional arguments cannot also declare `Command[T]` subcommands, and schema build rejects the mix with `ErrSchema`. Declare positionals on the leaf that executes, and pass shared context through inherited flags instead. Required arguments must come before optional ones. An optional argument declared before a required one fails schema build with `ErrSchema` (`optional positional argument cannot precede required positional argument <name>`).

| Tag | Optional | Required | Meaning |
|-----|:--------:|:--------:|---------|
| `bootstrap` | ✅ | ✅ | Bind the argument before the bootstrap phase runs and expose it typed to `Bootstrap`. Same restrictions as bootstrap flags. |
| `default` | ✅ | – | Default value for optional arguments. |
| `enum` | ✅ | ✅ | Comma-separated allowed values, checked against the coerced value, rendered in help as `[enum: …]`. |
| `enumhint` | ✅ | ✅ | Set to `-` to suppress the automatic `[enum: …]` help hint while keeping values validated and completed. |
| `help` | ✅ | ✅ | Help text. Supports [help variables](#help-variables). |
| `items` | ✅ | ✅ | Element count bound for slices or maps. |
| `len` | ✅ | ✅ | Length bound for strings, slices, or maps. |
| `name` | ✅ | ✅ | Argument placeholder in usage syntax (default: `<kebab-field-name>`). |
| `predictor` | ✅ | ✅ | Completion predictor identifier (see [Predictors](#predictors)). |
| `range` | ✅ | ✅ | Numeric or duration bounds. |
| `sep` | ✅ (slices) | ✅ (slices) | Slice separator (default `","`). Disabling separators with `none`, `off`, or `-` disables multi-value splitting; `sep:"none"` preserves string elements verbatim, including whitespace. |

## Bootstrap fields

`bootstrap` is an opt-in bridge for [`conga.Bootstrapper`](/how-to/load-domain-documents/). The field is bound in the bootstrap pass before `Bootstrap` runs, using `CLI > environment > default` precedence, and is bound again during the normal pass, which adds the config source:

```go
type CLI struct {
	Dir      conga.OptionalFlag[string] `bootstrap:"true" short:"d"`
	Taskfile conga.OptionalFlag[string] `bootstrap:"true" name:"taskfile"`
}
```

`Bootstrap` reads the typed containers directly (for example `c.Dir.Get()`); aliases, `env` tags, `EnvPrefix`, and defaults need no method-side parsing. Other fields hold their zero or caller-seeded value during the `Bootstrap` call.

Constraints:

- The root command, or an ancestor command, must implement `conga.Bootstrapper`; otherwise schema build rejects the tag.
- `bootstrap` cannot be combined with `feature`: the bootstrap phase runs before feature gates resolve.
- Allowed types are bool, string, signed/unsigned integers, floats, `time.Duration`, and slices or maps of those. Structs, pointers, and interfaces are rejected so the extra coercion stays cheap.
- Bootstrap errors are fatal for normal runs and `--why`. Completion tolerates them; `--help` runs the hook only when features are enabled, and a lone `--version` skips it. A tolerated failure restores the field to its pre-bootstrap state, and `Bootstrap` sees it unset.

`DashArgs` and `RawArgs` also accept `bootstrap`. Bootstrap fields are bound twice on a normal run, so keep them idempotent; avoid side-effecting domain types.

## Help variables

CONGA appends declared constraints after each field's help text: `range:"1..65535"` renders `[1..65535]`, `enum:"dev,prod"` renders `[enum: dev, prod]`, and `default:"8080"` displays in the flag's value slot. A `help` tag on a flag, positional argument, or config-file flag can inline those values using `{{...}}` variables, suppressing the matching automatic hint so the constraint appears once.

| Variable | Substituted text | Supported fields |
|----------|------------------|------------------|
| `{{range}}` | The `range` tag's bounds without brackets (e.g. `1..65535`, `>= 5`). | Flags and positional arguments with a `range` tag. |
| `{{len}}` | The `len` tag's bounds without brackets. | Flags and positional arguments with a `len` tag. |
| `{{items}}` | The `items` tag's bounds without brackets. | Flags and positional arguments with an `items` tag. |
| `{{enum}}` | The `enum` values joined with `, `. | Flags and positional arguments with an `enum` tag; the `Features` container substitutes the declared feature names. |
| `{{default}}` | The `default` tag's value. | Flags and positional arguments with a `default` tag. |
| `{{env}}` | The bound environment variable names joined with `, `. | Flags, config-file flags, and `Features` containers that declare a combined-map `env` tag. Per-feature variables (`envprefix` or field `env` tags) are rendered in the Features section instead. |

```go
type CLI struct {
	Port conga.OptionalFlag[int]      `range:"1..65535" help:"Port number ({{range}})"`
	Env  conga.OptionalFlag[string]   `enum:"dev,staging,prod" help:"Environment ({{enum}})"`
	Tags conga.OptionalFlag[[]string] `items:"1..5" help:"Tags ({{items}} allowed)"`
}
```

The help column renders `Port number (1..65535)`, `Environment (dev, staging, prod)`, and `Tags (1..5 allowed)`.

Suppression applies per hint: referencing `{{range}}` leaves `[len: …]`, `[items: …]`, `[enum: …]`, `[default: …]`, and `[env: …]` in place. On flags, referencing `{{default}}` also removes the default from the value slot (`--port=8080` becomes `--port=INT`), displaying the default exclusively where the help text places it. On positional arguments, `{{default}}` inlines the declared default into the prose. Config-file flags support only `{{env}}`.

Referencing an undeclared variable fails schema build with `ErrSchema`: `{{range}}` without a `range` tag, any variable other than `{{env}}` on a config-file flag, any variable other than `{{enum}}` and `{{env}}` on a `Features` container, `{{env}}` on a `Features` container without a combined-map `env` tag, or `{{env}}` on a positional argument. Unknown names fail the same way and list the six supported names. Empty constraint tags (`range:""`, `len:""`, `items:""`, and `enum:""`) are rejected at schema build unconditionally. Bare `{` and shell-style `${...}` sequences copy verbatim; double the opening braces (`{{{{`) to render a literal `{{`.

### Suppressing the enum hint

Long value lists can overwhelm a help line. Set `enumhint:"-"` on a flag, positional argument, or `Features` container to suppress the automatic `[enum: ...]` hint while keeping the values validated and completed. The tag's only accepted value is `-`; it is rejected on fields that declare no enum values, and it conflicts with a `help` tag that already references `{{enum}}` (inlining the values and hiding them are contradictory requests).

## Subcommands (`Command[T]`)

A command that declares `Command[T]` fields is a branch: it routes to its subcommands and may not also declare positional arguments. Built-in help, version, and completion command fields do not count as branching.

| Tag | Meaning |
|-----|---------|
| `aliases` | Comma-separated command aliases. |
| `deprecated` | Deprecation notice displayed in help. |
| `envprefix` | Environment prefix for this command's flags. Replaces any inherited prefix. Appends a trailing underscore if omitted. |
| `feature` | Feature-gate expression: the subcommand and its subtree exist only while the expression is satisfied (see [Feature toggles](#feature-toggles-congafeaturest)). |
| `hidden` | Suppress from help and completion (`hidden` or `hidden:"true"`). |
| `implicit` | Route tokens that no explicit subcommand claims into this subcommand. At most one per command. See [Implicit subcommands](#implicit-subcommands). |
| `name` | Subcommand token override (default: kebab-case field name). |
| `section` | Help-screen section title. Subcommands sharing a section render under their own titled section and are tagged as `<section> commands` in completion. |
| `summary` | One-line synopsis displayed in parent command listings and shell completion. |

Subcommands without an `envprefix` tag inherit their parent's prefix. Set the root command's prefix with `app.EnvPrefix(...)`.

A subcommand's flags read config keys from a section named after the subcommand, nested one level per command. `--concurrency` on `deploy worker` reads `deploy` → `worker` → `concurrency`, which is a `[deploy.worker]` table in TOML. A dotted key such as `deploy.worker.concurrency: 4` in YAML is an unknown field (`ErrConfig`, exit 1). [Nest subcommand settings](/how-to/manage-config-files/#nest-subcommand-settings) shows the layout.

### Command tree rules

| Situation | Result |
|---|---|
| A flag name, alias, or short rune declared on an ancestor is declared again on a descendant | Schema build fails with `ErrSchema` wrapping `ErrDuplicateFlag`: `duplicate flag: --verbose (already declared by ancestor command "tool")`. Sibling commands may share names. |
| A descendant's short rune matches the rune given to `app.HelpShortFlag` or `app.VersionShortFlag` | Schema build fails: `duplicate flag: built-in --help short flag -h collides with --host; assign a different rune to one of them`. |
| A branch that doesn't implement `Runner` is invoked with no subcommand | `subcommand required for "<name>"` and the command's help screen, exit 2. |
| A branch that implements `Runner` is invoked with no subcommand | The branch's own `Run` executes. |
| A leaf that doesn't implement `Runner` is invoked | `runner not implemented: command "<name>" does not implement conga.Runner`, exit 1. Its siblings still run. `conga-lint` reports every such leaf. |
| A subcommand off the selected path | Its containers receive no values and its lifecycle hooks don't run. `Command[T].IsSelected()` reports whether a subcommand is on the path. |
| A subcommand whose `feature` expression is false | See [Feature runtime behavior](#feature-runtime-behavior). |

### Implicit subcommands

`implicit:"true"` marks the subcommand that receives tokens no explicit subcommand claims. The examples below use this schema, where `-n` is declared only by `run`:

```go
type CLI struct {
	Exec    conga.Command[RunCmd]     `implicit:"true" name:"run"`
	Install conga.Command[InstallCmd]
}

type RunCmd struct {
	DryRun conga.OptionalFlag[bool] `short:"n"`
	File   conga.RequiredArg[conga.File]
	Args   conga.RawArgs
}
```

#### Schema rules

- The value is a strict boolean, `"true"` or `"false"`; an empty value means `true`.
- A command declares at most one implicit subcommand. A second fails schema build with `ErrSchema` and names the field that declared the first; `conga-lint` reports the same error.
- An implicit subcommand may declare its own implicit subcommand.

#### Help rendering

- The parent usage line shows the command token as optional: `tool [<command>]`.
- The parent's command listing marks the implicit subcommand `(default)`. Declaration order is unchanged.

#### Selection

A subcommand named explicitly, by name or alias, always takes precedence. Otherwise the implicit subcommand is selected when any of these holds:

- An operand matches no subcommand name or alias. A near-miss of a sibling name is an operand, so no "did you mean ...?" suggestion is offered.
- Operands follow `--`, unless the parent implements `Runner` and declares `DashArgs`; that parent binds them itself.
- A flag declared only by the implicit subtree is supplied. Selection descends through nested implicit subcommands only as deep as those flags require.
- The invocation selects no subcommand and the parent does not implement `Runner`.

A parent that implements `Runner` runs itself when the invocation supplies no operand and no flag declared only by the implicit subtree.

| Invocation | Runs |
|------------|------|
| `tool ./script.ts` | `run`, same as `tool run ./script.ts` |
| `tool -n ./script.ts` | `run` with `-n` set |
| `tool -- -e.ts` | `run` with the file `-e.ts` |
| `tool install` | `install` |
| `tool -n install` | Fails: `unknown flag: -n` |
| `tool -n` | Fails: `missing required positional argument <file>` |

#### Token order

Routing reads tokens left to right. A subcommand's value-taking flag must follow the subcommand name: in `tool --env prod deploy`, `prod` is an unmatched operand and selects the implicit subcommand. Write `tool deploy --env prod`. Root flags and flags of commands already named may appear on either side of a subcommand name.

#### Built-in requests

`--help`, `--version`, `--why`, and `--completion` never select a subcommand by themselves. A built-in request keeps the parent active unless an operand or child-only flag has already selected the implicit subcommand.

| Invocation | Renders |
|------------|---------|
| `tool --help` | The parent's help |
| `tool --help -- ignored.ts` | The parent's help |
| `tool -n --help` | `run`'s help, same as `tool run --help` |

#### `--version` after selection

`--version` is a root-only built-in. Once an invocation resolves to a subcommand, implicitly or by name, the token is not a built-in. Until the subcommand's declared positionals are full it is an unknown flag; after that, a subcommand that declares `RawArgs` receives it as a trailing argument.

| Invocation | Result |
|------------|--------|
| `tool --version` | Prints the version summary |
| `tool -n --version` | Fails: `unknown flag: --version`, same as `tool run --version` |
| `tool --version run` | Fails: `unknown flag: --version` |
| `tool ./script.ts --version` | `run` receives `--version` in `RawArgs`, same as `tool run ./script.ts --version` |

An implicit subcommand may declare its own `version` flag or claim the `app.VersionShortFlag` rune. The root built-in still decides routing: a bare `tool --version` prints the summary, and an operand after it selects the subcommand, which then binds exactly like the named form. With a value-taking `version` flag on `run`, `tool --version script.ts` behaves like `tool run --version script.ts`, and `script.ts` becomes the flag's value.

Help and shell completion advertise `--version` only while the invocation resolves to the root command. A bound child-only flag removes it from flag-name candidates.

The design rationale is in [About subcommands and hierarchy](/explanation/subcommands/#implicit-subcommands).

## Config file flags (`ConfigFile`)

| Tag | Meaning |
|-----|---------|
| `aliases` | Comma-separated long-name aliases. |
| `config` | Configuration file key feeding the flag. `config:"-"` disables configuration binding. |
| `deprecated` | Deprecation notice; suppresses flag from completion. |
| `env` | Comma-separated environment variable names. `env:"-"` disables environment binding. |
| `feature` | Feature-gate expression: the config-file flag exists only while the expression is satisfied (see [Feature toggles](#feature-toggles-congafeaturest)). |
| `help` | Help text (default: `Path to config file`, or `Path to config file(s)` when `multi:"true"`). |
| `hidden` | Suppress from help and completion (`hidden` or `hidden:"true"`). |
| `multi` | Enable multi-value arity for repeated flags and slice values (`multi` or `multi:"true"`). |
| `name` | Flag name (default: `--config-file`). |
| `placeholder` | Value placeholder in help (default: `PATH`). |
| `section` | Help-screen section title. Flags sharing a section render under their own titled section. |
| `sep` | Multi-value flags only: value separator for repeated flags and environment variables (default `","`). |
| `short` | Single-rune short flag. |

The `default` tag is rejected on `ConfigFile` flags. Declare default search paths using [`conga.ConfigPaths(...)` or `conga.ConfigExplicit(...)`](/explanation/config-cascade/).

## Feature toggles (`conga.Features[T]`)

`conga.Features[T]` declares feature toggles on the root command struct. Each exported field of `T` is one feature: the field name converts to kebab-case (or is overridden with the `name` tag), a `bool` field is an on/off toggle, and an integer field is a leveled toggle where `0` means off and every non-zero level must be listed in the `enum` tag.

```go
type Experiments struct {
	GentleForce bool `default:"true" help:"Force all dependent tasks"`
	Syntax      int  `enum:"1,2"     help:"Syntax version (0 = off)"`
}

type CLI struct {
	Features conga.Features[Experiments] `flag:"feature-gates" env:"APP_FEATURES" config:"experiments"`

	Force  conga.OptionalFlag[bool] `feature:"gentle-force"  help:"Force all dependent tasks"`
	V2Only conga.OptionalFlag[bool] `feature:"syntax"        help:"Syntax v2 configuration"`
	Legacy conga.OptionalFlag[bool] `feature:"!gentle-force" help:"Legacy force behavior"`

	Admin conga.Command[AdminCmd] `feature:"gentle-force" summary:"Admin operations"`
}

func (c *CLI) Run(ctx context.Context, session *conga.Session) error {
	if c.Features.Get().GentleForce {
		// ...
	}

	return nil
}
```

Declare the container once, on the root command struct, and call `app.EnableFeatures()` on the application builder to activate feature resolution. Without the builder call, schema build rejects the container and every `feature` tag with `ErrSchema`. The `feature` tag is accepted on `OptionalFlag`, `Counter`, `Command[T]`, and `ConfigFile` fields; `RequiredFlag` rejects it because a required input cannot be absent. See [How to gate features](/how-to/gate-behind-features/).

### Feature container tags

| Tag | Meaning |
|-----|---------|
| `flag` | Name of the synthesized CLI map flag carrying `name=value` pairs, for example `--feature-gates gentle-force=true`. Omit the tag or set `flag:"-"` to give the container no command-line flag. |
| `env` | Comma-separated environment variable names carrying the same `name=value` pairs. This is the combined-map variable shape, and it cannot be combined with `envprefix` or with `env` tags on feature fields. |
| `envprefix` | Per-feature env prefix: each feature field binds `<PREFIX>_<UPPER_SNAKE(feature name)>` unless the field overrides it with its own `env` tag. This is the per-feature variable shape, and it cannot be combined with `env`. |
| `config` | Config section name holding the `name=value` pairs. |
| `sep` | Pair separator inside one map value (default `","`). |
| `help` | Help text for the synthesized flag. Defaults to `Enable or disable features (name=value)`; the root help screen's [Features section](/how-to/gate-behind-features/#declare-the-features) lists every feature, and `{{enum}}` (declared feature names) and `{{env}}` (the container's combined-map environment variable names) can be inlined, suppressing the matching bracket hint. `{{env}}` requires an `env` tag: a container whose only environment variables come from `envprefix` or field `env` tags rejects it at schema build. Other help variables are rejected. |
| `enumhint` | Set to `-` to suppress the synthesized flag's `[enum: ...]` help hint. Redundant — and rejected — when the `help` tag already references `{{enum}}`. |

A container that declares neither `env` nor `envprefix` binds no environment variable at all — `env:"-"` is equivalent. The application-level `EnvPrefix(...)` never reaches features; it applies to normal flags only.

### Feature field tags

| Tag | Meaning |
|-----|---------|
| `name` | Feature name override (default: kebab-case of the field name). |
| `help` | Feature description shown in the root help screen's Features section and in completion candidate descriptions. |
| `stage` | Feature maturity: `alpha`, `beta`, `ga`, or `deprecated`. Optional, but all-or-nothing — when any feature field declares a stage, every field must. Features staged `ga` or `deprecated` are omitted from the Features section. |
| `default` | Base value when no source sets the key. Integer defaults must be a declared level; `0` is valid and means off. |
| `enum` | Integer features only: comma-separated non-zero levels, required on every integer feature. `0` is reserved for off and rejected in the tag. |
| `env` | Comma-separated environment variable names binding this feature individually (first set wins; an empty value is unset). `env:"-"` excludes the field from the environment source. Valid only when the container does not declare `env` (the combined-map variable); under a container `envprefix`, the tag overrides the derived variable name. |

Environment variables are validated for collisions at schema build: two feature fields resolving to the same variable (explicitly or through prefix derivation), or a feature variable a flag also binds, fail schema build with `ErrSchema`.

### Feature expressions

A `feature` tag holds a comma-separated conjunction of terms. Each term is a feature name, optionally prefixed with `!` to negate it. The expression is satisfied when every term holds:

| Expression | Satisfied when |
|------------|----------------|
| `http3` | `http3` is on. |
| `!strict-tls` | `strict-tls` is off. |
| `http3,!strict-tls` | `http3` is on and `strict-tls` is off. |

A `bool` feature is on when its value is `true`. An integer feature is on at any non-zero level; expressions cannot test for a specific level.

### Feature value resolution

Feature names are kebab-cased. A key may use any spelling of a declared name (kebab-case, snake_case, or camelCase); keys are canonicalized before the merge, so precedence follows the source rather than the spelling.

Every source contributes `name=value` pairs, merged key by key: command line over environment over config file over the field's `default` tag. A source changes only the keys it names, so unlisted features keep their lower-source values. The container's `Source()` reports the highest source that contributed a key. See [About value precedence](/explanation/value-precedence/).

### Feature runtime behavior

Inactive gated flags and subcommands are pruned from `--help`, `--why`, and shell completion. Input that cannot be applied fails by source:

| Input | Command line | Environment | Config file |
|---|---|---|---|
| Gated flag or subcommand whose expression does not hold | Usage error (exit 2) | Ignored | `ErrConfig` (exit 1) |
| Unknown feature name | Usage error (exit 2), with suggestion | Usage error (exit 2), with suggestion | `ErrConfig` (exit 1), with suggestion |
| Invalid feature value (not `true`/`false`, or a level not in `enum`) | Usage error (exit 2) | Usage error (exit 2), naming the variable | Usage error (exit 2), naming the config key |

Only environment variables that bind *gated flags* are ignored when inactive, so ambient variables never break a run they do not control; feature variables (from `env` or `envprefix`) are always validated. See [Errors and exit codes](/reference/errors/).

## Trailing arguments (`DashArgs`, `RawArgs`)

`DashArgs` and `RawArgs` accept:

| Tag | Meaning |
|-----|---------|
| `name` | Placeholder name in usage syntax. |
| `predictor` | Completion candidates for the collected words, using the same vocabulary as flags and positional arguments (`file`, `file:*.yml`, `dir`, `exec`, `symlink`, `none`, or a registered custom name). |

An untagged collector offers no completion candidates: the collected words are opaque forwarded arguments, so the framework does not guess a domain. `DashArgs` candidates cover the words after `--`; `RawArgs` candidates cover the words captured once raw mode begins (the first overflowing positional or unknown flag after every declared required positional is filled). Forwarded words never reuse a declared positional argument's predictor, and they do not suppress previously forwarded values.

`RawArgs` is a leaf-only collector: it cannot be combined with subcommands, and every positional argument declared before it must be required and scalar. Use `DashArgs` when the command also declares subcommands. No word after `--` selects a subcommand, whether or not the command declares `DashArgs`. On a branch without one, `tool -- deploy` fails with `unexpected argument "deploy"` (exit 2). Completion follows the same boundary and offers no subcommand names after `--`. A non-runnable branch cannot execute, so its `DashArgs` only binds on an invocation that then fails the runner contract; when the branch has an implicit child, terminal routing forwards `--` operands to that child instead. Schema build and `conga-lint` reject the ambiguous combinations.

## Domain-type tags

Domain types can register custom tags they accept by implementing `conga.TagSpecProvider`. To create custom tagged types, see [How to add custom flag types](/how-to/add-custom-flag-types/).

The built-in filesystem domain types register these tags:

### `conga.File`

| Tag | Meaning |
|-----|---------|
| `executable` | Path must be an executable regular file. Bare names resolve via `$PATH` to their absolute location at parse time. Completes from `$PATH`. Mutually exclusive with `existing`, `stdin`, `stdout`, `symlink`, and `glob`. |
| `existing` | Path must exist and be a regular file. Mutually exclusive with `stdout`, `executable`, and `symlink`. |
| `glob` | Comma-separated glob patterns filtering completion candidates (e.g. `glob:"*.yaml,Makefile"`). Patterns match entry names; malformed patterns and patterns containing path separators fail at schema build. Mutually exclusive with `executable` and `symlink`. |
| `stdin` | `"-"` reads standard input. Mutually exclusive with `stdout`, `executable`, and `symlink`. |
| `stdout` | `"-"` writes standard output. Mutually exclusive with `stdin`, `existing`, `executable`, and `symlink`. |
| `symlink` | Path must be a symbolic link. Completes as symlinks. Mutually exclusive with `existing`, `stdin`, `stdout`, `executable`, and `glob`. |

The `executable` and `symlink` capabilities are complete value kinds: they exclude each other and every other capability tag, so a field declares exactly one.

### `conga.Dir`

| Tag | Meaning |
|-----|---------|
| `existing` | Path must exist and be a directory. |

### `conga.ByteSize`

| Tag | Meaning |
|-----|---------|
| `units` | Accepted unit families: `binary` (IEC `KiB`…`PiB`), `decimal` (SI `kB`…`PB`), or `both` (default). A parsed unit outside the allowed family is a coercion error. Exact byte counts (`B` suffix or bare) are family-neutral and accepted by both. |
| `display` | Render family: `binary`, `decimal`, or `input` (default, echo the entered unit family). |

```go
type CLI struct {
	Input  conga.OptionalFlag[conga.File] `existing:"true" stdin:"true"`
	Output conga.OptionalFlag[conga.File] `stdout:"true"`
	Work   conga.OptionalFlag[conga.Dir]  `existing:"true"`
}
```

Tag values are strict booleans (`"true"` or `"false"`). Runtime validation behavior and the full interface catalog are in [Domain types](/reference/domain-types/).

## Allowed foreign tags

These tags are recognized as external and pass validation silently:

`json`, `yaml`, `yml`, `toml`, `hcl`, `xml`, `mapstructure`, `envconfig`, `validate`, `binding`, `gorm`, `db`, `doc`, `example`, `description`

Register additional allowed tags at runtime with `app.AllowTags(...)`:

```go
app.AllowTags("protobuf", "storm")
```

Any other unknown tag triggers a schema error with a closest-match suggestion. Registered keys are matched exactly, so a misspelling of a registered key is still rejected.

## Bounds syntax

The `range`, `len`, and `items` tags accept these formats:

| Form | Meaning |
|------|---------|
| `"1..100"` | Inclusive minimum and maximum. |
| `"..100"` | Maximum only. |
| `"1.."` | Minimum only. |
| `"5"` | Exact value. |

Applicability rules:

| Tag | Applicable types | Validation |
|-----|------------------|------------|
| `range` | Numeric types (`int*`, `uint*` except `uintptr`, `float*`), `time.Duration`, and domain types implementing `encoding.TextUnmarshaler` with an ordered projection (`Bytes() int64` or numeric underlying kind). | Bounds parse using the type's own unmarshaler (e.g. `ByteSize` accepts `range:"1MiB..1GiB"`), under the field's declared tags: a decimal bound on a `units:"binary"` field fails at schema build. Pointer element types and unprojected types are rejected. |
| `len` | Strings, slices, and maps. | Asserts character count on strings or entry count on slices and maps. |
| `items` | Slices and maps. | Asserts element count. |

## Predictors

The `predictor:"..."` tag sets completion behavior:

| Value | Meaning |
|-------|---------|
| `file` | File completion. |
| `file:*.ext` | File completion filtered by comma-separated patterns (e.g. `file:*.go,*.mod`). |
| `dir` | Directory completion. |
| `exec` / `executable` | Executable completion from `$PATH`. |
| `symlink` / `symlinks` | Symlink completion. |
| `none` / `off` | Disable completion for the field. |
| `<name>` | Custom predictor registered via `app.Predictor(name, action)`. |

## Examples

```go
type Deploy struct {
	Replicas conga.RequiredFlag[int]           `range:"1..100" help:"Number of replicas"`
	Image    conga.RequiredFlag[string]        `placeholder:"IMAGE" help:"Container image"`
	Tag      conga.OptionalFlag[string]        `enum:"latest,stable,canary" default:"latest"`
	Files    conga.OptionalFlag[[]string]      `sep:";" items:"1..5" predictor:"file"`
	Verbose  conga.Counter                     `short:"v" help:"Increase verbosity"`
	Timeout  conga.OptionalFlag[time.Duration] `range:"100ms..5s" default:"1s"`
	Color    conga.OptionalFlag[bool]          `negatable:"true" default:"true"`
	Region   conga.OptionalFlag[string]        `env:"AWS_REGION" config:"aws_region"`
	DryRun   conga.OptionalFlag[bool]          `xor:"mode"`
	Force    conga.OptionalFlag[bool]          `xor:"mode"`
	JSON     conga.OptionalFlag[bool]          `oneof:"format"`
	YAML     conga.OptionalFlag[bool]          `oneof:"format"`
}

type Root struct {
	Deploy conga.Command[Deploy] `summary:"Deploy the application"`
	Config conga.ConfigFile      `short:"c"`
}
```
