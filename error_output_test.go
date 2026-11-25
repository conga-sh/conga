package conga_test

import (
	"bytes"
	"context"
	"encoding"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/alecthomas/assert/v2"

	_ "embed"

	"github.com/conga-sh/conga"
	hcldecoder "github.com/conga-sh/conga/decoder/hcl"
	jsondecoder "github.com/conga-sh/conga/decoder/json"
	"github.com/conga-sh/conga/internal/errlines"
)

var (
	// Golden Exit output for every error-line layout and UsageOnError mode.
	//go:embed testdata/errors/output_group_usage_full.golden
	goldenErrorsOutputGroupUsageFull string

	//go:embed testdata/errors/output_group_usage_hint.golden
	goldenErrorsOutputGroupUsageHint string

	//go:embed testdata/errors/output_group_usage_silence.golden
	goldenErrorsOutputGroupUsageSilence string

	//go:embed testdata/errors/output_group_usage_summary.golden
	goldenErrorsOutputGroupUsageSummary string

	//go:embed testdata/errors/output_hint_only.golden
	goldenErrorsOutputHintOnly string

	//go:embed testdata/errors/output_note_only.golden
	goldenErrorsOutputNoteOnly string

	//go:embed testdata/errors/output_origin_note.golden
	goldenErrorsOutputOriginNote string

	//go:embed testdata/errors/output_origin_note_hint.golden
	goldenErrorsOutputOriginNoteHint string

	//go:embed testdata/errors/output_several_origins.golden
	goldenErrorsOutputSeveralOrigins string
)

// outputCLI declares flags whose failures cover each layout shape.
type outputCLI struct {
	noopRunner

	Timeout conga.OptionalFlag[time.Duration]  `env:"TOOL_TIMEOUT" name:"timeout"`
	Mode    conga.OptionalFlag[uint32]         `env:"TOOL_MODE"    name:"mode"`
	Level   conga.OptionalFlag[string]         `enum:"low,high"    name:"level"`
	Limit   conga.OptionalFlag[conga.ByteSize] `env:"TOOL_LIMIT"   name:"limit"`
}

// outputGroupCLI declares an xor group whose members can come from several
// sources.
type outputGroupCLI struct {
	noopRunner

	ToFile conga.OptionalFlag[string] `env:"TOOL_TO_FILE" name:"to-file" xor:"target"`
	ToURL  conga.OptionalFlag[string] `env:"TOOL_TO_URL"  name:"to-url"  xor:"target"`
}

// devError is a developer error that supplies its own pieces and wraps a
// cause.
type devError struct {
	message string
	origins []string
	note    string
	hint    string
	cause   error
}

func (e *devError) Error() string     { return e.message }
func (e *devError) Origins() []string { return e.origins }
func (e *devError) Note() string      { return e.note }
func (e *devError) Hint() string      { return e.hint }
func (e *devError) Unwrap() error     { return e.cause }

// devHintOnlyError is a developer error with a hint and no other piece.
type devHintOnlyError struct{ hint string }

func (e *devHintOnlyError) Error() string { return "dev failure" }
func (e *devHintOnlyError) Hint() string  { return e.hint }

// devFailCLI fails in Run with the error its test installs.
type devFailCLI struct {
	err error
}

func (c *devFailCLI) Run(_ context.Context, _ *conga.Session) error { return c.err }

// devValidateCLI fails in Validate with the function its test installs.
type devValidateCLI struct {
	validate func(session *conga.Session) error

	Limit conga.OptionalFlag[string] `name:"limit"`
}

func (c *devValidateCLI) Validate(_ context.Context, session *conga.Session) error {
	return c.validate(session)
}

func (c *devValidateCLI) Run(_ context.Context, _ *conga.Session) error { return nil }

// exitCapture captures what App.Exit printed and the code it exited with.
type exitCapture struct {
	output string
	code   int
}

// newExitApp returns an app named "app" that prints to buffer in the given
// usage mode and reports its exit code to onExit.
func newExitApp(buffer *bytes.Buffer, mode conga.UsageOnErrorMode, onExit func(int)) *conga.App[outputCLI] {
	return conga.New[outputCLI]().
		Name("app").
		UsageOnError(mode).
		Stderr(buffer).
		ExitFunc(onExit)
}

// exitOutputFor runs err through App.Exit of an app named "app" and captures it.
func exitOutputFor(t *testing.T, err error) exitCapture {
	t.Helper()

	var buffer bytes.Buffer

	code := -1

	newExitApp(&buffer, conga.UsageSummary, func(exitCode int) { code = exitCode }).Exit(err)

	return exitCapture{output: buffer.String(), code: code}
}

// exitLinesFor returns what App.Exit of an app named "app" prints for err
// without a usage block.
func exitLinesFor(err error) string {
	var buffer bytes.Buffer

	newExitApp(&buffer, conga.UsageSilence, func(int) {}).Exit(err)

	return buffer.String()
}

// failWith runs an app whose Run returns err and captures App.Exit's output.
func failWith(t *testing.T, err error) (exitCapture, error) {
	t.Helper()

	cli := &devFailCLI{err: err}

	var buffer bytes.Buffer

	code := -1
	app := bootstrapApp(cli).
		Name("app").
		Args([]string{}).
		Stderr(&buffer).
		ExitFunc(func(exitCode int) { code = exitCode })

	runErr := app.RunContext(t.Context())

	app.Exit(runErr)

	return exitCapture{output: buffer.String(), code: code}, runErr
}

//nolint:paralleltest // subtests mutate the process environment with t.Setenv
func TestErrorOutput_Layout(t *testing.T) {
	t.Run("an origin and a note", func(t *testing.T) {
		t.Setenv("TOOL_TIMEOUT", "30")

		result := runClassification(t, conga.New[outputCLI]().Args([]string{}))

		assert.Equal(t, goldenErrorsOutputOriginNote, result.output)
		assert.Equal(t, 1, result.exitCode)
	})

	t.Run("a note only", func(t *testing.T) {
		result := runClassification(t, conga.New[outputCLI]().Args([]string{"--level", "mid"}))

		assert.Equal(t, goldenErrorsOutputNoteOnly, result.output)
		assert.Equal(t, 2, result.exitCode)
	})

	t.Run("a hint only", func(t *testing.T) {
		result := runClassification(t, conga.New[configFlagCLI]().Args([]string{"--config", "x.json"}))

		assert.Equal(t, goldenErrorsOutputHintOnly, result.output)
		assert.Equal(t, 1, result.exitCode)
	})

	t.Run("an origin, a note and a hint", func(t *testing.T) {
		t.Setenv("TOOL_MODE", "0755")

		result := runClassification(t, conga.New[outputCLI]().Args([]string{}))

		assert.Equal(t, goldenErrorsOutputOriginNoteHint, result.output)
	})

	t.Run("several origins from an xor group", func(t *testing.T) {
		t.Setenv("TOOL_TO_FILE", "out.txt")

		path := writeConfigFile(t, `{"to-url": "https://example.com"}`)

		result := runClassification(t, conga.New[outputGroupCLI]().
			RegisterDecoder(".json", jsondecoder.Decode).
			ConfigOptions(conga.ConfigExplicit(path)).
			Args([]string{}))

		// The golden names the file <path>, since the test file lives in a
		// temporary directory.
		assert.Equal(t, goldenErrorsOutputSeveralOrigins, strings.ReplaceAll(result.output, path, "<path>"))
		assert.Equal(t, 1, result.exitCode)
	})
}

