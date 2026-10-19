// Package bootstrapfeature covers a bootstrap field that is also
// feature-gated: the loader runs before feature gates resolve, so the
// combination is rejected.
package bootstrapfeature

import (
	"context"

	"github.com/conga-sh/conga"
)

type BootstrapFeature struct {
	Dir conga.OptionalFlag[string] `bootstrap:"true" feature:"strict"` // want `tag "bootstrap" cannot be combined with tag "feature"`
}

func (c *BootstrapFeature) Run(ctx context.Context, session *conga.Session) error { return nil }

func loadConfig(ctx conga.ConfigContext, root *BootstrapFeature) (*conga.ConfigDocument, error) {
	return nil, nil
}

var _ = conga.New[BootstrapFeature]().
	EnableFeatures().
	ConfigOptions(conga.ConfigLoader(loadConfig))
