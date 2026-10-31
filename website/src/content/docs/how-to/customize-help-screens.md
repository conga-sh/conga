---
title: How to customize help screens
description: Configure command descriptions, short flags, footers, section groups, and terminal wrapping across help screens.
---

CONGA generates help screens automatically from your command structs. This guide explains how to customize top-matter descriptions, usage examples, global footers, built-in short flags, visual section groups, and terminal width wrapping.

## Configure descriptions, examples, and footers

Configure root-level help metadata on the `App` builder:

```go title="main.go"
package main

import (
	"context"

	"github.com/conga-sh/conga"
)

type CLI struct {
	Verbose conga.OptionalFlag[bool] `help:"Enable verbose output" short:"v"`
}

func (c *CLI) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

func main() {
	app := conga.New[CLI]().
		Description("Manage cloud infrastructure and deployments.").
		Examples("cloudctl --verbose\ncloudctl deploy worker").
		HelpFooter("Documentation: https://example.com/docs\nReport issues: https://example.com/issues")

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
}
```

- `Description("...")` sets the synopsis rendered at the top of the help screen.
- `Examples("...")` renders usage lines under the `Examples:` heading.
- `HelpFooter("...")` renders text at the bottom of every help screen across the application, including subcommands.

Configure descriptions and help footers for the root command directly on the App builder (`app.Description(...)`, `app.HelpFooter(...)`). Subcommand structs configure descriptions via `Description()` methods or `summary` struct tags.

## Add subcommand descriptions and examples

Subcommands define their help metadata by implementing `Description()` and `Examples()` methods:

```go title="deploy.go"
type DeployCmd struct {
	Replicas conga.OptionalFlag[int] `default:"3" help:"Number of replicas"`
}

// Description provides the multi-line overview displayed when running 'cloudctl deploy --help'.
func (d *DeployCmd) Description() string {
	return "Deploy application workloads to target clusters."
}

// Examples provides usage lines displayed under 'Examples:'.
func (d *DeployCmd) Examples() string {
	return "cloudctl deploy --replicas=5"
}

func (d *DeployCmd) Run(ctx context.Context, session *conga.Session) error {
	return nil
}
```

On the parent struct, the `summary` tag supplies the one-line synopsis shown in parent command listings:

```go title="cli.go"
type CLI struct {
	Deploy conga.Command[DeployCmd] `summary:"Deploy workloads to clusters"`
}
```

When a subcommand struct does not implement `Description()`, its own help screen falls back to the `summary` tag.

## Assign short flags to built-in help and version

By default, CONGA injects `--help` and `--version` as long-only flags so they do not consume short runes your application flags might need. To assign single-rune short flags, call `HelpShortFlag` and `VersionShortFlag` on the builder:

```go title="main.go"
app := conga.New[CLI]().
	HelpShortFlag('h').
	VersionShortFlag('V')
```

With these short flags assigned, help output renders `-h, --help` across all commands and `-V, --version` on the root command:

```text no-copy
Flags:
  -h, --help     Show help
  -V, --version  Show version
```

Both `-h` and `-V` parse to the matching built-in actions and appear in shell completion suggestions.

If an application flag claims the same short rune, or if help and version share the same rune, schema construction fails at startup with `conga.ErrDuplicateFlag` (and `conga-lint` reports the collision statically). Disabling a built-in flag via `app.DisableHelpFlag()` or `app.DisableVersionFlag()` leaves its configured short rune inert, keeping the rune available for your own flags. User-declared `--help` or `--version` flags shadow built-in rows and render long-only.

## Organize flags and subcommands into section groups

Use the `group:"..."` tag to divide flags or subcommands into titled sections:

```go title="cli.go"
type CLI struct {
	// Subcommand grouped under 'Workload Commands:'
	Deploy conga.Command[DeployCmd] `group:"Workload" summary:"Deploy workloads"`

	// Flags grouped under 'Authentication Flags:'
	Token  conga.RequiredFlag[string] `group:"Authentication" help:"API access token"`
	Region conga.OptionalFlag[string] `group:"Authentication" help:"Target cloud region"`

	// Ungrouped flag: renders under default 'Flags:' heading
	Debug  conga.OptionalFlag[bool]   `help:"Enable debug logging"`
}
```

Help screens collect fields sharing the same group under `<Group> Commands:` or `<Group> Flags:` headers, preserving declaration order within each section.

## Format constraint hints with help variables

By default, CONGA appends declared constraints after each field's help text: a `range:"1..65535"` flag appends `[1..65535]`, and a `default:"8080"` flag displays `8080` in the value slot.