func TestErrorOutput_UsageBlockFollowsTheLines(t *testing.T) {
	cases := []struct {
		name     string
		mode     conga.UsageOnErrorMode
		expected string
	}{
		{name: "summary", mode: conga.UsageSummary, expected: goldenErrorsOutputGroupUsageSummary},
		{name: "full", mode: conga.UsageFull, expected: goldenErrorsOutputGroupUsageFull},
		{name: "hint", mode: conga.UsageHint, expected: goldenErrorsOutputGroupUsageHint},
		{name: "silence", mode: conga.UsageSilence, expected: goldenErrorsOutputGroupUsageSilence},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv("TOOL_TO_FILE", "out.txt")

			result := runClassification(t, conga.New[outputGroupCLI]().
				UsageOnError(testCase.mode).
				Args([]string{"--to-url", "https://example.com"}))

			assert.Equal(t, 2, result.exitCode)
			assert.Equal(t, testCase.expected, result.output)
		})
	}
}

func TestUsageError_GroupConflictWithCommandLineAndEnvironmentMembers(t *testing.T) {
	t.Setenv("TOOL_TO_FILE", "out.txt")

	err := conga.New[outputGroupCLI]().Args([]string{"--to-url", "https://example.com"}).RunContext(t.Context())

	usage := assertUsageError(t, err)

	assert.Equal(t, "flags --to-file and --to-url are mutually exclusive", err.Error())
	assert.Equal(t, []string{"--to-file: environment variable TOOL_TO_FILE"}, usage.Origins())
	assert.Equal(t, 2, usage.ExitCode())
}

//nolint:paralleltest // subtests mutate the process environment with t.Setenv
func TestErrorOutput_LineOneOnly(t *testing.T) {
	t.Run("a value that did not convert", func(t *testing.T) {
		t.Setenv("TOOL_TIMEOUT", "30")

		err := conga.New[outputCLI]().Args([]string{}).RunContext(t.Context())

		assert.Equal(t, `invalid duration "30" for flag --timeout`, err.Error())
		assert.Equal(t, []string{"environment variable TOOL_TIMEOUT"}, errlines.Origins(err))
		assert.Equal(t, "accepted units: ns, us (or µs), ms, s, m, h", errlines.Note(err))
		assert.Equal(t, "", errlines.Hint(err))
	})

	t.Run("a constraint violation", func(t *testing.T) {
		err := conga.New[outputCLI]().Args([]string{"--level", "mid"}).RunContext(t.Context())

		assert.Equal(t, `invalid value "mid" for flag --level`, err.Error())
		assert.Equal(t, "accepted values: low, high", errlines.Note(err))
		assert.Zero(t, errlines.Origins(err))
	})
}

// limitValue is a text unmarshaler that wraps a ByteSize failure with a
// prefix, as an author would.
type limitValue struct{ size conga.ByteSize }

func (l *limitValue) UnmarshalText(text []byte) error {
	if err := l.size.UnmarshalText(text); err != nil {
		return fmt.Errorf("limit: %w", err)
	}

	return nil
}

var _ encoding.TextUnmarshaler = (*limitValue)(nil)

type wrappedLimitCLI struct {
	noopRunner

	Max conga.OptionalFlag[limitValue] `env:"TOOL_MAX" name:"max"`
}

//nolint:paralleltest // subtests mutate the process environment with t.Setenv
func TestErrorOutput_ChainRule(t *testing.T) {
	t.Run("an author wrapper around a ByteSize error keeps the note from the command line", func(t *testing.T) {
		result := runClassification(t, conga.New[wrappedLimitCLI]().Args([]string{"--max", "5XB"}))

		assert.Equal(t, "app: error: limit: unknown byte size unit \"XB\" in \"5XB\" for flag --max\n"+
			"  note: accepted units: B, kB, MB, GB, TB, PB, EB, KiB, MiB, GiB, TiB, PiB, EiB\n", result.output)
		assert.Equal(t, 2, result.exitCode)
	})

	t.Run("an author wrapper around a ByteSize error keeps the note from the environment", func(t *testing.T) {
		t.Setenv("TOOL_MAX", "5XB")

		result := runClassification(t, conga.New[wrappedLimitCLI]().Args([]string{}))

		assert.Equal(t, "app: error: limit: unknown byte size unit \"XB\" in \"5XB\" for flag --max\n"+
			"  from: environment variable TOOL_MAX\n"+
			"  note: accepted units: B, kB, MB, GB, TB, PB, EB, KiB, MiB, GiB, TiB, PiB, EiB\n", result.output)
		assert.Equal(t, 1, result.exitCode)
	})

	t.Run("a developer hint replaces CONGA's hint", func(t *testing.T) {
		var size conga.ByteSize

		inner := size.UnmarshalText([]byte("5KB"))
		wrapped := &devError{message: "config failed: " + inner.Error(), hint: "ask the platform team", cause: inner}

		result, _ := failWith(t, wrapped)

		assert.Equal(t, "app: error: config failed: ambiguous byte size unit \"KB\" in \"5KB\"\n"+
			"  hint: ask the platform team\n", result.output)
	})

	t.Run("the note and the hint can come from different errors", func(t *testing.T) {
		var size conga.ByteSize

		inner := size.UnmarshalText([]byte("5XB"))
		wrapped := &devError{message: "deploy: " + inner.Error(), hint: "fix the manifest", cause: inner}

		result, _ := failWith(t, wrapped)

		assert.Equal(t, "app: error: deploy: unknown byte size unit \"XB\" in \"5XB\"\n"+
			"  note: accepted units: B, kB, MB, GB, TB, PB, EB, KiB, MiB, GiB, TiB, PiB, EiB\n"+
			"  hint: fix the manifest\n", result.output)
	})

	t.Run("a type with only a hint prints one hint line", func(t *testing.T) {
		result, _ := failWith(t, &devHintOnlyError{hint: "retry later"})

		assert.Equal(t, "app: error: dev failure\n  hint: retry later\n", result.output)
	})

	t.Run("empty methods print no lines", func(t *testing.T) {
		result, _ := failWith(t, &devError{message: "dev failure"})

		assert.Equal(t, "app: error: dev failure\n", result.output)
	})

	t.Run("an empty first line prints no lines even with a hinter cause", func(t *testing.T) {
		wrapped := &devError{message: "", cause: &devHintOnlyError{hint: "retry later"}}

		result := exitOutputFor(t, wrapped)

		assert.Equal(t, "", result.output)
		assert.Equal(t, 1, result.code)
	})
}

