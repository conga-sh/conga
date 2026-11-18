package conga

import (
	"errors"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/configengine"
	"github.com/conga-sh/conga/internal/spec"
)

func TestIsUsageError_Classification(t *testing.T) {
	t.Parallel()

	assert.False(t, IsUsageError(nil))

	var err error

	err = &parseError{msg: "unknown flag", cause: ErrParse}
	assert.True(t, IsUsageError(err))

	err = &coercionError{targetName: "port", cause: ErrCoerce}
	assert.True(t, IsUsageError(err))

	err = &validationError{msg: "missing flag", cause: ErrValidation}
	assert.True(t, IsUsageError(err))

	err = &coercionError{targetName: "port", source: SourceEnv, cause: ErrCoerce}
	assert.False(t, IsUsageError(err))
	assert.True(t, errors.Is(err, ErrConfig))
	assert.False(t, errors.Is(err, ErrCoerce))
	assert.False(t, errors.Is(err, ErrValidation))

	err = &validationError{msg: "bad value", source: SourceDefault, cause: ErrValidation, valueShaped: true}
	assert.False(t, IsUsageError(err))
	assert.True(t, errors.Is(err, ErrConfig))
	assert.False(t, errors.Is(err, ErrValidation))
	assert.False(t, errors.Is(err, ErrCoerce))

	missingPath := filepath.Join(t.TempDir(), "missing.json")

	loadedPaths, values, source, err := configengine.Resolver{Registry: configengine.NewRegistry()}.Resolve([]string{missingPath}, nil)

	assert.Error(t, err)
	assert.Zero(t, len(loadedPaths))
	assert.Zero(t, values)
	assert.Equal(t, SourceNone, source)
	assert.False(t, IsUsageError(err))

	assert.False(t, IsUsageError(ErrSchema))

	err = errors.New("database connection refused")
	assert.False(t, IsUsageError(err))
}

func TestSelfDescribingError_Classification(t *testing.T) {
	t.Parallel()

	t.Run("coercion errors are always self-describing", func(t *testing.T) {
		t.Parallel()

		err := &coercionError{targetName: "port", cause: ErrCoerce}
		assert.True(t, err.selfDescribing())
	})

	t.Run("validation errors are not self-describing by default", func(t *testing.T) {
		t.Parallel()

		err := &validationError{msg: "missing required flag: --port", cause: errMissingFlag}
		assert.False(t, err.selfDescribing())
	})

	t.Run("value-shaped validation errors are self-describing", func(t *testing.T) {
		t.Parallel()

		err := &validationError{msg: "score 150 must be between 0 and 100 for flag --score", cause: errOutOfRange, valueShaped: true}
		assert.True(t, err.selfDescribing())
	})

	t.Run("parse errors do not implement the property", func(t *testing.T) {
		t.Parallel()

		err := &parseError{msg: "unknown command", cause: errUnknownCommand}

		_, ok := errors.AsType[selfDescribingError](err)
		assert.False(t, ok)
	})
}

func TestErrorCommandPath(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{Name: "testcmd"}

	t.Run("parse error with chain", func(t *testing.T) {
		t.Parallel()

		var emptyParse parseError

		assert.Equal(t, "", emptyParse.CommandPath())

		err := &parseError{cmdChain: []*spec.Command{cmd}}

		assert.Equal(t, "testcmd", err.CommandPath())
	})

	t.Run("coercion error with chain", func(t *testing.T) {
		t.Parallel()

		var emptyCoerce coercionError

		assert.Equal(t, "", emptyCoerce.CommandPath())

		err := &coercionError{cmdChain: []*spec.Command{cmd}}

		assert.Equal(t, "testcmd", err.CommandPath())
	})
}

