package coerce

import (
	"errors"
	"reflect"
	"strconv"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/assertx"
)

func TestUnsupportedTypeError(t *testing.T) {
	t.Parallel()

	err := &unsupportedTypeError{targetType: reflect.TypeFor[chan int]()}

	assert.Equal(t, "unsupported type chan int", err.Error())
	assert.IsError(t, err, ErrUnsupportedType)
	assert.IsError(t, err, ErrCoerce)
}

func TestBooleanError(t *testing.T) {
	t.Parallel()

	err := &booleanError{raw: "maybe"}

	assert.Equal(t, `invalid boolean value "maybe", expected true or false`, err.Error())
	assert.IsError(t, err, ErrCoerce)
}

func TestDurationError(t *testing.T) {
	t.Parallel()

	err := &durationError{raw: "5parsecs"}

	assert.Equal(t, `invalid duration "5parsecs" (valid units: ms, s, m, h)`, err.Error())
	assert.IsError(t, err, ErrCoerce)
}

func TestURLError(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		kind     urlErrorKind
		expected string
	}{
		{name: "malformed", kind: urlErrorMalformed, expected: `invalid URL "ht tp://x"`},
		{name: "missing scheme", kind: urlErrorMissingScheme, expected: `invalid URL "example.com/x" (missing scheme, e.g. https://)`},
		{name: "unknown kind", kind: urlErrorKind(255), expected: `invalid URL "example.com/x"`},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			err := &urlError{raw: "example.com/x", kind: testCase.kind}
			if testCase.kind == urlErrorMalformed {
				err = &urlError{raw: "ht tp://x", kind: testCase.kind}
			}

			assert.Equal(t, testCase.expected, err.Error())
			assert.IsError(t, err, ErrCoerce)
		})
	}
}

func TestNumError(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		err      *numError
		expected string
	}{
		{
			name:     "invalid integer",
			err:      &numError{raw: "abc", noun: "integer", kind: numErrorInvalid},
			expected: `invalid integer value "abc"`,
		},
		{
			name:     "invalid unsigned integer",
			err:      &numError{raw: "-1", noun: "unsigned integer", kind: numErrorInvalid},
			expected: `invalid unsigned integer value "-1"`,
		},
		{
			name:     "invalid float",
			err:      &numError{raw: "1.2.3", noun: "float", kind: numErrorInvalid},
			expected: `invalid float value "1.2.3"`,
		},
		{
			name:     "out of range",
			err:      &numError{raw: "300", targetType: reflect.TypeFor[int8](), kind: numErrorOutOfRange},
			expected: `value "300" is out of range for int8`,
		},
		{
			name:     "unknown kind",
			err:      &numError{raw: "abc", kind: numErrorKind(255)},
			expected: `invalid numeric value "abc"`,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, testCase.expected, testCase.err.Error())
			assert.IsError(t, testCase.err, ErrCoerce)
		})
	}
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

func TestConversionError(t *testing.T) {
	t.Parallel()

	err := &conversionError{value: int64(42), targetType: reflect.TypeFor[string]()}

	assert.Equal(t, "cannot convert int64 to string", err.Error())
	assert.IsError(t, err, ErrCoerce)
}

func TestAssignabilityError(t *testing.T) {
	t.Parallel()

	err := &assignabilityError{valueType: reflect.TypeFor[int64](), targetType: reflect.TypeFor[int]()}

	assert.Equal(t, "registered parser returned int64 which is not assignable to int", err.Error())
	assert.IsError(t, err, ErrCoerce)
}

func TestWrappedError(t *testing.T) {
	t.Parallel()

	t.Run("delegates prose and reachability to cause", func(t *testing.T) {
		t.Parallel()

		marker := errors.New("parser exploded")
		err := &wrappedError{cause: marker}

		assert.Equal(t, "parser exploded", err.Error())
		assert.IsError(t, err, ErrCoerce)
		assert.IsError(t, err, marker)
	})

	t.Run("nil cause panics", func(t *testing.T) {
		t.Parallel()

		err := &wrappedError{}

		assertx.PanicsWithValue(t, "coerce: invariant violation: wrappedError requires a non-nil cause", func() {
			_ = err.Error()
		})
	})
}

func TestWrappedErrorAnchorsExternalCoerceLabel(t *testing.T) {
	t.Parallel()

	marker := errors.New("direct coerce error")
	external := errors.Join(ErrCoerce, marker)
	err := &wrappedError{cause: external}

	assert.Equal(t, external.Error(), err.Error())
	assert.IsError(t, err, ErrCoerce)
}

func TestNumErrorKindFromStrconv(t *testing.T) {
	t.Parallel()

	_, parseErr := strconv.Atoi("abc")

	err := wrapParseNumError(parseErr, "abc", reflect.TypeFor[int](), "integer")

	assert.Equal(t, `invalid integer value "abc"`, err.Error())
	assert.IsError(t, err, ErrCoerce)
}