func TestUsageError_Pieces(t *testing.T) {
	t.Parallel()

	t.Run("UsageErrorf keeps the note from the wrapped chain and prints the usage block", func(t *testing.T) {
		t.Parallel()

		var size conga.ByteSize

		byteSizeErr := size.UnmarshalText([]byte("5XB"))

		cli := &devValidateCLI{validate: func(session *conga.Session) error {
			return session.UsageErrorf("--limit: %w", byteSizeErr)
		}}

		var buffer bytes.Buffer

		code := -1
		app := bootstrapApp(cli).
			Name("app").
			Args([]string{}).
			Stderr(&buffer).
			ExitFunc(func(exitCode int) { code = exitCode })

		err := app.RunContext(t.Context())

		app.Exit(err)

		usage := assertUsageError(t, err)

		assert.Equal(t, `--limit: unknown byte size unit "XB" in "5XB"`, err.Error())
		assert.Equal(t, "accepted units: B, kB, MB, GB, TB, PB, EB, KiB, MiB, GiB, TiB, PiB, EiB", usage.Note())
		assert.Equal(t, usage.Note(), errlines.Note(err))
		assert.Equal(t, 2, code)
		assert.True(t, strings.HasPrefix(buffer.String(), "app: error: --limit: unknown byte size unit \"XB\" in \"5XB\"\n  note: accepted units:"))
		assert.Contains(t, buffer.String(), "\n\nUsage:\n")
	})

	t.Run("UsageErrorf with a developer hinter shows its hint and the usage block", func(t *testing.T) {
		t.Parallel()

		cli := &devValidateCLI{validate: func(session *conga.Session) error {
			return session.UsageErrorf("%w", &devHintOnlyError{hint: "pass --limit"})
		}}

		var buffer bytes.Buffer

		app := bootstrapApp(cli).Name("app").Args([]string{}).Stderr(&buffer).ExitFunc(func(int) {})

		err := app.RunContext(t.Context())

		app.Exit(err)

		usage := assertUsageError(t, err)

		assert.Equal(t, "pass --limit", usage.Hint())
		assert.True(t, strings.HasPrefix(buffer.String(), "app: error: dev failure\n  hint: pass --limit\n\nUsage:\n"))
	})

	t.Run("an unknown command suggestion is the hint", func(t *testing.T) {
		t.Parallel()

		err := conga.New[outputSubcommandCLI]().Args([]string{"deplo"}).RunContext(t.Context())

		usage := assertUsageError(t, err)

		assert.Equal(t, "unknown command: deplo", err.Error())
		assert.Equal(t, "did you mean deploy?", usage.Hint())
		assert.Equal(t, usage.Hint(), errlines.Hint(err))
	})

	t.Run("an unknown flag suggestion is the hint", func(t *testing.T) {
		t.Parallel()

		err := conga.New[outputCLI]().Args([]string{"--timeot", "1s"}).RunContext(t.Context())

		usage := assertUsageError(t, err)

		assert.Equal(t, "unknown flag: --timeot", err.Error())
		assert.Equal(t, "did you mean --timeout?", usage.Hint())
	})

	t.Run("a conversion error's pieces equal Error's", func(t *testing.T) {
		t.Parallel()

		err := conga.New[outputCLI]().Args([]string{"--limit", "5XB"}).RunContext(t.Context())

		usage := assertUsageError(t, err)

		assert.Equal(t, "accepted units: B, kB, MB, GB, TB, PB, EB, KiB, MiB, GiB, TiB, PiB, EiB", usage.Note())
		assertSamePieces(t, err, usage)
	})

	t.Run("a SelfValidator error's pieces equal Error's", func(t *testing.T) {
		t.Parallel()

		type notedCLI struct {
			noopRunner

			Size conga.OptionalFlag[notedNumber] `name:"size"`
		}

		err := conga.New[notedCLI]().Args([]string{"--size", "3"}).RunContext(t.Context())

		usage := assertUsageError(t, err)

		assert.Equal(t, []string{"the manifest"}, usage.Origins())
		assert.Equal(t, "detail", usage.Note())
		assert.Equal(t, "act", usage.Hint())
		assertSamePieces(t, err, usage)
	})

	t.Run("a UsageErrorf error's pieces equal Error's", func(t *testing.T) {
		t.Parallel()

		cli := &devValidateCLI{validate: func(session *conga.Session) error {
			return session.UsageErrorf("--limit: %w", &devError{message: "rejected", origins: []string{"the manifest"}, note: "detail", hint: "act"})
		}}

		err := bootstrapApp(cli).Args([]string{}).RunContext(t.Context())

		usage := assertUsageError(t, err)

		assert.Equal(t, []string{"the manifest"}, usage.Origins())
		assert.Equal(t, "detail", usage.Note())
		assert.Equal(t, "act", usage.Hint())
		assertSamePieces(t, err, usage)
	})
}

// notedNumber is a value whose validation fails with an error that supplies
// every piece.
type notedNumber int

func (n *notedNumber) UnmarshalText(text []byte) error {
	value, err := strconv.Atoi(string(text))
	if err != nil {
		return err
	}

	*n = notedNumber(value)

	return nil
}

func (n notedNumber) Validate(_ context.Context) error {
	return &devError{message: "size rejected", origins: []string{"the manifest"}, note: "detail", hint: "act"}
}

// assertSamePieces fails unless usage reports the same origins, note and hint
// as DiagnosticOf(err).
func assertSamePieces(t *testing.T, err error, usage *conga.UsageError) {
	t.Helper()

	resolved := conga.DiagnosticOf(err)

	assert.Equal(t, resolved.Origins(), usage.Origins())
	assert.Equal(t, resolved.Note(), usage.Note())
	assert.Equal(t, resolved.Hint(), usage.Hint())
}

type outputSubcommandDeploy struct{ noopRunner }

type outputSubcommandCLI struct {
	noopRunner

	Deploy conga.Command[outputSubcommandDeploy] `summary:"Deploy"`
}

func TestErrorOutput_Wrapping(t *testing.T) {
	t.Parallel()

	t.Run("a prefix wrap lands on line one", func(t *testing.T) {
		t.Parallel()

		var size conga.ByteSize

		inner := size.UnmarshalText([]byte("5XB"))

		result, _ := failWith(t, fmt.Errorf("load profile: %w", inner))

		assert.True(t, strings.HasPrefix(result.output, "app: error: load profile: unknown byte size unit \"XB\" in \"5XB\"\n  note: accepted units:"))
	})

	t.Run("a suffix wrap lands on line one", func(t *testing.T) {
		t.Parallel()

		var size conga.ByteSize

		inner := size.UnmarshalText([]byte("5XB"))

		result, _ := failWith(t, fmt.Errorf("%w (attempt 3)", inner))

		assert.True(t, strings.HasPrefix(result.output, "app: error: unknown byte size unit \"XB\" in \"5XB\" (attempt 3)\n  note: accepted units:"))
	})

	t.Run("a wrapper that transforms the text keeps the lines", func(t *testing.T) {
		t.Parallel()

		var size conga.ByteSize

		inner := size.UnmarshalText([]byte("5XB"))
		transformed := &devError{message: strings.ToUpper(inner.Error()), cause: inner}

		result, _ := failWith(t, transformed)

		assert.True(t, strings.HasPrefix(result.output, "app: error: UNKNOWN BYTE SIZE UNIT \"XB\" IN \"5XB\"\n  note: accepted units:"))
	})
}

func TestErrorOutput_DeveloperWrapperAroundRun(t *testing.T) {
	t.Parallel()

	hinted := &devError{message: "main: run failed", hint: "see the log", cause: errors.New("boom")}

	result := exitOutputFor(t, hinted)

	assert.Equal(t, "app: error: main: run failed\n  hint: see the log\n", result.output)
	assert.Equal(t, 1, result.code)
}

func TestErrorOutput_Exit_RendersLikeARunError(t *testing.T) {
	t.Parallel()

	var size conga.ByteSize

	inner := size.UnmarshalText([]byte("5XB"))

	viaRun, _ := failWith(t, inner)
	direct := exitOutputFor(t, inner)

	assert.Equal(t, viaRun.output, direct.output)
}

type nilMemberError struct{ members []error }

func (e *nilMemberError) Error() string {
	texts := make([]string, 0, len(e.members))

	for _, member := range e.members {
		if member != nil {
			texts = append(texts, member.Error())
		}
	}

	return strings.Join(texts, "\n")
}

func (e *nilMemberError) Unwrap() []error { return e.members }

