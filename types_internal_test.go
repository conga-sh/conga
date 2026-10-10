package conga

import (
	"reflect"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/container"
)

func TestClassifyContainer(t *testing.T) {
	t.Parallel()

	var target resolverCLI

	assertClassifiedAs(t, target, "OptionalFlag", container.KindFlag, false)
	assertClassifiedAs(t, target, "RequiredFlag", container.KindFlag, true)
	assertClassifiedAs(t, target, "OptionalArg", container.KindArg, false)
	assertClassifiedAs(t, target, "RequiredArg", container.KindArg, true)
	assertClassifiedAs(t, target, "Config", container.KindConfigFile, false)
	assertClassifiedAs(t, target, "Count", container.KindCounter, false)
	assertClassifiedAs(t, target, "Subcommand", container.KindCommand, false)
	assertClassifiedAs(t, target, "Dash", container.KindDashArgs, false)
	assertClassifiedAs(t, target, "Raw", container.KindRawArgs, false)
	assertClassifiedAs(t, target, "HelpCmd", container.KindHelpCommand, false)
	assertClassifiedAs(t, target, "VersionCmd", container.KindVersionCommand, false)
	assertClassifiedAs(t, target, "Completion", container.KindCompletionCommand, false)

	assert.Equal(t, container.Classification{}, classifyContainer(&target.Plain))
}

func TestClassifyContainerCommandTarget(t *testing.T) {
	t.Parallel()

	var target resolverCLI

	assert.Equal(t, any(&target.Subcommand.value), classifyContainer(&target.Subcommand).Target)
	assert.Equal(t, any(&target.HelpCmd), classifyContainer(&target.HelpCmd).Target)
	assert.Equal(t, any(&target.VersionCmd), classifyContainer(&target.VersionCmd).Target)
	assert.Equal(t, any(&target.Completion), classifyContainer(&target.Completion).Target)
}

func TestPathKindError(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		path     string
		kind     pathKind
		expected string
	}{
		{name: "expect file", path: "/tmp", kind: pathKindExpectFile, expected: `"/tmp" is a directory, but a file was expected`},
		{name: "expect directory", path: "/tmp/data.txt", kind: pathKindExpectDir, expected: `"/tmp/data.txt" is a file, but a directory was expected`},
		{name: "expect executable file", path: "/tmp", kind: pathKindExpectExecutableFile, expected: `"/tmp" is a directory, but an executable file was expected`},
		{name: "expect symlink", path: "/tmp/data.txt", kind: pathKindExpectSymlink, expected: `"/tmp/data.txt" is not a symbolic link`},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			err := &pathKindError{path: testCase.path, kind: testCase.kind}

			assert.Equal(t, testCase.expected, err.Error())
			assert.IsError(t, err, errPathWrongKind)
			assert.IsError(t, err, ErrCoerce)
		})
	}
}

func TestPathKindErrorUnknownKind(t *testing.T) {
	t.Parallel()

	err := &pathKindError{path: "/tmp", kind: pathKind(255)}

	assert.Equal(t, `invalid path "/tmp"`, err.Error())
}

func TestExecutableError(t *testing.T) {
	t.Parallel()

	err := &executableError{path: "/opt/bin/tool"}

	assert.Equal(t, `"/opt/bin/tool" is not executable`, err.Error())
	assert.IsError(t, err, ErrValidation)
}

func TestByteSizeError(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		err      *byteSizeError
		expected string
	}{
		{name: "empty", err: &byteSizeError{kind: byteSizeErrorEmpty}, expected: "empty byte size"},
		{name: "invalid", err: &byteSizeError{raw: "10G", kind: byteSizeErrorInvalid}, expected: `invalid byte size "10G"`},
		{name: "invalid number", err: &byteSizeError{raw: "1.2.3", kind: byteSizeErrorInvalidNumber}, expected: `invalid byte size number "1.2.3"`},
		{name: "inexact fraction", err: &byteSizeError{raw: "0.5B", kind: byteSizeErrorInexactFraction}, expected: `byte size "0.5B" is not a whole number of bytes`},
		{name: "unknown unit", err: &byteSizeError{raw: "10ZB", unit: "ZB", kind: byteSizeErrorUnknownUnit}, expected: `unknown byte size unit "ZB" in "10ZB"`},
		{name: "ambiguous unit", err: &byteSizeError{raw: "5KB", unit: "KB", kind: byteSizeErrorAmbiguousUnit}, expected: `ambiguous byte size unit "KB" in "5KB": use kB (1000) or KiB (1024)`},
		{name: "unknown kind", err: &byteSizeError{raw: "10", kind: byteSizeErrorKind(255)}, expected: `invalid byte size "10"`},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, testCase.expected, testCase.err.Error())
			assert.IsError(t, testCase.err, ErrCoerce)
		})
	}
}

