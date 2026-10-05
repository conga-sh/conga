# Testing Guidelines & Style Guide

This document outlines the conventions and architectural style for writing tests in CONGA.

---

## 1. Single Responsibility per Test / Subtest

Every test function (`TestXxx`) or subtest (`t.Run`) must test **exactly one scenario or execution path**.

- **No Multi-Step Tests**: Never combine multiple sequential commands or executions (e.g. `// 1. ...`, `// 2. ...`) into a single test function or `t.Run` block.
- **Isolated Subtests**: When testing variations or edge cases of a feature, create dedicated `t.Run("descriptive name", func(t *testing.T) { ... })` blocks.
- **Zero Variable Shadowing/Reuse**: Each subtest must construct its own fresh inputs, schemas, and structs to guarantee zero state leakage across cases.

### Example (Good)
```go
func TestSliceEnumValidation_Issue03(t *testing.T) {
	t.Parallel()

	t.Run("valid slice enum elements", func(t *testing.T) {
		t.Parallel()

		app := conga.New[SliceEnumCLI]().Name("app").Args([]string{"--format", "json,yaml"})

		err := app.RunContext(t.Context())

		assert.NoError(t, err)
	})

	t.Run("invalid slice enum element", func(t *testing.T) {
		t.Parallel()

		app := conga.New[SliceEnumCLI]().Name("app").Args([]string{"--format", "json,xml"})

		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)
		assert.IsError(t, err, conga.ErrInvalidValue)
	})
}
```

---

## 2. Arrange-Act-Assert (AAA) Whitespace Formatting

All tests must follow a strict three-phase visual structure separated by blank lines. Keep setup, the invocation under test, and verification visually distinct without adding phase-label comments:

1. **Arrange (Setup)**:
   - Define test data, declare target structs, build schemas, and prepare input argument slices.
2. **Act (Execution)**:
   - Exactly one invocation under test (e.g. `err := app.Run()`, `err = app.RunContext(ctx)`). When the exit mapping itself is under test, `app.Exit(err)` is the invocation; the `Run` that produces its error input is setup, followed immediately by its precondition assertion.
   - Must be preceded by a blank line and followed by a blank line.
3. **Assert (Verification)**:
   - All `assert.*` statements verifying the outcome of the Act step.

```go
	cli := new(SimpleAppCLI)
	app := conga.New(cli).
		Name("testapp").
		Args([]string{"--no-cache"})

	err := app.Run()

	assert.NoError(t, err)
	assert.False(t, cli.CacheEnabled.Get())
	assert.Equal(t, conga.SourceCLI, cli.CacheEnabled.Source())
```

---

## 3. Assertion Grouping & Sub-Target Inspection

### Continuous Grouping
Assertions evaluating the direct outcome of an action must be grouped on consecutive lines without empty lines separating them:

```go
	cli := new(UserCLI)
	app := conga.New(cli).Name("app").Args(args)

	err := app.RunContext(context.Background())

	assert.NoError(t, err)
	assert.Equal(t, "postgres", cli.User.Get())
	assert.True(t, cli.Command.IsSet())
	assert.Equal(t, 5, cli.Command.Len())
```

### Logical Sub-Inspection Blocks
When extracting a helper variable to inspect a downcasted error, unwrapped value, or nested subcommand:
- Insert a blank line before the helper variable declaration.
- Keep the variable declaration and all subsequent assertions on that variable grouped on consecutive lines.

```go
	var buffer bytes.Buffer

	app := conga.New[UserCLI]().Name("app").Args([]string{"--help"}).Stdout(&buffer)

	err := app.Run()

	assert.NoError(t, err)
	assert.Contains(t, buffer.String(), "Usage:\n  testapp")
}
```

---

## 4. Assertion Library

Use `github.com/alecthomas/assert/v2` for all test assertions.

- Use `assert.NoError(t, err)` for successful operations.
- Use `assert.Error(t, err)` and `assert.IsError(t, err, sentinel)` for error checks.
- Use `assert.Equal(t, expected, actual)` for value equivalence.
- Use `assert.Zero(t, actual)` for zero values, empty slices, and empty maps instead of `assert.Equal(t, 0, len(actual))`.
- Use `assert.True(t, condition)` and `assert.False(t, condition)` for booleans.
- Do not use manual `if err != nil { t.Fatalf(...) }` checks.

---

## 5. Parallel Execution

All test functions and `t.Run` blocks should invoke `t.Parallel()` unless the test modifies process-wide state (such as `t.Setenv(...)` or process working directories).

---

## 6. Running Tests via Task

Always execute tests via `Taskfile.yaml` commands:

- **Fast unit test suite**:
  ```bash
  task test
  ```
