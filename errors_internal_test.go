package conga

import (
	"bytes"
	"errors"
	"os/exec"
	"reflect"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/coerce"
	"github.com/conga-sh/conga/internal/intx"
	"github.com/conga-sh/conga/internal/spec"
)

func TestUsageError_Methods(t *testing.T) {
	t.Parallel()

	rootCommand := &spec.Command{Name: "tool"}
	childCommand := &spec.Command{Name: "deploy"}
	cause := errors.New("underlying")

	t.Run("message, path, code and no cause", func(t *testing.T) {
		t.Parallel()

		err := usageError([]*spec.Command{rootCommand, childCommand}, "missing required flag: --port")

		assert.Equal(t, "missing required flag: --port", err.Error())
		assert.Equal(t, "tool deploy", err.CommandPath())
		assert.Equal(t, 2, err.ExitCode())
		assert.Equal(t, nil, err.Unwrap())
		assert.Equal(t, "Usage:\n  tool deploy", err.Usage())
	})

	t.Run("suggestion is appended to the message", func(t *testing.T) {
		t.Parallel()

		err := usageError(nil, "unknown flag: --prot").withSuggestion("--port", suggestionKindFlag)

		assert.Equal(t, `unknown flag: --prot (did you mean "--port"?)`, err.Error())
	})

	t.Run("an empty suggestion sets nothing", func(t *testing.T) {
		t.Parallel()

		err := usageError(nil, "unknown flag: --zzz").withSuggestion("", suggestionKindFlag)

		assert.Equal(t, "unknown flag: --zzz", err.Error())
		assert.Equal(t, suggestionKindNone, err.suggestionKind)
	})

	t.Run("shape defaults to the mode and builds on request", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, usageShapeMode, usageError(nil, "x").shape)
		assert.Equal(t, usageShapeFullHelp, usageError(nil, "x").withShape(usageShapeFullHelp).shape)
	})

	t.Run("cause is reachable", func(t *testing.T) {
		t.Parallel()

		err := usageError(nil, "bad").withCause(cause)

		assert.Equal(t, cause, err.Unwrap())
		assert.True(t, errors.Is(err, cause))
	})

	t.Run("zero value renders empty without panicking", func(t *testing.T) {
		t.Parallel()

		err := &UsageError{}

		assert.Equal(t, "", err.Error())
		assert.Equal(t, "", err.CommandPath())
		assert.Equal(t, "", err.Usage())
		assert.Equal(t, 2, err.ExitCode())
		assert.Equal(t, nil, err.Unwrap())
	})
}

func TestInputError(t *testing.T) {
	t.Parallel()

	cause := errors.New("underlying")

	portFlag := &spec.Flag{Name: "port"}
	err := &inputError{flag: portFlag, origin: fromCommandLine, msg: "port must be even", cause: cause}

	assert.Equal(t, `port must be even for flag --port`, err.Error())
	assert.Equal(t, cause, err.Unwrap())
	assert.Equal(t, nil, (&inputError{msg: "plain"}).Unwrap())

	_, isCoder := errors.AsType[ExitCoder](err)
	assert.False(t, isCoder)
}

func TestConversionError_Message(t *testing.T) {
	t.Parallel()

	countArg := &spec.Arg{Name: "COUNT", TargetType: reflect.TypeFor[int]()}
	portFlag := &spec.Flag{Name: "port", TargetType: reflect.TypeFor[int]()}
	leadingZero := &intx.Error{Kind: intx.KindLeadingZero, Raw: "0755"}

	tests := []struct {
		name string
		err  *conversionError
		want string
	}{
		{
			name: "no cause names the type",
			err:  &conversionError{arg: countArg, value: "x", origin: fromCommandLine},
			want: `invalid integer value "x" for argument COUNT`,
		},
		{
			name: "a cause with no text names the type",
			err:  &conversionError{arg: countArg, value: "x", origin: fromCommandLine, cause: errors.New("")},
			want: `invalid integer value "x" for argument COUNT`,
		},
		{
			name: "a cause's text is the message",
			err:  &conversionError{flag: portFlag, value: "3", origin: valueOrigin{source: SourceEnv, name: "PORT"}, cause: errors.New("port must be even")},
			want: "port must be even for flag --port (from environment variable PORT)",
		},
		{
			name: "a default names its origin",
			err:  &conversionError{arg: countArg, value: "x", origin: fromDefault},
			want: `invalid integer value "x" for argument COUNT (from default value)`,
		},
		{
			name: "the cause's leading-zero correction ends the message",
			err:  &conversionError{flag: portFlag, value: "0755", origin: fromCommandLine, cause: leadingZero},
			want: `integer "0755" has a leading zero for flag --port (a leading zero is ambiguous: write 0o755 for octal or 755 for decimal)`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.want, test.err.Error())
		})
	}
}

