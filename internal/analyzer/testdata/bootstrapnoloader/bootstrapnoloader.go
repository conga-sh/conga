// Package bootstrapnoloader covers a bootstrap tag without a configuration
// loader: the tag is useless and schema build rejects it.
package bootstrapnoloader

import (
	"context"

	"github.com/conga-sh/conga"
)

type BootstrapNoLoader struct {
	Dir conga.OptionalFlag[string] `bootstrap:"true"` // want `tag "bootstrap" requires a configuration loader`
}

func (c *BootstrapNoLoader) Run(ctx context.Context, session *conga.Session) error { return nil }

var _ = conga.New[BootstrapNoLoader]()
