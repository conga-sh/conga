package cond_test

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/cond"
)

func TestSetOrDefault(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "hello", cond.SetOrDefault("hello", "fallback"))
	assert.Equal(t, "fallback", cond.SetOrDefault("", "fallback"))
	assert.Equal(t, 42, cond.SetOrDefault(42, 100))
	assert.Equal(t, 100, cond.SetOrDefault(0, 100))

	var (
		writer io.Writer
		buffer bytes.Buffer
	)

	assert.Equal(t, io.Writer(&buffer), cond.SetOrDefault(writer, io.Writer(&buffer)))
	writer = os.Stdout
	assert.Equal(t, io.Writer(os.Stdout), cond.SetOrDefault(writer, io.Writer(&buffer)))
}

func TestSetOrDefaultFunc(t *testing.T) {
	t.Parallel()

	called := false
	lazyFn := func() string {
		called = true

		return "computed"
	}

	assert.Equal(t, "already-set", cond.SetOrDefaultFunc("already-set", lazyFn))
	assert.False(t, called)

	assert.Equal(t, "computed", cond.SetOrDefaultFunc("", lazyFn))
	assert.True(t, called)
}

func TestFirstSet(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "first", cond.FirstSet("", "first", "second"))
	assert.Equal(t, "a", cond.FirstSet("a", "b", "c"))
	assert.Equal(t, "", cond.FirstSet("", "", ""))
	assert.Equal(t, 5, cond.FirstSet(0, 0, 5, 10))
	assert.Equal(t, 0, cond.FirstSet[int]())
}

func TestIf(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "true-val", cond.If(true, "true-val", "false-val"))
	assert.Equal(t, "false-val", cond.If(false, "true-val", "false-val"))
	assert.Equal(t, 10, cond.If(true, 10, 20))
	assert.Equal(t, 20, cond.If(false, 10, 20))
}
