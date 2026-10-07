// Package cascadingmasked pins the conservative degradation of the
// cascading⟹multi guard for multi-anchor packages: builder-option evidence is
// aggregated package-wide, so a multi:"true" root walked first cannot mask a
// later violating root either. The analyzer skips the guard entirely for
// packages anchoring more than one conga.New root — it may only miss
// diagnostics, never invent them — and the runtime schema guard stays the
// per-root source of truth.
package cascadingmasked

import (
	"context"

	"github.com/conga-sh/conga"
)

// CascadingMultiRoot satisfies the guard with the multi tag.
type CascadingMultiRoot struct {
	Config conga.ConfigFile `multi:"true" name:"config"`
}

func (c *CascadingMultiRoot) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

// CascadingViolatingRoot declares a single-value config-file flag with
// Cascading(): the runtime schema rejects the combination, but the analyzer
// cannot attribute the package-wide Cascading evidence to this root and stays
// silent.
type CascadingViolatingRoot struct {
	Config conga.ConfigFile `name:"config"`
}

func (c *CascadingViolatingRoot) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[CascadingMultiRoot]().ConfigOptions(conga.ConfigCascading())

var _ = conga.New[CascadingViolatingRoot]().ConfigOptions(conga.ConfigCascading())
