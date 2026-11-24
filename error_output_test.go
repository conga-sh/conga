package conga_test

import (
	"bytes"
	"context"
	_ "embed"
	"encoding"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga"
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
	Limit   conga.OptionalFlag[conga.ByteSize] `env:"TOOL_LIMIT"  name:"limit"`
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

// devHintOnly is a developer error with a hint and no other piece.
type devHintOnly struct{ hint string }

func (e *devHintOnly) Error() string { return "dev failure" }
func (e *devHintOnly) Hint() string  { return e.hint }

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

// exitOutputFor runs err through App.Exit of an app named "app" and captures it.
func exitOutputFor(t *testing.T, err error) exitCapture {
	t.Helper()

	var buffer bytes.Buffer

	code := -1

	conga.New[outputCLI]().
		Name("app").
		Stderr(&buffer).
		ExitFunc(func(exitCode int) { code = exitCode }).
		Exit(err)

	return exitCapture{output: buffer.String(), code: code}
}

// failWith runs an app whose Run returns err and captures App.Exit's output.
func failWith(t *testing.T, err error) (error, exitCapture) {
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

	return runErr, exitCapture{output: buffer.String(), code: code}
}

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

		_, result := failWith(t, wrapped)

		assert.Equal(t, "app: error: config failed: ambiguous byte size unit \"KB\" in \"5KB\"\n"+
			"  hint: ask the platform team\n", result.output)
	})

	t.Run("the note and the hint can come from different errors", func(t *testing.T) {
		var size conga.ByteSize

		inner := size.UnmarshalText([]byte("5XB"))
		wrapped := &devError{message: "deploy: " + inner.Error(), hint: "fix the manifest", cause: inner}

		_, result := failWith(t, wrapped)

		assert.Equal(t, "app: error: deploy: unknown byte size unit \"XB\" in \"5XB\"\n"+
			"  note: accepted units: B, kB, MB, GB, TB, PB, EB, KiB, MiB, GiB, TiB, PiB, EiB\n"+
			"  hint: fix the manifest\n", result.output)
	})

	t.Run("a type with only a hint prints one hint line", func(t *testing.T) {
		_, result := failWith(t, &devHintOnly{hint: "retry later"})

		assert.Equal(t, "app: error: dev failure\n  hint: retry later\n", result.output)
	})

	t.Run("empty methods print no lines", func(t *testing.T) {
		_, result := failWith(t, &devError{message: "dev failure"})

		assert.Equal(t, "app: error: dev failure\n", result.output)
	})

	t.Run("an empty first line prints no lines even with a hinter cause", func(t *testing.T) {
		wrapped := &devError{message: "", cause: &devHintOnly{hint: "retry later"}}

		result := exitOutputFor(t, wrapped)

		assert.Equal(t, "", result.output)
		assert.Equal(t, 1, result.code)
	})
}

