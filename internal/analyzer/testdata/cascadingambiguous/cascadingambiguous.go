// Package cascadingambiguous covers the conservative degradation of the
// cascading⟹multi guard: an unclassifiable app.ConfigOptions argument could hide a
// Cascading option, so the analyzer must stay silent instead of inventing a
// diagnostic.
package cascadingambiguous

import (
	"context"

	"github.com/conga-sh/conga"
)

// NonLiteralOption feeds an unclassifiable ConfigOptions argument.
type NonLiteralOption struct {
	Config conga.ConfigFile `name:"config"`
}

func (c *NonLiteralOption) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

func cascadingOption() conga.ConfigOption {
	return conga.ConfigCascading()
}

var _ = conga.New[NonLiteralOption]().ConfigOptions(cascadingOption())
