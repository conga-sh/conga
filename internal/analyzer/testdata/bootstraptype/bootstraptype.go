// Package bootstraptype covers the bootstrap type restriction: struct-valued
// fields are rejected because bootstrap coercion must stay cheap and free of
// I/O.
package bootstraptype

import (
	"context"

	"github.com/conga-sh/conga"
)

type BootstrapType struct {
	Input conga.OptionalFlag[conga.File] `bootstrap:"true"` // want `bootstrap fields must be scalars`
}

func (c *BootstrapType) Bootstrap(ctx context.Context, b *conga.Bootstrap) error { return nil }

func (c *BootstrapType) Run(ctx context.Context, session *conga.Session) error { return nil }

var _ = conga.New[BootstrapType]()
