// Package completionargeak pins the escape classification for a builder call
// whose arguments mention the app variable: the app value leaks into the
// argument, so the completion guard stays silent instead of treating the call
// as a pure builder run.
package completionargeak

import (
	"context"

	"github.com/conga-sh/conga"
)

// CLI declares a completion command.
type CLI struct {
	Completion conga.CompletionCommand `summary:"Generate shell completion scripts"`
}

// Run executes the CLI.
func (c *CLI) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

// label reads the app value, hiding the builder calls it may perform.
func label(app *conga.App[CLI]) string {
	return "labelled"
}

func run() {
	app := conga.New[CLI]()
	app.Name(label(app))
	app.Run()
}
