// Package shortpaircollision pins the analyzer's mirror of the runtime guard
// against assigning the same rune to both built-in flags: the binder registers
// --help first, so the --version registration would be dropped while both help
// rows still advertise the rune.
package shortpaircollision

import (
	"context"

	"github.com/conga-sh/conga"
)

// App is a leaf command implementing conga.Runner.
type App struct { // want `built-in --help and --version both claim short flag -h`
	Verbose conga.OptionalFlag[bool] `name:"verbose" help:"Verbose output"`
}

// Run executes the pair-collision fixture.
func (a *App) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[App]().HelpShortFlag('h').VersionShortFlag('h')