func TestInputError_Message(t *testing.T) {
	t.Parallel()

	portFlag := &spec.Flag{Name: "port", TargetType: reflect.TypeFor[int]()}

	tests := []struct {
		name string
		err  *inputError
		want string
	}{
		{
			name: "the message names the flag and origin",
			err:  &inputError{flag: portFlag, origin: valueOrigin{source: SourceEnv, name: "PORT"}, msg: "value 9 is out of range [1..5]"},
			want: "value 9 is out of range [1..5] for flag --port (from environment variable PORT)",
		},
		{
			name: "an environment origin with no name renders nothing",
			err:  &inputError{flag: portFlag, origin: valueOrigin{source: SourceEnv}, msg: "item count 1 is out of range [2..3]"},
			want: "item count 1 is out of range [2..3] for flag --port",
		},
		{
			name: "a default renders its origin without a name",
			err:  &inputError{flag: portFlag, origin: fromDefault, msg: "item count 1 is out of range [2..3]"},
			want: "item count 1 is out of range [2..3] for flag --port (from default value)",
		},
		{
			name: "a failure for no flag or argument is the message alone",
			err:  &inputError{origin: fromCommandLine, msg: "flags --a and --b are mutually exclusive"},
			want: "flags --a and --b are mutually exclusive",
		},
		{
			name: "a hint ends the message",
			err:  &inputError{flag: portFlag, origin: fromCommandLine, msg: "rejected", hint: "write 9"},
			want: "rejected for flag --port (write 9)",
		},
		{
			name: "a feature is the subject",
			err:  &inputError{feature: "http3", origin: valueOrigin{source: SourceEnv, name: "TOOL_FEATURES"}, msg: `invalid value "maybe"`},
			want: `invalid value "maybe" for feature "http3" (from environment variable TOOL_FEATURES)`,
		},
		{
			name: "a map entry follows the flag",
			err:  &inputError{flag: portFlag, entry: new("web"), origin: valueOrigin{source: SourceConfig, name: "counts"}, msg: "value 3 is odd"},
			want: `value 3 is odd for flag --port entry "web" (from config key "counts")`,
		},
		{
			name: "an empty map key is still an entry",
			err:  &inputError{flag: portFlag, entry: new(""), origin: fromCommandLine, msg: "bad"},
			want: `bad for flag --port entry ""`,
		},
		{
			name: "the note follows the origin and precedes the hint",
			err:  &inputError{feature: "http3", origin: valueOrigin{source: SourceConfig, name: "features.http3"}, msg: `invalid value "x"`, note: "must be true or false", hint: "write true"},
			want: `invalid value "x" for feature "http3" (from config key "features.http3"): must be true or false (write true)`,
		},
		{
			name: "an unknown feature carries only a message and a hint",
			err:  &inputError{origin: fromCommandLine, msg: `unknown feature "htp3"`, hint: `did you mean "http3"?`},
			want: `unknown feature "htp3" (did you mean "http3"?)`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.want, test.err.Error())
		})
	}
}