// TestByteSizeStringIECFormatting pins the canonical IEC integer model for values
// with the unset render family (the zero value and internal constructions).
func TestByteSizeStringIECFormatting(t *testing.T) {
	t.Parallel()

	formatCases := []struct {
		name     string
		bytes    int64
		expected string
	}{
		{name: "zero bytes", bytes: 0, expected: "0B"},
		{name: "sub-kibibyte bytes", bytes: 512, expected: "512B"},
		{name: "kibibyte boundary", bytes: 1024, expected: "1KiB"},
		{name: "fractional kibibytes", bytes: 1536, expected: "1536B"},
		{name: "mebibyte boundary", bytes: 1024 * 1024, expected: "1MiB"},
		{name: "512 mebibytes", bytes: 512 * 1024 * 1024, expected: "512MiB"},
		{name: "gibibyte boundary", bytes: 1024 * 1024 * 1024, expected: "1GiB"},
		{name: "2 gibibytes", bytes: 2 * 1024 * 1024 * 1024, expected: "2GiB"},
		{name: "tebibyte boundary", bytes: 1024 * 1024 * 1024 * 1024, expected: "1TiB"},
		{name: "pebibyte boundary", bytes: 1024 * 1024 * 1024 * 1024 * 1024, expected: "1PiB"},
		{name: "indivisible remainder", bytes: 1048577, expected: "1048577B"},
		{name: "boundary minus one", bytes: 1048575, expected: "1048575B"},
	}

	for _, formatCase := range formatCases {
		t.Run(formatCase.name, func(t *testing.T) {
			t.Parallel()

			byteSize := ByteSize{bytes: formatCase.bytes}

			assert.Equal(t, formatCase.expected, byteSize.String())
		})
	}
}

// TestByteSizeMarshalTextRoundTrip verifies MarshalText/UnmarshalText preserve the
// exact byte count for values with the unset render family.
func TestByteSizeMarshalTextRoundTrip(t *testing.T) {
	t.Parallel()

	for _, value := range []int64{0, 1, 1023, 1024, 1536, 1048576, 1048577, 536870912, 1500000000, 1 << 40} {
		byteSize := ByteSize{bytes: value}

		text, err := byteSize.MarshalText()
		assert.NoError(t, err)

		var reparsed ByteSize
		assert.NoError(t, reparsed.UnmarshalText(text))
		assert.Equal(t, value, reparsed.Bytes())
	}
}

// resolverDummyCmd is the inner command struct for classifier tests.
type resolverDummyCmd struct{}

// resolverCLI declares every public container type to pin the classifier mapping.
type resolverCLI struct {
	OptionalFlag OptionalFlag[string]
	RequiredFlag RequiredFlag[int]
	OptionalArg  OptionalArg[string]
	RequiredArg  RequiredArg[bool]
	Config       ConfigFile
	Count        Counter
	Subcommand   Command[resolverDummyCmd]
	Dash         DashArgs
	Raw          RawArgs
	HelpCmd      HelpCommand
	VersionCmd   VersionCommand
	Completion   CompletionCommand
	Plain        string
}

// assertClassifiedAs classifies a field address through the engine classifier
// and asserts the resulting kind and required flag.
func assertClassifiedAs(t *testing.T, target resolverCLI, fieldName string, expectedKind container.Kind, expectedRequired bool) {
	t.Helper()

	fieldValue := reflect.ValueOf(&target).Elem().FieldByName(fieldName)

	classification := classifyContainer(fieldValue.Addr().Interface())

	assert.Equal(t, expectedKind, classification.Kind, "%s must classify as %v", fieldName, expectedKind)
	assert.Equal(t, expectedRequired, classification.Required, "%s required flag", fieldName)
}
