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
The `-n` flag belongs to `run`, so it may appear before the operand:

```sh
bunlike -n ./script.ts
```

A dash-prefixed operand needs the terminator; it still reaches `run`:

```sh
bunlike -- -e.ts
```

`bunlike` on its own prints the parent help screen because the root command
implements `Runner`.