func TestUsageError_Pieces(t *testing.T) {
	t.Run("UsageErrorf keeps the note from the wrapped chain and prints the usage block", func(t *testing.T) {
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
		cli := &devValidateCLI{validate: func(session *conga.Session) error {
			return session.UsageErrorf("%w", &devHintOnly{hint: "pass --limit"})
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
		err := conga.New[outputSubcommandCLI]().Args([]string{"deplo"}).RunContext(t.Context())

		usage := assertUsageError(t, err)

		assert.Equal(t, "unknown command: deplo", err.Error())
		assert.Equal(t, "did you mean deploy?", usage.Hint())
		assert.Equal(t, usage.Hint(), errlines.Hint(err))
	})

	t.Run("an unknown flag suggestion is the hint", func(t *testing.T) {
		err := conga.New[outputCLI]().Args([]string{"--timeot", "1s"}).RunContext(t.Context())

		usage := assertUsageError(t, err)

		assert.Equal(t, "unknown flag: --timeot", err.Error())
		assert.Equal(t, "did you mean --timeout?", usage.Hint())
	})

	t.Run("a conversion error's pieces equal Error's", func(t *testing.T) {
		err := conga.New[outputCLI]().Args([]string{"--limit", "5XB"}).RunContext(t.Context())

		usage := assertUsageError(t, err)

		assert.Equal(t, "accepted units: B, kB, MB, GB, TB, PB, EB, KiB, MiB, GiB, TiB, PiB, EiB", usage.Note())
		assertSamePieces(t, err, usage)
	})

	t.Run("a SelfValidator error's pieces equal Error's", func(t *testing.T) {
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
// as the *conga.Error err.
func assertSamePieces(t *testing.T, err error, usage *conga.UsageError) {
	t.Helper()

	runErr, ok := errors.AsType[*conga.Error](err)

	assert.True(t, ok)
	assert.Equal(t, runErr.Origins(), usage.Origins())
	assert.Equal(t, runErr.Note(), usage.Note())
	assert.Equal(t, runErr.Hint(), usage.Hint())
}

type outputSubcommandDeploy struct{ noopRunner }

type outputSubcommandCLI struct {
	noopRunner

	Deploy conga.Command[outputSubcommandDeploy] `summary:"Deploy"`
}

func TestErrorOutput_Wrapping(t *testing.T) {
	t.Run("a prefix wrap lands on line one", func(t *testing.T) {
		var size conga.ByteSize

		inner := size.UnmarshalText([]byte("5XB"))

		_, result := failWith(t, fmt.Errorf("load profile: %w", inner))

		assert.True(t, strings.HasPrefix(result.output, "app: error: load profile: unknown byte size unit \"XB\" in \"5XB\"\n  note: accepted units:"))
	})

	t.Run("a suffix wrap lands on line one", func(t *testing.T) {
		var size conga.ByteSize

		inner := size.UnmarshalText([]byte("5XB"))

		_, result := failWith(t, fmt.Errorf("%w (attempt 3)", inner))

		assert.True(t, strings.HasPrefix(result.output, "app: error: unknown byte size unit \"XB\" in \"5XB\" (attempt 3)\n  note: accepted units:"))
	})

	t.Run("a wrapper that transforms the text keeps the lines", func(t *testing.T) {
		var size conga.ByteSize

		inner := size.UnmarshalText([]byte("5XB"))
		transformed := &devError{message: strings.ToUpper(inner.Error()), cause: inner}

		_, result := failWith(t, transformed)

		assert.True(t, strings.HasPrefix(result.output, "app: error: UNKNOWN BYTE SIZE UNIT \"XB\" IN \"5XB\"\n  note: accepted units:"))
	})
}

func TestErrorOutput_DeveloperWrapperAroundRun(t *testing.T) {
	hinted := &devError{message: "main: run failed", hint: "see the log", cause: errors.New("boom")}

	result := exitOutputFor(t, hinted)

	assert.Equal(t, "app: error: main: run failed\n  hint: see the log\n", result.output)
	assert.Equal(t, 1, result.code)
}

func TestErrorOutput_Exit_RendersLikeARunError(t *testing.T) {
	var size conga.ByteSize

	inner := size.UnmarshalText([]byte("5XB"))

	_, viaRun := failWith(t, inner)
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

	err := conga.New[outputSubcommandCLI]().Args([]string{"deplo"}).RunContext(t.Context())

	return errors.Unwrap(err)
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

func TestError_RunReturnsIt(t *testing.T) {
	t.Run("a hook's error is found with errors.Is and keeps its text", func(t *testing.T) {
		hookErr := errors.New("hook failed")

		err, _ := failWith(t, hookErr)

		wrapped, ok := errors.AsType[*conga.Error](err)

		assert.True(t, ok)
		assert.IsError(t, err, hookErr)
		assert.Equal(t, "hook failed", err.Error())
		assert.Equal(t, "hook failed", wrapped.Error())
		assert.Equal(t, hookErr, wrapped.Unwrap())
	})

	t.Run("a prepare error is an Error", func(t *testing.T) {
		err := conga.New[outputCLI]().Args([]string{"--nope"}).RunContext(t.Context())

		_, isError := err.(*conga.Error) //nolint:errorlint // the concrete type Run returns is the property under test

		assert.True(t, isError)
		assertUsageError(t, err)
	})

	t.Run("a dotenv load failure is an Error", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), "missing.env")

		err := conga.New[outputCLI]().Dotenv(missing).Args([]string{}).RunContext(t.Context())

		wrapped, ok := errors.AsType[*conga.Error](err)

		assert.True(t, ok)
		assert.Equal(t, fmt.Sprintf("cannot read .env file %q: open %s: no such file or directory", missing, missing), wrapped.Error())
		assert.Equal(t, err, error(wrapped))
	})

	t.Run("a completion invocation failure is an Error", func(t *testing.T) {
		err := conga.New[outputCLI]().
			EnableCompletion().
			Stdout(failingWriter{}).
			Args([]string{"__complete", "bash", ""}).
			RunContext(t.Context())

		wrapped, ok := errors.AsType[*conga.Error](err)

		assert.True(t, ok)
		assert.True(t, strings.HasPrefix(wrapped.Error(), "internal completion failure: "))
		assert.Equal(t, err, error(wrapped))
	})

	t.Run("success is an untyped nil", func(t *testing.T) {
		err := conga.New[outputCLI]().Args([]string{}).RunContext(t.Context())

		assert.True(t, err == nil)
	})

	t.Run("the exit code works through it", func(t *testing.T) {
		err := conga.New[outputCLI]().Args([]string{"--level", "mid"}).RunContext(t.Context())

		assert.Equal(t, 2, conga.New[outputCLI]().ExitCode(err))

		coder, ok := errors.AsType[conga.ExitCoder](err)

		assert.True(t, ok)
		assert.Equal(t, 2, coder.ExitCode())
	})

	t.Run("a developer error with an exit code decides the code", func(t *testing.T) {
		err, result := failWith(t, &exitCodeError{code: 75, message: "exit 75"})

		assert.Equal(t, 75, conga.New[outputCLI]().ExitCode(err))
		assert.Equal(t, 75, result.code)
	})

	t.Run("Hinter on Run's result agrees with Exit", func(t *testing.T) {
		err, result := failWith(t, &devHintOnly{hint: "retry later"})

		hinter, ok := errors.AsType[conga.Hinter](err)

		assert.True(t, ok)
		assert.Equal(t, "retry later", hinter.Hint())
		assert.Contains(t, result.output, "  hint: retry later\n")
	})

	t.Run("a nested run is not wrapped twice", func(t *testing.T) {
		inner := conga.New[outputCLI]().Args([]string{"--level", "mid"}).RunContext(t.Context())

		err, _ := failWith(t, inner)

		assert.True(t, err == inner) //nolint:errorlint // identity of the returned value is the property under test
	})
}

// finallyCLI records the error its Finally hook receives.
type finallyCLI struct {
	received error
}

func (c *finallyCLI) Run(_ context.Context, _ *conga.Session) error { return errors.New("run failed") }

func (c *finallyCLI) Finally(_ context.Context, _ *conga.Session, err error) error {
	c.received = err

	return nil
}

func TestError_FinallyReceivesTheRawError(t *testing.T) {
	cli := &finallyCLI{}

	err := conga.New(cli).Args([]string{}).RunContext(t.Context())

	_, wrapped := err.(*conga.Error)           //nolint:errorlint // the concrete type is the property under test
	_, received := cli.received.(*conga.Error) //nolint:errorlint // the concrete type is the property under test

	assert.True(t, wrapped)
	assert.False(t, received)
	assert.Equal(t, "run failed", cli.received.Error())
}

func TestError_Formatting(t *testing.T) {
	t.Setenv("TOOL_TIMEOUT", "30")

	err := conga.New[outputCLI]().Args([]string{}).RunContext(t.Context())

	line := `invalid duration "30" for flag --timeout`

	t.Run("%v and %s print line one", func(t *testing.T) {
		assert.Equal(t, line, fmt.Sprintf("%v", err))
		assert.Equal(t, line, fmt.Sprintf("%s", err)) //nolint:gosimple,perfsprint // %s is the verb under test
	})

	t.Run("%+v prints line one and the lines, plain", func(t *testing.T) {
		assert.Equal(t, line+"\n"+
			"  from: environment variable TOOL_TIMEOUT\n"+
			"  note: accepted units: ns, us (or µs), ms, s, m, h", fmt.Sprintf("%+v", err))
	})

	t.Run("%q quotes line one", func(t *testing.T) {
		assert.Equal(t, fmt.Sprintf("%q", line), fmt.Sprintf("%q", err))
	})

	t.Run("width, precision, padding, and # format line one", func(t *testing.T) {
		assert.Equal(t, line[:10], fmt.Sprintf("%.10s", err))
		assert.Equal(t, fmt.Sprintf("%50s", line), fmt.Sprintf("%50s", err))
		assert.Equal(t, fmt.Sprintf("%-50v", line), fmt.Sprintf("%-50v", err))
		assert.Equal(t, fmt.Sprintf("%#v", line), fmt.Sprintf("%#v", err))
	})

	t.Run("a UsageError formats the same way", func(t *testing.T) {
		t.Setenv("TOOL_TIMEOUT", "")

		usageErr := conga.New[outputCLI]().Args([]string{"--level", "mid"}).RunContext(t.Context())

		usage := assertUsageError(t, usageErr)

		assert.Equal(t, `invalid value "mid" for flag --level`, fmt.Sprintf("%v", usage))
		assert.Equal(t, `invalid value "mid" for flag --level`+"\n  note: accepted values: low, high", fmt.Sprintf("%+v", usage))
		assert.Equal(t, `"invalid value \"mid\" for flag --level"`, fmt.Sprintf("%q", usage))
		assert.Equal(t, `invalid value`, fmt.Sprintf("%.13s", usage))
		assert.Equal(t, fmt.Sprintf("%-45v", `invalid value "mid" for flag --level`), fmt.Sprintf("%-45v", usage))
		assert.Equal(t, fmt.Sprintf("%#v", `invalid value "mid" for flag --level`), fmt.Sprintf("%#v", usage))
	})
}

// recordingTB records the message of the Fatalf an assertion makes.
type recordingTB struct {
	testing.TB

	message string
}

func (r *recordingTB) Helper() {}

func (r *recordingTB) Fatalf(format string, args ...any) {
	r.message = fmt.Sprintf(format, args...)
}

func TestError_AssertNoErrorShowsTheLines(t *testing.T) {
	t.Setenv("TOOL_TIMEOUT", "30")

	err := conga.New[outputCLI]().Args([]string{}).RunContext(t.Context())

	recorder := &recordingTB{}

	assert.NoError(recorder, err)

	assert.Equal(t, "Did not expect an error but got:\n"+
		"invalid duration \"30\" for flag --timeout\n"+
		"  from: environment variable TOOL_TIMEOUT\n"+
		"  note: accepted units: ns, us (or µs), ms, s, m, h", recorder.message)
}

func TestError_Marshalers(t *testing.T) {
	var size conga.ByteSize

	conga1 := size.UnmarshalText([]byte("5XB"))

	marshal := func(t *testing.T, err error) (string, string) {
		t.Helper()

		wrapped, ok := errors.AsType[*conga.Error](newRunError(t, err))

		assert.True(t, ok)

		jsonBytes, jsonErr := json.Marshal(wrapped)
		assert.NoError(t, jsonErr)

		textBytes, textErr := wrapped.MarshalText()
		assert.NoError(t, textErr)

		return string(jsonBytes), string(textBytes)
	}

	t.Run("a single error", func(t *testing.T) {
		jsonText, plainText := marshal(t, conga1)

		assert.Equal(t, `{"error":"unknown byte size unit \"XB\" in \"5XB\"","note":"accepted units: B, kB, MB, GB, TB, PB, EB, KiB, MiB, GiB, TiB, PiB, EiB"}`, jsonText)
		assert.Equal(t, `unknown byte size unit "XB" in "5XB" (note: accepted units: B, kB, MB, GB, TB, PB, EB, KiB, MiB, GiB, TiB, PiB, EiB)`, plainText)
	})

	t.Run("origins, a note and a hint", func(t *testing.T) {
		jsonText, plainText := marshal(t, &devError{message: "failed", origins: []string{"a", "b"}, note: "n", hint: "h"})

		assert.Equal(t, `{"error":"failed","from":["a","b"],"note":"n","hint":"h"}`, jsonText)
		assert.Equal(t, "failed (from: a; b; note: n; hint: h)", plainText)
	})

	t.Run("a join keeps the first member as the object and the rest under more", func(t *testing.T) {
		jsonText, plainText := marshal(t, errors.Join(&devError{message: "first", hint: "h1"}, &devError{message: "second", note: "n2"}))

		assert.Equal(t, `{"error":"first","hint":"h1","more":[{"error":"second","note":"n2"}]}`, jsonText)
		assert.Equal(t, "first (hint: h1) | second (note: n2)", plainText)
	})

	t.Run("a message with nothing after it has no brackets", func(t *testing.T) {
		jsonText, plainText := marshal(t, errors.New("plain"))

		assert.Equal(t, `{"error":"plain"}`, jsonText)
		assert.Equal(t, "plain", plainText)
	})

	t.Run("an empty message keeps the error key", func(t *testing.T) {
		jsonText, plainText := marshal(t, &devError{message: "", hint: "ignored"})

		assert.Equal(t, `{"error":""}`, jsonText)
		assert.Equal(t, "", plainText)
	})
}

// newRunError returns err as Run would, wrapped in a *conga.Error.
func newRunError(t *testing.T, err error) error {
	t.Helper()

	runErr, _ := failWith(t, err)

	return runErr
}

func TestError_Slog(t *testing.T) {
	err, _ := failWith(t, &devError{message: "failed", origins: []string{"the manifest"}, hint: "act"})

	t.Run("the JSON handler records an object", func(t *testing.T) {
		var buffer bytes.Buffer

		slog.New(slog.NewJSONHandler(&buffer, &slog.HandlerOptions{ReplaceAttr: dropTime})).Error("run failed", "err", err)

		assert.Equal(t, `{"level":"ERROR","msg":"run failed","err":{"error":"failed","from":["the manifest"],"hint":"act"}}`+"\n", buffer.String())
	})

	t.Run("the text handler records a line", func(t *testing.T) {
		var buffer bytes.Buffer

		slog.New(slog.NewTextHandler(&buffer, &slog.HandlerOptions{ReplaceAttr: dropTime})).Error("run failed", "err", err)

		assert.Equal(t, `level=ERROR msg="run failed" err="failed (from: the manifest; hint: act)"`+"\n", buffer.String())
	})

	t.Run("an error the developer wrapped falls back to Error", func(t *testing.T) {
		var buffer bytes.Buffer

		slog.New(slog.NewJSONHandler(&buffer, &slog.HandlerOptions{ReplaceAttr: dropTime})).Error("run failed", "err", fmt.Errorf("main: %w", err))

		assert.Equal(t, `{"level":"ERROR","msg":"run failed","err":"main: failed"}`+"\n", buffer.String())
	})
}

// dropTime removes the timestamp so the handler output is stable.
func dropTime(_ []string, attr slog.Attr) slog.Attr {
	if attr.Key == slog.TimeKey {
		return slog.Attr{}
	}

	return attr
}

func TestError_ReportsItself(t *testing.T) {
	err, _ := failWith(t, &devError{message: "failed", origins: []string{"a"}, note: "n", hint: "h"})

	wrapped, ok := errors.AsType[*conga.Error](err)

	assert.True(t, ok)
	assert.Equal(t, []string{"a"}, wrapped.Origins())
	assert.Equal(t, "n", wrapped.Note())
	assert.Equal(t, "h", wrapped.Hint())
	assert.Equal(t, 1, conga.New[outputCLI]().ExitCode(err))
}

// configFlagCLI declares a config file flag on an app with no decoder.
type configFlagCLI struct {
	noopRunner

	Config conga.ConfigFile `name:"config"`
}

// innerJoinedFinallyCLI returns a multi-member *conga.Error from an inner Run
// and adds an outer Finally failure.
type innerJoinedFinallyCLI struct {
	innerErr error
}

func (c *innerJoinedFinallyCLI) Run(_ context.Context, _ *conga.Session) error {
	return c.innerErr
}

func (c *innerJoinedFinallyCLI) Finally(_ context.Context, _ *conga.Session, _ error) error {
	return &devError{message: "outer finally failed", hint: "outer hint"}
}

func TestError_JoinedCongaErrorFlattens(t *testing.T) {
	innerApp := conga.New(&devValidateCLI{
		validate: func(_ *conga.Session) error {
			return errors.Join(
				&devError{message: "first inner", origins: []string{"origin a"}, note: "note a"},
				&devError{message: "second inner", hint: "hint b"},
			)
		},
	}).Args([]string{})

	innerErr := innerApp.RunContext(t.Context())

	t.Run("errors.Join with a multi-member *conga.Error", func(t *testing.T) {
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

	t.Run("an outer Finally failure joined with an inner *conga.Error", func(t *testing.T) {
		var buffer bytes.Buffer

		app := conga.New(&innerJoinedFinallyCLI{innerErr: innerErr}).
			Name("app").
			Args([]string{}).
			Stderr(&buffer).
			ExitFunc(func(int) {})

		err := app.RunContext(t.Context())
		app.Exit(err)

		wrapped, ok := errors.AsType[*conga.Error](err)
		assert.True(t, ok)

		jsonBytes, jsonErr := json.Marshal(wrapped)
		assert.NoError(t, jsonErr)

		textBytes, textErr := wrapped.MarshalText()
		assert.NoError(t, textErr)

		assert.Equal(t, "app: error: first inner\n"+
			"  from: origin a\n"+
			"  note: note a\n"+
			"app: error: second inner\n"+
			"  hint: hint b\n"+
			"app: error: outer finally failed\n"+
			"  hint: outer hint\n", buffer.String())
		assert.Equal(t, "first inner\n  from: origin a\n  note: note a\nsecond inner\n  hint: hint b\nouter finally failed\n  hint: outer hint", fmt.Sprintf("%+v", wrapped))
		assert.Equal(t, `{"error":"first inner","from":["origin a"],"note":"note a","more":[{"error":"second inner","hint":"hint b"},{"error":"outer finally failed","hint":"outer hint"}]}`, string(jsonBytes))
		assert.Equal(t, "first inner (from: origin a; note: note a) | second inner (hint: hint b) | outer finally failed (hint: outer hint)", string(textBytes))
	})
}

func TestError_EmptyMessageConsistency(t *testing.T) {
	t.Run("a single empty-message error reports no pieces through *Error", func(t *testing.T) {
		err, result := failWith(t, &devError{message: "", origins: []string{"o"}, note: "n", hint: "h"})

		wrapped, ok := errors.AsType[*conga.Error](err)
		assert.True(t, ok)

		assert.Equal(t, "", result.output)
		assert.Zero(t, wrapped.Origins())
		assert.Equal(t, "", wrapped.Note())
		assert.Equal(t, "", wrapped.Hint())
		assert.Equal(t, "", fmt.Sprintf("%+v", wrapped))

		hinter, hasHinter := errors.AsType[conga.Hinter](err)
		assert.True(t, hasHinter)
		assert.Equal(t, "", hinter.Hint())
	})

	t.Run("UsageErrorf with an empty message reports no pieces", func(t *testing.T) {
		cli := &devValidateCLI{validate: func(session *conga.Session) error {
			return session.UsageErrorf("%w", &devError{message: "", origins: []string{"o"}, note: "n", hint: "h"})
		}}

		err := bootstrapApp(cli).Args([]string{}).RunContext(t.Context())
		usage := assertUsageError(t, err)

		assert.Zero(t, usage.Origins())
		assert.Equal(t, "", usage.Note())
		assert.Equal(t, "", usage.Hint())
	})

	t.Run("a join with a leading empty-message member uses the first non-empty member", func(t *testing.T) {
		joined := errors.Join(
			&devError{message: "", origins: []string{"ignored"}, note: "ignored", hint: "ignored"},
			&devError{message: "real", origins: []string{"real origin"}, note: "real note", hint: "real hint"},
		)

		err, result := failWith(t, joined)

		wrapped, ok := errors.AsType[*conga.Error](err)
		assert.True(t, ok)

		jsonBytes, jsonErr := json.Marshal(wrapped)
		assert.NoError(t, jsonErr)

		textBytes, textErr := wrapped.MarshalText()
		assert.NoError(t, textErr)

		assert.Equal(t, "app: error: real\n  from: real origin\n  note: real note\n  hint: real hint\n", result.output)
		assert.Equal(t, []string{"real origin"}, wrapped.Origins())
		assert.Equal(t, "real note", wrapped.Note())
		assert.Equal(t, "real hint", wrapped.Hint())
		assert.Equal(t, "real\n  from: real origin\n  note: real note\n  hint: real hint", fmt.Sprintf("%+v", wrapped))
		assert.Equal(t, `{"error":"real","from":["real origin"],"note":"real note","hint":"real hint"}`, string(jsonBytes))
		assert.Equal(t, "real (from: real origin; note: real note; hint: real hint)", string(textBytes))
	})
}
