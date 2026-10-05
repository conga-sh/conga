// Package completionenabled pins the clean path of the completion guard: the
// anchored app enables completion in its construction chain, so the declared
// completion command is accepted.
package completionenabled

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
	app := conga.New[CLI]().EnableCompletion()
	app.Run()
}
