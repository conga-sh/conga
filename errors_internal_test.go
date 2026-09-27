package conga

import (
	"errors"
	"path/filepath"
	"reflect"
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

	missingPath := filepath.Join(t.TempDir(), "missing.json")

	loadedPaths, values, source, configErr := configengine.Resolver{Registry: configengine.NewRegistry()}.Resolve([]string{missingPath}, nil)

	assert.Error(t, configErr)
	assert.Zero(t, len(loadedPaths))
	assert.Zero(t, len(values))
	assert.Equal(t, SourceNone, source)
	assert.False(t, IsUsageError(configErr))

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

	eEmpty := &coercionError{}
	assert.Equal(t, "", FormatCoercionLocation(eEmpty))
}

func TestMapPairError(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		kind     mapPairErrorKind
		expected string
	}{
		{name: "invalid pair", kind: mapPairErrorInvalid, expected: `invalid key-value pair "BROKEN", expected KEY=VALUE`},
		{name: "empty key", kind: mapPairErrorEmptyKey, expected: `empty key in key-value pair "=value"`},
		{name: "unknown kind", kind: mapPairErrorKind(255), expected: `invalid key-value pair "=value"`},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			item := "=value"
			if testCase.kind == mapPairErrorInvalid {
				item = "BROKEN"
			}

			err := &mapPairError{item: item, kind: testCase.kind}

			assert.Equal(t, testCase.expected, err.Error())
			assert.IsError(t, err, ErrCoerce)
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

	t.Run("prose and taxonomy", func(t *testing.T) {
		t.Parallel()

		_, numError := strconv.Atoi("abc")
		err := &counterError{raw: "abc", cause: numError}

		assert.Equal(t, `invalid integer value "abc"`, err.Error())
		assert.IsError(t, err, ErrCoerce)
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

func TestMapRegistrationError(t *testing.T) {
	t.Parallel()

	err := &mapRegistrationError{targetType: reflect.TypeFor[map[string]int64]()}

	assert.Equal(t, "map type map[string]int64 requires registration via app.RegisterMap", err.Error())
	assert.IsError(t, err, ErrCoerce)
}

func TestMapParserError(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		kind     mapParserErrorKind
		expected string
	}{
		{
			name:     "not a map",
			kind:     mapParserErrorNotMap,
			expected: "registered parser for map[string]int returned int, expected a map",
		},
		{
			name:     "not convertible",
			kind:     mapParserErrorNotConvertible,
			expected: "registered parser for map[string]int returned map[string]string",
		},
		{
			name:     "unknown kind",
			kind:     mapParserErrorKind(255),
			expected: "registered parser for map[string]int returned map[string]string",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			returnedType := reflect.TypeFor[map[string]string]()
			if testCase.kind == mapParserErrorNotMap {
				returnedType = reflect.TypeFor[int]()
			}

			err := &mapParserError{targetType: reflect.TypeFor[map[string]int](), returnedType: returnedType, kind: testCase.kind}

			assert.Equal(t, testCase.expected, err.Error())
			assert.IsError(t, err, ErrCoerce)
		})
	}
}
