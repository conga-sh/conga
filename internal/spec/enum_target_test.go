package spec_test

import (
	"os"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/spec"
	"github.com/conga-sh/conga/predict"
)

// enumFileFilter implements predict.FileFilterable with a value receiver.
type enumFileFilter string

func (enumFileFilter) MatchFile(string, os.DirEntry) bool { return true }

func (f *enumFileFilter) UnmarshalText(text []byte) error {
	*f = enumFileFilter(text)

	return nil
}

// enumDirFilter implements predict.DirFilterable with a pointer receiver.
type enumDirFilter string

func (*enumDirFilter) MatchDir(string, os.DirEntry) bool { return true }

func (f *enumDirFilter) UnmarshalText(text []byte) error {
	*f = enumDirFilter(text)

	return nil
}

// enumPathFilter implements predict.PathFilterable.
type enumPathFilter string

func (enumPathFilter) MatchPath(string, os.DirEntry) bool { return true }

func (f *enumPathFilter) UnmarshalText(text []byte) error {
	*f = enumPathFilter(text)

	return nil
}

func TestBuild_EnumOnFilterableTypeRejected(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		newCLI   func() any
		expected string
	}{
		{
			name: "FileFilterable value receiver",
			newCLI: func() any {
				type CLI struct {
					Env mockFlag[enumFileFilter] `enum:"dev,prod"`
				}

				return &CLI{}
			},
			expected: `struct CLI field Env: tag "enum" is not supported on spec_test.enumFileFilter (hint: use a string field with tag "enum" to accept a fixed set of names)`,
		},
		{
			name: "DirFilterable pointer receiver",
			newCLI: func() any {
				type CLI struct {
					Env mockArg[enumDirFilter] `enum:"dev,prod"`
				}

				return &CLI{}
			},
			expected: `struct CLI field Env: tag "enum" is not supported on spec_test.enumDirFilter (hint: use a string field with tag "enum" to accept a fixed set of names)`,
		},
		{
			name: "PathFilterable slice",
			newCLI: func() any {
				type CLI struct {
					Env mockFlag[[]enumPathFilter] `enum:"dev,prod"`
				}

				return &CLI{}
			},
			expected: `struct CLI field Env: tag "enum" is not supported on spec_test.enumPathFilter (hint: use a string field with tag "enum" to accept a fixed set of names)`,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			_, err := buildSchema(testCase.newCLI(), "app", spec.Options{})

			assert.True(t, spec.IsSchemaError(err))
			assert.Equal(t, testCase.expected, err.Error())
		})
	}
}

func TestBuild_EnumWithPathPredictorRejected(t *testing.T) {
	t.Parallel()

	const fileMessage = `struct CLI field Env: tag "enum" cannot be combined with predictor:"file" (note: predictor:"file:<patterns>" filters completion candidates; hint: remove tag "enum", or remove the predictor tag to complete the enum values)`

	testCases := []struct {
		name     string
		newCLI   func() any
		expected string
	}{
		{
			name: "file",
			newCLI: func() any {
				type CLI struct {
					Env mockFlag[string] `enum:"dev,prod" predictor:"file"`
				}

				return &CLI{}
			},
			expected: fileMessage,
		},
		{
			name: "file with patterns",
			newCLI: func() any {
				type CLI struct {
					Env mockFlag[string] `enum:"dev,prod" predictor:"file:*.yaml"`
				}

				return &CLI{}
			},
			expected: fileMessage,
		},
		{
			name: "file with surrounding spaces",
			newCLI: func() any {
				type CLI struct {
					Env mockArg[string] `enum:"dev,prod" predictor:" file "`
				}

				return &CLI{}
			},
			expected: fileMessage,
		},
		{
			name: "dir",
			newCLI: func() any {
				type CLI struct {
					Env mockFlag[string] `enum:"dev,prod" predictor:"dir"`
				}

				return &CLI{}
			},
			expected: `struct CLI field Env: tag "enum" cannot be combined with predictor:"dir" (hint: remove tag "enum", or remove the predictor tag to complete the enum values)`,
		},
		{
			name: "exec",
			newCLI: func() any {
				type CLI struct {
					Env mockFlag[string] `enum:"dev,prod" predictor:"exec"`
				}

				return &CLI{}
			},
			expected: `struct CLI field Env: tag "enum" cannot be combined with predictor:"exec" (hint: remove tag "enum", or remove the predictor tag to complete the enum values)`,
		},
		{
			name: "executable names exec",
			newCLI: func() any {
				type CLI struct {
					Env mockFlag[string] `enum:"dev,prod" predictor:"executable"`
				}

				return &CLI{}
			},
			expected: `struct CLI field Env: tag "enum" cannot be combined with predictor:"exec" (hint: remove tag "enum", or remove the predictor tag to complete the enum values)`,
		},
		{
			name: "symlink",
			newCLI: func() any {
				type CLI struct {
					Env mockFlag[string] `enum:"dev,prod" predictor:"symlink"`
				}

				return &CLI{}
			},
			expected: `struct CLI field Env: tag "enum" cannot be combined with predictor:"symlink" (hint: remove tag "enum", or remove the predictor tag to complete the enum values)`,
		},
		{
			name: "symlinks names symlink",
			newCLI: func() any {
				type CLI struct {
					Env mockFlag[string] `enum:"dev,prod" predictor:"symlinks"`
				}

				return &CLI{}
			},
			expected: `struct CLI field Env: tag "enum" cannot be combined with predictor:"symlink" (hint: remove tag "enum", or remove the predictor tag to complete the enum values)`,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			_, err := buildSchema(testCase.newCLI(), "app", spec.Options{})

			assert.True(t, spec.IsSchemaError(err))
			assert.Equal(t, testCase.expected, err.Error())
		})
	}
}

func TestBuild_EnumWithMalformedFilePredictorReportsSyntax(t *testing.T) {
	t.Parallel()

	type CLI struct {
		Env mockFlag[string] `enum:"dev,prod" predictor:"file:"`
	}

	_, err := buildSchema(&CLI{}, "app", spec.Options{})

	assert.True(t, spec.IsSchemaError(err))
	assert.Equal(t, `struct CLI field Env: invalid predictor syntax "file:": file filter pattern cannot be empty (note: expected file:*.ext)`, err.Error())
}

func TestBuild_EnumWithNonPathPredictorBuilds(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		newCLI func() any
	}{
		{
			name: "none",
			newCLI: func() any {
				type CLI struct {
					Env mockFlag[string] `enum:"dev,prod" predictor:"none"`
				}

				return &CLI{}
			},
		},
		{
			name: "off",
			newCLI: func() any {
				type CLI struct {
					Env mockFlag[string] `enum:"dev,prod" predictor:"off"`
				}

				return &CLI{}
			},
		},
		{
			name: "registered",
			newCLI: func() any {
				type CLI struct {
					Env mockFlag[string] `enum:"dev,prod" predictor:"envs"`
				}

				return &CLI{}
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cmdSpec, err := buildSchema(testCase.newCLI(), "app", spec.Options{
				RegisteredPredictors: map[string]predict.Predictor{"envs": predict.Values("dev", "prod")},
			})

			assert.NoError(t, err)
			assert.Equal(t, []string{"dev", "prod"}, cmdSpec.Flags[0].Enum)
		})
	}
}
