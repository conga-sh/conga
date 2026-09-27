// Package cascadingclean covers the static cascading⟹multi guard for a
// satisfying root: the multi:"true" tag lets a single-anchor root config-file
// flag report every merged file, so Cascading() is accepted without a
// diagnostic.
package cascadingclean

import (
	"context"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/config"
)

// CascadingMultiFlag satisfies the guard: the multi tag lets the flag report
// every merged file.
type CascadingMultiFlag struct {
	Config conga.ConfigFile `multi:"true" name:"config"`
}

func (c *CascadingMultiFlag) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[CascadingMultiFlag]().ConfigOptions(config.Cascading())