To state a constraint once in your own prose, inline it using `{{...}}` variables in the `help` tag:

```go title="cli.go"
type CLI struct {
	Port conga.OptionalFlag[int]    `default:"8080" help:"Listen port (default {{default}}, allowed: {{range}})" range:"1..65535"`
	Env  conga.OptionalFlag[string] `enum:"dev,stage,prod" help:"Target environment ({{enum}})"`
}
```

Referencing a variable suppresses its matching automatic hint:
- `{{range}}` suppresses the `[<min>..<max>]` bracket hint.
- `{{enum}}` suppresses the `[enum: ...]` bracket hint.
- `{{items}}` and `{{len}}` suppress the `[items: ...]` and `[len: ...]` hints.
- `{{default}}` suppresses the `[default: ...]` bracket hint and removes the default from the flag's value slot (`--port=8080` becomes `--port=INT`).
- `{{env}}` suppresses the `[env: ...]` bracket hint.

Suppression applies per hint: unreferenced hints remain intact. Referencing an undeclared variable or referencing any variable other than `{{env}}` on a `ConfigFile` flag fails schema construction with `ErrSchema`. Double opening braces (`{{{{`) to print a literal `{{`.

For the complete variable catalog and availability rules, see [Struct tag reference: help variables](/reference/struct-tags/#help-variables).

## Suppress enum hints with enumhint

When a flag, positional argument, or feature container specifies a long list of allowed values, the automatic `[enum: ...]` bracket hint can crowd the help screen. Add `enumhint:"-"` to suppress the hint while keeping values validated and completable:

```go title="cli.go"
type CLI struct {
	Region conga.OptionalFlag[string] `enum:"us-east-1,us-west-1,us-west-2,eu-west-1,eu-central-1" enumhint:"-" help:"Target AWS deployment region"`
}
```

The flag enforces the allowed values during resolution and completes them during shell tab completion, but omits `[enum: ...]` from the help screen. The tag's only accepted value is `-`. Setting `enumhint:"-"` on a field without an `enum` tag, or combining it with a `help` tag that references `{{enum}}`, fails schema build with `ErrSchema`.

## Wrap help text to terminal width

CONGA wraps description text and table columns to fit the active terminal width. The wrap width resolves through three tiers:

1. `COLUMNS` environment variable: An explicit value of at least 20 columns takes precedence.
2. Terminal probing: If stdout connects to an interactive terminal, CONGA queries the terminal width via ioctl.
3. Fallback default: Piped commands, redirected output, and environments reporting no width wrap at 100 columns.

Wrapping and formatting follow these rules:

- **Cell width alignment**: Column widths and wrapping measure terminal cell width rather than byte or rune counts. Wide CJK characters, combining marks, and emojis align without column drift.
- **Preserved line breaks**: Explicit newlines and blank lines inside `help:"..."` tags or `Description()` methods are preserved, with continuation lines indented to match the description column.
- **Bracketed hint preservation**: Bracketed constraint hints such as `[env: PORT]` or `[default: 8080]` stay unbroken on a single line where possible.
- **Boolean defaults**: Flag listings display boolean defaults only when explicitly `true` (`[default: true]`). False defaults are omitted because boolean flags default to false when unset.
- **Subcommand deprecation banners**: Deprecated subcommands display a `[deprecated: <reason>]` banner at the top of their help screen.

## Automatic Features section

When the root command declares a `conga.Features[T]` container (see [How to gate features](/how-to/feature-flags/)), CONGA automatically renders a `Features:` section on the root help screen. The section lists each experimental toggle (`alpha` and `beta`), its help text, maturity stage, integer levels, and default value. Mature toggles staged `ga` or `deprecated` are omitted from the section.

The synthesized feature-gates flag automatically suppresses its own `[enum: ...]` hint because the `Features:` section already documents every available toggle.

## Query help and usage strings in commands

Command implementations can access the formatted help screen and syntax usage line as plain strings through the execution session:

```go title="guide.go"
func (g *GuideCmd) Run(ctx context.Context, session *conga.Session) error {
	helpText := session.Help()
	usageText := session.Usage()

	fmt.Fprintf(session.Stdout(), "Syntax: %s\n\n%s\n", usageText, helpText)
	return nil
}
```

- `session.Help()` returns the complete help screen string.
- `session.Usage()` returns the one-line syntax usage summary (for example, `"cloudctl deploy [flags]"`).
- Both accessors return strings without writing to streams, giving commands full control over formatting, paging, or stream routing.