//nolint:paralleltest // subtests mutate the process environment with t.Setenv
func TestErrorOutput_JoinedErrors(t *testing.T) {
	var size conga.ByteSize

	conga1 := size.UnmarshalText([]byte("5XB"))
	conga2 := size.UnmarshalText([]byte("5KB"))

	t.Run("a join of two CONGA errors prints each with its lines", func(t *testing.T) {
		t.Setenv("CONGA_COLOR", "1")

		result := exitOutputFor(t, errors.Join(conga1, conga2))

		assert.Equal(t, "app: \x1b[1;31merror:\x1b[0m unknown byte size unit \"XB\" in \"5XB\"\n"+
			"  \x1b[2mnote:\x1b[0m accepted units: B, kB, MB, GB, TB, PB, EB, KiB, MiB, GiB, TiB, PiB, EiB\n"+
			"app: \x1b[1;31merror:\x1b[0m ambiguous byte size unit \"KB\" in \"5KB\"\n"+
			"  \x1b[1;36mhint:\x1b[0m use kB (1000) or KiB (1024)\n", result.output)
	})

	t.Run("an author error joined with a CONGA error", func(t *testing.T) {
		result := exitOutputFor(t, errors.Join(errors.New("cleanup failed"), conga2))

		assert.Equal(t, "app: error: cleanup failed\n"+
			"app: error: ambiguous byte size unit \"KB\" in \"5KB\"\n"+
			"  hint: use kB (1000) or KiB (1024)\n", result.output)
	})

	t.Run("a nested join is flattened", func(t *testing.T) {
		result := exitOutputFor(t, errors.Join(errors.Join(errors.New("first"), errors.New("second")), errors.New("third")))

		assert.Equal(t, "app: error: first\napp: error: second\napp: error: third\n", result.output)
	})

	t.Run("an author member whose text spans lines is not split", func(t *testing.T) {
		result := exitOutputFor(t, errors.Join(errors.New("first line\nsecond line"), errors.New("other")))

		assert.Equal(t, "app: error: first line\nsecond line\napp: error: other\n", result.output)
	})

	t.Run("fmt.Errorf with two %w prints as one error", func(t *testing.T) {
		result := exitOutputFor(t, fmt.Errorf("both: %w and %w", conga1, conga2))

		assert.True(t, strings.HasPrefix(result.output, "app: error: both: unknown byte size unit \"XB\" in \"5XB\" and ambiguous byte size unit \"KB\" in \"5KB\"\n"))
		assert.Equal(t, 1, strings.Count(result.output, "app: error:"))
		assert.Contains(t, result.output, "  note: accepted units:")
	})

	t.Run("a wrapped join prints as one error", func(t *testing.T) {
		result := exitOutputFor(t, fmt.Errorf("outer: %w", errors.Join(errors.New("first"), errors.New("second"))))

		assert.Equal(t, "app: error: outer: first\nsecond\n", result.output)
	})

	t.Run("a third-party join with a nil member skips it", func(t *testing.T) {
		result := exitOutputFor(t, &nilMemberError{members: []error{errors.New("first"), nil, errors.New("second")}})

		assert.Equal(t, "app: error: first\napp: error: second\n", result.output)
	})

	t.Run("a runtime error joined with a usage error exits 2 with the usage block", func(t *testing.T) {
		usage := usageErrorFor(t)

		result := exitOutputFor(t, errors.Join(errors.New("runtime"), usage))

		assert.Equal(t, 2, result.code)
		assert.Contains(t, result.output, "\nUsage:\n")
		assert.Equal(t, 1, strings.Count(result.output, "Usage:"))
	})

	t.Run("an exit code error joined before a usage error exits with its code and no block", func(t *testing.T) {
		result := exitOutputFor(t, errors.Join(&exitCodeError{code: 75, message: "exit 75"}, usageErrorFor(t)))

		assert.Equal(t, 75, result.code)
		assert.NotContains(t, result.output, "Usage:")
	})

	t.Run("a usage error joined before an exit code error exits 2 with the usage block", func(t *testing.T) {
		result := exitOutputFor(t, errors.Join(usageErrorFor(t), &exitCodeError{code: 75, message: "exit 75"}))

		assert.Equal(t, 2, result.code)
		assert.Contains(t, result.output, "\nUsage:\n")
	})
}

// usageErrorFor returns a usage error that has a command chain.
func usageErrorFor(t *testing.T) error {
	t.Helper()

	return conga.New[outputSubcommandCLI]().Args([]string{"deplo"}).RunContext(t.Context())
}

func TestErrorOutput_Color(t *testing.T) {
	var size conga.ByteSize

	inner := size.UnmarshalText([]byte("5XB"))
	hinted := &devError{message: "failed", origins: []string{"the manifest"}, note: "detail", hint: "act", cause: inner}

	const coloredPlain = "app: \x1b[1;31merror:\x1b[0m failed\n" +
		"  \x1b[2mfrom:\x1b[0m the manifest\n" +
		"  \x1b[2mnote:\x1b[0m detail\n" +
		"  \x1b[1;36mhint:\x1b[0m act\n"

	const plain = "app: error: failed\n  from: the manifest\n  note: detail\n  hint: act\n"

	cases := []struct {
		name     string
		env      map[string]string
		expected string
	}{
		{name: "CONGA_COLOR forces color on a buffer", env: map[string]string{"CONGA_COLOR": "1"}, expected: coloredPlain},
		{name: "CONGA_COLOR outranks NO_COLOR", env: map[string]string{"CONGA_COLOR": "1", "NO_COLOR": "1"}, expected: coloredPlain},
		{name: "CONGA_COLOR outranks CLICOLOR 0", env: map[string]string{"CONGA_COLOR": "1", "CLICOLOR": "0"}, expected: coloredPlain},
		{name: "a buffer is not a terminal", env: map[string]string{}, expected: plain},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			for name, value := range testCase.env {
				t.Setenv(name, value)
			}

			result := exitOutputFor(t, hinted)

			assert.Equal(t, testCase.expected, result.output)
		})
	}
}

func TestError_RunReturnsRawError(t *testing.T) {
	t.Parallel()

	t.Run("a hook's error is returned as is so == and type switches match", func(t *testing.T) {
		t.Parallel()

		hookErr := &devError{message: "hook failed", hint: "fix it"}

		_, err := failWith(t, hookErr)

		assert.True(t, err == hookErr) //nolint:errorlint // identity of the returned error is the property under test

		typed, isDev := err.(*devError) //nolint:errorlint // direct type assertion is the property under test

		assert.True(t, isDev)
		assert.Equal(t, hookErr, typed)
		assert.Equal(t, "hook failed", err.Error())
	})

	t.Run("a prepare error is returned directly as *UsageError", func(t *testing.T) {
		t.Parallel()

		err := conga.New[outputCLI]().Args([]string{"--nope"}).RunContext(t.Context())

		_, isUsage := err.(*conga.UsageError) //nolint:errorlint // the concrete type Run returns is the property under test

		assert.True(t, isUsage)
		assertUsageError(t, err)
	})

	t.Run("a dotenv load failure is returned directly", func(t *testing.T) {
		t.Parallel()

		missing := filepath.Join(t.TempDir(), "missing.env")

		err := conga.New[outputCLI]().Dotenv(missing).Args([]string{}).RunContext(t.Context())

		assert.Error(t, err)
		assert.Equal(t, fmt.Sprintf("cannot read .env file %q: open %s: no such file or directory", missing, missing), err.Error())
	})

	t.Run("a completion invocation failure is returned directly", func(t *testing.T) {
		t.Parallel()

		err := conga.New[outputCLI]().
			EnableCompletion().
			Stdout(failingWriter{}).
			Args([]string{"__complete", "bash", ""}).
			RunContext(t.Context())

		assert.Error(t, err)
		assert.True(t, strings.HasPrefix(err.Error(), "internal completion failure: "))
	})

	t.Run("success is an untyped nil", func(t *testing.T) {
		t.Parallel()

		err := conga.New[outputCLI]().Args([]string{}).RunContext(t.Context())

		assert.True(t, err == nil)
	})

	t.Run("the exit code works directly", func(t *testing.T) {
		t.Parallel()

		err := conga.New[outputCLI]().Args([]string{"--level", "mid"}).RunContext(t.Context())

		assert.Equal(t, 2, conga.New[outputCLI]().ExitCode(err))

		coder, ok := errors.AsType[conga.ExitCoder](err)

		assert.True(t, ok)
		assert.Equal(t, 2, coder.ExitCode())
	})

	t.Run("a developer error with an exit code decides the code", func(t *testing.T) {
		t.Parallel()

		result, err := failWith(t, &exitCodeError{code: 75, message: "exit 75"})

		assert.Equal(t, 75, conga.New[outputCLI]().ExitCode(err))
		assert.Equal(t, 75, result.code)
	})

	t.Run("DiagnosticOf agrees with Exit", func(t *testing.T) {
		t.Parallel()

		result, err := failWith(t, &devHintOnlyError{hint: "retry later"})

		resolved := conga.DiagnosticOf(err)

		assert.Equal(t, "retry later", resolved.Hint())
		assert.Contains(t, result.output, "  hint: retry later\n")
	})

	t.Run("a nested run returns the inner error unchanged", func(t *testing.T) {
		t.Parallel()

		inner := conga.New[outputCLI]().Args([]string{"--level", "mid"}).RunContext(t.Context())

		_, err := failWith(t, inner)

		assert.True(t, err == inner) //nolint:errorlint // identity of the returned value is the property under test
	})
}

