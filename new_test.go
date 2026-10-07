package conga_test

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/internal/assertx"
)

type newRootCLI struct {
	Verbose conga.OptionalFlag[bool] `help:"Verbose mode"`
}

func (c *newRootCLI) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

type rootIdentityCLI struct {
	rootMatches bool
}

func (c *rootIdentityCLI) Run(ctx context.Context, session *conga.Session) error {
	c.rootMatches = session.Root[rootIdentityCLI]() == c

	return nil
}

type engineRootCLI struct{}

func (c *engineRootCLI) Run(ctx context.Context, session *conga.Session) error {
	if root := session.Root[engineRootCLI](); root != c {
		return fmt.Errorf("Root returned %p, want receiver %p", root, c)
	}

	return nil
}

func TestNew_BindsCallerInstance(t *testing.T) {
	t.Parallel()

	cli := new(rootIdentityCLI)

	app := conga.New(cli).Args([]string{})

	err := app.Run()

	assert.NoError(t, err)
	assert.True(t, cli.rootMatches)
}

func TestNew_BindsCallerValues(t *testing.T) {
	t.Parallel()

	cli := new(newRootCLI)

	app := conga.New(cli).Args([]string{"--verbose"})

	err := app.Run()

	assert.NoError(t, err)
	assert.True(t, cli.Verbose.Get())
	assert.Equal(t, conga.SourceCLI, cli.Verbose.Source())
}

func TestNew_EngineAllocatesWhenUnset(t *testing.T) {
	t.Parallel()

	app := conga.New[engineRootCLI]().Args([]string{})

	err := app.Run()

	assert.NoError(t, err)
}

func TestNew_ArgsMutableAfterCompletionInvocation(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	cli := new(newRootCLI)

	app := conga.New(cli).EnableCompletion().Args([]string{"__complete", "bash"}).Stdout(&buffer)

	err := app.Run()

	assert.NoError(t, err)

	app.Args([]string{"--verbose"})

	err = app.Run()

	assert.NoError(t, err)
	assert.True(t, cli.Verbose.Get())
}

func TestNew_NilRootPanics(t *testing.T) {
	t.Parallel()

	assertx.PanicsWithValue(t, "conga: New requires a non-nil root instance", func() {
		_ = conga.New[newRootCLI](nil)
	})
}

func TestNew_MultipleRootsPanic(t *testing.T) {
	t.Parallel()

	assertx.PanicsWithValue(t, "conga: New accepts at most one root instance", func() {
		_ = conga.New(new(newRootCLI), new(newRootCLI))
	})
}
