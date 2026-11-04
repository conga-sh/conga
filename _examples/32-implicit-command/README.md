# 32 — Implicit command

`bunlike` mirrors Bun's shape: a file path is shorthand for the `run`
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
  bunlike [<command>]

Commands:
  run      Execute a file (default)
  install  Install dependencies
```

All of these are equivalent:

```sh
bunlike ./script.ts
bunlike run ./script.ts
```

An unmatched operand enters `run`; `install` still wins when named explicitly.
The `-n` flag belongs to `run`, so it selects `run` on its own and may appear
before the operand:

```sh
bunlike -n
bunlike -n ./script.ts
```

Without a file, `bunlike -n` reports the missing required argument instead of
an unknown flag. Help follows the selected command, so `bunlike -n --help`
renders `run`'s help screen. That screen mirrors `bunlike run --help`, so it
does not list the root-only `--version`; the root help documents it.

`--version` belongs to the root invocation. Once a command is selected, the
flag is no longer a builtin: `bunlike -n --version` fails with
`unknown flag: --version` because the raw-args fallback has not started yet
(`run` still needs its file argument), and these reject it because they name
the child explicitly:

```sh
bunlike run --version
bunlike --version run
```

After the operand fills the declared positionals, the token is a raw argument
for `run`, identical to the named form:

```sh
bunlike ./script.ts --version
bunlike run ./script.ts --version
```

Both print `run file=script.ts dry-run=false args=[--version]`. Use the bare
root form for the version summary:

```sh
bunlike --version
```

A dash-prefixed operand needs the terminator; it still reaches `run`:

```sh
bunlike -- -e.ts
```

`bunlike` on its own prints the parent help screen because the root command
implements `Runner`.