// finallyCLI records the error its Finally hook receives.
type finallyCLI struct {
	runErr   error
	received error
}

func (c *finallyCLI) Run(_ context.Context, _ *conga.Session) error { return c.runErr }

func (c *finallyCLI) Finally(_ context.Context, _ *conga.Session, err error) error {
	c.received = err

	return nil
}

func TestError_RunAndFinallySeeTheSameError(t *testing.T) {
	t.Parallel()

	hookErr := errors.New("run failed")
	cli := &finallyCLI{runErr: hookErr}

	err := conga.New(cli).Args([]string{}).RunContext(t.Context())

	assert.True(t, err == hookErr)      //nolint:errorlint // identity, not errors.Is, is the property under test
	assert.True(t, cli.received == err) //nolint:errorlint // Run and Finally must observe the same value
	assert.Equal(t, "run failed", cli.received.Error())
}

//nolint:paralleltest // subtests mutate the process environment with t.Setenv
func TestDiagnosticOf_FormattingAndRender(t *testing.T) {
	t.Setenv("TOOL_TIMEOUT", "30")

	err := conga.New[outputCLI]().Args([]string{}).RunContext(t.Context())
	resolved := conga.DiagnosticOf(err)

	line := `invalid duration "30" for flag --timeout`
	rendered := line + "\n" +
		"  from: environment variable TOOL_TIMEOUT\n" +
		"  note: accepted units: ns, us (or µs), ms, s, m, h"

	t.Run("%v and %s print line one", func(t *testing.T) {
		assert.Equal(t, line, fmt.Sprintf("%v", resolved))
		assert.Equal(t, line, fmt.Sprintf("%s", resolved))
	})

	t.Run("Render and %+v print line one and the lines, plain", func(t *testing.T) {
		assert.Equal(t, rendered, resolved.Render())
		assert.Equal(t, rendered, fmt.Sprintf("%+v", resolved))
	})

	t.Run("%q quotes line one", func(t *testing.T) {
		assert.Equal(t, fmt.Sprintf("%q", line), fmt.Sprintf("%q", resolved))
	})

	t.Run("width, precision, padding, and # format line one", func(t *testing.T) {
		assert.Equal(t, line[:10], fmt.Sprintf("%.10s", resolved))
		assert.Equal(t, fmt.Sprintf("%50s", line), fmt.Sprintf("%50s", resolved))
		assert.Equal(t, fmt.Sprintf("%-50v", line), fmt.Sprintf("%-50v", resolved))
		assert.Equal(t, fmt.Sprintf("%#v", line), fmt.Sprintf("%#v", resolved))
	})

	t.Run("wrapped CONGA error keeps outer text and outer hint", func(t *testing.T) {
		outer := &devError{message: "startup: " + err.Error(), hint: "check TOOL_TIMEOUT", cause: err}
		outerResolved := conga.DiagnosticOf(outer)

		assert.Equal(t, "startup: "+line, outerResolved.Message())
		assert.Equal(t, []string{"environment variable TOOL_TIMEOUT"}, outerResolved.Origins())
		assert.Equal(t, "accepted units: ns, us (or µs), ms, s, m, h", outerResolved.Note())
		assert.Equal(t, "check TOOL_TIMEOUT", outerResolved.Hint())
		assert.Equal(t, "startup: "+line+"\n"+
			"  from: environment variable TOOL_TIMEOUT\n"+
			"  note: accepted units: ns, us (or µs), ms, s, m, h\n"+
			"  hint: check TOOL_TIMEOUT", outerResolved.Render())

		result := exitOutputFor(t, outer)
		assert.Equal(t, "app: error: startup: "+line+"\n"+
			"  from: environment variable TOOL_TIMEOUT\n"+
			"  note: accepted units: ns, us (or µs), ms, s, m, h\n"+
			"  hint: check TOOL_TIMEOUT\n", result.output)
	})

	t.Run("zero Diagnostic from DiagnosticOf(nil) is safe", func(t *testing.T) {
		zero := conga.DiagnosticOf(nil)

		assert.Equal(t, "", zero.Message())
		assert.Zero(t, zero.Origins())
		assert.Equal(t, "", zero.Note())
		assert.Equal(t, "", zero.Hint())
		assert.Zero(t, zero.Members())
		assert.Equal(t, "", zero.Render())
		assert.Equal(t, "", fmt.Sprintf("%+v", zero))
		assert.Equal(t, "", fmt.Sprintf("%v", zero))

		jsonBytes, jsonErr := json.Marshal(zero)
		assert.NoError(t, jsonErr)
		assert.Equal(t, "null", string(jsonBytes))

		textBytes, textErr := zero.MarshalText()
		assert.NoError(t, textErr)
		assert.Equal(t, "", string(textBytes))
	})
}

func TestDiagnosticOf_Marshalers(t *testing.T) {
	t.Parallel()

	var size conga.ByteSize

	conga1 := size.UnmarshalText([]byte("5XB"))

	marshal := func(t *testing.T, err error) (string, string) {
		t.Helper()

		resolved := conga.DiagnosticOf(err)

		jsonBytes, jsonErr := json.Marshal(resolved)
		assert.NoError(t, jsonErr)

		textBytes, textErr := resolved.MarshalText()
		assert.NoError(t, textErr)

		return string(jsonBytes), string(textBytes)
	}

	t.Run("a single error", func(t *testing.T) {
		t.Parallel()

		jsonText, plainText := marshal(t, conga1)

		assert.Equal(t, `{"error":"unknown byte size unit \"XB\" in \"5XB\"","note":"accepted units: B, kB, MB, GB, TB, PB, EB, KiB, MiB, GiB, TiB, PiB, EiB"}`, jsonText)
		assert.Equal(t, `unknown byte size unit "XB" in "5XB" (note: accepted units: B, kB, MB, GB, TB, PB, EB, KiB, MiB, GiB, TiB, PiB, EiB)`, plainText)
	})

	t.Run("origins, a note and a hint", func(t *testing.T) {
		t.Parallel()

		jsonText, plainText := marshal(t, &devError{message: "failed", origins: []string{"a", "b"}, note: "n", hint: "h"})

		assert.Equal(t, `{"error":"failed","from":["a","b"],"note":"n","hint":"h"}`, jsonText)
		assert.Equal(t, "failed (from: a; b; note: n; hint: h)", plainText)
	})

	t.Run("a join keeps the first member as the object and the rest under more", func(t *testing.T) {
		t.Parallel()

		jsonText, plainText := marshal(t, errors.Join(&devError{message: "first", hint: "h1"}, &devError{message: "second", note: "n2"}))

		assert.Equal(t, `{"error":"first","hint":"h1","more":[{"error":"second","note":"n2"}]}`, jsonText)
		assert.Equal(t, "first (hint: h1) | second (note: n2)", plainText)
	})

	t.Run("a message with nothing after it has no brackets", func(t *testing.T) {
		t.Parallel()

		jsonText, plainText := marshal(t, errors.New("plain"))

		assert.Equal(t, `{"error":"plain"}`, jsonText)
		assert.Equal(t, "plain", plainText)
	})

	t.Run("an empty message keeps the error key", func(t *testing.T) {
		t.Parallel()

		jsonText, plainText := marshal(t, &devError{message: "", hint: "ignored"})

		assert.Equal(t, `{"error":""}`, jsonText)
		assert.Equal(t, "", plainText)
	})
}

