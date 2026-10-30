// Package shortcollisions pins the analyzer's mirror of the runtime built-in
// short guards: a user-declared rune may not claim the rune assigned to the
// built-in --help or --version flag. The checks require a single conga.New
// anchor, and both runes are distinct here, so the package-wide option scan
// sees unambiguous evidence and reports no pair collision.
package shortcollisions

import (
	"context"

	"github.com/conga-sh/conga"
)

// ShortCollisionApp declares shorts that claim both built-in runes.
type ShortCollisionApp struct {
	Host    conga.OptionalFlag[string] `name:"host"    short:"h" help:"Host"`           // want `built-in --help short flag -h collides with --host`
	Verbose conga.OptionalFlag[bool]   `name:"verbose" short:"V" help:"Verbose output"` // want `built-in --version short flag -V collides with --verbose`
}

// Run executes the fixture.
func (s *ShortCollisionApp) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[ShortCollisionApp]().HelpShortFlag('h').VersionShortFlag('V')
