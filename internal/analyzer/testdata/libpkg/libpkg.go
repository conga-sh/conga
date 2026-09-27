// Package libpkg declares command structs for the anchorpkg fixture, which
// instantiates a CLI whose hierarchy lives in another package — the layout
// real consumers use when the conga.New anchor sits in a main package.
package libpkg

import (
	"context"

	"github.com/conga-sh/conga"
)

// GoodLeaf is a leaf command that satisfies conga.Runner.
type GoodLeaf struct{}

// Run executes the good leaf command.
func (g *GoodLeaf) Run(ctx context.Context, session *conga.Session) error { return nil }

// BadLeaf is a leaf command without a Run method; the diagnostic surfaces at
// the referencing conga.New anchor in the anchoring package.
type BadLeaf struct{}

// Root groups the leaf commands; grouping commands do not need a Runner.
type Root struct {
	Good conga.Command[GoodLeaf] `summary:"good leaf"`
	Bad  conga.Command[BadLeaf]  `summary:"bad leaf"`
}
