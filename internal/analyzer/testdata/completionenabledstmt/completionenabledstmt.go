// Package completionenabledstmt pins the statement form of the completion
// guard's clean path: EnableCompletion() is called on the app variable bound by
// the construction assignment.
package completionenabledstmt

import (
	"context"

	"github.com/conga-sh/conga"
)

// CLI declares a completion command and enables completion.
type CLI struct {
	Completion conga.CompletionCommand `summary:"Generate shell completion scripts"`
}

// Run executes the CLI.
func (c *CLI) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

func run() {
	app := conga.New[CLI]()
	app.EnableCompletion()
	app.Run()
}
