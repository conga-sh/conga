package conga_test

import (
	"context"
	"io"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga"
)

type contextTestCLI struct {
	Port conga.OptionalFlag[int] `default:"8080"`
}

func TestHelpCommand_NilContext(t *testing.T) {
	t.Parallel()

	var nilCtx context.Context

	cmd := &conga.HelpCommand{}

	assert.Panics(t, func() {
		_ = cmd.Run(nilCtx, nil)
	})
}

func TestVersionCommand_NilContext(t *testing.T) {
	t.Parallel()

	var nilCtx context.Context

	cmd := &conga.VersionCommand{}

	assert.Panics(t, func() {
		_ = cmd.Run(nilCtx, nil)
	})
}

func TestHelpCommand_NilSession(t *testing.T) {
	t.Parallel()

	cmd := &conga.HelpCommand{}

	err := cmd.Run(t.Context(), nil)

	assert.NoError(t, err)
}

func TestCompletionCommand_NilContext(t *testing.T) {
	t.Parallel()

	var nilCtx context.Context

	cmd := &conga.CompletionCommand{}

	assert.Panics(t, func() {
		_ = cmd.Run(nilCtx, nil)
	})
}

func TestCompletionCommand_NilSession(t *testing.T) {
	t.Parallel()

	cmd := &conga.CompletionCommand{}

	err := cmd.Run(t.Context(), nil)

	assert.Error(t, err)
	assert.Equal(t, "conga: session is nil or uninitialized", err.Error())
}

func TestSessionGetters_NilSession(t *testing.T) {
	t.Parallel()

	var nilSession *conga.Session

	t.Run("CandidateConfigFiles", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, []string(nil), nilSession.CandidateConfigFiles())
	})

	t.Run("LoadedConfigFiles", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, []string(nil), nilSession.LoadedConfigFiles())
	})

	t.Run("Root", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, (*contextTestCLI)(nil), nilSession.Root[contextTestCLI]())
	})

	t.Run("Path", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "", nilSession.Path())
	})

	t.Run("Name", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "", nilSession.Name())
	})

	t.Run("Stdout", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, io.Discard, nilSession.Stdout())
	})

	t.Run("Stderr", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, io.Discard, nilSession.Stderr())
	})

	t.Run("Stdin", func(t *testing.T) {
		t.Parallel()

		read, err := nilSession.Stdin().Read(make([]byte, 8))

		assert.Zero(t, read)
		assert.Equal(t, io.EOF, err)
	})

	t.Run("Help", func(t *testing.T) {
		t.Parallel()

		nilSession.Help()
	})

	t.Run("Usage", func(t *testing.T) {
		t.Parallel()

		nilSession.Usage()
	})

	t.Run("Version", func(t *testing.T) {
		t.Parallel()

		nilSession.Version()
	})
}
