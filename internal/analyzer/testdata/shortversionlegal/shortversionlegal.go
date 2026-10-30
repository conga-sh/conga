// Package shortversionlegal pins that the version rune stays available on
// subcommands: the built-in --version flag is root-only, so a leaf short that
// matches app.VersionShortFlag is legal.
package shortversionlegal

import (
	"context"

	"github.com/conga-sh/conga"
)

// Leaf declares the version rune on a subcommand flag.
type Leaf struct {
	Verbose conga.OptionalFlag[bool] `name:"verbose" short:"V" help:"Verbose output"`
}

// Run executes the leaf.
func (l *Leaf) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

// Root hosts the subcommand and assigns the version rune to the root's
// built-in --version flag.
type Root struct {
	Sub conga.Command[Leaf] `name:"sub" summary:"Sub"`
}

// Run executes the root.
func (r *Root) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[Root]().VersionShortFlag('V')
