// Package bootstrapclean covers bootstrap tags on a loader-enabled app:
// root and subcommand bootstrap fields are accepted.
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

func (c *BootstrapClean) Run(ctx context.Context, session *conga.Session) error { return nil }

func loadConfig(ctx conga.ConfigContext, root *BootstrapClean) (*conga.ConfigDocument, error) {
	return nil, nil
}

var _ = conga.New[BootstrapClean]().ConfigOptions(conga.ConfigLoader(loadConfig))
