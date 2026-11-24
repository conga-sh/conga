package conga_test

import (
	"context"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/internal/errlines"
	"github.com/conga-sh/conga/network"
	"github.com/conga-sh/conga/predict"
)

type enumFileCLI struct {
	Env conga.OptionalFlag[conga.File] `enum:"dev.yaml,prod.yaml"`
}

func (c *enumFileCLI) Run(context.Context, *conga.Session) error { return nil }

type enumDirArgCLI struct {
	Env conga.RequiredArg[conga.Dir] `enum:"dev,prod"`
}

func (c *enumDirArgCLI) Run(context.Context, *conga.Session) error { return nil }

type enumFileContentCLI struct {
	Env conga.OptionalFlag[conga.FileContent] `enum:"dev.yaml,prod.yaml"`
}

func (c *enumFileContentCLI) Run(context.Context, *conga.Session) error { return nil }

type enumFileSliceCLI struct {
	Env conga.OptionalFlag[[]conga.File] `enum:"dev.yaml,prod.yaml"`
}

func (c *enumFileSliceCLI) Run(context.Context, *conga.Session) error { return nil }

type enumFilePointerCLI struct {
	Env conga.OptionalFlag[*conga.File] `enum:"dev.yaml,prod.yaml"`
}

func (c *enumFilePointerCLI) Run(context.Context, *conga.Session) error { return nil }

type enumFilePointerSliceCLI struct {
	Env conga.OptionalFlag[[]*conga.File] `enum:"dev.yaml,prod.yaml"`
}

func (c *enumFilePointerSliceCLI) Run(context.Context, *conga.Session) error { return nil }

type enumFileNoPredictorCLI struct {
	Env conga.OptionalFlag[conga.File] `enum:"dev.yaml,prod.yaml" predictor:"none"`
}

func (c *enumFileNoPredictorCLI) Run(context.Context, *conga.Session) error { return nil }

type enumFileExecutableCLI struct {
	Tool conga.OptionalFlag[conga.File] `enum:"make,go" executable:"true"`
}

func (c *enumFileExecutableCLI) Run(context.Context, *conga.Session) error { return nil }

type enumFileExecutableFalseCLI struct {
	Tool conga.OptionalFlag[conga.File] `enum:"make,go" executable:"false"`
}

func (c *enumFileExecutableFalseCLI) Run(context.Context, *conga.Session) error { return nil }

type enumFileSymlinkCLI struct {
	Link conga.OptionalFlag[conga.File] `enum:"current,previous" symlink:"true"`
}

func (c *enumFileSymlinkCLI) Run(context.Context, *conga.Session) error { return nil }

type globFileCLI struct {
	Env conga.OptionalFlag[conga.File] `glob:"*.yaml"`
}

func (c *globFileCLI) Run(context.Context, *conga.Session) error { return nil }

type pointerFileExistingCLI struct {
	Manifest conga.OptionalFlag[*conga.File] `existing:"true"`
}

func (c *pointerFileExistingCLI) Run(context.Context, *conga.Session) error { return nil }

type enumRegisteredPredictorCLI struct {
	Env conga.OptionalFlag[string] `enum:"dev,prod" predictor:"envs"`
}

func (c *enumRegisteredPredictorCLI) Run(context.Context, *conga.Session) error { return nil }

type enumHTTPMethodCLI struct {
	Method conga.OptionalFlag[network.HTTPMethod] `enum:"GET,POST"`
}

func (c *enumHTTPMethodCLI) Run(context.Context, *conga.Session) error { return nil }

