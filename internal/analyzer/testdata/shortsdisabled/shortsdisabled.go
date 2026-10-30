// Package shortsdisabled pins that a disabled built-in leaves its rune free for
// application flags: app.HelpShortFlag('h') combined with
// app.DisableHelpFlag() must not report a collision with a user-declared
// short:"h" flag.
package shortsdisabled

import (
	"context"

	"github.com/conga-sh/conga"
)

// App is a leaf command implementing conga.Runner.
type App struct {
	Host conga.OptionalFlag[string] `name:"host" short:"h" help:"Host"`
}

// Run executes the disabled-builtin fixture.
func (a *App) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[App]().DisableHelpFlag().HelpShortFlag('h')