func TestDiagnosticOf_Slog(t *testing.T) {
	t.Parallel()

	_, err := failWith(t, &devError{message: "failed", origins: []string{"the manifest"}, hint: "act"})

	t.Run("the JSON handler records an object", func(t *testing.T) {
		t.Parallel()

		line := slogLine(err, slogJSON)

		assert.Equal(t, `{"level":"ERROR","msg":"run failed","err":{"error":"failed","from":["the manifest"],"hint":"act"}}`+"\n", line)
	})

	t.Run("the text handler records a line", func(t *testing.T) {
		t.Parallel()

		line := slogLine(err, slogText)

		assert.Equal(t, `level=ERROR msg="run failed" err="failed (from: the manifest; hint: act)"`+"\n", line)
	})

	t.Run("a developer-wrapped error keeps its pieces in both handlers", func(t *testing.T) {
		t.Parallel()

		wrapped := fmt.Errorf("main: %w", err)

		assert.Equal(t, `{"level":"ERROR","msg":"run failed","err":{"error":"main: failed","from":["the manifest"],"hint":"act"}}`+"\n", slogLine(wrapped, slogJSON))
		assert.Equal(t, `level=ERROR msg="run failed" err="main: failed (from: the manifest; hint: act)"`+"\n", slogLine(wrapped, slogText))
	})

	t.Run("a join records its members in both handlers", func(t *testing.T) {
		t.Parallel()

		_, joined := failWith(t, errors.Join(
			&devError{message: "first", hint: "act"},
			&devError{message: "second", note: "detail"},
		))

		assert.Equal(t, `{"level":"ERROR","msg":"run failed","err":{"error":"first","hint":"act","more":[{"error":"second","note":"detail"}]}}`+"\n", slogLine(joined, slogJSON))
		assert.Equal(t, `level=ERROR msg="run failed" err="first (hint: act) | second (note: detail)"`+"\n", slogLine(joined, slogText))
	})
}

// slogFormat selects the slog handler slogLine logs through.
type slogFormat int

const (
	slogJSON slogFormat = iota
	slogText
)

// slogLine logs the Diagnostic of err through the chosen handler without a
// timestamp and returns the line it wrote.
func slogLine(err error, format slogFormat) string {
	var buffer bytes.Buffer

	options := &slog.HandlerOptions{ReplaceAttr: dropTime}

	var handler slog.Handler = slog.NewTextHandler(&buffer, options)
	if format == slogJSON {
		handler = slog.NewJSONHandler(&buffer, options)
	}

	slog.New(handler).Error("run failed", "err", conga.DiagnosticOf(err))

	return buffer.String()
}

// dropTime removes the timestamp so the handler output is stable.
func dropTime(_ []string, attr slog.Attr) slog.Attr {
	if attr.Key == slog.TimeKey {
		return slog.Attr{}
	}

	return attr
}

func TestDiagnosticOf_ReportsPiecesAndMembers(t *testing.T) {
	t.Parallel()

	_, err := failWith(t, &devError{message: "failed", origins: []string{"a"}, note: "n", hint: "h"})

	resolved := conga.DiagnosticOf(err)

	assert.Equal(t, []string{"a"}, resolved.Origins())
	assert.Equal(t, "n", resolved.Note())
	assert.Equal(t, "h", resolved.Hint())
	assert.Equal(t, 1, len(resolved.Members()))
	assert.Equal(t, "failed", resolved.Members()[0].Message())
	assert.Equal(t, 1, conga.New[outputCLI]().ExitCode(err))
}

// configFlagCLI declares a config file flag on an app with no decoder.
type configFlagCLI struct {
	noopRunner

	Config conga.ConfigFile `name:"config"`
}

// innerJoinCLI fails Validate with a join of two developer errors.
type innerJoinCLI struct{}

func (*innerJoinCLI) Validate(_ context.Context, _ *conga.Session) error {
	return errors.Join(
		&devError{message: "first inner", origins: []string{"origin a"}, note: "note a"},
		&devError{message: "second inner", hint: "hint b"},
	)
}

func (*innerJoinCLI) Run(_ context.Context, _ *conga.Session) error { return nil }

// outerNestedRunCLI runs innerJoinCLI from Run and adds an outer Finally
// failure.
type outerNestedRunCLI struct{}

func (*outerNestedRunCLI) Run(ctx context.Context, _ *conga.Session) error {
	return conga.New[innerJoinCLI]().
		Args([]string{}).
		RunContext(ctx)
}

func (*outerNestedRunCLI) Finally(_ context.Context, _ *conga.Session, _ error) error {
	return &devError{message: "outer finally failed", hint: "outer hint"}
}

func TestError_NestedJoinFlattens(t *testing.T) {
	t.Parallel()

	innerErr := conga.New[innerJoinCLI]().
		Args([]string{}).
		RunContext(t.Context())

	t.Run("errors.Join with a nested run's join", func(t *testing.T) {
		t.Parallel()

		joined := errors.Join(innerErr, &devError{message: "close failed", hint: "hint c"})
		result := exitOutputFor(t, joined)

		assert.Equal(t, "app: error: first inner\n"+
			"  from: origin a\n"+
			"  note: note a\n"+
			"app: error: second inner\n"+
			"  hint: hint b\n"+
			"app: error: close failed\n"+
			"  hint: hint c\n", result.output)
	})

	t.Run("an outer Finally failure joined with an inner run's join", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := conga.New[outerNestedRunCLI]().
			Name("app").
			Args([]string{}).
			Stderr(&buffer).
			ExitFunc(func(int) {})

		err := app.RunContext(t.Context())
		app.Exit(err)

		resolved := conga.DiagnosticOf(err)

		jsonBytes, jsonErr := json.Marshal(resolved)
		assert.NoError(t, jsonErr)

		textBytes, textErr := resolved.MarshalText()
		assert.NoError(t, textErr)

		assert.Equal(t, "app: error: first inner\n"+
			"  from: origin a\n"+
			"  note: note a\n"+
			"app: error: second inner\n"+
			"  hint: hint b\n"+
			"app: error: outer finally failed\n"+
			"  hint: outer hint\n", buffer.String())
		assert.Equal(t, "first inner\n  from: origin a\n  note: note a\nsecond inner\n  hint: hint b\nouter finally failed\n  hint: outer hint", resolved.Render())
		assert.Equal(t, resolved.Render(), fmt.Sprintf("%+v", resolved))
		assert.Equal(t, `{"error":"first inner","from":["origin a"],"note":"note a","more":[{"error":"second inner","hint":"hint b"},{"error":"outer finally failed","hint":"outer hint"}]}`, string(jsonBytes))
		assert.Equal(t, "first inner (from: origin a; note: note a) | second inner (hint: hint b) | outer finally failed (hint: outer hint)", string(textBytes))

		members := resolved.Members()
		assert.Equal(t, 3, len(members))
		assert.Equal(t, "first inner", members[0].Message())
		assert.Equal(t, []string{"origin a"}, members[0].Origins())
		assert.Equal(t, "note a", members[0].Note())
		assert.Equal(t, "second inner", members[1].Message())
		assert.Equal(t, "hint b", members[1].Hint())
		assert.Equal(t, "outer finally failed", members[2].Message())
		assert.Equal(t, "outer hint", members[2].Hint())
	})
}