func TestSchema_EnumOnPathTypeRejected(t *testing.T) {
	t.Parallel()

	const (
		fileHint   = `use tag "glob" to filter completion candidates, or a string field with tag "enum" to accept a fixed set of names`
		stringHint = `use a string field with tag "enum" to accept a fixed set of names`
	)

	testCases := []struct {
		name     string
		run      func() error
		expected string
		note     string
		hint     string
	}{
		{
			name:     "File flag",
			run:      func() error { return conga.New[enumFileCLI]().Args(nil).Run() },
			expected: `struct enumFileCLI field Env: tag "enum" is not supported on conga.File`,
			hint:     fileHint,
		},
		{
			name:     "Dir argument",
			run:      func() error { return conga.New[enumDirArgCLI]().Args(nil).Run() },
			expected: `struct enumDirArgCLI field Env: tag "enum" is not supported on conga.Dir`,
			hint:     stringHint,
		},
		{
			name:     "FileContent flag",
			run:      func() error { return conga.New[enumFileContentCLI]().Args(nil).Run() },
			expected: `struct enumFileContentCLI field Env: tag "enum" is not supported on conga.FileContent`,
			hint:     stringHint,
		},
		{
			name:     "File slice flag",
			run:      func() error { return conga.New[enumFileSliceCLI]().Args(nil).Run() },
			expected: `struct enumFileSliceCLI field Env: tag "enum" is not supported on conga.File`,
			hint:     fileHint,
		},
		{
			name:     "File pointer flag",
			run:      func() error { return conga.New[enumFilePointerCLI]().Args(nil).Run() },
			expected: `struct enumFilePointerCLI field Env: tag "enum" is not supported on conga.File`,
			hint:     fileHint,
		},
		{
			name:     "File pointer slice flag",
			run:      func() error { return conga.New[enumFilePointerSliceCLI]().Args(nil).Run() },
			expected: `struct enumFilePointerSliceCLI field Env: tag "enum" is not supported on conga.File`,
			hint:     fileHint,
		},
		{
			name:     "File with predictor none",
			run:      func() error { return conga.New[enumFileNoPredictorCLI]().Args(nil).Run() },
			expected: `struct enumFileNoPredictorCLI field Env: tag "enum" is not supported on conga.File`,
			hint:     fileHint,
		},
		{
			name:     "File with executable",
			run:      func() error { return conga.New[enumFileExecutableCLI]().Args(nil).Run() },
			expected: `struct enumFileExecutableCLI field Tool: tag "enum" is not supported on conga.File`,
			hint:     stringHint,
		},
		{
			name:     "File with executable false",
			run:      func() error { return conga.New[enumFileExecutableFalseCLI]().Args(nil).Run() },
			expected: `struct enumFileExecutableFalseCLI field Tool: tag "enum" is not supported on conga.File`,
			hint:     stringHint,
		},
		{
			name:     "File with symlink",
			run:      func() error { return conga.New[enumFileSymlinkCLI]().Args(nil).Run() },
			expected: `struct enumFileSymlinkCLI field Link: tag "enum" is not supported on conga.File`,
			hint:     stringHint,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			err := testCase.run()

			assert.Error(t, err)
			assert.Equal(t, testCase.expected, err.Error())
			assert.Equal(t, testCase.note, errlines.Note(err))
			assert.Equal(t, testCase.hint, errlines.Hint(err))
		})
	}
}

func TestSchema_EnumNextToPathRulesBuilds(t *testing.T) {
	t.Parallel()

	t.Run("File with glob and no enum", func(t *testing.T) {
		t.Parallel()

		err := conga.New[globFileCLI]().Args(nil).Run()

		assert.NoError(t, err)
	})

	t.Run("File pointer with a domain tag", func(t *testing.T) {
		t.Parallel()

		err := conga.New[pointerFileExistingCLI]().Args(nil).Run()

		assert.NoError(t, err)
	})

	t.Run("string with enum and a registered predictor", func(t *testing.T) {
		t.Parallel()

		err := conga.New[enumRegisteredPredictorCLI]().
			Predictor("envs", predict.Values("dev", "prod")).
			Args(nil).
			Run()

		assert.NoError(t, err)
	})

	t.Run("predictable domain type with enum", func(t *testing.T) {
		t.Parallel()

		cli := new(enumHTTPMethodCLI)

		err := conga.New(cli).Args([]string{"--method", "POST"}).Run()

		assert.NoError(t, err)
		assert.Equal(t, network.HTTPMethod("POST"), cli.Method.Get())
	})
}