func TestCommandLineError(t *testing.T) {
	t.Parallel()

	chain := []*spec.Command{{Name: "tool"}}
	countArg := &spec.Arg{Name: "COUNT", TargetType: reflect.TypeFor[int]()}
	cause := errors.New("bad count")

	t.Run("a command-line conversion becomes a usage error printing the line alone", func(t *testing.T) {
		t.Parallel()

		err := commandLineError(chain, &conversionError{arg: countArg, value: "x", origin: fromCommandLine, cause: cause})

		usageErr, ok := errors.AsType[*UsageError](err)
		assert.True(t, ok)
		assert.Equal(t, "bad count for argument COUNT", usageErr.Error())
		assert.Equal(t, usageShapeLineOnly, usageErr.shape)
		assert.Equal(t, "tool", usageErr.CommandPath())
		assert.Equal(t, cause, usageErr.Unwrap())
	})

	t.Run("a command-line constraint naming its argument prints the line alone", func(t *testing.T) {
		t.Parallel()

		err := commandLineError(chain, &inputError{arg: countArg, origin: fromCommandLine, msg: "value 9 is out of range [1..5]"})

		usageErr, ok := errors.AsType[*UsageError](err)
		assert.True(t, ok)
		assert.Equal(t, "value 9 is out of range [1..5] for argument COUNT", usageErr.Error())
		assert.Equal(t, usageShapeLineOnly, usageErr.shape)
	})

	t.Run("a command-line failure naming no flag or argument keeps the mode's usage block", func(t *testing.T) {
		t.Parallel()

		err := commandLineError(chain, &inputError{origin: fromCommandLine, msg: "flags --a and --b are mutually exclusive"})

		usageErr, ok := errors.AsType[*UsageError](err)
		assert.True(t, ok)
		assert.Equal(t, "flags --a and --b are mutually exclusive", usageErr.Error())
		assert.Equal(t, usageShapeMode, usageErr.shape)
	})

	t.Run("a command-line feature failure prints the line alone", func(t *testing.T) {
		t.Parallel()

		err := commandLineError(chain, &inputError{feature: "http3", origin: fromCommandLine, msg: `invalid value "x"`, note: "must be true or false"})

		usageErr, ok := errors.AsType[*UsageError](err)
		assert.True(t, ok)
		assert.Equal(t, `invalid value "x" for feature "http3": must be true or false`, usageErr.Error())
		assert.Equal(t, usageShapeLineOnly, usageErr.shape)
	})

	t.Run("a command-line map entry failure prints the line alone", func(t *testing.T) {
		t.Parallel()

		err := commandLineError(chain, &inputError{flag: &spec.Flag{Name: "evens"}, entry: new("a"), origin: fromCommandLine, msg: "value 3 is odd", cause: cause})

		usageErr, ok := errors.AsType[*UsageError](err)
		assert.True(t, ok)
		assert.Equal(t, `value 3 is odd for flag --evens entry "a"`, usageErr.Error())
		assert.Equal(t, usageShapeLineOnly, usageErr.shape)
		assert.Equal(t, cause, usageErr.Unwrap())
	})

	t.Run("an unknown feature from the command line keeps the mode's usage block", func(t *testing.T) {
		t.Parallel()

		err := commandLineError(chain, &inputError{origin: fromCommandLine, msg: `unknown feature "htp3"`, hint: `did you mean "http3"?`})

		usageErr, ok := errors.AsType[*UsageError](err)
		assert.True(t, ok)
		assert.Equal(t, `unknown feature "htp3" (did you mean "http3"?)`, usageErr.Error())
		assert.Equal(t, usageShapeMode, usageErr.shape)
	})

	t.Run("a conversion from elsewhere is returned unchanged", func(t *testing.T) {
		t.Parallel()

		conversion := &conversionError{arg: countArg, value: "x", origin: fromDefault}

		assert.Equal(t, error(conversion), commandLineError(chain, conversion))
	})

	t.Run("a constraint from elsewhere is returned unchanged", func(t *testing.T) {
		t.Parallel()

		input := &inputError{arg: countArg, origin: valueOrigin{source: SourceEnv, name: "COUNT"}, msg: "too big"}

		assert.Equal(t, error(input), commandLineError(chain, input))
	})

	t.Run("any other error is returned unchanged", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, cause, commandLineError(chain, cause))
	})
}

func TestCompletionError_ExitCoder(t *testing.T) {
	t.Parallel()

	err := &completionError{cause: "boom"}

	coder, ok := errors.AsType[ExitCoder](err)

	assert.True(t, ok)
	assert.Equal(t, 2, coder.ExitCode())
	assert.Equal(t, "internal completion failure: boom", err.Error())
}

func TestApp_ExitCode_CompletionError(t *testing.T) {
	t.Parallel()

	var stderrBuffer bytes.Buffer

	exitCode := -1
	err := &completionError{cause: "boom"}

	app := New[emptyCLI]().
		Stderr(&stderrBuffer).
		ExitFunc(func(code int) { exitCode = code })

	app.Exit(err)

	assert.Equal(t, 2, app.ExitCode(err))
	assert.Equal(t, 2, exitCode)
	assert.Equal(t, "internal completion failure: boom\n", stderrBuffer.String())
}

