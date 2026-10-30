// Package shortsambiguous pins the analyzer's degradation when the rune
// assigned to a built-in short is not a constant: collision checks are skipped
// rather than inventing a diagnostic.
package shortsambiguous

import (
	"context"

	"github.com/conga-sh/conga"
)

// helpShort is a variable, so app.HelpShortFlag leaves the analyzer without
// evidence about the effective rune.
var helpShort = 'h'

// App is a leaf command implementing conga.Runner.
type App struct {
	Host conga.OptionalFlag[string] `name:"host" short:"h" help:"Host"`
}

// Run executes the ambiguous-rune fixture.
func (a *App) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[App]().HelpShortFlag(helpShort)
