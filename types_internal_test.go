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

// resolverDummyCmd is the inner command struct for classifier tests.
type resolverDummyCmd struct{}

// resolverCLI declares every public container type to pin the classifier mapping.
type resolverCLI struct {
	OptionalFlag OptionalFlag[string]      `name:"optional-flag"`
	RequiredFlag RequiredFlag[int]         `name:"required-flag"`
	OptionalArg  OptionalArg[string]       `name:"[optional-arg]"`
	RequiredArg  RequiredArg[bool]         `name:"<required-arg>"`
	Config       ConfigFile                `name:"config"`
	Count        Counter                   `name:"counter"`
	Subcommand   Command[resolverDummyCmd] `name:"command"`
	Dash         DashArgs                  `name:"dash"`
	Raw          RawArgs                   `name:"raw"`
	HelpCmd      HelpCommand               `name:"help"`
	VersionCmd   VersionCommand            `name:"version"`
	Completion   CompletionCommand         `name:"completion"`
	Plain        string                    `name:"plain"`
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