func TestFormatCoercionLocation(t *testing.T) {
	t.Parallel()

	eEnvFlag := &coercionError{source: SourceEnv, sourceName: "MY_VAR", isFlag: true, targetName: "port"}
	assert.Equal(t, "for flag --port (from environment variable MY_VAR)", FormatCoercionLocation(eEnvFlag))

	eEnvArg := &coercionError{source: SourceEnv, sourceName: "MY_VAR", isFlag: false}
	assert.Equal(t, "for environment variable MY_VAR", FormatCoercionLocation(eEnvArg))

	eCfgFlag := &coercionError{source: SourceConfig, sourceName: "server.port", isFlag: true, targetName: "port"}
	assert.Equal(t, `for flag --port (from config key "server.port")`, FormatCoercionLocation(eCfgFlag))

	eCfgArg := &coercionError{source: SourceConfig, sourceName: "server.port", isFlag: false}
	assert.Equal(t, `for config key "server.port"`, FormatCoercionLocation(eCfgArg))

	eFlag := &coercionError{isFlag: true, targetName: "port"}
	assert.Equal(t, "for flag --port", FormatCoercionLocation(eFlag))

	eArg := &coercionError{isFlag: false, targetName: "path"}
	assert.Equal(t, "for argument path", FormatCoercionLocation(eArg))

	eDefaultFlag := &coercionError{source: SourceDefault, sourceName: "abc", isFlag: true, targetName: "port"}
	assert.Equal(t, "for flag --port (from default value)", FormatCoercionLocation(eDefaultFlag))

	eDefaultArg := &coercionError{source: SourceDefault, sourceName: "abc", isFlag: false, targetName: "[count]"}
	assert.Equal(t, "for argument [count] (from default value)", FormatCoercionLocation(eDefaultArg))

	eEmpty := &coercionError{}
	assert.Equal(t, "", FormatCoercionLocation(eEmpty))
}

func TestMapEntryError(t *testing.T) {
	t.Parallel()

	t.Run("key coercion failure", func(t *testing.T) {
		t.Parallel()

		cause := errors.New("invalid value")
		err := &mapEntryError{key: "k", kind: mapEntryErrorKey, cause: cause}

		assert.Equal(t, `key "k": invalid value`, err.Error())
		assert.IsError(t, err, ErrCoerce)
		assert.IsError(t, err, cause)
	})

	t.Run("value coercion failure", func(t *testing.T) {
		t.Parallel()

		cause := errors.New("invalid integer value")
		err := &mapEntryError{key: "web", kind: mapEntryErrorValue, cause: cause}

		assert.Equal(t, `value for key "web": invalid integer value`, err.Error())
		assert.IsError(t, err, ErrCoerce)
		assert.IsError(t, err, cause)
	})

	t.Run("unknown kind", func(t *testing.T) {
		t.Parallel()

		cause := errors.New("invalid value")
		err := &mapEntryError{key: "k", kind: mapEntryErrorKind(255), cause: cause}

		assert.Equal(t, `invalid map entry for key "k": invalid value`, err.Error())
		assert.IsError(t, err, ErrCoerce)
	})
}

func TestMapEntryErrorNilCausePanics(t *testing.T) {
	t.Parallel()

	err := &mapEntryError{key: "k", kind: mapEntryErrorKey}

	assert.Panics(t, func() {
		_ = err.Error()
	})
}

func TestCounterError(t *testing.T) {
	t.Parallel()

	t.Run("prose without category", func(t *testing.T) {
		t.Parallel()

		_, numError := strconv.ParseUint("abc", 10, strconv.IntSize-1)
		err := &counterError{raw: "abc", cause: numError}

		assert.Equal(t, `invalid unsigned integer value "abc"`, err.Error())
		assert.False(t, errors.Is(err, ErrCoerce))
	})

	t.Run("cause preserved for errors.As", func(t *testing.T) {
		t.Parallel()

		_, numError := strconv.Atoi("abc")
		err := &counterError{raw: "abc", cause: numError}

		var numErr *strconv.NumError
		assert.True(t, errors.As(err, &numErr))
		assert.Equal(t, "abc", numErr.Num)
	})
}
