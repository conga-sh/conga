// Package bootstraprecursive covers self-referential bootstrap container
// types: schema build and lint must reject them instead of recursing forever.
package bootstraprecursive

import (
	"context"

	"github.com/conga-sh/conga"
)

type RecursiveSlice []RecursiveSlice

type BootstrapRecursive struct {
	Input conga.OptionalFlag[RecursiveSlice] `bootstrap:"true"` // want `bootstrap fields must be scalars`
}

func (c *BootstrapRecursive) Bootstrap(ctx context.Context, b *conga.Bootstrap) error { return nil }

func (c *BootstrapRecursive) Run(ctx context.Context, session *conga.Session) error { return nil }

var _ = conga.New[BootstrapRecursive]()