func TestMapEntryCause(t *testing.T) {
	t.Parallel()

	authorCause := errors.New("author failure")
	_, leafErr := coerce.Value[bool]("maybe")

	t.Run("a map entry over a CONGA error reaches it", func(t *testing.T) {
		t.Parallel()

		entryErr := &mapEntryError{key: "k", kind: mapEntryErrorValue, cause: leafErr}

		assert.Equal(t, leafErr, errors.Unwrap(entryErr))
	})

	t.Run("a map entry over an author error reaches it", func(t *testing.T) {
		t.Parallel()

		entryErr := &mapEntryError{key: "k", kind: mapEntryErrorValue, cause: authorCause}

		assert.True(t, errors.Is(entryErr, authorCause))
	})
}

type codedError struct {
	code    int
	message string
	cause   error
}

func (e *codedError) Error() string { return e.message }

func (e *codedError) ExitCode() int { return e.code }

func (e *codedError) Unwrap() error { return e.cause }

func TestExitCodeOf(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		coder    ExitCoder
		expected int
	}{
		{name: "custom code", coder: &codedError{code: 42}, expected: 42},
		{name: "lowest valid code", coder: &codedError{code: 1}, expected: 1},
		{name: "highest valid code", coder: &codedError{code: 255}, expected: 255},
		{name: "zero becomes 1", coder: &codedError{code: 0}, expected: 1},
		{name: "negative becomes 1", coder: &codedError{code: -1}, expected: 1},
		{name: "256 becomes 1", coder: &codedError{code: 256}, expected: 1},
		{name: "usage error", coder: &UsageError{}, expected: 2},
		{name: "completion error", coder: &completionError{cause: "x"}, expected: 2},
		{name: "exec error without process state", coder: &exec.ExitError{}, expected: 1},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, testCase.expected, exitCodeOf(testCase.coder))
		})
	}
}

func TestMapEntryError(t *testing.T) {
	t.Parallel()

	t.Run("key coercion failure", func(t *testing.T) {
		t.Parallel()

		cause := errors.New("invalid value")
		err := &mapEntryError{key: "k", kind: mapEntryErrorKey, cause: cause}

		assert.Equal(t, `key "k": invalid value`, err.Error())
		assert.Equal(t, cause, err.Unwrap())
	})

	t.Run("value coercion failure", func(t *testing.T) {
		t.Parallel()

		cause := errors.New("invalid integer value")
		err := &mapEntryError{key: "web", kind: mapEntryErrorValue, cause: cause}

		assert.Equal(t, `value for key "web": invalid integer value`, err.Error())
		assert.Equal(t, cause, err.Unwrap())
	})

	t.Run("unknown kind", func(t *testing.T) {
		t.Parallel()

		cause := errors.New("invalid value")
		err := &mapEntryError{key: "k", kind: mapEntryErrorKind(255), cause: cause}

		assert.Equal(t, `invalid map entry for key "k": invalid value`, err.Error())
	})
}

func TestMapEntryErrorNilCausePanics(t *testing.T) {
	t.Parallel()

	err := &mapEntryError{key: "k", kind: mapEntryErrorKey}

	assert.Panics(t, func() {
		_ = err.Error()
	})
}

func TestInvalidFeatureValueError(t *testing.T) {
	t.Parallel()

	authorCause := errors.New("author failure")

	err := invalidFeatureValueError(&spec.FeatureField{Name: "http3"}, "maybe", authorCause, SourceEnv, "TOOL_FEATURES")

	assert.Equal(t, `invalid value "maybe" for feature "http3" (from environment variable TOOL_FEATURES): author failure`, err.Error())
	assert.Equal(t, authorCause, errors.Unwrap(err))
}

func TestDecoderRequiredError(t *testing.T) {
	t.Parallel()

	err := newDecoderRequiredError("flag --config-file")

	assert.Equal(t, "flag --config-file requires a registered config decoder: call App.RegisterDecoder with a format decoder (e.g. decoder/json)", err.Error())

	_, isInput := errors.AsType[*inputError](err)
	assert.False(t, isInput)
	assert.Equal(t, err, commandLineError(nil, err))
}
