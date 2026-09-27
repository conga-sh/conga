// Package cascadinginnocent pins the conservative degradation of the
// cascading⟹multi guard for multi-anchor packages: package-wide Cascading()
// evidence from one anchor must not produce a diagnostic on an innocent
// sibling root. The analyzer skips the guard entirely for packages anchoring
// more than one conga.New root — it may only miss diagnostics, never invent
// them.
package cascadinginnocent

import (
	"context"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/config"
)

// InnocentPlainFlag declares a single-value config-file flag without
// Cascading(): no diagnostic may ever fire on this root, regardless of sibling
// builder options in the same package.
type InnocentPlainFlag struct {
	Config conga.ConfigFile `name:"config"`
}

func (c *InnocentPlainFlag) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

// CascadingSibling enables Cascading() without declaring a config-file field;
// the runtime guard does not apply to it.
type CascadingSibling struct {
	Port conga.OptionalFlag[int] `name:"port" default:"8080"`
}

func (c *CascadingSibling) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[InnocentPlainFlag]()

var _ = conga.New[CascadingSibling]().ConfigOptions(config.Cascading())
