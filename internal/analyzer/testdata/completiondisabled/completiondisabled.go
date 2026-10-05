// Package completiondisabled pins the lint mirror of the runtime completion
// opt-in guard: a struct declaring conga.CompletionCommand under an app that
// never calls EnableCompletion() is rejected at schema build.
package completiondisabled

import (
	"context"

	"github.com/conga-sh/conga"
)

// CLI declares a completion command without enabling completion.
type CLI struct {
	Completion conga.CompletionCommand `summary:"Generate shell completion scripts"` // want `completion command declared without completion enabled; call app\.EnableCompletion\(\) when constructing the app, or remove the field`
}

// Run executes the CLI.
func (c *CLI) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

func run() {
	app := conga.New[CLI]()
	app.Run()
}
