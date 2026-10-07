// Package cascadingnoflag covers the static cascading⟹multi guard for a root
// without a config-file field: apps cascade without a root config-file flag,
// so the guard does not apply.
package cascadingnoflag

import (
	"context"

	"github.com/conga-sh/conga"
)

// CascadingWithoutFlag cascades without a root config-file field.
type CascadingWithoutFlag struct {
	Port conga.OptionalFlag[int] `name:"port" default:"8080"`
}

func (c *CascadingWithoutFlag) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[CascadingWithoutFlag]().ConfigOptions(conga.ConfigCascading())