func TestError_EmptyMessageConsistency(t *testing.T) {
	t.Parallel()

	t.Run("a single empty-message error reports no pieces through DiagnosticOf", func(t *testing.T) {
		t.Parallel()

		result, err := failWith(t, &devError{message: "", origins: []string{"o"}, note: "n", hint: "h"})

		resolved := conga.DiagnosticOf(err)

		assert.Equal(t, "", result.output)
		assert.Zero(t, resolved.Origins())
		assert.Equal(t, "", resolved.Note())
		assert.Equal(t, "", resolved.Hint())
		assert.Zero(t, resolved.Members())
		assert.Equal(t, "", resolved.Render())
		assert.Equal(t, "", fmt.Sprintf("%+v", resolved))
	})

	t.Run("UsageErrorf wrapping an empty-message error keeps its pieces", func(t *testing.T) {
		t.Parallel()

		emptyHinted := &devError{message: "", hint: "run login"}

		wrappers := map[string]func(*conga.Session) error{
			"UsageErrorf": func(session *conga.Session) error {
				return session.UsageErrorf("not logged in: %w", emptyHinted)
			},
			"UsageErrorf in fmt.Errorf": func(session *conga.Session) error {
				return fmt.Errorf("startup: %w", session.UsageErrorf("not logged in: %w", emptyHinted))
			},
		}

		for name, validate := range wrappers {
			err := bootstrapApp(&devValidateCLI{validate: validate}).Args([]string{}).RunContext(t.Context())
			usage := assertUsageError(t, err)

			hinter, ok := errors.AsType[conga.Hinter](err)
			assert.True(t, ok, name)
			assert.Equal(t, "run login", usage.Hint(), name)
			assert.Equal(t, "run login", hinter.Hint(), name)
			assert.Equal(t, "run login", conga.DiagnosticOf(err).Hint(), name)
		}

		err := bootstrapApp(&devValidateCLI{validate: wrappers["UsageErrorf"]}).Args([]string{}).RunContext(t.Context())
		result := exitOutputFor(t, err)

		assert.Equal(t, "app: error: not logged in: \n  hint: run login\n\nUsage:\n  app [flags]\n\nRun 'app --help' for more information.\n", result.output)
		assert.Equal(t, 2, result.code)
	})

	t.Run("a single empty-message error prints nothing and exits 1", func(t *testing.T) {
		t.Parallel()

		result, _ := failWith(t, &devError{message: "", hint: "ignored"})

		assert.Equal(t, "", result.output)
		assert.Equal(t, 1, result.code)
	})

	t.Run("session.UsageError with an empty message prints the usage block only", func(t *testing.T) {
		t.Parallel()

		cli := &devValidateCLI{validate: func(session *conga.Session) error {
			return session.UsageError("")
		}}

		var buffer bytes.Buffer

		code := -1
		app := bootstrapApp(cli).
			Name("app").
			Args([]string{}).
			Stderr(&buffer).
			ExitFunc(func(exitCode int) { code = exitCode })

		err := app.RunContext(t.Context())
		app.Exit(err)

		assert.Equal(t, "", conga.DiagnosticOf(err).Message())
		assert.Zero(t, conga.DiagnosticOf(err).Members())
		assert.Equal(t, "Usage:\n  app [flags]\n\nRun 'app --help' for more information.\n", buffer.String())
		assert.Equal(t, 2, code)
	})

	t.Run("a join with a leading empty-message member uses the first non-empty member", func(t *testing.T) {
		t.Parallel()

		joined := errors.Join(
			&devError{message: "", origins: []string{"ignored"}, note: "ignored", hint: "ignored"},
			&devError{message: "real", origins: []string{"real origin"}, note: "real note", hint: "real hint"},
		)

		result, err := failWith(t, joined)

		resolved := conga.DiagnosticOf(err)

		jsonBytes, jsonErr := json.Marshal(resolved)
		assert.NoError(t, jsonErr)

		textBytes, textErr := resolved.MarshalText()
		assert.NoError(t, textErr)

		assert.Equal(t, "app: error: real\n  from: real origin\n  note: real note\n  hint: real hint\n", result.output)
		assert.Equal(t, []string{"real origin"}, resolved.Origins())
		assert.Equal(t, "real note", resolved.Note())
		assert.Equal(t, "real hint", resolved.Hint())
		assert.Equal(t, 1, len(resolved.Members()))
		assert.Equal(t, "real", resolved.Members()[0].Message())
		assert.Equal(t, "real\n  from: real origin\n  note: real note\n  hint: real hint", resolved.Render())
		assert.Equal(t, resolved.Render(), fmt.Sprintf("%+v", resolved))
		assert.Equal(t, `{"error":"real","from":["real origin"],"note":"real note","hint":"real hint"}`, string(jsonBytes))
		assert.Equal(t, "real (from: real origin; note: real note; hint: real hint)", string(textBytes))
	})
}

// emptyHintWrapperError is a third-party-style wrapper whose Hint is empty.
type emptyHintWrapperError struct{ cause error }

func (e *emptyHintWrapperError) Error() string { return "wrapped: " + e.cause.Error() }
func (e *emptyHintWrapperError) Hint() string  { return "" }
func (e *emptyHintWrapperError) Unwrap() error { return e.cause }

//nolint:paralleltest // subtests mutate the process environment with t.Setenv
func TestDiagnosticOf_Shapes(t *testing.T) {
	t.Setenv("TOOL_TIMEOUT", "30")

	congaErr := conga.New[outputCLI]().Args([]string{}).RunContext(t.Context())
	line := `invalid duration "30" for flag --timeout`
	timeoutOrigin := []string{"environment variable TOOL_TIMEOUT"}
	timeoutNote := "accepted units: ns, us (or µs), ms, s, m, h"
	typoErr := conga.New[outputCLI]().Args([]string{"--timout", "30s"}).RunContext(t.Context())
	first := &devError{message: "first", origins: []string{"a"}, note: "n1", hint: "h1"}
	second := &devError{message: "second", hint: "h2"}

	tests := []struct {
		name        string
		err         error
		message     string
		origins     []string
		note        string
		hint        string
		memberCount int
	}{
		{name: "a CONGA error", err: congaErr, message: line, origins: timeoutOrigin, note: timeoutNote, memberCount: 1},
		{name: "a CONGA error under fmt.Errorf", err: fmt.Errorf("startup: %w", congaErr), message: "startup: " + line, origins: timeoutOrigin, note: timeoutNote, memberCount: 1},
		{name: "a CONGA error with its own hint", err: typoErr, message: "unknown flag: --timout", hint: "did you mean --timeout?", memberCount: 1},
		{
			name:        "a developer wrapper with its own hint wins over a CONGA hint",
			err:         &devError{message: "startup failed", hint: "check the flag name", cause: typoErr},
			message:     "startup failed",
			hint:        "check the flag name",
			memberCount: 1,
		},
		{
			name:        "a developer wrapper with its own hint wins",
			err:         &devError{message: "startup failed", hint: "check TOOL_TIMEOUT", cause: congaErr},
			message:     "startup failed",
			origins:     timeoutOrigin,
			note:        timeoutNote,
			hint:        "check TOOL_TIMEOUT",
			memberCount: 1,
		},
		{name: "a developer type implementing only Hinter", err: &devHintOnlyError{hint: "act"}, message: "dev failure", hint: "act", memberCount: 1},
		{name: "a wrapper whose Hint is empty", err: &emptyHintWrapperError{cause: &devHintOnlyError{hint: "act"}}, message: "wrapped: dev failure", hint: "act", memberCount: 1},
		{name: "a join", err: errors.Join(first, second), message: "first\nsecond", origins: []string{"a"}, note: "n1", hint: "h1", memberCount: 2},
		{name: "a join with a leading empty-message member", err: errors.Join(&devError{message: "", hint: "ignored"}, first), message: "\nfirst", origins: []string{"a"}, note: "n1", hint: "h1", memberCount: 1},
		{name: "fmt.Errorf with two %w is one member", err: fmt.Errorf("%w and %w", first, second), message: "first and second", origins: []string{"a"}, note: "n1", hint: "h1", memberCount: 1},
		{name: "a single empty-message error", err: &devError{message: "", hint: "ignored"}},
		{name: "nil"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			diagnostic := conga.DiagnosticOf(test.err)

			assert.Equal(t, test.message, diagnostic.Message())
			assert.Equal(t, test.origins, diagnostic.Origins())
			assert.Equal(t, test.note, diagnostic.Note())
			assert.Equal(t, test.hint, diagnostic.Hint())
			assert.Equal(t, test.memberCount, len(diagnostic.Members()))
			assert.Equal(t, diagnostic.Render(), fmt.Sprintf("%+v", diagnostic))

			// Render is App.Exit's output without each member's prefix, the
			// trailing newline and any usage block.
			if test.err != nil {
				unprefixed := strings.ReplaceAll(exitLinesFor(test.err), "app: error: ", "")

				assert.Equal(t, diagnostic.Render(), strings.TrimSuffix(unprefixed, "\n"))
			}
		})
	}
}

