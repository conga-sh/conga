// Package bootstrapnobootstrapper covers a bootstrap tag without a Bootstrapper:
// the tag is useless and schema build rejects it.
package bootstrapnobootstrapper

import (
	"context"

	"github.com/conga-sh/conga"
)

type BootstrapNoBootstrapper struct {
	Dir conga.OptionalFlag[string] `bootstrap:"true"` // want `tag "bootstrap" requires the command or an ancestor command to implement conga.Bootstrapper`
}

func (c *BootstrapNoBootstrapper) Run(ctx context.Context, session *conga.Session) error { return nil }

var _ = conga.New[BootstrapNoBootstrapper]()
