// Package completionescape pins the conservative degradation when the app
// value escapes the construction run: the analyzer cannot attribute builder
// calls to the anchor, so the completion guard stays silent.
package completionescape

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

// configure receives the app value, hiding the builder calls it performs.
func configure(app *conga.App[CLI]) {
	app.EnableCompletion()
}

func run() {
	app := conga.New[CLI]()
	configure(app)
	app.Run()
}