- **Complete test suite**:
  ```bash
  task test:all
  ```
- **Test coverage report**:
  ```bash
  task coverage
  task coverage -- ./internal/coerce
  ```

---

## 7. Shell Completion Tests: User-Visible Capture Matrix

The interactive shell completion tests (`test/shell`, `shelltest` build tag) drive real shell sessions and assert on what the user actually sees. The shells expose different observable state non-interactively, so each harness result type captures the strongest signal its shell provides:

| Shell | Post-Tab buffer (`BufferTab1`/`BufferTab2`) | Purity channel | Insertion semantics |
|-------|---------------------------------------------|----------------|---------------------|
| bash | Real (Readline buffer captured by the harness) | `StdoutLeak` | Buffer + cursor capture |
| zsh | Real (`zle` buffer captured by the harness) | `Messages` | `CompstateInsert`, buffer + cursor capture |
| elvish | Simulated (derived from candidates by the harness parser) | `StdoutLeak` | Buffer derivation models prefix insertion |
| fish | Simulated (derived from candidates by the harness parser) | `Stderr` (must be empty) | Buffer derivation models prefix insertion |
| powershell | Simulated (derived from `ReplacementIndex`/candidates by the harness parser) | `Stderr` + `StdoutLeak` (must be empty) | `ReplacementIndex`/`CompletionPrefix`: insertion must start at the word being completed so earlier command-line content is never replaced |
| nushell | Simulated (derived from candidates by the harness parser) | `Stderr` (must be empty) | Buffer derivation models prefix insertion |

Test suites must keep feature parity across all six shells (candidates, filtering, attached/separate flag values, deduplication exceptions, deprecation filtering, nested subcommands, quoting, descriptions) and always assert the purity channel that the shell exposes. Documented limitations that are inherent to the host shell (not CONGA): PowerShell's parser rejects a bare trailing comma inside a command line (array operator in argument mode), so the delimited-slice deduplication contract is exercised with the backtick-escaped form (`` canary`, ``) that real users must type and that the generated snippet unwraps before invoking `__complete`; only bash and zsh observe genuine shell line-editor buffer state — the fish, powershell, and nushell post-Tab buffers are simulated by the Go test harness from returned candidates, never read from a real interactive PTY session.

---

## 8. Embedded Test Fixtures & Goldens

Test fixtures live in `testdata/` and are embedded one-to-one with `//go:embed` directives, so a missing or renamed fixture fails the build instead of failing at read time. Identifiers are category-prefixed so IDE completion lists related fixtures together and readers can tell inputs from expected output:

| Category | Pattern | Example |
|----------|---------|---------|
| Input fixture | `fixture<Subject>` | `//go:embed testdata/fixtures/native_map.toml` → `fixtureNativeMap` |
| Expected output | `golden<Subject>` | `//go:embed testdata/grouped_flags.golden` → `goldenGroupedFlags` |
| Directory corpus | `<domain>Fixtures` (`embed.FS`) | `corpusFixtures`, `elvishSnapshotFixtures` |

Rules:

- `<Subject>` is the CamelCase form of the fixture filename stem. Append the uppercase format tag only when the same stem is embedded in more than one format within the file: `fixtureNativeMapTOML` sits next to `fixtureNativeMapYAML` in `config_test.go`, while `decoder/json_test.go` needs no tag at all (`fixtureValidFlat`).
- Order the block alphabetically by embedded fixture path, so multi-format families stay contiguous and diffs stay mechanical.
- Separate families with a blank line and give each family of two or more members one period-terminated doc comment:

```go
var (
	// The same server payload in four encodings; the decoder matrix asserts
	// each decoder yields identical bindings.
	//go:embed testdata/fixtures/decoder_test.hcl
	fixtureDecoderHCL []byte

	//go:embed testdata/fixtures/decoder_test.json
	fixtureDecoderJSON []byte

	//go:embed testdata/fixtures/excluded_field.toml
	fixtureExcludedField []byte
)
```

- A file embedding exactly one fixture declares it as a bare `var`, without parentheses.
- Colliding identifiers inside one package are a compile error; repair a collision by qualifying only the colliding declaration with its owning file stem (e.g. `fixtureDecoderHCL` in `config_test.go` becomes `fixtureConfigDecoderHCL` if a second file in the package embeds the same stem), never as a default.
- Fixtures mirroring real system file layouts (`passwd`, `group`, `known_hosts`, `ssh_config`, `zone1970.tab`) keep their canonical filenames; shell harness scripts and production embedded assets are not fixtures and stay outside this convention.
- Full authoring rules — including `[]byte` vs `string` selection, the declaration-ordering interaction, and testscript separation — live in [`AGENTS.md` §3](../AGENTS.md).
