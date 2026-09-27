package conga

import (
	"context"
	"testing"

	"github.com/alecthomas/assert/v2"
)

type reloadInternalTargetCLI struct {
	Port OptionalFlag[int] `default:"8080"`
}

type reloadInternalOtherCLI struct {
	Host OptionalFlag[string] `default:"localhost"`
}

func TestReload_NilSession(t *testing.T) {
	t.Parallel()

	result, err := (*Session)(nil).Reload[reloadInternalTargetCLI](t.Context())

	assert.Error(t, err)
	assert.Equal(t, "conga: session not initialized with reloader", err.Error())
	assert.Equal(t, (*reloadInternalTargetCLI)(nil), result)
}

func TestReload_NilReloadFunc(t *testing.T) {
	t.Parallel()

	session := &Session{}

	result, err := session.Reload[reloadInternalTargetCLI](t.Context())

	assert.Error(t, err)
	assert.Equal(t, "conga: session not initialized with reloader", err.Error())
	assert.Equal(t, (*reloadInternalTargetCLI)(nil), result)
}

func TestReload_TypeMismatch(t *testing.T) {
	t.Parallel()

	session := &Session{
		reloadFunc: func(ctx context.Context) (any, error) {
			return &reloadInternalOtherCLI{}, nil
		},
	}

	result, err := session.Reload[reloadInternalTargetCLI](t.Context())

	assert.Error(t, err)
	assert.Equal(t, "conga: reload result type mismatch: type *conga.reloadInternalOtherCLI does not match requested type *conga.reloadInternalTargetCLI", err.Error())
	assert.Equal(t, (*reloadInternalTargetCLI)(nil), result)
}
