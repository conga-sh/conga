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

func TestReload_NilContext(t *testing.T) {
	t.Parallel()

	var nilCtx context.Context

	assert.Panics(t, func() {
		_, _ = (*conga.Session)(nil).Reload[contextTestCLI](nilCtx)
	})
}

func TestReload_UninitializedSession(t *testing.T) {
	t.Parallel()

	result, err := (*conga.Session)(nil).Reload[contextTestCLI](t.Context())

	assert.Error(t, err)
	assert.Equal(t, "conga: session not initialized with reloader", err.Error())
	assert.Equal(t, (*contextTestCLI)(nil), result)
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

	cmd := &conga.CompletionCommand{Shell: "bash"}

	assert.Panics(t, func() {
		_ = cmd.Run(nilCtx, nil)
	})
}

func TestCompletionCommand_NilSession(t *testing.T) {
	t.Parallel()

	cmd := &conga.CompletionCommand{Shell: "bash"}

	err := cmd.Run(t.Context(), nil)

	assert.Error(t, err)
	assert.Equal(t, "conga: session is nil or uninitialized", err.Error())
}

func TestSessionGetters_NilSession(t *testing.T) {
	t.Parallel()

	var nilSession *conga.Session

	t.Run("CandidateConfigFiles", func(t *testing.T) {
		t.Parallel()

		candidates := nilSession.CandidateConfigFiles()

		assert.Equal(t, []string(nil), candidates)
	})

	t.Run("LoadedConfigFiles", func(t *testing.T) {
		t.Parallel()

		loaded := nilSession.LoadedConfigFiles()

		assert.Equal(t, []string(nil), loaded)
	})

	t.Run("Root", func(t *testing.T) {
		t.Parallel()

		root := nilSession.Root[contextTestCLI]()

		assert.Equal(t, (*contextTestCLI)(nil), root)
	})

	t.Run("Path", func(t *testing.T) {
		t.Parallel()

		path := nilSession.Path()

		assert.Equal(t, "", path)
	})

	t.Run("Name", func(t *testing.T) {
		t.Parallel()

		name := nilSession.Name()

		assert.Equal(t, "", name)
	})

	t.Run("Stdout", func(t *testing.T) {
		t.Parallel()

		stdout := nilSession.Stdout()

		assert.Equal(t, io.Discard, stdout)
	})

	t.Run("Stderr", func(t *testing.T) {
		t.Parallel()

		stderr := nilSession.Stderr()

		assert.Equal(t, io.Discard, stderr)
	})

	t.Run("Stdin", func(t *testing.T) {
		t.Parallel()

		stdin := nilSession.Stdin()

		buffer := make([]byte, 8)

		read, err := stdin.Read(buffer)

		assert.Zero(t, read)
		assert.Equal(t, io.EOF, err)
	})

	t.Run("Args", func(t *testing.T) {
		t.Parallel()

		args := nilSession.Args()

		assert.Equal(t, []string(nil), args)
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
