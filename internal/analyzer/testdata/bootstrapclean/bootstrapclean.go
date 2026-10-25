// Package bootstrapclean covers bootstrap tags on an app whose command
// implements conga.Bootstrapper: root and subcommand bootstrap fields are
// accepted.
package bootstrapclean

import (
	"context"

	"github.com/conga-sh/conga"
)

type Deploy struct {
	Profile conga.OptionalFlag[string] `bootstrap:"true"`
}

func (c *Deploy) Run(ctx context.Context, session *conga.Session) error { return nil }

type BootstrapClean struct {
	Dir    conga.OptionalFlag[string] `bootstrap:"true"`
	Deploy conga.Command[Deploy]
}

func (c *BootstrapClean) Bootstrap(ctx context.Context, b *conga.Bootstrap) error { return nil }

func (c *BootstrapClean) Run(ctx context.Context, session *conga.Session) error { return nil }

var _ = conga.New[BootstrapClean]()
