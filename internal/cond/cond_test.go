package cond_test

import (
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/cond"
)

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

func TestIf(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "true-val", cond.If(true, "true-val", "false-val"))
	assert.Equal(t, "false-val", cond.If(false, "true-val", "false-val"))
	assert.Equal(t, 10, cond.If(true, 10, 20))
	assert.Equal(t, 20, cond.If(false, 10, 20))
}
