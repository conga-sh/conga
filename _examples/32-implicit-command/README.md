# 32 — Implicit command

`implicit-command` mirrors Bun's shape: a file path is shorthand for the `run`
subcommand, while `install` remains an explicit sibling.

```go
type CLI struct {
	Exec    conga.Command[RunCmd]     `name:"run" implicit:"true" summary:"Execute a file"`
	Install conga.Command[InstallCmd] `summary:"Install dependencies"`
}
```

The `(default)` marker appears in help without changing declaration order:

```text
Usage:
  implicit-command [<command>]

Commands:
  run      Execute a file (default)
  install  Install dependencies
```

All of these are equivalent:

```sh
implicit-command ./script.ts
implicit-command run ./script.ts
```

An unmatched operand enters `run`; `install` still wins when named explicitly.
The `-n` flag belongs to `run`, so it selects `run` on its own and may appear
before the operand:

```sh
implicit-command -n
implicit-command -n ./script.ts
```

Without a file, `implicit-command -n` reports the missing required argument instead of
an unknown flag. Help follows the selected command, so `implicit-command -n --help`
renders `run`'s help screen. That screen mirrors `implicit-command run --help`, so it
does not list the root-only `--version`; the root help documents it.

`--version` belongs to the root invocation. Once a command is selected, the
flag is no longer a builtin: `implicit-command -n --version` fails with
`unknown flag: --version` because the raw-args fallback has not started yet
(`run` still needs its file argument), and these reject it because they name
the child explicitly:

```sh
implicit-command run --version
implicit-command --version run
```

After the operand fills the declared positionals, the token is a raw argument
for `run`, identical to the named form:

```sh
implicit-command ./script.ts --version
implicit-command run ./script.ts --version
```

Both print `run file=script.ts dry-run=false args=[--version]`. Use the bare
root form for the version summary:

```sh
implicit-command --version
```

A dash-prefixed operand needs the terminator; it still reaches `run`:

```sh
implicit-command -- -e.ts
```

`implicit-command` on its own prints the parent help screen because the root command
implements `Runner`.