func TestDiagnosticOf_FormatVerbs(t *testing.T) {
	t.Parallel()

	diagnostic := conga.DiagnosticOf(&devError{message: "failed", hint: "act"})

	assert.Equal(t, "failed", fmt.Sprintf("%v", diagnostic))
	assert.Equal(t, "failed\n  hint: act", fmt.Sprintf("%+v", diagnostic))
	assert.Equal(t, `"failed"`, fmt.Sprintf("%q", diagnostic))
	assert.Equal(t, "  failed", fmt.Sprintf("%8s", diagnostic))
	assert.Equal(t, "fai", fmt.Sprintf("%.3s", diagnostic))
	assert.Equal(t, `"failed"`, fmt.Sprintf("%#v", diagnostic))
}

// hintedNumber is a value whose validation fails with a hint and no other
// piece.
type hintedNumber int

func (n *hintedNumber) UnmarshalText(text []byte) error {
	value, err := strconv.Atoi(string(text))
	if err != nil {
		return err
	}

	*n = hintedNumber(value)

	return nil
}

func (n hintedNumber) Validate(_ context.Context) error {
	return &devHintOnlyError{hint: "pick another"}
}

//nolint:paralleltest // subtests mutate the process environment with t.Setenv
func TestErrorOutput_CausePiecesShowThrough(t *testing.T) {
	type hintedCLI struct {
		noopRunner

		Size conga.OptionalFlag[hintedNumber] `config:"size" default:"3" env:"TOOL_SIZE" name:"size"`
	}

	type limitCLI struct {
		noopRunner

		Limit conga.OptionalFlag[conga.ByteSize] `config:"limit" default:"5XB" env:"TOOL_LIMIT" name:"limit"`
	}

	unitNote := "accepted units: B, kB, MB, GB, TB, PB, EB, KiB, MiB, GiB, TiB, PiB, EiB"

	assertSurfaces := func(t *testing.T, err error, wantLines string, wantNote string, wantHint string) {
		t.Helper()

		result := exitOutputFor(t, err)

		assert.Equal(t, wantLines, result.output)
		assert.Equal(t, 1, result.code)
		assertNotUsageError(t, err)

		diagnostic := conga.DiagnosticOf(err)

		assert.Equal(t, wantNote, diagnostic.Note())
		assert.Equal(t, wantHint, diagnostic.Hint())

		noter, hasNoter := errors.AsType[conga.Noter](err)
		assert.True(t, hasNoter)
		assert.Equal(t, wantNote, noter.Note())

		hinter, hasHinter := errors.AsType[conga.Hinter](err)
		assert.True(t, hasHinter)
		assert.Equal(t, wantHint, hinter.Hint())

		originer, hasOriginer := errors.AsType[conga.Originer](err)
		assert.True(t, hasOriginer)
		assert.Equal(t, diagnostic.Origins(), originer.Origins())
	}

	t.Run("a SelfValidator hint from the environment", func(t *testing.T) {
		t.Setenv("TOOL_SIZE", "4")

		err := conga.New[hintedCLI]().Args([]string{}).RunContext(t.Context())

		assertSurfaces(t, err, "app: error: dev failure for flag --size\n  from: environment variable TOOL_SIZE\n  hint: pick another\n", "", "pick another")
	})

	t.Run("a SelfValidator hint from a config file", func(t *testing.T) {
		path := writeConfigFile(t, `{"size": 4}`)

		err := conga.New[hintedCLI]().
			RegisterDecoder(".json", jsondecoder.Decode).
			ConfigOptions(conga.ConfigExplicit(path)).
			Args([]string{}).
			RunContext(t.Context())

		assertSurfaces(t, err, "app: error: dev failure for flag --size\n  from: config key \"size\" in config file \""+path+"\"\n  hint: pick another\n", "", "pick another")
	})

	t.Run("a SelfValidator hint from a default", func(t *testing.T) {
		err := conga.New[hintedCLI]().Args([]string{}).RunContext(t.Context())

		assertSurfaces(t, err, "app: error: dev failure for flag --size\n  from: default value\n  hint: pick another\n", "", "pick another")
	})

	t.Run("a ByteSize note from the environment", func(t *testing.T) {
		t.Setenv("TOOL_LIMIT", "5XB")

		err := conga.New[limitCLI]().Args([]string{}).RunContext(t.Context())

		assertSurfaces(t, err, "app: error: unknown byte size unit \"XB\" in \"5XB\" for flag --limit\n  from: environment variable TOOL_LIMIT\n  note: "+unitNote+"\n", unitNote, "")
	})

	t.Run("a ByteSize note from a config file", func(t *testing.T) {
		path := writeConfigFile(t, `{"limit": "5XB"}`)

		err := conga.New[limitCLI]().
			RegisterDecoder(".json", jsondecoder.Decode).
			ConfigOptions(conga.ConfigExplicit(path)).
			Args([]string{}).
			RunContext(t.Context())

		assertSurfaces(t, err, "app: error: unknown byte size unit \"XB\" in \"5XB\" for flag --limit\n  from: config key \"limit\" in config file \""+path+"\"\n  note: "+unitNote+"\n", unitNote, "")
	})

	t.Run("a ByteSize note from a default", func(t *testing.T) {
		err := conga.New[limitCLI]().Args([]string{}).RunContext(t.Context())

		assertSurfaces(t, err, "app: error: unknown byte size unit \"XB\" in \"5XB\" for flag --limit\n  from: default value\n  note: "+unitNote+"\n", unitNote, "")
	})

	t.Run("an HCL labeled block note and hint", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "app.hcl")
		assert.NoError(t, os.WriteFile(path, []byte("server \"web\" {}\n"), 0o600))

		err := conga.New[hintedCLI]().
			RegisterDecoder(".hcl", hcldecoder.Decode).
			ConfigOptions(conga.ConfigExplicit(path)).
			Args([]string{}).
			RunContext(t.Context())

		assertSurfaces(t, err, "app: error: cannot parse config file \""+path+"\": line 1, column 1: block \"server\" has labels [\"web\"]\n  note: labeled blocks are not supported\n  hint: use an unlabeled block\n", "labeled blocks are not supported", "use an unlabeled block")
	})

	t.Run("an HCL syntax error's detail as the note", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "app.hcl")
		assert.NoError(t, os.WriteFile(path, []byte("timeout = \n"), 0o600))

		err := conga.New[hintedCLI]().
			RegisterDecoder(".hcl", hcldecoder.Decode).
			ConfigOptions(conga.ConfigExplicit(path)).
			Args([]string{}).
			RunContext(t.Context())

		wantLines := "app: error: cannot parse config file \"" + path + "\": line 1, column 11: Invalid expression\n" +
			"  note: Expected the start of an expression, but found an invalid expression token.\n"

		assertSurfaces(t, err, wantLines, "Expected the start of an expression, but found an invalid expression token.", "")
	})
}
